//go:build linux && !android

package main

import (
	"fmt"
	"unsafe"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform"
	"github.com/golang-gui/goui/platform/common"
	"github.com/golang-gui/goui/platform/linux/libs/xlib"
	"github.com/golang-gui/goui/platform/linux/libs/xrandr"
)

func nativeArea(w platform.Window) (geometry.Rectangle, geometry.Size, error) {
	d := xlib.OpenDisplay("")
	if d == 0 {
		return geometry.Rectangle{}, geometry.Size{}, fmt.Errorf("无法查询原生显示")
	}
	defer d.Close()
	root, wid := d.DefaultRootWindow(), xlib.Window(w.NativeHandle())
	x, y, ok := d.TranslateCoordinatesChecked(wid, root, 0, 0)
	var a xlib.WindowAttributes
	if !ok || d.GetWindowAttributes(wid, &a) == 0 {
		return geometry.Rectangle{}, geometry.Size{}, fmt.Errorf("无法查询客户区")
	}
	scale := common.GetPreferScale()
	if paintedScale > 0 {
		scale = paintedScale
	}
	if scale <= 0 {
		scale = 1
	}
	size := geometry.Size{Width: float32(a.Width) / scale, Height: float32(a.Height) / scale}
	m, n := xrandr.GetMonitors(d, root, true)
	if m == nil {
		return geometry.Rectangle{}, size, fmt.Errorf("无法查询显示器")
	}
	defer xrandr.FreeMonitors(m)
	for _, r := range unsafe.Slice(m, int(n)) {
		cx, cy := x+int(a.Width)/2, y+int(a.Height)/2
		if cx >= int(r.X) && cx < int(r.X+r.Width) && cy >= int(r.Y) && cy < int(r.Y+r.Height) {
			return geometry.Rect(float32(int(r.X)-x)/scale, float32(int(r.Y)-y)/scale, float32(r.Width)/scale, float32(r.Height)/scale), size, nil
		}
	}
	return geometry.Rectangle{}, size, fmt.Errorf("客户区不在有效显示器中")
}
