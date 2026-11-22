package gui

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/golang-gui/goui/platform"
)

// FileFilter selects file extensions without a leading dot or wildcards.
// Extensions are single suffixes, such as "txt" or "png"; ASCII letters are
// matched case-insensitively. An empty list allows all files. Filters aid native
// selection, not content validation; callers must validate selected files.
// Native presentation of multiple named filters may differ between platforms.
type FileFilter struct {
	Name       string
	Extensions []string
}

// DialogOptions configures a native file dialog.
type DialogOptions struct {
	Title         string       // Empty uses the native title.
	InitialDir    string       // Empty uses the native initial directory.
	DefaultName   string       // Complete suggested save filename, including extension.
	Filters       []FileFilter // Empty leaves file types unrestricted; ignored for directories.
	AllowMultiple bool         // Only effective for OpenFile.
}

// FileAccess owns a selected file/directory's native access. Use on the GUI
// thread and Close only after all I/O finishes. See platform.FileAccess.
type FileAccess = platform.FileAccess

// FileDialog selects files/directories and restores persistent access. It is
// never nil; unavailable services report their creation error. Cancellation is
// an empty result with nil error. While the event loop runs, callbacks execute
// once on the GUI thread and transfer successful access to the caller. Native
// startup is posted, so opening does not wait for selection. Undelivered access
// is released when the event loop exits.
type FileDialog interface {
	// OpenFile opens a native file-selection dialog for selecting one or more files.
	// owner is blocked until completion; nil opens an unowned dialog.
	// Close each successful result after all file operations finish.
	OpenFile(owner Surface, opts DialogOptions, cb func([]FileAccess, error))
	// OpenDirectory opens a native dialog for selecting a directory.
	// owner is blocked until completion; nil opens an unowned dialog.
	// Close each successful result after all file operations finish.
	OpenDirectory(owner Surface, opts DialogOptions, cb func([]FileAccess, error))
	// SaveFile opens a native save-file dialog.
	// owner is blocked until completion; nil opens an unowned dialog.
	// Selection does not create or write the file.
	SaveFile(owner Surface, opts DialogOptions, cb func([]FileAccess, error))
	// RestoreAccess returns caller-owned access without showing a dialog.
	// Persist its current Credential again to refresh stale native references.
	RestoreAccess(credential []byte) (FileAccess, error)
}

type fileDialog struct {
	dlg     platform.FileDialog // may be nil when platform does not support dialogs
	loop    platform.EventLoop
	err     error
	closed  bool
	pending map[*fileDialogResult]struct{}
}

func newFileDialog(dlg platform.FileDialog, loop platform.EventLoop, err error) FileDialog {
	if dlg == nil && err == nil {
		err = platform.ErrUnsupported
	}
	return &fileDialog{dlg: dlg, loop: loop, err: err}
}

// OpenFile opens an owner-modal or unowned native file picker.
func (fd *fileDialog) OpenFile(owner Surface, opts DialogOptions, cb func([]FileAccess, error)) {
	fd.open(owner, opts, cb, func(w platform.Window, o platform.DialogOptions, done func([]FileAccess, error)) {
		fd.dlg.OpenFile(w, o, done)
	})
}

func (fd *fileDialog) OpenDirectory(owner Surface, opts DialogOptions, cb func([]FileAccess, error)) {
	opts.Filters = nil
	fd.open(owner, opts, cb, func(w platform.Window, o platform.DialogOptions, done func([]FileAccess, error)) {
		fd.dlg.OpenDirectory(w, o, done)
	})
}

func (fd *fileDialog) SaveFile(owner Surface, opts DialogOptions, cb func([]FileAccess, error)) {
	fd.open(owner, opts, cb, func(w platform.Window, o platform.DialogOptions, done func([]FileAccess, error)) {
		fd.dlg.SaveFile(w, o, done)
	})
}

// ErrDialogBusy means this window already owns a file-selection operation.
var ErrDialogBusy = errors.New("file dialog: owner already has an active dialog")

