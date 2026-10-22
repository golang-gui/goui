package xcursor

import (
	"runtime"
	"unsafe"

	"github.com/goexlib/cgo"
	"github.com/golang-gui/goui/platform/linux/libs/xlib"
)

var (
	lib             = cgo.NewLazyLibrary("libXcursor.so.1")
	loadCursor      = lib.NewSymbol("XcursorLibraryLoadCursor")
	getTheme        = lib.NewSymbol("XcursorGetTheme")
	setTheme        = lib.NewSymbol("XcursorSetTheme")
	getDefaultSize  = lib.NewSymbol("XcursorGetDefaultSize")
	setDefaultSize  = lib.NewSymbol("XcursorSetDefaultSize")
	imageCreate     = lib.NewSymbol("XcursorImageCreate")
	imageDestroy    = lib.NewSymbol("XcursorImageDestroy")
	imageLoadCursor = lib.NewSymbol("XcursorImageLoadCursor")
)

func Available() bool {
	for _, symbol := range []*cgo.LazySymbol{loadCursor, getTheme, setTheme, getDefaultSize, setDefaultSize} {
		if symbol.Find() != nil {
			return false
		}
	}
	return true
}

// Image is the libXcursor XcursorImage ABI. Pixels point to the library's
// allocation and contain premultiplied ARGB32 values.
type Image struct {
	Version, Size, Width, Height, XHot, YHot, Delay uint32
	Pixels                                          *uint32
}

func CreateImage(width, height int) *Image {
	if imageCreate.Find() != nil || imageDestroy.Find() != nil || imageLoadCursor.Find() != nil {
		return nil
	}
	return cgo.CallRet[*Image](imageCreate.Addr(), int32(width), int32(height))
}

func (i *Image) Destroy() {
	if i != nil {
		imageDestroy.CallRaw(uintptr(unsafe.Pointer(i)))
	}
}

func (i *Image) PixelBuffer() []uint32 {
	if i == nil || i.Pixels == nil {
		return nil
	}
	return unsafe.Slice(i.Pixels, int(i.Width)*int(i.Height))
}

func (i *Image) LoadCursor(display xlib.Display) xlib.Cursor {
	if i == nil {
		return 0
	}
	ret, _, _ := imageLoadCursor.CallRaw(uintptr(display), uintptr(unsafe.Pointer(i)))
	return xlib.Cursor(ret)
}

func LibraryLoadCursor(display xlib.Display, name string) xlib.Cursor {
	cName := cgo.CStringTemp(name)
	ret, _, _ := loadCursor.CallRaw(uintptr(display), uintptr(cName))
	runtime.KeepAlive(cName)
	return xlib.Cursor(ret)
}

func GetTheme(display xlib.Display) string {
	if getTheme.Find() != nil {
		return ""
	}
	ret := cgo.CallRet[cgo.Pointer](getTheme.Addr(), display)
	if ret == nil {
		return ""
	}
	return cgo.GoString(ret)
}

func SetTheme(display xlib.Display, theme string) {
	cTheme := cgo.CStringTemp(theme)
	setTheme.CallRaw(uintptr(display), uintptr(cTheme))
	runtime.KeepAlive(cTheme)
}

func GetDefaultSize(display xlib.Display) int {
	ret, _, _ := getDefaultSize.CallRaw(uintptr(display))
	return int(int32(ret))
}

func SetDefaultSize(display xlib.Display, size int) {
	setDefaultSize.CallRaw(uintptr(display), uintptr(size))
}
