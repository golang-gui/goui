package x11

import (
	"errors"
	"os"
	"os/exec"
	"strings"

	"github.com/golang-gui/goui/platform/common"
)

// fileDialog implements the native file dialog capability for X11.
type fileDialog struct {
	helper dialogHelper
}

func newFileDialog() (common.FileDialog, error) {
	helper := detectDialogHelper()
	if helper == dialogNone {
		return nil, common.ErrUnsupported
	}
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
func (fd *fileDialog) OpenFile(owner common.Window, opts common.DialogOptions, cb func([]string, error)) {
	args := buildOpenFileArgs(fd.helper, opts)
	output, err := runDialog(fd.helper, args)
	if err != nil {
		if errors.Is(err, errDialogCancelled) {
			cb(nil, nil)
			return
		}
		cb(nil, err)
		return
	}

	paths := parseOutput(fd.helper, output, opts.AllowMultiple)
	cb(paths, nil)
}

// OpenDirectory opens a native dialog for selecting a directory.
func (fd *fileDialog) OpenDirectory(owner common.Window, opts common.DialogOptions, cb func([]string, error)) {
	args := buildOpenDirectoryArgs(fd.helper, opts)
	output, err := runDialog(fd.helper, args)
	if err != nil {
		if errors.Is(err, errDialogCancelled) {
			cb(nil, nil)
			return
		}
		cb(nil, err)
		return
	}

	paths := parseOutput(fd.helper, output, false)
	cb(paths, nil)
}

// SaveFile opens a native save-file dialog.
func (fd *fileDialog) SaveFile(owner common.Window, opts common.DialogOptions, cb func([]string, error)) {
	args := buildSaveFileArgs(fd.helper, opts)
	output, err := runDialog(fd.helper, args)
	if err != nil {
		if errors.Is(err, errDialogCancelled) {
			cb(nil, nil)
			return
		}
		cb(nil, err)
		return
	}

	paths := parseOutput(fd.helper, output, false)
	cb(paths, nil)
}

var errDialogCancelled = errors.New("dialog cancelled")

// runDialog executes a dialog helper command and returns its stdout output.
func runDialog(helper dialogHelper, args []string) (string, error) {
	var name string
	switch helper {
	case dialogZenity:
		name = "zenity"
	case dialogKdialog:
		name = "kdialog"
	case dialogYad:
		name = "yad"
	default:
		return "", errors.New("unknown dialog helper")
	}

	cmd := exec.Command(name, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// zenity/kdialog exit code 1 means user cancelled
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return "", errDialogCancelled
		}
		return "", errors.New("dialog failed: " + string(output))
	}
	return strings.TrimSpace(string(output)), nil
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
