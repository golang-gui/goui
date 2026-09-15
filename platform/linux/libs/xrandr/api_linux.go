package xrandr

import (
	"runtime"

	"github.com/goexlib/cgo"
	"github.com/golang-gui/goui/platform/linux/libs/xlib"
)

var (
	lib          = cgo.NewLazyLibrary("libXrandr.so.2")
	queryVersion = lib.NewSymbol("XRRQueryVersion")
	getMonitors  = lib.NewSymbol("XRRGetMonitors")
	freeMonitors = lib.NewSymbol("XRRFreeMonitors")
)

// MonitorInfo mirrors XRRMonitorInfo (RandR 1.5).
type MonitorInfo struct {
	Name                xlib.Atom
	Primary, Automatic  int32
	NOutput             int32
	X, Y, Width, Height int32
	MWidth, MHeight     int32
	Outputs             *xlib.ID
}

func QueryVersion(d xlib.Display) (major, minor int32, ok bool) {
	if queryVersion.Find() != nil || getMonitors.Find() != nil || freeMonitors.Find() != nil {
		return
	}
	var pin runtime.Pinner
	pin.Pin(&major)
	pin.Pin(&minor)
	defer pin.Unpin()
	ret, _, _ := queryVersion.CallRaw(uintptr(d), uintptr(cgo.Pointer(&major)), uintptr(cgo.Pointer(&minor)))
	ok = ret != 0
	return
}

// GetMonitors returns an Xlib allocation; the caller must call FreeMonitors.
func GetMonitors(d xlib.Display, window xlib.Window, active bool) (*MonitorInfo, int32) {
	var count int32
	var pin runtime.Pinner
	pin.Pin(&count)
	defer pin.Unpin()
	ret, _, _ := getMonitors.CallRaw(uintptr(d), uintptr(window), uintptr(cgo.CBool(active)), uintptr(cgo.Pointer(&count)))
	return (*MonitorInfo)(cgo.Pointer(ret)), count
}

func FreeMonitors(monitors *MonitorInfo) { freeMonitors.CallRaw(uintptr(cgo.Pointer(monitors))) }
