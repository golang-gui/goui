package x11

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/golang-gui/goui/platform/common"
	"github.com/golang-gui/goui/platform/internal/fileaccess"
	"github.com/golang-gui/goui/platform/linux/internal/portal"
)

// fileDialog implements the native file dialog capability for X11.
type fileDialog struct {
	helper dialogHelper
}

func newFileDialog() (common.FileDialog, error) {
	helper := detectDialogHelper()
	return &fileDialog{helper: helper}, nil
}

// dialogHelper represents an available dialog helper command.
type dialogHelper int

const (
	dialogNone dialogHelper = iota
	dialogZenity
	dialogKdialog
	dialogYad
)

// detectDialogHelper returns the first available dialog helper based on the
// current desktop environment.
func detectDialogHelper() dialogHelper {
	desktop := os.Getenv("XDG_CURRENT_DESKTOP")
	session := os.Getenv("DESKTOP_SESSION")

	// KDE prefers kdialog
	if strings.Contains(desktop, "KDE") || strings.Contains(session, "plasma") {
		if _, err := exec.LookPath("kdialog"); err == nil {
			return dialogKdialog
		}
	}

	// GNOME, XFCE, Cinnamon, MATE, and most others prefer zenity
	if _, err := exec.LookPath("zenity"); err == nil {
		return dialogZenity
	}

	// yad is a zenity-compatible fallback
	if _, err := exec.LookPath("yad"); err == nil {
		return dialogYad
	}

	// KDE fallback if kdialog wasn't found earlier
	if _, err := exec.LookPath("kdialog"); err == nil {
		return dialogKdialog
	}

	return dialogNone
}

// buildFilterArgs builds command-line filter arguments for the given helper.
func buildFilterArgs(helper dialogHelper, filters []common.FileFilter) []string {
	if len(filters) == 0 {
		return nil
	}

	var args []string
	for _, f := range filters {
		pattern := strings.Join(f.Patterns, " ")
		switch helper {
		case dialogZenity, dialogYad:
			// zenity/yad: --file-filter="Name|*.png *.jpg"
			args = append(args, "--file-filter="+f.Name+"|"+pattern)
		case dialogKdialog:
			// kdialog: --filter "Name (*.png *.jpg)|*.png *.jpg"
			args = append(args, "--filter", f.Name+" ("+pattern+")|"+pattern)
		}
	}
	return args
}

// OpenFile opens a native file-selection dialog for selecting one or more files.
func (fd *fileDialog) OpenFile(owner common.Window, opts common.DialogOptions, cb func([]common.FileAccess, error)) {
	fd.open(owner, portal.OpenFile, opts, cb)
}

// OpenDirectory opens a native dialog for selecting a directory.
func (fd *fileDialog) OpenDirectory(owner common.Window, opts common.DialogOptions, cb func([]common.FileAccess, error)) {
	fd.open(owner, portal.OpenDirectory, opts, cb)
}

// SaveFile opens a native save-file dialog.
func (fd *fileDialog) SaveFile(owner common.Window, opts common.DialogOptions, cb func([]common.FileAccess, error)) {
	fd.open(owner, portal.SaveFile, opts, cb)
}

func (fd *fileDialog) RestoreAccess(credential []byte) (common.FileAccess, error) {
	return fileaccess.RestorePath(credential)
}

var errDialogCancelled = errors.New("dialog cancelled")

// Registry, owner and helper state belong to the platform thread. The mutex only
// serializes worker posting with cancellation before the wake pipe is closed.
type fileDialogCall struct {
	owner     *Window
	cmd       *exec.Cmd
	cancel    context.CancelFunc
	mutex     sync.Mutex // serialize worker posting with GUI-thread cancellation
	done      func([]common.FileAccess, error)
	cancelled bool
}

var nativeFileDialogs = map[*fileDialogCall]struct{}{}

func cancelFileDialogs(owner *Window) {
	for call := range nativeFileDialogs {
		if owner == nil || call.owner == owner {
			call.mutex.Lock()
			call.cancelled = true
			call.mutex.Unlock()
			delete(nativeFileDialogs, call)
			call.cancel()
			if call.cmd != nil {
				_ = call.cmd.Process.Kill()
			}
			call.done(nil, common.ErrUnavailable)
		}
	}
}

