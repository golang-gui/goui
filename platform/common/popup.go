package common

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
	// SetSize sets the popup's logical size.
	SetSize(width, height float32)

	Show() error
	Hide() error

	// RequestPaint asks the platform to schedule a paint notification.
	RequestPaint() error
}
