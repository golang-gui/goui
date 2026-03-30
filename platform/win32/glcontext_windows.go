package win32

import (
	"errors"
	"github.com/golang-gui/goui/platform/common"
	"github.com/golang-gui/goui/platform/win32/wgl"
	"github.com/golang-gui/goui/platform/win32/winapi"
)

type GlContext struct {
	hdc winapi.HDC
	hrc wgl.HGLRC
}

func NewGlContext(win common.Window, share common.GlContext) (c GlContext, err error) {
	hdc, err := winapi.GetDC(winapi.HWND(win.NativeHandle()))
	if err != nil {
		return
	}
	return newGlContext(hdc, share)
}

func newGlContext(hdc winapi.HDC, share common.GlContext) (c GlContext, err error) {
	var shareRc wgl.HGLRC
	if shareCtx, ok := share.(GlContext); ok {
		shareRc = shareCtx.hrc
	}

	pixelFormat, err := choosePixelFormat(hdc)
	if err != nil {
		return
	}

	var pfd winapi.PIXELFORMATDESCRIPTOR
	_, err = winapi.DescribePixelFormat(hdc, pixelFormat, winapi.Sizeof_PIXELFORMATDESCRIPTOR, &pfd)
	if err != nil {
		return
	}

	err = winapi.SetPixelFormat(hdc, pixelFormat, &pfd)
	if err != nil {
		return
	}

	//mask := wgl.WGL_CONTEXT_ES2_PROFILE_BIT_EXT
	mask := wgl.WGL_CONTEXT_CORE_PROFILE_BIT_ARB
	//flags := wgl.WGL_CONTEXT_DEBUG_BIT_ARB
	if wgl.CreateContextAttribsARB != nil {
		c.hdc = hdc
		c.hrc, err = wgl.CreateContextAttribsARB(hdc, shareRc, []int32{wgl.WGL_CONTEXT_PROFILE_MASK_ARB, int32(mask), 0, 0})
		return
	}

	err = errors.New("can not create wgl context")
	return
}

func (c GlContext) Name() string {
	return "WGL"
}

func (c GlContext) Destroy() {
	if c.hrc != 0 {
		wgl.DeleteContext(c.hrc)
	}
	if c.hdc != 0 {
		winapi.ReleaseDC(c.hdc)
	}
}

func (c GlContext) MakeCurrent() error {
	return wgl.MakeCurrent(c.hdc, c.hrc)
}

func (c GlContext) ClearCurrent() error {
	return wgl.MakeCurrent(0, 0)
}

func (c GlContext) SwapBuffers() error {
	return winapi.SwapBuffers(c.hdc)
}

func (c GlContext) SwapInterval(v int) error {
	wgl.SwapIntervalEXT(v)
	return nil
}

func choosePixelFormat(hdc winapi.HDC) (pixelFormat winapi.INT, err error) {
	var (
		attrs      [40]int32
		values     [40]int32
		attrsCount int
		addAttr    = func(attr int32) {
			attrs[attrsCount] = attr
			attrsCount++
		}
		findAttrValue = func(attr int32) int32 {
			for i := 0; i < attrsCount; i++ {
				if attrs[i] == attr {
					return values[i]
				}
			}
			return 0
		}
	)

	addAttr(wgl.WGL_SUPPORT_OPENGL_ARB)
	addAttr(wgl.WGL_DRAW_TO_WINDOW_ARB)
	addAttr(wgl.WGL_PIXEL_TYPE_ARB)
	addAttr(wgl.WGL_ACCELERATION_ARB)
	addAttr(wgl.WGL_RED_BITS_ARB)
	addAttr(wgl.WGL_RED_SHIFT_ARB)
	addAttr(wgl.WGL_GREEN_BITS_ARB)
	addAttr(wgl.WGL_GREEN_SHIFT_ARB)
	addAttr(wgl.WGL_BLUE_BITS_ARB)
	addAttr(wgl.WGL_BLUE_SHIFT_ARB)
	addAttr(wgl.WGL_ALPHA_BITS_ARB)
	addAttr(wgl.WGL_ALPHA_SHIFT_ARB)
	addAttr(wgl.WGL_DEPTH_BITS_ARB)
	addAttr(wgl.WGL_STENCIL_BITS_ARB)
	addAttr(wgl.WGL_ACCUM_BITS_ARB)
	addAttr(wgl.WGL_ACCUM_RED_BITS_ARB)
	addAttr(wgl.WGL_ACCUM_GREEN_BITS_ARB)
	addAttr(wgl.WGL_ACCUM_BLUE_BITS_ARB)
	addAttr(wgl.WGL_ACCUM_ALPHA_BITS_ARB)
	addAttr(wgl.WGL_AUX_BUFFERS_ARB)
	addAttr(wgl.WGL_STEREO_ARB)
	addAttr(wgl.WGL_DOUBLE_BUFFER_ARB)

	if wgl.GetPixelFormatAttribivARB != nil {
		err = wgl.GetPixelFormatAttribivARB(hdc, 1, 0, []int32{wgl.WGL_NUMBER_PIXEL_FORMATS_ARB}, values[:])
		if err != nil {
			return
		}

		pixelFormatCount := values[0]
		for i := int32(0); i < pixelFormatCount; i++ {
			pixelFormat = i + 1
			err = wgl.GetPixelFormatAttribivARB(hdc, pixelFormat, 0, attrs[:attrsCount], values[:])
			if err != nil {
				return
			}

			if findAttrValue(wgl.WGL_SUPPORT_OPENGL_ARB) == 0 ||
				findAttrValue(wgl.WGL_DRAW_TO_WINDOW_ARB) == 0 {
				continue
			}

			if findAttrValue(wgl.WGL_PIXEL_TYPE_ARB) != wgl.WGL_TYPE_RGBA_ARB {
				continue
			}

			if findAttrValue(wgl.WGL_ACCELERATION_ARB) == wgl.WGL_NO_ACCELERATION_ARB {
				continue
			}

			if findAttrValue(wgl.WGL_DOUBLE_BUFFER_ARB) != 1 {
				continue
			}

			redBits := findAttrValue(wgl.WGL_RED_BITS_ARB)
			greenBits := findAttrValue(wgl.WGL_GREEN_BITS_ARB)
			blueBits := findAttrValue(wgl.WGL_BLUE_BITS_ARB)
			alphaBits := findAttrValue(wgl.WGL_ALPHA_BITS_ARB)
			depthBits := findAttrValue(wgl.WGL_DEPTH_BITS_ARB)
			stencilBits := findAttrValue(wgl.WGL_STENCIL_BITS_ARB)
			//accumRedBits := findAttrValue(wgl.WGL_ACCUM_RED_BITS_ARB)
			//accumGreenBits := findAttrValue(wgl.WGL_ACCUM_GREEN_BITS_ARB)
			//accumBlueBits := findAttrValue(wgl.WGL_ACCUM_BLUE_BITS_ARB)
			//accumAlphaBits := findAttrValue(wgl.WGL_ACCUM_ALPHA_BITS_ARB)
			//auxBuffers := findAttrValue(wgl.WGL_AUX_BUFFERS_ARB)
			if redBits == 8 && greenBits == 8 && blueBits == 8 && alphaBits == 8 && stencilBits == 8 && depthBits == 24 {
				return
			}
		}
	}
	return 0, errors.New("can not choose useful pixel format")
}
