package common

import "image"

// Window is thread-affine. All methods must be called on the thread that owns
// the platform.
type Window interface {
	NativeHandle() uintptr
	// Destroy closes the native window and releases its resources without
	// sending a close request.
	Destroy()
	Parent() Window
	SetParent(parent Window) error
	Title() string
	SetTitle(title string) error
	Show() error
	// RequestClose sends a close request notification. It does not destroy the
	// window; the event handler decides whether to call Destroy.
	RequestClose() error
	Draw(img image.Image) error
	ScaleFactor() (float64, error)
}
