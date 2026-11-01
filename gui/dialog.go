package gui

import (
	"errors"
	"fmt"
	"slices"

	"github.com/golang-gui/goui/platform"
)

// FileFilter defines a file type filter for dialog boxes.
type FileFilter = platform.FileFilter

// DialogOptions configures a native file dialog.
type DialogOptions = platform.DialogOptions

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
	OpenFile(owner Window, opts DialogOptions, cb func([]FileAccess, error))
	// OpenDirectory opens a native dialog for selecting a directory.
	// owner is blocked until completion; nil opens an unowned dialog.
	// Close each successful result after all file operations finish.
	OpenDirectory(owner Window, opts DialogOptions, cb func([]FileAccess, error))
	// SaveFile opens a native save-file dialog.
	// owner is blocked until completion; nil opens an unowned dialog.
	// Selection does not create or write the file.
	SaveFile(owner Window, opts DialogOptions, cb func([]FileAccess, error))
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
func (fd *fileDialog) OpenFile(owner Window, opts DialogOptions, cb func([]FileAccess, error)) {
	fd.open(owner, opts, cb, func(w platform.Window, o DialogOptions, done func([]FileAccess, error)) { fd.dlg.OpenFile(w, o, done) })
}

func (fd *fileDialog) OpenDirectory(owner Window, opts DialogOptions, cb func([]FileAccess, error)) {
	fd.open(owner, opts, cb, func(w platform.Window, o DialogOptions, done func([]FileAccess, error)) {
		fd.dlg.OpenDirectory(w, o, done)
	})
}

func (fd *fileDialog) SaveFile(owner Window, opts DialogOptions, cb func([]FileAccess, error)) {
	fd.open(owner, opts, cb, func(w platform.Window, o DialogOptions, done func([]FileAccess, error)) { fd.dlg.SaveFile(w, o, done) })
}

// ErrDialogBusy means this window already owns a file-selection operation.
var ErrDialogBusy = errors.New("file dialog: owner already has an active dialog")

func (fd *fileDialog) open(owner Window, opts DialogOptions, cb func([]FileAccess, error), start func(platform.Window, DialogOptions, func([]FileAccess, error))) {
	if cb == nil {
		panic("gui: nil file dialog callback")
	}
	fail := func(err error) { fd.loop.Post(func() { cb(nil, err) }) }
	if fd.dlg == nil {
		fail(fd.err)
		return
	}
	var w *window
	if owner != nil {
		var ok bool
		w, ok = owner.(*window)
		if !ok {
			fail(fmt.Errorf("file dialog: owner is not a GOUI window"))
			return
		}
		if w.destroyed || w.platformWindow == nil {
			fail(platform.ErrUnavailable)
			return
		}
		if modalRootBlocked(w) {
			fail(ErrDialogBusy)
			return
		}
		blockModalInput(w)
	}
	// Options outlive this call once startup has been posted.
	opts.Filters = slices.Clone(opts.Filters)
	for i := range opts.Filters {
		opts.Filters[i].Patterns = slices.Clone(opts.Filters[i].Patterns)
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
				unblockModalInput(w)
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
		start(native, opts, finish)
	})
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
