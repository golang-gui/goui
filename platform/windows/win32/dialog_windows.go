package win32

import (
	"runtime"
	"strings"
	"syscall"

	"github.com/golang-gui/goui/platform/common"
	"github.com/golang-gui/goui/platform/windows/sdk/shell"
)

// fileDialog implements the native file dialog capability for Windows
// using the Common Item Dialog API (IFileDialog/IFileOpenDialog/IFileSaveDialog).
type fileDialog struct {
	p *Platform
}

func newFileDialog() (common.FileDialog, error) {
	return &fileDialog{p: platform}, nil
}

// OpenFile opens a file selection dialog.
func (fd *fileDialog) OpenFile(owner common.Window, opts common.DialogOptions, cb func([]string, error)) {
	dlg, hr := shell.CreateFileOpenDialog()
	if hr.Failed() {
		cb(nil, hr)
		return
	}
	defer dlg.Release()

	if opts.Title != "" {
		if hr := dlg.SetTitle(opts.Title); hr.Failed() {
			cb(nil, hr)
			return
		}
	}

	if opts.InitialDir != "" {
		if item := shell.CreateShellItemFromPath(opts.InitialDir); item != nil {
			dlg.SetFolder(item)
			item.Release()
		}
	}

	if len(opts.Filters) > 0 {
		specs, cleanup := toFilterSpecs(opts.Filters)
		defer cleanup()
		if hr := dlg.SetFileTypes(specs); hr.Failed() {
			cb(nil, hr)
			return
		}
	}

	var fos shell.FILEOPENDIALOGOPTIONS = shell.FOS_PATHMUSTEXIST | shell.FOS_FILEMUSTEXIST | shell.FOS_FORCEFILESYSTEM
	if opts.AllowMultiple {
		fos |= shell.FOS_ALLOWMULTISELECT
	}
	if hr := dlg.SetOptions(fos); hr.Failed() {
		cb(nil, hr)
		return
	}

	hwnd := uintptr(0)
	if owner != nil {
		hwnd = owner.NativeHandle()
	}
	hr = dlg.Show(hwnd)
	if hr.Failed() {
		if hr == shell.HRESULT_CANCEL {
			cb(nil, nil)
			return
		}
		cb(nil, hr)
		return
	}

	if opts.AllowMultiple {
		arr, hr := dlg.GetResults()
		if hr.Failed() {
			cb(nil, hr)
			return
		}
		defer arr.Release()
		count, hr := arr.GetCount()
		if hr.Failed() {
			cb(nil, hr)
			return
		}
		paths := make([]string, 0, count)
		for i := uint32(0); i < count; i++ {
			item, hr := arr.GetItemAt(i)
			if hr.Failed() {
				continue
			}
			name, hr := item.GetDisplayName(shell.SIGDN_FILESYSPATH)
			item.Release()
			if hr.Succeeded() && name != "" {
				paths = append(paths, name)
			}
		}
		cb(paths, nil)
		return
	}

	item, hr := dlg.GetResult()
	if hr.Failed() {
		cb(nil, hr)
		return
	}
	defer item.Release()
	name, hr := item.GetDisplayName(shell.SIGDN_FILESYSPATH)
	if hr.Failed() {
		cb(nil, hr)
		return
	}
	cb([]string{name}, nil)
}

