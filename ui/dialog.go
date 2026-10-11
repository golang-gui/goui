package ui

import (
	"os"

	"github.com/golang-gui/goui/gui"
)

// FileFilter defines a file type filter for dialog boxes.
type FileFilter = gui.FileFilter

// DialogOptions configures a native file dialog.
type DialogOptions = gui.DialogOptions

// FileDialog is the thread-safe UI-layer view of the system file dialog,
// obtained via App.FileDialog(). Operations run on the UI thread automatically
// while the application is running. Cancellation is an empty result with nil
// error; system failures are returned to the callback.
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
func (f FileDialog) OpenFile(opts DialogOptions, cb func([]FileAccess, error)) {
	if f.current == nil || f.dlg == nil {
		return
	}
	f.current.Sync(func() {
		f.dlg.OpenFile(nil, opts, f.callback(cb))
	})
}

// OpenDirectory opens a native dialog for selecting a directory.
// It is safe to call from any goroutine; the callback is invoked on the UI thread.
func (f FileDialog) OpenDirectory(opts DialogOptions, cb func([]FileAccess, error)) {
	if f.current == nil || f.dlg == nil {
		return
	}
	f.current.Sync(func() {
		f.dlg.OpenDirectory(nil, opts, f.callback(cb))
	})
}

// SaveFile opens a native save-file dialog.
// It is safe to call from any goroutine; the callback is invoked on the UI thread.
func (f FileDialog) SaveFile(opts DialogOptions, cb func([]FileAccess, error)) {
	if f.current == nil || f.dlg == nil {
		return
	}
	f.current.Sync(func() {
		f.dlg.SaveFile(nil, opts, f.callback(cb))
	})
}

func (f FileDialog) callback(cb func([]FileAccess, error)) func([]gui.FileAccess, error) {
	return func(accesses []gui.FileAccess, err error) {
		result := make([]FileAccess, len(accesses))
		for i, access := range accesses {
			result[i] = FileAccess{current: f.current, access: access, name: access.Name()}
		}
		cb(result, err)
	}
}

// RestoreAccess restores an application's saved opaque credential without a picker.
// It is safe to call from any goroutine while the application is running.
func (f FileDialog) RestoreAccess(credential []byte) (result FileAccess, err error) {
	err = ErrAppStopped
	if f.current == nil || f.dlg == nil {
		return
	}
	f.current.Sync(func() {
		var access gui.FileAccess
		access, err = f.dlg.RestoreAccess(credential)
		if err == nil {
			result = FileAccess{current: f.current, access: access, name: access.Name()}
		}
	})
	return
}

// FileAccess owns a selected resource's native access. Copies share the same
// resource; Close is idempotent and must run before the application stops.
// Keep it open until all background I/O using LocalPath has finished.
type FileAccess struct {
	current *app
	access  gui.FileAccess
	name    string
}

// Name returns the display name, including after Close.
func (f FileAccess) Name() string { return f.name }

// LocalPath returns the absolute local path on the UI thread. It does not copy
// the file or transfer ownership of the access to the caller.
func (f FileAccess) LocalPath() (path string, err error) {
	err = os.ErrClosed
	if f.current == nil || f.access == nil {
		return
	}
	err = ErrAppStopped
	f.current.Sync(func() { path, err = f.access.LocalPath() })
	return
}

// Credential creates opaque bytes that the application can persist and later
// pass to RestoreAccess. It is not a password or a permanent permission grant.
func (f FileAccess) Credential() (data []byte, err error) {
	err = os.ErrClosed
	if f.current == nil || f.access == nil {
		return
	}
	err = ErrAppStopped
	f.current.Sync(func() { data, err = f.access.Credential() })
	return
}

// Close releases native access on the UI thread. It never deletes the resource.
func (f FileAccess) Close() (err error) {
	if f.current == nil || f.access == nil {
		return nil
	}
	err = ErrAppStopped
	f.current.Sync(func() { err = f.access.Close() })
	return
}