func closeFileDialogs() {
	cancelFileDialogs(nil)
	if platform != nil && platform.fileChooser != nil {
		platform.fileChooser.Close()
		platform.fileChooser = nil
	}
}

func (fd *fileDialog) open(owner common.Window, action portal.Action, opts common.DialogOptions, cb func([]common.FileAccess, error)) {
	if platform == nil || platform.eventLoop == nil || platform.eventLoop.state.Quitting() || platform.eventLoop.state.Destroyed() {
		cb(nil, common.ErrUnavailable)
		return
	}
	var w *Window
	if owner != nil {
		var ok bool
		w, ok = owner.(*Window)
		if !ok || w.wid == 0 {
			cb(nil, common.ErrUnavailable)
			return
		}
	}
	parent := ""
	if w != nil {
		parent = fmt.Sprintf("x11:%x", w.wid)
	}
	opts.Filters = slices.Clone(opts.Filters)
	for i := range opts.Filters {
		opts.Filters[i].Patterns = slices.Clone(opts.Filters[i].Patterns)
	}
	if platform.fileChooser == nil {
		platform.fileChooser = portal.NewFileChooser()
	}
	chooser := platform.fileChooser
	ctx, cancel := context.WithCancel(context.Background())
	call := &fileDialogCall{owner: w, cancel: cancel, done: cb}
	nativeFileDialogs[call] = struct{}{}
	loop := platform.eventLoop
	go func() {
		paths, err := chooser.Open(ctx, action, parent, opts)
		call.post(loop, func() {
			if errors.Is(err, portal.ErrUnsupported) {
				fd.startHelper(loop, call, action, opts)
				return
			}
			call.finish(paths, err)
		})
	}()
}

func (call *fileDialogCall) post(loop *EventLoop, task func()) {
	call.mutex.Lock()
	defer call.mutex.Unlock()
	if !call.cancelled {
		loop.Post(func() {
			if !call.cancelled {
				task()
			}
		})
	}
}

func (call *fileDialogCall) finish(paths []string, err error) {
	delete(nativeFileDialogs, call)
	call.cancel()
	if call.owner != nil && call.owner.wid == 0 {
		call.done(nil, common.ErrUnavailable)
		return
	}
	var access []common.FileAccess
	if err == nil && len(paths) > 0 {
		access, err = fileaccess.Paths(paths)
	}
	call.done(access, err)
}

func (fd *fileDialog) startHelper(loop *EventLoop, call *fileDialogCall, action portal.Action, opts common.DialogOptions) {
	if fd.helper == dialogNone {
		call.finish(nil, common.ErrUnsupported)
		return
	}
	var args []string
	switch action {
	case portal.OpenFile:
		args = buildOpenFileArgs(fd.helper, opts)
	case portal.OpenDirectory:
		args = buildOpenDirectoryArgs(fd.helper, opts)
	case portal.SaveFile:
		args = buildSaveFileArgs(fd.helper, opts)
	}
	if call.owner != nil {
		if call.owner.wid == 0 {
			call.finish(nil, common.ErrUnavailable)
			return
		}
		args = append(args, dialogOwnerArgs(fd.helper, uintptr(call.owner.wid))...)
	}
	cmd, err := dialogCommand(fd.helper, args)
	if err != nil {
		call.finish(nil, err)
		return
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		call.finish(nil, fmt.Errorf("dialog failed: %w", err))
		return
	}
	call.cmd = cmd
	go func() {
		waitErr := cmd.Wait()
		call.post(loop, func() {
			output, err := dialogOutput(stdout.Bytes(), stderr.Bytes(), waitErr)
			if errors.Is(err, errDialogCancelled) {
				call.finish(nil, nil)
				return
			}
			if err != nil {
				call.finish(nil, err)
				return
			}
			paths := parseOutput(fd.helper, output, action == portal.OpenFile && opts.AllowMultiple)
			if len(paths) == 0 {
				err = errors.New("file dialog returned no selection")
			}
			call.finish(paths, err)
		})
	}()
}

func dialogOwnerArgs(helper dialogHelper, owner uintptr) []string {
	id := strconv.FormatUint(uint64(owner), 10)
	switch helper {
	case dialogZenity, dialogYad:
		return []string{"--attach=" + id, "--modal"}
	case dialogKdialog:
		return []string{"--attach", id}
	}
	return nil
}

