package shell

import (
	"runtime"
	"syscall"
	"unsafe"

	"github.com/goexlib/cgo"
	"github.com/golang-gui/goui/platform/windows/sdk/com"
)

// CLSID for FileOpenDialog and FileSaveDialog (Common Item Dialog).
var (
	CLSID_FileOpenDialog = com.DefineGuid(0xdc1c5a9c, 0xe88a, 0x4dde, 0xa5, 0xa1, 0x60, 0xf8, 0x2a, 0x20, 0xae, 0xf7)
	CLSID_FileSaveDialog = com.DefineGuid(0xc0b4e2f3, 0xba21, 0x4773, 0x8d, 0xba, 0x33, 0x5e, 0xc9, 0x46, 0xeb, 0x8b)
)

// IIDs.
var (
	IID_IFileDialog     = com.DefineGuid(0x42f85136, 0xdb7e, 0x439c, 0x85, 0xf1, 0xe4, 0x07, 0x5d, 0x13, 0x5f, 0xc8)
	IID_IFileOpenDialog = com.DefineGuid(0xd57c7288, 0xd4ad, 0x4768, 0xbe, 0x02, 0x9d, 0x96, 0x95, 0x32, 0xd9, 0x60)
	IID_IFileSaveDialog = com.DefineGuid(0x84bccd23, 0x5fde, 0x4cdb, 0xae, 0xa4, 0xaf, 0x64, 0xb8, 0x3d, 0x78, 0xab)
	IID_IShellItem      = com.DefineGuid(0x43826d1e, 0xe718, 0x42ee, 0xbc, 0x55, 0xa1, 0xe2, 0x61, 0xc3, 0x7b, 0xfe)
	IID_IShellItemArray = com.DefineGuid(0xb63ea76d, 0x1f85, 0x456f, 0xa1, 0x9c, 0x48, 0x15, 0x9e, 0xfa, 0x85, 0x8b)
)

var (
	shell32                         = cgo.NewLazyLibrary("shell32.dll")
	procSHCreateItemFromParsingName = shell32.NewSymbol("SHCreateItemFromParsingName")
)

// CreateShellItemFromPath creates an IShellItem from a file system path.
// Returns nil on failure.
func CreateShellItemFromPath(path string) *ShellItem {
	wPath, _ := syscall.UTF16PtrFromString(path)
	defer runtime.KeepAlive(wPath)
	var item *ShellItem
	ret, _, _ := procSHCreateItemFromParsingName.CallRaw(
		uintptr(unsafe.Pointer(wPath)),
		0,
		uintptr(unsafe.Pointer(&IID_IShellItem)),
		uintptr(unsafe.Pointer(&item)),
	)
	if com.HRESULT(ret).Failed() {
		return nil
	}
	return item
}

// --- IShellItem ---

type ShellItemClass struct {
	com.UnknownClass
	BindToHandler  cgo.Symbol // HRESULT(IShellItem*, IBindCtx*, REFGUID, REFIID, void**)
	GetParent      cgo.Symbol // HRESULT(IShellItem*, IShellItem**)
	GetDisplayName cgo.Symbol // HRESULT(IShellItem*, SIGDN, LPWSTR*)
	GetAttributes  cgo.Symbol // HRESULT(IShellItem*, SFGAOF, SFGAOF*)
	Compare        cgo.Symbol // HRESULT(IShellItem*, IShellItem*, SICHINTF, int*)
}

type ShellItem struct {
	com.Unknown
}

func (s *ShellItem) GetDisplayName(sigdn SIGDN) (string, com.HRESULT) {
	var ptr *uint16
	ret, _, _ := s.class().GetDisplayName.CallRaw(
		uintptr(cgo.Pointer(s)),
		uintptr(sigdn),
		uintptr(unsafe.Pointer(&ptr)),
	)
	hr := com.HRESULT(ret)
	if hr.Failed() {
		return "", hr
	}
	name := utf16PtrToString(ptr)
	com.CoTaskMemFree(unsafe.Pointer(ptr))
	return name, hr
}

func (s *ShellItem) class() *ShellItemClass {
	return (*ShellItemClass)(s.Class)
}

// --- IShellItemArray ---

type ShellItemArrayClass struct {
	com.UnknownClass
	BindToHandler              cgo.Symbol
	GetPropertyStore           cgo.Symbol
	GetPropertyDescriptionList cgo.Symbol
	GetAttributes              cgo.Symbol
	GetCount                   cgo.Symbol // HRESULT(IShellItemArray*, DWORD*)
	GetItemAt                  cgo.Symbol // HRESULT(IShellItemArray*, UINT, IShellItem**)
	EnumItems                  cgo.Symbol
}

