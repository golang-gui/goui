package x11

import (
	"github.com/goexlib/cgo"
	"github.com/golang-gui/goui/platform/linux/libs/xlib"
	"github.com/golang-gui/goui/platform/linux/libs/xsync"
)

type resizeSyncState uint8

const (
	resizeSyncNone resizeSyncState = iota
	resizeSyncWaitConfigure
	resizeSyncWaitPaint
)

// resizeSync owns the EWMH _NET_WM_SYNC_REQUEST state for one top-level
// window. It deliberately stays in the X11 layer: renderers only need to
// finish presenting before the synchronous PaintEvent callback returns.
type resizeSync struct {
	counter xsync.Counter
	value   xsync.Value
	state   resizeSyncState
}

func (s *resizeSync) initialize(display xlib.Display, window xlib.Window, property xlib.Atom) bool {
	if display == 0 || window == 0 || property == 0 || s.counter != 0 {
		return false
	}

	s.counter = xsync.CreateCounter(display, xsync.Value{})
	if s.counter == 0 {
		return false
	}

	// XChangeProperty's format is 32 bits on the wire, but Xlib expects each
	// format-32 element to occupy a native-long slot in client memory.
	counter := uintptr(s.counter)
	display.ChangeProperty(
		window,
		property,
		xlib.AtomCardinal,
		32,
		xlib.PropModeReplace,
		cgo.Pointer(&counter),
		1,
	)
	return true
}

func (s *resizeSync) request(lo uint32, hi int32) {
	if s.counter == 0 {
		return
	}
	s.value = xsync.Value{Lo: lo, Hi: hi}
	s.state = resizeSyncWaitConfigure
}

// configured pairs the most recently received sync request with its following
// ConfigureNotify, as required by EWMH. It reports whether a paint is needed to
// complete the handshake.
func (s *resizeSync) configured() bool {
	if s.state != resizeSyncWaitConfigure {
		return false
	}
	s.state = resizeSyncWaitPaint
	return true
}

func (s *resizeSync) finishPaint() (xsync.Counter, xsync.Value, bool) {
	if s.state != resizeSyncWaitPaint {
		return 0, xsync.Value{}, false
	}
	value := s.value
	s.value = xsync.Value{}
	s.state = resizeSyncNone
	if s.counter == 0 {
		return 0, xsync.Value{}, false
	}
	return s.counter, value, true
}

func (s *resizeSync) complete(display xlib.Display) {
	counter, value, ok := s.finishPaint()
	if !ok {
		return
	}
	xsync.SetCounter(display, counter, value)
	// Paint tasks run after the native-event drain, so there may be no later X11
	// event processing to flush the acknowledgement promptly.
	display.Flush()
}

func (s *resizeSync) destroy(display xlib.Display) {
	counter := s.counter
	*s = resizeSync{}
	if counter != 0 {
		xsync.DestroyCounter(display, counter)
	}
}
