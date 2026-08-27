package gui

import (
	"github.com/golang-gui/goui/platform"
)

// FileFilter defines a file type filter for dialog boxes.
type FileFilter = platform.FileFilter

// DialogOptions configures a native file dialog.
type DialogOptions = platform.DialogOptions

// FileDialog is the system file dialog. It is always usable: when the platform
// does not support file dialogs, operations report ErrUnsupported.
type FileDialog interface {
	// OpenFile opens a native file-selection dialog for selecting one or more files.
	// owner is the parent window; nil means desktop-centered.
	// cb is called exactly once on the GUI thread with selected paths.
	// Cancel is reported as empty slice with nil error.
	OpenFile(owner Window, opts DialogOptions, cb func([]string, error))
	// OpenDirectory opens a native dialog for selecting a directory.
	// owner is the parent window; nil means desktop-centered.
	// cb is called exactly once on the GUI thread with selected paths.
	// Cancel is reported as empty slice with nil error.
	OpenDirectory(owner Window, opts DialogOptions, cb func([]string, error))
	// SaveFile opens a native save-file dialog.
	// owner is the parent window; nil means desktop-centered.
	// cb is called exactly once on the GUI thread with selected path.
	// Cancel is reported as empty slice with nil error.
	SaveFile(owner Window, opts DialogOptions, cb func([]string, error))
}

type fileDialog struct {
	dlg  platform.FileDialog // may be nil when platform does not support dialogs
	loop platform.EventLoop
}

func newFileDialog(dlg platform.FileDialog, loop platform.EventLoop) FileDialog {
	return &fileDialog{dlg: dlg, loop: loop}
}

// OpenFile opens a native file-selection dialog for selecting one or more files.
func (fd *fileDialog) OpenFile(owner Window, opts DialogOptions, cb func([]string, error)) {
	if fd.dlg == nil {
		fd.loop.Post(func() {
			cb(nil, platform.ErrUnsupported)
		})
		return
	}
	var ownerWindow platform.Window
	if owner != nil {
		ownerWindow = owner.PlatformWindow()
	}
	fd.dlg.OpenFile(ownerWindow, opts, func(paths []string, err error) {
		fd.loop.Post(func() {
			cb(paths, err)
		})
	})
}

// OpenDirectory opens a native dialog for selecting a directory.
func (fd *fileDialog) OpenDirectory(owner Window, opts DialogOptions, cb func([]string, error)) {
	if fd.dlg == nil {
		fd.loop.Post(func() {
			cb(nil, platform.ErrUnsupported)
		})
		return
	}
	var ownerWindow platform.Window
	if owner != nil {
		ownerWindow = owner.PlatformWindow()
	}
	fd.dlg.OpenDirectory(ownerWindow, opts, func(paths []string, err error) {
		fd.loop.Post(func() {
			cb(paths, err)
		})
	})
}

// SaveFile opens a native save-file dialog.
func (fd *fileDialog) SaveFile(owner Window, opts DialogOptions, cb func([]string, error)) {
	if fd.dlg == nil {
		fd.loop.Post(func() {
			cb(nil, platform.ErrUnsupported)
		})
		return
	}
	var ownerWindow platform.Window
	if owner != nil {
		ownerWindow = owner.PlatformWindow()
	}
	fd.dlg.SaveFile(ownerWindow, opts, func(paths []string, err error) {
		fd.loop.Post(func() {
			cb(paths, err)
		})
	})
}
