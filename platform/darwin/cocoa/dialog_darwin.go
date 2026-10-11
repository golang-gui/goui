package cocoa

import (
	"fmt"

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

	defer func() {
		AutoReleasePool(func() {
			panel.Release()
		})
	}()

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

	// Run modal
	result := panel.RunModal()
	if result != NSModalResponseOK {
		cb(nil, fileDialogResponseError(result))
		return
	}

	// Get selected URLs
	var access []common.FileAccess
	var err error
	AutoReleasePool(func() {
		access, err = fd.selectedAccess(panel.URLs())
	})

	cb(access, err)
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

	defer func() {
		AutoReleasePool(func() {
			panel.Release()
		})
	}()

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

	// Run modal
	result := panel.RunModal()
	if result != NSModalResponseOK {
		cb(nil, fileDialogResponseError(result))
		return
	}

	// Get selected URL
	var access []common.FileAccess
	var err error
	AutoReleasePool(func() {
		access, err = fd.selectedAccess(panel.URLs())
	})

	cb(access, err)
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

	defer func() {
		AutoReleasePool(func() {
			panel.Release()
		})
	}()

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

	// Run modal
	result := panel.RunModal()
	if result != NSModalResponseOK {
		cb(nil, fileDialogResponseError(result))
		return
	}

	// Get selected URL
	var access common.FileAccess
	var err error
	AutoReleasePool(func() {
		access, err = fd.newAccess(panel.URL(), false)
	})

	if err != nil {
		cb(nil, err)
		return
	}
	cb([]common.FileAccess{access}, nil)
}

func fileDialogResponseError(response NSModalResponse) error {
	if response == NSModalResponseCancel {
		return nil
	}
	return fmt.Errorf("file dialog: unexpected modal response %d", response)
}
