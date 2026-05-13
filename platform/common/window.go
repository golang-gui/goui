package common

import "image"

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
