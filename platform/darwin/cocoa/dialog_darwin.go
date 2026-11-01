package cocoa

import (
	"fmt"

	"github.com/ebitengine/purego/objc"

	"github.com/golang-gui/goui/platform/common"

	. "github.com/golang-gui/goui/platform/darwin/frameworks/appkit"
	. "github.com/golang-gui/goui/platform/darwin/frameworks/foundation"
)

// fileDialog implements the native file dialog capability for macOS.
type fileDialog struct {
	sandbox         bool
	bookmarkOptions NSURLBookmarkCreationOptions
}

func newFileDialog() (common.FileDialog, error) {
	sandbox, err := processEntitlement("com.apple.security.app-sandbox")
	if err != nil {
		return nil, err
	}
	fd := &fileDialog{sandbox: sandbox}
	if sandbox {
		fd.bookmarkOptions = NSURLBookmarkCreationWithSecurityScope
		readOnly, err := processEntitlement("com.apple.security.files.user-selected.read-only")
		if err != nil {
			return nil, err
		}
		if readOnly {
			readWrite, err := processEntitlement("com.apple.security.files.user-selected.read-write")
			if err != nil {
				return nil, err
			}
			if !readWrite {
				fd.bookmarkOptions |= NSURLBookmarkCreationSecurityScopeAllowOnlyReadAccess
			}
		}
	}
	return fd, nil
}

// OpenFile opens a native file-selection dialog for selecting one or more files.
func (fd *fileDialog) OpenFile(owner common.Window, opts common.DialogOptions, cb func([]common.FileAccess, error)) {
	var panel NSOpenPanel
	AutoReleasePool(func() {
		panel = NSOpenPanelClassId.Alloc().Init()
	})
	if panel.IsNil() {
		cb(nil, fmt.Errorf("file dialog: create NSOpenPanel failed"))
		return
	}

	if opts.Title != "" {
		AutoReleasePool(func() {
			panel.SetTitle(opts.Title)
		})
	}

	// Set allowed file types
	if len(opts.Filters) > 0 {
		var types []string
		for _, f := range opts.Filters {
			for _, p := range f.Patterns {
				types = append(types, p)
			}
		}
		AutoReleasePool(func() {
			panel.SetAllowedFileTypes(types)
		})
	}

	// Set initial directory
	if opts.InitialDir != "" {
		AutoReleasePool(func() {
			panel.SetDirectoryURL(opts.InitialDir)
		})
	}

	// Set multiple selection
	AutoReleasePool(func() {
		panel.SetAllowsMultipleSelection(opts.AllowMultiple)
	})

	fd.present(owner, panel.NSSavePanel, func(result NSModalResponse, presentErr error) {
		if presentErr != nil {
			cb(nil, presentErr)
			return
		}
		if result != NSModalResponseOK {
			cb(nil, fileDialogResponseError(result))
			return
		}
		var access []common.FileAccess
		var err error
		AutoReleasePool(func() { access, err = fd.selectedAccess(panel.URLs()) })
		cb(access, err)
	})
}

// OpenDirectory opens a native dialog for selecting a directory.
func (fd *fileDialog) OpenDirectory(owner common.Window, opts common.DialogOptions, cb func([]common.FileAccess, error)) {
	var panel NSOpenPanel
	AutoReleasePool(func() {
		panel = NSOpenPanelClassId.Alloc().Init()
	})
	if panel.IsNil() {
		cb(nil, fmt.Errorf("file dialog: create NSOpenPanel failed"))
		return
	}

	if opts.Title != "" {
		AutoReleasePool(func() {
			panel.SetTitle(opts.Title)
		})
	}

	// Set initial directory
	if opts.InitialDir != "" {
		AutoReleasePool(func() {
			panel.SetDirectoryURL(opts.InitialDir)
		})
	}

	// Configure for directory selection
	AutoReleasePool(func() {
		panel.SetCanChooseFiles(false)
		panel.SetCanChooseDirectories(true)
	})

	fd.present(owner, panel.NSSavePanel, func(result NSModalResponse, presentErr error) {
		if presentErr != nil {
			cb(nil, presentErr)
			return
		}
		if result != NSModalResponseOK {
			cb(nil, fileDialogResponseError(result))
			return
		}
		var access []common.FileAccess
		var err error
		AutoReleasePool(func() { access, err = fd.selectedAccess(panel.URLs()) })
		cb(access, err)
	})
}

