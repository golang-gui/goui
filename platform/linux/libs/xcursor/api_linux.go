package xcursor

import (
	"runtime"

	"github.com/goexlib/cgo"
	"github.com/golang-gui/goui/platform/linux/libs/xlib"
)

var (
	lib            = cgo.NewLazyLibrary("libXcursor.so.1")
	loadCursor     = lib.NewSymbol("XcursorLibraryLoadCursor")
	getTheme       = lib.NewSymbol("XcursorGetTheme")
	setTheme       = lib.NewSymbol("XcursorSetTheme")
	getDefaultSize = lib.NewSymbol("XcursorGetDefaultSize")
	setDefaultSize = lib.NewSymbol("XcursorSetDefaultSize")
)

func Available() bool {
	for _, symbol := range []*cgo.LazySymbol{loadCursor, getTheme, setTheme, getDefaultSize, setDefaultSize} {
		if symbol.Find() != nil {
			return false
		}
	}
	return true
}

func LibraryLoadCursor(display xlib.Display, name string) xlib.Cursor {
	cName := cgo.CStringTemp(name)
	ret, _, _ := loadCursor.CallRaw(uintptr(display), uintptr(cName))
	runtime.KeepAlive(cName)
	return xlib.Cursor(ret)
}

func GetTheme(display xlib.Display) string {
	ret, _, _ := getTheme.CallRaw(uintptr(display))
	if ret == 0 {
		return ""
	}
	return cgo.GoString(cgo.Pointer(ret))
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
