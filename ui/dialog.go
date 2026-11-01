package ui

import (
	"fmt"
	"os"

	"github.com/golang-gui/goui/gui"
)

// FileFilter defines a file type filter for dialog boxes.
type FileFilter = gui.FileFilter

// DialogOptions configures a native file dialog.
type DialogOptions = gui.DialogOptions

// ErrDialogBusy means the owner already has an active file picker.
var ErrDialogBusy = gui.ErrDialogBusy

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

// OpenFile requests a file picker owned by windowID. Empty means unowned.
// It returns ErrAppStopped without calling cb if dispatch is not accepted.
// Accepted requests deliver results on the UI thread while the app runs.
// Late access during shutdown is released without calling cb. A missing nonempty ID
// is reported to cb; it never falls back to an unowned picker.
func (f FileDialog) OpenFile(windowID string, opts DialogOptions, cb func([]FileAccess, error)) error {
	return f.open(windowID, opts, cb, func(w gui.Window, o DialogOptions, done func([]gui.FileAccess, error)) { f.dlg.OpenFile(w, o, done) })
}

// OpenDirectory follows OpenFile's owner, dispatch and completion rules.
func (f FileDialog) OpenDirectory(windowID string, opts DialogOptions, cb func([]FileAccess, error)) error {
	return f.open(windowID, opts, cb, func(w gui.Window, o DialogOptions, done func([]gui.FileAccess, error)) {
		f.dlg.OpenDirectory(w, o, done)
	})
}

// SaveFile follows OpenFile's owner, dispatch and completion rules.
func (f FileDialog) SaveFile(windowID string, opts DialogOptions, cb func([]FileAccess, error)) error {
	return f.open(windowID, opts, cb, func(w gui.Window, o DialogOptions, done func([]gui.FileAccess, error)) { f.dlg.SaveFile(w, o, done) })
}

func (f FileDialog) open(id string, opts DialogOptions, cb func([]FileAccess, error), start func(gui.Window, DialogOptions, func([]gui.FileAccess, error))) (err error) {
	if cb == nil {
		panic("ui: nil file dialog callback")
	}
	err = ErrAppStopped
	if f.current == nil {
		return
	}
	f.current.Sync(func() {
		err = nil
		if f.dlg == nil {
			cb(nil, fmt.Errorf("file dialog: unavailable"))
			return
		}
		var owner gui.Window
		if id != "" {
			owner = f.current.FindWindow(id)
			if owner == nil {
				cb(nil, fmt.Errorf("file dialog: window %q is unavailable", id))
				return
			}
		}
		start(owner, opts, f.callback(cb))
	})
	return
}

func (f FileDialog) callback(cb func([]FileAccess, error)) func([]gui.FileAccess, error) {
	return func(accesses []gui.FileAccess, err error) {
		if f.current == nil || f.current.isStopping() {
			for _, access := range accesses {
				_ = access.Close()
			}
			return
		}
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
