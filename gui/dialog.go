package gui

import (
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
// an empty result with nil error. Callbacks run exactly once on the GUI thread
// and transfer ownership of successful results to the caller.
type FileDialog interface {
	// OpenFile opens a native file-selection dialog for selecting one or more files.
	// owner is the parent window; nil means desktop-centered.
	// Close each successful result after all file operations finish.
	OpenFile(owner Window, opts DialogOptions, cb func([]FileAccess, error))
	// OpenDirectory opens a native dialog for selecting a directory.
	// owner is the parent window; nil means desktop-centered.
	// Close each successful result after all file operations finish.
	OpenDirectory(owner Window, opts DialogOptions, cb func([]FileAccess, error))
	// SaveFile opens a native save-file dialog.
	// owner is the parent window; nil means desktop-centered.
	// Selection does not create or write the file.
	SaveFile(owner Window, opts DialogOptions, cb func([]FileAccess, error))
	// RestoreAccess returns caller-owned access without showing a dialog.
	// Persist its current Credential again to refresh stale native references.
	RestoreAccess(credential []byte) (FileAccess, error)
}

type fileDialog struct {
	dlg  platform.FileDialog // may be nil when platform does not support dialogs
	loop platform.EventLoop
	err  error
}

func newFileDialog(dlg platform.FileDialog, loop platform.EventLoop, err error) FileDialog {
	if dlg == nil && err == nil {
		err = platform.ErrUnsupported
	}
	return &fileDialog{dlg: dlg, loop: loop, err: err}
}

// OpenFile opens a native file-selection dialog for selecting one or more files.
func (fd *fileDialog) OpenFile(owner Window, opts DialogOptions, cb func([]FileAccess, error)) {
	if fd.dlg == nil {
		fd.loop.Post(func() {
			cb(nil, fd.err)
		})
		return
	}
	var ownerWindow platform.Window
	if owner != nil {
		ownerWindow = owner.PlatformWindow()
	}
	fd.dlg.OpenFile(ownerWindow, opts, func(access []FileAccess, err error) {
		fd.loop.Post(func() {
			cb(access, err)
		})
	})
}

// OpenDirectory opens a native dialog for selecting a directory.
func (fd *fileDialog) OpenDirectory(owner Window, opts DialogOptions, cb func([]FileAccess, error)) {
	if fd.dlg == nil {
		fd.loop.Post(func() {
			cb(nil, fd.err)
		})
		return
	}
	var ownerWindow platform.Window
	if owner != nil {
		ownerWindow = owner.PlatformWindow()
	}
	fd.dlg.OpenDirectory(ownerWindow, opts, func(access []FileAccess, err error) {
		fd.loop.Post(func() {
			cb(access, err)
		})
	})
}

// SaveFile opens a native save-file dialog.
func (fd *fileDialog) SaveFile(owner Window, opts DialogOptions, cb func([]FileAccess, error)) {
	if fd.dlg == nil {
		fd.loop.Post(func() {
			cb(nil, fd.err)
		})
		return
	}
	var ownerWindow platform.Window
	if owner != nil {
		ownerWindow = owner.PlatformWindow()
	}
	fd.dlg.SaveFile(ownerWindow, opts, func(access []FileAccess, err error) {
		fd.loop.Post(func() {
			cb(access, err)
		})
	})
}

func (fd *fileDialog) RestoreAccess(credential []byte) (FileAccess, error) {
	if fd.dlg == nil {
		return nil, fd.err
	}
	return fd.dlg.RestoreAccess(credential)
}