// OpenDirectory opens a directory selection dialog.
func (fd *fileDialog) OpenDirectory(owner common.Window, opts common.DialogOptions, cb func([]string, error)) {
	dlg, hr := shell.CreateFileOpenDialog()
	if hr.Failed() {
		cb(nil, hr)
		return
	}
	defer dlg.Release()

	if opts.Title != "" {
		if hr := dlg.SetTitle(opts.Title); hr.Failed() {
			cb(nil, hr)
			return
		}
	}

	if opts.InitialDir != "" {
		if item := shell.CreateShellItemFromPath(opts.InitialDir); item != nil {
			dlg.SetFolder(item)
			item.Release()
		}
	}

	fos := shell.FOS_PICKFOLDERS | shell.FOS_PATHMUSTEXIST | shell.FOS_FORCEFILESYSTEM
	if hr := dlg.SetOptions(fos); hr.Failed() {
		cb(nil, hr)
		return
	}

	hwnd := uintptr(0)
	if owner != nil {
		hwnd = owner.NativeHandle()
	}
	hr = dlg.Show(hwnd)
	if hr.Failed() {
		if hr == shell.HRESULT_CANCEL {
			cb(nil, nil)
			return
		}
		cb(nil, hr)
		return
	}

	item, hr := dlg.GetResult()
	if hr.Failed() {
		cb(nil, hr)
		return
	}
	defer item.Release()
	name, hr := item.GetDisplayName(shell.SIGDN_FILESYSPATH)
	if hr.Failed() {
		cb(nil, hr)
		return
	}
	cb([]string{name}, nil)
}

// SaveFile opens a save file dialog.
func (fd *fileDialog) SaveFile(owner common.Window, opts common.DialogOptions, cb func([]string, error)) {
	dlg, hr := shell.CreateFileSaveDialog()
	if hr.Failed() {
		cb(nil, hr)
		return
	}
	defer dlg.Release()

	if opts.Title != "" {
		if hr := dlg.SetTitle(opts.Title); hr.Failed() {
			cb(nil, hr)
			return
		}
	}

	if opts.InitialDir != "" {
		if item := shell.CreateShellItemFromPath(opts.InitialDir); item != nil {
			dlg.SetFolder(item)
			item.Release()
		}
	}

	if opts.DefaultName != "" {
		if hr := dlg.SetFileName(opts.DefaultName); hr.Failed() {
			cb(nil, hr)
			return
		}
	}

	if len(opts.Filters) > 0 {
		specs, cleanup := toFilterSpecs(opts.Filters)
		defer cleanup()
		if hr := dlg.SetFileTypes(specs); hr.Failed() {
			cb(nil, hr)
			return
		}
	}

	fos := shell.FOS_OVERWRITEPROMPT | shell.FOS_PATHMUSTEXIST | shell.FOS_FORCEFILESYSTEM
	if hr := dlg.SetOptions(fos); hr.Failed() {
		cb(nil, hr)
		return
	}

	hwnd := uintptr(0)
	if owner != nil {
		hwnd = owner.NativeHandle()
	}
	hr = dlg.Show(hwnd)
	if hr.Failed() {
		if hr == shell.HRESULT_CANCEL {
			cb(nil, nil)
			return
		}
		cb(nil, hr)
		return
	}

	item, hr := dlg.GetResult()
	if hr.Failed() {
		cb(nil, hr)
		return
	}
	defer item.Release()
	name, hr := item.GetDisplayName(shell.SIGDN_FILESYSPATH)
	if hr.Failed() {
		cb(nil, hr)
		return
	}
	cb([]string{name}, nil)
}

// toFilterSpecs converts common.FileFilter to shell.COMDLG_FILTERSPEC.
// The cleanup function must be called to free UTF16 memory.
func toFilterSpecs(filters []common.FileFilter) ([]shell.COMDLG_FILTERSPEC, func()) {
	specs := make([]shell.COMDLG_FILTERSPEC, 0, len(filters))
	var keepers []*uint16
	for _, f := range filters {
		wName, _ := syscall.UTF16PtrFromString(f.Name)
		pattern := strings.Join(f.Patterns, ";")
		wSpec, _ := syscall.UTF16PtrFromString(pattern)
		keepers = append(keepers, wName, wSpec)
		specs = append(specs, shell.COMDLG_FILTERSPEC{
			Name: wName,
			Spec: wSpec,
		})
	}
	return specs, func() {
		runtime.KeepAlive(keepers)
	}
}