type ShellItemArray struct {
	com.Unknown
}

func (a *ShellItemArray) GetCount() (uint32, com.HRESULT) {
	var count uint32
	ret, _, _ := a.class().GetCount.CallRaw(
		uintptr(cgo.Pointer(a)),
		uintptr(unsafe.Pointer(&count)),
	)
	return count, com.HRESULT(ret)
}

func (a *ShellItemArray) GetItemAt(index uint32) (*ShellItem, com.HRESULT) {
	var item *ShellItem
	ret, _, _ := a.class().GetItemAt.CallRaw(
		uintptr(cgo.Pointer(a)),
		uintptr(index),
		uintptr(unsafe.Pointer(&item)),
	)
	return item, com.HRESULT(ret)
}

func (a *ShellItemArray) class() *ShellItemArrayClass {
	return (*ShellItemArrayClass)(a.Class)
}

// --- IFileDialog (inherits IModalWindow) ---

type FileDialogClass struct {
	com.UnknownClass
	// IModalWindow
	Show cgo.Symbol // HRESULT(IModalWindow*, HWND)
	// IFileDialog
	SetFileTypes        cgo.Symbol
	SetFileTypeIndex    cgo.Symbol
	GetFileTypeIndex    cgo.Symbol
	Advise              cgo.Symbol
	Unadvise            cgo.Symbol
	SetOptions          cgo.Symbol
	GetOptions          cgo.Symbol
	SetDefaultFolder    cgo.Symbol
	SetFolder           cgo.Symbol
	GetFolder           cgo.Symbol
	GetCurrentSelection cgo.Symbol
	SetFileName         cgo.Symbol
	GetFileName         cgo.Symbol
	SetTitle            cgo.Symbol
	SetOkButtonLabel    cgo.Symbol
	SetFileNameLabel    cgo.Symbol
	GetResult           cgo.Symbol
	AddPlace            cgo.Symbol
	SetDefaultExtension cgo.Symbol
	Close               cgo.Symbol
	SetClientGuid       cgo.Symbol
	ClearClientData     cgo.Symbol
	SetFilter           cgo.Symbol
}

type FileDialog struct {
	com.Unknown
}

func (d *FileDialog) Show(hwnd uintptr) com.HRESULT {
	ret, _, _ := d.class().Show.CallRaw(
		uintptr(cgo.Pointer(d)),
		hwnd,
	)
	return com.HRESULT(ret)
}

func (d *FileDialog) SetTitle(title string) com.HRESULT {
	wTitle, _ := syscall.UTF16PtrFromString(title)
	defer runtime.KeepAlive(wTitle)
	ret, _, _ := d.class().SetTitle.CallRaw(
		uintptr(cgo.Pointer(d)),
		uintptr(unsafe.Pointer(wTitle)),
	)
	return com.HRESULT(ret)
}

func (d *FileDialog) SetFileTypes(specs []COMDLG_FILTERSPEC) com.HRESULT {
	if len(specs) == 0 {
		return 0
	}
	ret, _, _ := d.class().SetFileTypes.CallRaw(
		uintptr(cgo.Pointer(d)),
		uintptr(len(specs)),
		uintptr(cgo.CSlice(specs)),
	)
	return com.HRESULT(ret)
}

func (d *FileDialog) SetOptions(fos FILEOPENDIALOGOPTIONS) com.HRESULT {
	ret, _, _ := d.class().SetOptions.CallRaw(
		uintptr(cgo.Pointer(d)),
		uintptr(fos),
	)
	return com.HRESULT(ret)
}

func (d *FileDialog) SetFolder(item *ShellItem) com.HRESULT {
	ret, _, _ := d.class().SetFolder.CallRaw(
		uintptr(cgo.Pointer(d)),
		uintptr(cgo.Pointer(item)),
	)
	return com.HRESULT(ret)
}

func (d *FileDialog) GetResult() (*ShellItem, com.HRESULT) {
	var item *ShellItem
	ret, _, _ := d.class().GetResult.CallRaw(
		uintptr(cgo.Pointer(d)),
		uintptr(unsafe.Pointer(&item)),
	)
	return item, com.HRESULT(ret)
}