// SaveFile opens a native save-file dialog.
func (fd *fileDialog) SaveFile(owner common.Window, opts common.DialogOptions, cb func([]common.FileAccess, error)) {
	var panel NSSavePanel
	AutoReleasePool(func() {
		panel = NSSavePanelClassId.Alloc().Init()
	})
	if panel.IsNil() {
		cb(nil, fmt.Errorf("file dialog: create NSSavePanel failed"))
		return
	}

	if opts.Title != "" {
		AutoReleasePool(func() {
			panel.SetTitle(opts.Title)
		})
	}

	// Set allowed file types
	if len(opts.Filters) > 0 {
		var types []string
		for _, f := range opts.Filters {
			for _, p := range f.Patterns {
				types = append(types, p)
			}
		}
		AutoReleasePool(func() {
			panel.SetAllowedFileTypes(types)
		})
	}

	// Set initial directory
	if opts.InitialDir != "" {
		AutoReleasePool(func() {
			panel.SetDirectoryURL(opts.InitialDir)
		})
	}

	// Set default filename
	if opts.DefaultName != "" {
		AutoReleasePool(func() {
			panel.SetNameFieldStringValue(opts.DefaultName)
		})
	}

	fd.present(owner, panel, func(result NSModalResponse, presentErr error) {
		if presentErr != nil {
			cb(nil, presentErr)
			return
		}
		if result != NSModalResponseOK {
			cb(nil, fileDialogResponseError(result))
			return
		}
		var access common.FileAccess
		var err error
		AutoReleasePool(func() { access, err = fd.newAccess(panel.URL(), false) })
		if err != nil {
			cb(nil, err)
			return
		}
		cb([]common.FileAccess{access}, nil)
	})
}

type fileDialogCall struct {
	owner     *Window
	panel     NSSavePanel
	handler   objc.Block
	cancelled bool
	finished  bool
	done      func(NSModalResponse, error)
}

var nativeFileDialogs = map[*fileDialogCall]struct{}{}
var fileDialogsStopping bool

func cancelFileDialogs(owner *Window) {
	if owner == nil {
		fileDialogsStopping = true
	}
	for call := range nativeFileDialogs {
		if !call.cancelled && (owner == nil || call.owner == owner) {
			call.cancelled = true
			call.panel.Cancel(0)
		}
	}
}

func (fd *fileDialog) present(owner common.Window, panel NSSavePanel, done func(NSModalResponse, error)) {
	if fileDialogsStopping {
		panel.Release()
		done(NSModalResponseCancel, common.ErrUnavailable)
		return
	}
	var w *Window
	if owner != nil {
		var ok bool
		w, ok = owner.(*Window)
		if !ok || w.destroying || w.closed || !w.window.Valid() {
			panel.Release()
			done(NSModalResponseCancel, common.ErrUnavailable)
			return
		}
	}
	call := &fileDialogCall{owner: w, panel: panel, done: done}
	call.handler = objc.NewBlock(func(_ objc.Block, response NSModalResponse) { call.finish(response) })
	panel.SetReleasedWhenClosed(false)
	nativeFileDialogs[call] = struct{}{}
	if w != nil {
		panel.BeginSheetModal(w.window, call.handler)
	} else {
		panel.Begin(call.handler)
	}
}

func (call *fileDialogCall) finish(response NSModalResponse) {
	if call.finished {
		return
	}
	call.finished = true
	delete(nativeFileDialogs, call)
	// A sheet's completion can run while it is still onscreen. Hide it before
	// application completion, and release our allocation and block afterwards.
	call.panel.OrderOut(0)
	defer call.panel.Release()
	defer call.handler.Release()
	if call.cancelled || call.owner != nil && call.owner.closed {
		call.done(NSModalResponseCancel, common.ErrUnavailable)
		return
	}
	call.done(response, nil)
}

func fileDialogResponseError(response NSModalResponse) error {
	if response == NSModalResponseCancel {
		return nil
	}
	return fmt.Errorf("file dialog: unexpected modal response %d", response)
}
