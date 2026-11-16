package common

import "github.com/golang-gui/goui/core/geometry"

// PopupOptions configures the popup surface, independently of its owner.
type PopupOptions struct {
	Transparent bool
}

// Popup is a borderless, transient surface (menu, dropdown, tooltip, ...) that
// belongs to an owner Window and floats above it. It is thread-affine and must
// be used on the thread that owns the platform.
//
// Positioning is window-local: A popup's position is relative to the owner
// window's content origin (in logical coordinates); the backend converts it
// to native screen coordinates.
type Popup interface {
	// Surface provides NativeHandle and Draw: a Popup is a paint target, so a
	// Painter can be created for it just like for a Window.
	Surface
	// Destroy closes the native popup and releases its resources.
	Destroy()
	// SetPosition sets the popup's top-left relative to the owner window's
	// content origin, in logical coordinates.
	SetPosition(x, y float32)
	// SetSize requests the popup's logical size. The authoritative client size
	// is observed through SizeEvent or ClientSize.
	SetSize(width, height float32)
	// ClientSize queries the actual client extent in DIP and physical pixels,
	// including while hidden. It does not dispatch events. Previously submitted
	// size requests on this thread are reflected in the observation; a missing
	// native surface returns ErrUnavailable. The result is not a requested size.
	ClientSize() (logical, pixels geometry.Size, err error)

	Show() error
	Hide() error
	// Repaint synchronously dispatches a paint notification in the native drawing
	// context of a popup whose native drawing target is available. A hidden
	// target may defer painting until its native backing exists. It is used to replace retained contents on Show,
	// before returning control to the event loop. It must not be called from a
	// paint handler. It does not guarantee compositor presentation or bypass a
	// painter's device-loss/backpressure handling. A destroyed popup returns
	// ErrUnavailable; ordinary updates should use RequestPaint.
	Repaint() error

	// RequestPaint asks the platform to schedule a paint notification.
	RequestPaint() error
}
