package events

import "github.com/golang-gui/goui/platform/common"

// SurfaceEvent reports a replaceable drawing target becoming available or lost.
// It does not unmount the logical window. Terminal destruction uses DestroyEvent.
type SurfaceEvent struct{ State common.SurfaceState }

func (SurfaceEvent) Type() EventType { return Surface }
func (SurfaceEvent) isEvent()        {}

// VisibilityEvent reports whether the native content is visible. Backgrounding
// is distinct from desktop minimization, focus, and drawing-target availability.
type VisibilityEvent struct{ Visible bool }

func (VisibilityEvent) Type() EventType { return Visibility }
func (VisibilityEvent) isEvent()        {}

// StateEvent reports presentation observed in response to native events.
// Requests never synthesize completion. Unknown means observation is unavailable.
type StateEvent struct {
	State common.WindowState
}

func (e StateEvent) Type() EventType { return State }
func (e StateEvent) isEvent()        {}

type CloseEvent struct{}

func (e CloseEvent) Type() EventType {
	return CloseRequest
}

func (e CloseEvent) isEvent() {}

// DestroyEvent reports the end of a native host's usable lifetime. It cannot
// be cancelled. Owned native objects may remain alive until cleanup completes.
// Delivery is at most once, on the owning thread, after NativeHandle becomes
// zero. Hide, minimization and occlusion do not emit this event.
type DestroyEvent struct{}

func (DestroyEvent) Type() EventType { return Destroy }
func (DestroyEvent) isEvent()        {}

type SizeEvent struct {
	// Width, Height are the logical (DIP) size used for layout and hit-testing.
	Width  float32
	Height float32
	// PixelWidth, PixelHeight are the exact physical (backing) pixel size used
	// for the painter surface. scale = PixelWidth / Width.
	PixelWidth  float32
	PixelHeight float32
}

func (e SizeEvent) Type() EventType {
	return Size
}

func (e SizeEvent) isEvent() {}

type PaintEvent struct{}

func (e PaintEvent) Type() EventType {
	return Paint
}

func (e PaintEvent) isEvent() {}

type FocusEvent struct {
	Focused bool
}

func (e FocusEvent) Type() EventType {
	return Focus
}

func (e FocusEvent) isEvent() {}

var (
	_ Event = CloseEvent{}
	_ Event = DestroyEvent{}
	_ Event = SizeEvent{}
	_ Event = PaintEvent{}
	_ Event = FocusEvent{}
	_ Event = StateEvent{}
)
