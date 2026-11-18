//go:build darwin && !ios

package main

import (
	"fmt"

	"github.com/ebitengine/purego/objc"
	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform"
	"github.com/golang-gui/goui/platform/common"
	. "github.com/golang-gui/goui/platform/darwin/frameworks/appkit"
	. "github.com/golang-gui/goui/platform/darwin/frameworks/core_graphics"
	. "github.com/golang-gui/goui/platform/darwin/frameworks/foundation"
)

func nativeArea(w platform.Window) (area geometry.Rectangle, size geometry.Size, err error) {
	AutoReleasePool(func() {
		n := Cast[NSWindow](objc.ID(w.NativeHandle()))
		c := n.ContentRectForFrameRect(n.Frame())
		scale := CGFloat(1)
		if prefer := common.GetPreferScale(); prefer > 0 {
			scale = CGFloat(prefer) / n.BackingScaleFactor()
		}
		size = geometry.Size{Width: float32(c.Size.Width / scale), Height: float32(c.Size.Height / scale)}
		screens := NSScreenClassId.Screens()
		for i := uintptr(0); i < screens.Count(); i++ {
			s := Cast[NSScreen](screens.ObjectAtIndex(i))
			r := s.Frame()
			x, y := c.Origin.X+c.Size.Width/2, c.Origin.Y+c.Size.Height/2
			if x >= r.Origin.X && x < r.Origin.X+r.Size.Width && y >= r.Origin.Y && y < r.Origin.Y+r.Size.Height {
				area = geometry.Rect(float32((r.Origin.X-c.Origin.X)/scale), float32((c.Origin.Y+c.Size.Height-r.Origin.Y-r.Size.Height)/scale), float32(r.Size.Width/scale), float32(r.Size.Height/scale))
				// 全屏内容由 AppKit 决定，包括有系统安全区域的显示器。
				if n.StyleMask()&NSWindowStyleMaskFullScreen != 0 {
					area = area.Intersect(geometry.Rect(0, 0, size.Width, size.Height))
				}
				return
			}
		}
		err = fmt.Errorf("客户区不在有效显示器中")
	})
	return
}
