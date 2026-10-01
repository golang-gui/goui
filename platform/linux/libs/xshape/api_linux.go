package xshape

import (
	"runtime"

	"github.com/goexlib/cgo"
	"github.com/golang-gui/goui/platform/linux/libs/xlib"
)

var (
	lib               = cgo.NewLazyLibrary("libXext.so.6")
	queryVersion      = lib.NewSymbol("XShapeQueryVersion")
	combineRectangles = lib.NewSymbol("XShapeCombineRectangles")
)

const (
	Bounding = 0
	Input    = 2
	Set      = 0
	Union    = 1
	Unsorted = 0
)

// Rectangle mirrors XRectangle, in native pixels.
type Rectangle struct {
	X, Y          int16
	Width, Height uint16
}

func QueryVersion(display xlib.Display) (major, minor int, ok bool) {
	if queryVersion.Find() != nil || combineRectangles.Find() != nil {
		return 0, 0, false
	}
	var a, b int32
	r, _, _ := queryVersion.CallRaw(uintptr(display), uintptr(cgo.Pointer(&a)), uintptr(cgo.Pointer(&b)))
	return int(a), int(b), r != 0
}

func CombineRectangles(display xlib.Display, window xlib.Window, kind, x, y int, rectangles []Rectangle, operation, ordering int) {
	combineRectangles.CallRaw(uintptr(display), uintptr(window), uintptr(kind), uintptr(x), uintptr(y),
		uintptr(cgo.CSlice(rectangles)), uintptr(len(rectangles)), uintptr(operation), uintptr(ordering))
	runtime.KeepAlive(rectangles)
}
