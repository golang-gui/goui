package xrender

import "github.com/golang-gui/goui/platform/linux/libs/xlib"

type DirectFormat struct {
	Red, RedMask, Green, GreenMask, Blue, BlueMask, Alpha, AlphaMask int16
}

type PictFormat struct {
	ID          xlib.ID
	Type, Depth int32
	Direct      DirectFormat
	Colormap    xlib.Colormap
}