func dialogCommand(helper dialogHelper, args []string) (*exec.Cmd, error) {
	var name string
	switch helper {
	case dialogZenity:
		name = "zenity"
	case dialogKdialog:
		name = "kdialog"
	case dialogYad:
		name = "yad"
	default:
		return nil, errors.New("unknown dialog helper")
	}
	return exec.Command(name, args...), nil
}

func dialogOutput(output, stderr []byte, err error) (string, error) {
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return "", errDialogCancelled
		}
		if len(stderr) > 0 {
			return "", fmt.Errorf("dialog failed: %w: %s", err, strings.TrimSpace(string(stderr)))
		}
		return "", fmt.Errorf("dialog failed: %w", err)
	}
	return strings.TrimSuffix(string(output), "\n"), nil
}

// buildOpenFileArgs builds command-line arguments for file selection.
func buildOpenFileArgs(helper dialogHelper, opts common.DialogOptions) []string {
	switch helper {
	case dialogZenity, dialogYad:
		args := []string{"--file-selection", "--title=" + opts.Title}
		if opts.AllowMultiple {
			args = append(args, "--multiple", "--separator=\n")
		}
		if opts.InitialDir != "" {
			args = append(args, "--filename="+opts.InitialDir+"/")
		}
		args = append(args, buildFilterArgs(helper, opts.Filters)...)
		return args
	case dialogKdialog:
		args := []string{"--getopenfilename"}
		if opts.InitialDir != "" {
			args = append(args, opts.InitialDir+"/")
		} else {
			args = append(args, "/")
		}
		args = append(args, buildFilterArgs(helper, opts.Filters)...)
		if opts.Title != "" {
			args = append(args, "--title", opts.Title)
		}
		if opts.AllowMultiple {
			args = append(args, "--multiple")
		}
		return args
	}
	return nil
}

// buildOpenDirectoryArgs builds command-line arguments for directory selection.
func buildOpenDirectoryArgs(helper dialogHelper, opts common.DialogOptions) []string {
	switch helper {
	case dialogZenity, dialogYad:
		args := []string{"--file-selection", "--directory", "--title=" + opts.Title}
		if opts.InitialDir != "" {
			args = append(args, "--filename="+opts.InitialDir+"/")
		}
		return args
	case dialogKdialog:
		args := []string{"--getexistingdirectory"}
		if opts.InitialDir != "" {
			args = append(args, opts.InitialDir+"/")
		} else {
			args = append(args, "/")
		}
		if opts.Title != "" {
			args = append(args, "--title", opts.Title)
		}
		return args
	}
	return nil
}

// buildSaveFileArgs builds command-line arguments for save dialog.
func buildSaveFileArgs(helper dialogHelper, opts common.DialogOptions) []string {
	switch helper {
	case dialogZenity, dialogYad:
		args := []string{"--file-selection", "--save", "--title=" + opts.Title}
		if opts.InitialDir != "" && opts.DefaultName != "" {
			args = append(args, "--filename="+opts.InitialDir+"/"+opts.DefaultName)
		} else if opts.DefaultName != "" {
			args = append(args, "--filename="+opts.DefaultName)
		} else if opts.InitialDir != "" {
			args = append(args, "--filename="+opts.InitialDir+"/")
		}
		args = append(args, buildFilterArgs(helper, opts.Filters)...)
		return args
	case dialogKdialog:
		args := []string{"--getsavefilename"}
		if opts.InitialDir != "" && opts.DefaultName != "" {
			args = append(args, opts.InitialDir+"/"+opts.DefaultName)
		} else if opts.InitialDir != "" {
			args = append(args, opts.InitialDir+"/")
		} else {
			args = append(args, "/")
		}
		args = append(args, buildFilterArgs(helper, opts.Filters)...)
		if opts.Title != "" {
			args = append(args, "--title", opts.Title)
		}
		return args
	}
	return nil
}

// parseOutput parses the stdout output from a dialog helper.
func parseOutput(helper dialogHelper, output string, multiple bool) []string {
	if output == "" {
		return nil
	}

	switch helper {
	case dialogZenity, dialogYad:
		// zenity/yad with --separator=\n uses newline for multiple files
		if multiple {
			return strings.Split(output, "\n")
		}
		return []string{output}
	case dialogKdialog:
		// kdialog --multiple returns paths separated by newline
		if multiple {
			return strings.Split(output, "\n")
		}
		return []string{output}
	}
	return []string{output}
}
