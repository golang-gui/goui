package cocoa

import (
	"github.com/golang-gui/goui/platform/common"

	. "github.com/golang-gui/goui/platform/darwin/frameworks/appkit"
	. "github.com/golang-gui/goui/platform/darwin/frameworks/foundation"
)

// fileDialog implements the native file dialog capability for macOS.
type fileDialog struct {
}

func newFileDialog() (common.FileDialog, error) {
	return &fileDialog{}, nil
}

// OpenFile opens a native file-selection dialog for selecting one or more files.
func (fd *fileDialog) OpenFile(owner common.Window, opts common.DialogOptions, cb func([]string, error)) {
	var panel NSOpenPanel
	AutoReleasePool(func() {
		panel = NSOpenPanelClassId.Alloc().Init()
	})

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
		cb(nil, nil)
		return
	}

	// Get selected URLs
	var paths []string
	AutoReleasePool(func() {
		urls := panel.URLs()
		count := urls.Count()
		for i := uintptr(0); i < count; i++ {
			url := NSURL{}
			url.ID = urls.ObjectAtIndex(i)
			paths = append(paths, url.Path())
		}
	})

	cb(paths, nil)
}

// OpenDirectory opens a native dialog for selecting a directory.
func (fd *fileDialog) OpenDirectory(owner common.Window, opts common.DialogOptions, cb func([]string, error)) {
	var panel NSOpenPanel
	AutoReleasePool(func() {
		panel = NSOpenPanelClassId.Alloc().Init()
	})

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
		cb(nil, nil)
		return
	}

	// Get selected URL
	var paths []string
	AutoReleasePool(func() {
		urls := panel.URLs()
		count := urls.Count()
		for i := uintptr(0); i < count; i++ {
			url := NSURL{}
			url.ID = urls.ObjectAtIndex(i)
			paths = append(paths, url.Path())
		}
	})

	cb(paths, nil)
}

// SaveFile opens a native save-file dialog.
func (fd *fileDialog) SaveFile(owner common.Window, opts common.DialogOptions, cb func([]string, error)) {
	var panel NSSavePanel
	AutoReleasePool(func() {
		panel = NSSavePanelClassId.Alloc().Init()
	})

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
		cb(nil, nil)
		return
	}

	// Get selected URL
	var path string
	AutoReleasePool(func() {
		url := panel.URL()
		path = url.Path()
	})

	cb([]string{path}, nil)
}
