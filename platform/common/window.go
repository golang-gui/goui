package common

import "image"

// Window is thread-affine. All methods must be called on the thread that owns
// the platform.
type Window interface {
	NativeHandle() uintptr
	Destroy()
	Parent() Window
	SetParent(parent Window) error
	Title() string
	SetTitle(title string) error
	Show() error
	Close() error
	Draw(img image.Image) error
	ScaleFactor() (float64, error)
}
