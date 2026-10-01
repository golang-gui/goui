package xsync

import (
	"errors"

	"github.com/golang-gui/goui/platform/linux/libs/xlib"

	"github.com/goexlib/cgo"
)

var (
	libXext = cgo.NewLazyLibrary("libXext.so.6")

	xSyncQueryExtension = libXext.NewSymbol("XSyncQueryExtension")
	xSyncInitialize     = libXext.NewSymbol("XSyncInitialize")
	xSyncCreateCounter  = libXext.NewSymbol("XSyncCreateCounter")
	xSyncSetCounter     = libXext.NewSymbol("XSyncSetCounter")
	xSyncDestroyCounter = libXext.NewSymbol("XSyncDestroyCounter")
)

// Counter identifies an X Sync extension counter.
type Counter uintptr

// Value mirrors XSyncValue. The Xlib ABI passes this eight-byte value by
// value, with the signed high word stored before the unsigned low word.
type Value struct {
	Hi int32
	Lo uint32
}

// Initialize verifies that the server and libXext both provide the Sync
// extension. Callers may safely omit resize synchronization when it fails.
func Initialize(display xlib.Display) error {
	if err := xSyncQueryExtension.Find(); err != nil {
		return err
	}
	if err := xSyncInitialize.Find(); err != nil {
		return err
	}

	var eventBase, errorBase int32
	available, _, _ := xSyncQueryExtension.CallRaw(
		uintptr(display),
		uintptr(cgo.Pointer(&eventBase)),
		uintptr(cgo.Pointer(&errorBase)),
	)
	if available == 0 {
		return errors.New("X Sync extension is unavailable")
	}

	var major, minor int32
	initialized, _, _ := xSyncInitialize.CallRaw(
		uintptr(display),
		uintptr(cgo.Pointer(&major)),
		uintptr(cgo.Pointer(&minor)),
	)
	if initialized == 0 {
		return errors.New("initialize X Sync extension failed")
	}
	return nil
}

func CreateCounter(display xlib.Display, initial Value) Counter {
	return cgo.CallRet[Counter](xSyncCreateCounter.Addr(), display, initial)
}

func SetCounter(display xlib.Display, counter Counter, value Value) {
	cgo.Call(xSyncSetCounter.Addr(), display, counter, value)
}

func DestroyCounter(display xlib.Display, counter Counter) {
	if counter != 0 {
		xSyncDestroyCounter.CallRaw(uintptr(display), uintptr(counter))
	}
}