func (d *FileDialog) SetFileName(name string) com.HRESULT {
	wName, _ := syscall.UTF16PtrFromString(name)
	defer runtime.KeepAlive(wName)
	ret, _, _ := d.class().SetFileName.CallRaw(
		uintptr(cgo.Pointer(d)),
		uintptr(unsafe.Pointer(wName)),
	)
	return com.HRESULT(ret)
}

func (d *FileDialog) GetFileName() (string, com.HRESULT) {
	var ptr *uint16
	ret, _, _ := d.class().GetFileName.CallRaw(
		uintptr(cgo.Pointer(d)),
		uintptr(unsafe.Pointer(&ptr)),
	)
	hr := com.HRESULT(ret)
	if hr.Failed() {
		return "", hr
	}
	name := utf16PtrToString(ptr)
	com.CoTaskMemFree(unsafe.Pointer(ptr))
	return name, hr
}

func (d *FileDialog) SetDefaultExtension(ext string) com.HRESULT {
	wExt, _ := syscall.UTF16PtrFromString(ext)
	defer runtime.KeepAlive(wExt)
	ret, _, _ := d.class().SetDefaultExtension.CallRaw(
		uintptr(cgo.Pointer(d)),
		uintptr(unsafe.Pointer(wExt)),
	)
	return com.HRESULT(ret)
}

func (d *FileDialog) class() *FileDialogClass {
	return (*FileDialogClass)(d.Class)
}

// --- IFileOpenDialog ---

type FileOpenDialogClass struct {
	FileDialogClass
	GetResults       cgo.Symbol // HRESULT(IFileOpenDialog*, IShellItemArray**)
	GetSelectedItems cgo.Symbol
}

type FileOpenDialog struct {
	FileDialog
}

func (d *FileOpenDialog) GetResults() (*ShellItemArray, com.HRESULT) {
	var arr *ShellItemArray
	ret, _, _ := d.class().GetResults.CallRaw(
		uintptr(cgo.Pointer(d)),
		uintptr(unsafe.Pointer(&arr)),
	)
	return arr, com.HRESULT(ret)
}

func (d *FileOpenDialog) class() *FileOpenDialogClass {
	return (*FileOpenDialogClass)(d.Class)
}

// --- IFileSaveDialog ---

type FileSaveDialogClass struct {
	FileDialogClass
	SetSaveAsItem     cgo.Symbol // HRESULT(IFileSaveDialog*, IShellItem*)
	SetProperties     cgo.Symbol
	GetProperties     cgo.Symbol
	SetCollectedItems cgo.Symbol
	GetCollectedItems cgo.Symbol
	SetDefaultFolder  cgo.Symbol // hides IFileDialog version
}

type FileSaveDialog struct {
	FileDialog
}

func (d *FileSaveDialog) SetSaveAsItem(item *ShellItem) com.HRESULT {
	ret, _, _ := d.class().SetSaveAsItem.CallRaw(
		uintptr(cgo.Pointer(d)),
		uintptr(cgo.Pointer(item)),
	)
	return com.HRESULT(ret)
}

func (d *FileSaveDialog) class() *FileSaveDialogClass {
	return (*FileSaveDialogClass)(d.Class)
}

// --- Create helpers ---

// CreateFileOpenDialog creates a new IFileOpenDialog instance.
func CreateFileOpenDialog() (*FileOpenDialog, com.HRESULT) {
	return com.CreateInstance[FileOpenDialog](CLSID_FileOpenDialog, nil, com.CLSCTX_INPROC_SERVER, IID_IFileOpenDialog)
}

// CreateFileSaveDialog creates a new IFileSaveDialog instance.
func CreateFileSaveDialog() (*FileSaveDialog, com.HRESULT) {
	return com.CreateInstance[FileSaveDialog](CLSID_FileSaveDialog, nil, com.CLSCTX_INPROC_SERVER, IID_IFileSaveDialog)
}

// HRESULT_CANCEL is the HRESULT returned when the user cancels a dialog.
// 0x800704C7 = HRESULT_FROM_WIN32(ERROR_CANCELLED)
var HRESULT_CANCEL = com.HRESULT(-2147023417)

func utf16PtrToString(p *uint16) string {
	if p == nil {
		return ""
	}
	var buf []uint16
	for {
		if *p == 0 {
			break
		}
		buf = append(buf, *p)
		p = (*uint16)(unsafe.Pointer(uintptr(unsafe.Pointer(p)) + 2))
	}
	return syscall.UTF16ToString(buf)
}