func (fd *fileDialog) open(owner Surface, opts DialogOptions, cb func([]FileAccess, error), start func(platform.Window, platform.DialogOptions, func([]FileAccess, error))) {
	if cb == nil {
		panic("gui: nil file dialog callback")
	}
	fail := func(err error) { fd.loop.Post(func() { cb(nil, err) }) }
	if fd.dlg == nil {
		fail(fd.err)
		return
	}
	options, err := fd.nativeOptions(opts)
	if err != nil {
		fail(err)
		return
	}
	var w *surfaceBase
	if owner != nil {
		var ok bool
		w = surfaceCore(owner)
		ok = w != nil
		if !ok {
			fail(fmt.Errorf("file dialog: owner is not a GOUI window"))
			return
		}
		if w.destroyed || w.platformWindow == nil {
			fail(platform.ErrUnavailable)
			return
		}
		if modalRootBlocked(w.owner()) {
			fail(ErrDialogBusy)
			return
		}
		blockModalInput(w.owner())
	}
	// Native backends retain their resources until completion. Keep the input
	// boundary through the posted completion, and remove it before application code.
	finish := func(access []FileAccess, err error) {
		if fd.closed {
			for _, a := range access {
				_ = a.Close()
			}
			return
		}
		result := &fileDialogResult{access: access}
		if fd.pending == nil {
			fd.pending = make(map[*fileDialogResult]struct{})
		}
		fd.pending[result] = struct{}{}
		fd.loop.Post(func() {
			delete(fd.pending, result)
			if fd.closed {
				return
			}
			access := result.access
			result.access = nil
			if w != nil {
				unblockModalInput(w.owner())
				if w.destroyed {
					if !errors.Is(err, platform.ErrUnavailable) {
						err = errors.Join(err, platform.ErrUnavailable)
					}
					for _, a := range access {
						err = errors.Join(err, a.Close())
					}
					access = nil
				}
			}
			cb(access, err)
		})
	}

	// Posting the start also makes Windows' synchronous native Show usable as
	// a callback API; its own modal loop continues dispatching native events.
	fd.loop.Post(func() {
		var native platform.Window
		if w != nil {
			if w.destroyed || w.platformWindow == nil {
				finish(nil, platform.ErrUnavailable)
				return
			}
			native = w.platformWindow
		}
		start(native, options, finish)
	})
}

// Build caller-independent native values before posting startup. The backend
// reports syntax; extension validation and case adaptation belong to the GUI.
func (fd *fileDialog) nativeOptions(opts DialogOptions) (platform.DialogOptions, error) {
	native := platform.DialogOptions{
		Title: opts.Title, InitialDir: opts.InitialDir, DefaultName: opts.DefaultName,
		AllowMultiple: opts.AllowMultiple,
	}
	if len(opts.Filters) == 0 {
		return native, nil
	}
	format := fd.dlg.FilterFormat()
	if format != platform.FileFilterGlob && format != platform.FileFilterWildcard && format != platform.FileFilterExtension {
		return native, fmt.Errorf("file dialog filters: %w", platform.ErrUnsupported)
	}
	native.Filters = make([]platform.FileFilter, len(opts.Filters))
	for i, filter := range opts.Filters {
		if !utf8.ValidString(filter.Name) || strings.ContainsRune(filter.Name, 0) {
			return platform.DialogOptions{}, fmt.Errorf("file dialog filter %d: invalid name", i)
		}
		values := make([]string, 0, len(filter.Extensions))
		for _, extension := range filter.Extensions {
			if extension == "" || !utf8.ValidString(extension) || strings.ContainsAny(extension, ".*?[]{}<>:\"/\\|;") || strings.ContainsFunc(extension, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }) {
				return platform.DialogOptions{}, fmt.Errorf("file dialog filter %d: invalid extension %q", i, extension)
			}
			extension = strings.Map(func(r rune) rune {
				if r >= 'A' && r <= 'Z' {
					return r + ('a' - 'A')
				}
				return r
			}, extension)
			var value string
			switch format {
			case platform.FileFilterGlob:
				var pattern strings.Builder
				pattern.WriteString("*.")
				for _, r := range extension {
					if r >= 'a' && r <= 'z' {
						pattern.WriteRune('[')
						pattern.WriteRune(r)
						pattern.WriteRune(r - ('a' - 'A'))
						pattern.WriteRune(']')
					} else {
						pattern.WriteRune(r)
					}
				}
				value = pattern.String()
			case platform.FileFilterWildcard:
				value = "*." + extension
			case platform.FileFilterExtension:
				value = extension
			}
			if !slices.Contains(values, value) {
				values = append(values, value)
			}
		}
		if len(values) == 0 {
			switch format {
			case platform.FileFilterGlob:
				values = []string{"*"}
			case platform.FileFilterWildcard:
				values = []string{"*.*"}
			}
		}
		native.Filters[i] = platform.FileFilter{Name: filter.Name, Values: values}
	}
	return native, nil
}

// Results remain GUI-owned until their posted callback transfers ownership.
// Native-loop exit can discard that callback, so close undelivered access here.
type fileDialogResult struct{ access []FileAccess }

func (fd *fileDialog) close() {
	fd.closed = true
	for result := range fd.pending {
		for _, a := range result.access {
			_ = a.Close()
		}
		result.access = nil
	}
	fd.pending = nil
}

func (fd *fileDialog) RestoreAccess(credential []byte) (FileAccess, error) {
	if fd.dlg == nil {
		return nil, fd.err
	}
	return fd.dlg.RestoreAccess(credential)
}
