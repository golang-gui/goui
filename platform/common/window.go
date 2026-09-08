package common

import (
	"fmt"
	"math"

	"github.com/golang-gui/goui/core/geometry"
)

// Window is thread-affine. All methods must be called on the thread that owns
// the platform.
type Window interface {
	// Surface provides NativeHandle and Draw: a Window is a paint target.
	Surface
	// Destroy closes the native window and releases its resources without
	// sending a close request.
	Destroy()
	Parent() Window
	SetParent(parent Window) error
	Title() string
	SetTitle(title string) error
	Show() error
	// Hide unmaps the native window without destroying it, so it can be shown
	// again later.
	Hide() error
	// RequestClose sends a close request notification. It does not destroy the
	// window; the event handler decides whether to call Destroy.
	RequestClose() error
	// RequestPaint asks the platform to schedule a paint notification. It does
	// not draw immediately, and multiple requests may be coalesced.
	RequestPaint() error
}

// DesktopWindow extends the window host with desktop window-manager facilities.
// An implementation may reject individual requests with ErrUnsupported. Mobile
// hosts need not implement this interface. All calls are thread-affine.
type DesktopWindow interface {
	Window

	// SetMinSize sets the window-manager minimum size hint in logical (DIP)
	// units. A value of (0, 0) clears the hint (no minimum). The hint is
	// advisory: the window manager may ignore it.
	SetMinSize(width, height float32)

	// Chrome reports observable native decoration, or WindowChromeUnknown when
	// it cannot be established. It does not echo the creation preference.
	Chrome() WindowChrome
	// ControlsRect reports the union of visible native caption buttons in
	// window-client DIP, not screen or outer-frame coordinates. It may lie
	// outside the client area with Native chrome. Empty means an observed absence
	// of visible buttons; ErrUnsupported means no such facility, ErrUnavailable
	// means it cannot be observed now. It never positions or owns GUI controls.
	// Query during layout and after existing size/state events; there is no
	// dedicated controls-geometry notification.
	ControlsRect() (geometry.Rectangle, error)

	// State reports one observed presentation state, not a combination of native
	// flags or the last requested state. Unavailable observations are Unknown.
	State() WindowState
	// RequestState requests a presentation change. Normal means an ordinary
	// window, not restoration of a previous mode. Success means submitted, not
	// completed; the native window manager can reject or delay the request.
	RequestState(state WindowState) error

	// SetHitTest replaces the synchronous native-region query. A nil callback
	// restores platform defaults. p is in window-client DIP and may be outside
	// the client bounds. Default defers to native handling; Client explicitly
	// keeps a customizable region in the client input path.
	//
	// The callback runs on the window thread and may be called frequently or
	// reentrantly by native code. It must only read completed layout/state: no
	// layout, drawing, event dispatch, window mutation or destruction. The window
	// releases it on Destroy. Native controls remain native-owned. Supported
	// custom frame roles differ by backend; this is not an operation-permission
	// API and does not block system-menu, keyboard or window-manager commands.
	SetHitTest(f func(p geometry.Point) WindowHit) error
}

// WindowState is the current presentation, not native restoration bookkeeping.
// Hidden is a withdrawn/hidden window, not an occluded window or an inactive
// workspace. Minimized remains distinct even when no window pixels are visible.
type WindowState int

// WindowStateUnknown is an observation only, never a request target.
const WindowStateUnknown WindowState = -1

const (
	WindowStateNormal WindowState = iota
	WindowStateHidden
	WindowStateMinimized
	WindowStateMaximized
	WindowStateFullscreen
)

// WindowChrome is a native decoration mode, not a GUI title-bar or button policy.
// In WindowOptions it is a preference; DesktopWindow.Chrome reports observations.
type WindowChrome uint8

const (
	WindowChromeNative WindowChrome = iota
	WindowChromeIntegrated
	WindowChromeNone
)

// WindowChromeUnknown is an observation only, not a creation option.
const WindowChromeUnknown WindowChrome = 255

// Validate rejects invalid creation values without allocating native resources.
func (o WindowOptions) Validate() error {
	if o.Chrome > WindowChromeNone {
		return fmt.Errorf("invalid window chrome: %d", o.Chrome)
	}
	return nil
}

// ValidateWindowSize rejects invalid initial DIP sizes before native allocation.
func ValidateWindowSize(size geometry.Size) error {
	for _, value := range []float32{size.Width, size.Height} {
		if value <= 0 || math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) {
			return fmt.Errorf("invalid initial window size: %v", size)
		}
	}
	return nil
}

// WindowOptions must be supplied before the native window is created.
//
// Chrome cannot be changed on an existing window.
type WindowOptions struct {
	Chrome WindowChrome
}

// WindowHit identifies a native frame role, not a Widget action or pointer state.
// Its values are independent of native platform constants. A backend without a
// native caption-button role delivers that region through ordinary client input;
// it never implements GUI button behavior on behalf of the callback.
type WindowHit uint8

const (
	WindowHitDefault WindowHit = iota
	WindowHitClient
	WindowHitSysMenu
	WindowHitCaption
	WindowHitMinimize
	WindowHitMaximize
	WindowHitClose
	WindowHitTop
	WindowHitBottom
	WindowHitLeft
	WindowHitRight
	WindowHitTopLeft
	WindowHitTopRight
	WindowHitBottomLeft
	WindowHitBottomRight
)
