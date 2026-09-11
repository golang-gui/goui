package xrender

import (
	"github.com/goexlib/cgo"
	"github.com/golang-gui/goui/platform/linux/libs/xlib"
)

var (
	lib              = cgo.NewLazyLibrary("libXrender.so.1")
	findVisualFormat = lib.NewSymbol("XRenderFindVisualFormat")
)

// FindVisualFormat returns an Xlib-owned format, valid for the display lifetime.
func FindVisualFormat(display xlib.Display, visual *xlib.Visual) *PictFormat {
	if err := findVisualFormat.Find(); err != nil {
		return nil
	}
	ret, _, _ := findVisualFormat.CallRaw(uintptr(display), uintptr(cgo.Pointer(visual)))
	return (*PictFormat)(cgo.Pointer(ret))
}
