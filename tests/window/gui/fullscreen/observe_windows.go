package main

import (
	"fmt"
	"unsafe"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform"
	"github.com/golang-gui/goui/platform/common"
	"github.com/golang-gui/goui/platform/windows/sdk/winapi"
)

func nativeArea(w platform.Window) (geometry.Rectangle, geometry.Size, error) {
	h := winapi.HWND(w.NativeHandle())
	var p winapi.POINT
	var r winapi.RECT
	if winapi.ClientToScreen(h, &p) == 0 {
		return geometry.Rectangle{}, geometry.Size{}, fmt.Errorf("无法查询客户区原点")
	}
	if e := winapi.GetClientRect(h, &r); e != nil {
		return geometry.Rectangle{}, geometry.Size{}, e
	}
	dpi, e := winapi.GetDpiForWindow(h)
	if e != nil {
		return geometry.Rectangle{}, geometry.Size{}, e
	}
	scale := float32(dpi) / 96
	if prefer := common.GetPreferScale(); prefer > 0 {
		scale = prefer
	}
	info := winapi.MONITORINFO{Size: winapi.DWORD(unsafe.Sizeof(winapi.MONITORINFO{}))}
	if e := winapi.GetMonitorInfo(winapi.MonitorFromWindow(h, winapi.MONITOR_DEFAULTTONEAREST), &info); e != nil {
		return geometry.Rectangle{}, geometry.Size{}, e
	}
	m := info.Monitor
	return geometry.Rect(float32(m.Left-p.X)/scale, float32(m.Top-p.Y)/scale, float32(m.Right-m.Left)/scale, float32(m.Bottom-m.Top)/scale), geometry.Size{Width: float32(r.Right-r.Left) / scale, Height: float32(r.Bottom-r.Top) / scale}, nil
}
