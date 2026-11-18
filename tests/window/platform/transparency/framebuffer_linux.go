package main

import (
	"errors"
	"fmt"
	"unsafe"

	"github.com/goexlib/cgo"
	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/platform/graphics/opengl"
	"github.com/golang-gui/goui/platform/linux/libs/glx"
	"github.com/golang-gui/goui/platform/linux/libs/xlib"
	"github.com/golang-gui/goui/platform/linux/libs/xrender"
)

func checkTransparentGLFramebuffer(plat platform.Platform) {
	var testErr error
	func() {
		owner, err := plat.NewWindow(geometry.Size{Width: 64, Height: 64}, func(platform.Event) {}, platform.WindowOptions{})
		if err != nil {
			testErr = err
			return
		}
		defer owner.Destroy()
		popup, err := plat.NewPopup(owner, geometry.Size{Width: 32, Height: 32}, func(platform.Event) {}, platform.PopupOptions{Transparent: true})
		if err != nil {
			testErr = err
			return
		}
		defer popup.Destroy()
		painter, err := opengl.NewPainter(popup)
		if err != nil {
			testErr = err
			return
		}
		defer painter.Destroy()
		if err := popup.Show(); err != nil {
			testErr = err
			return
		}
		display := xlib.OpenDisplay("")
		if display == 0 {
			testErr = fmt.Errorf("open display for visual query")
			return
		}
		defer display.Close()
		var attrs xlib.WindowAttributes
		if display.GetWindowAttributes(xlib.Window(popup.NativeHandle()), &attrs) == 0 {
			testErr = fmt.Errorf("query alpha window")
			return
		}
		f := xrender.FindVisualFormat(display, attrs.Visual)
		if attrs.Depth != 32 || f == nil || f.Direct.AlphaMask != 255 {
			testErr = fmt.Errorf("surface does not have an ARGB visual")
			return
		}
		addr, err := glx.GetProcAddress("glReadPixels")
		if err != nil {
			testErr = err
			return
		}
		// 输入及读回均为预乘 RGBA；Clear 直接写 framebuffer，不替输入预乘。
		for _, color := range []graphics.Color{{R: .5, G: .25, A: .5}, {}} {
			painter.Begin(32, 32, 1)
			painter.Clear(color)

			var rgba [4]byte
			cgo.Symbol(addr).CallRaw(0, 0, 1, 1, 0x1908, 0x1401, uintptr(unsafe.Pointer(&rgba)))
			painter.End()
			if color.A == 0 {
				if rgba != [4]byte{} {
					testErr = fmt.Errorf("clear retained pixels: %v", rgba)
					return
				}
			} else if rgba[0] < 127 || rgba[0] > 128 || rgba[1] < 63 || rgba[1] > 64 || rgba[2] != 0 || rgba[3] < 127 || rgba[3] > 128 {
				testErr = fmt.Errorf("premultiplied framebuffer RGBA = %v", rgba)
				return
			}
		}
	}()
	if errors.Is(testErr, platform.ErrUnsupported) || errors.Is(testErr, platform.ErrUnavailable) {
		panic(testErr)
	}
	if testErr != nil {
		panic(testErr)
	}
}

func checkNativeAlpha(plat platform.Platform) { checkTransparentGLFramebuffer(plat) }
