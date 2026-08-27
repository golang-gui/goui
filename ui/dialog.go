package ui

import (
	"github.com/golang-gui/goui/gui"
)

// FileFilter defines a file type filter for dialog boxes.
type FileFilter = gui.FileFilter

// DialogOptions configures a native file dialog.
type DialogOptions = gui.DialogOptions

// FileDialog is the thread-safe UI-layer view of the system file dialog,
// obtained via App.FileDialog(). It is always usable: operations run on the
// UI thread automatically, and when the platform file dialog is unavailable
// they degrade to reporting ErrUnsupported. Cancel is reported as empty
// slice with nil error.
type FileDialog struct {
	current *app
	dlg     gui.FileDialog
}

// FileDialog returns the system file dialog view. It is never nil.
func (a *app) FileDialog() FileDialog {
	return FileDialog{
		current: a,
		dlg:     a.gui.FileDialog(),
	}
}

// OpenFile opens a native file-selection dialog for selecting one or more files.
// It is safe to call from any goroutine; the callback is invoked on the UI thread.
func (f FileDialog) OpenFile(opts DialogOptions, cb func([]string, error)) {
	if f.current == nil || f.dlg == nil {
		return
	}
	f.current.Sync(func() {
		f.dlg.OpenFile(nil, opts, cb)
	})
}

// OpenDirectory opens a native dialog for selecting a directory.
// It is safe to call from any goroutine; the callback is invoked on the UI thread.
func (f FileDialog) OpenDirectory(opts DialogOptions, cb func([]string, error)) {
	if f.current == nil || f.dlg == nil {
		return
	}
	f.current.Sync(func() {
		f.dlg.OpenDirectory(nil, opts, cb)
	})
}

// SaveFile opens a native save-file dialog.
// It is safe to call from any goroutine; the callback is invoked on the UI thread.
func (f FileDialog) SaveFile(opts DialogOptions, cb func([]string, error)) {
	if f.current == nil || f.dlg == nil {
		return
	}
	f.current.Sync(func() {
		f.dlg.SaveFile(nil, opts, cb)
	})
}
