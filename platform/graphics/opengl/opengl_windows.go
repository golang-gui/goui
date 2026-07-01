package opengl

import (
	"errors"
	"strings"
	"syscall"

	"github.com/golang-gui/goui/platform/windows/sdk/wgl"
	"github.com/golang-gui/goui/platform/windows/sdk/winapi"
)

type wglContext struct {
	hwnd winapi.HWND
	hdc  winapi.HDC
	hrc  wgl.HGLRC
}

func newContext(win NativeWindow, share Context, config Config) (_ Context, err error) {
	err = platform.Init()
	if err != nil {
		return
	}

	hwnd := winapi.HWND(win.NativeHandle())
	hdc, err := winapi.GetDC(hwnd)
	if err != nil {
		return
	}

	var shareRc wgl.HGLRC
	if shareCtx, ok := share.(wglContext); ok {
		shareRc = shareCtx.hrc
	}

	pixelFormat, err := platform.choosePixelFormat(hdc, config.PixelFormat)
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

	// TODO: support OpenGL ES?
	if wgl.CreateContextAttribsARB != nil {
		attrs := []int32{
			wgl.WGL_CONTEXT_PROFILE_MASK_ARB, wgl.WGL_CONTEXT_CORE_PROFILE_BIT_ARB,
			0, 0,
		}

		hrc, err := wgl.CreateContextAttribsARB(hdc, shareRc, attrs)
		if err != nil {
			return nil, err
		}

		return wglContext{
			hwnd: hwnd,
			hdc:  hdc,
			hrc:  hrc,
		}, nil
	}

	return nil, errors.New("can not create wgl context")
}

func (c wglContext) Name() string {
	return "WGL"
}

func (c wglContext) Destroy() {
	if c.hrc != 0 {
		wgl.DeleteContext(c.hrc)
	}
	if c.hdc != 0 {
		winapi.ReleaseDC(c.hwnd, c.hdc)
	}
}

func (c wglContext) MakeCurrent() error {
	return wgl.MakeCurrent(c.hdc, c.hrc)
}

func (c wglContext) ClearCurrent() error {
	return wgl.MakeCurrent(0, 0)
}

func (c wglContext) SwapBuffers() error {
	return winapi.SwapBuffers(c.hdc)
}

func (c wglContext) SwapInterval(v int) error {
	wgl.SwapIntervalEXT(v)
	return nil
}

func (c wglContext) GetProcAddress(name string) (proc uintptr, err error) {
	return wgl.GetProcAddress(name)
}

func (c wglContext) GetExtensions() string {
	return platform.Extensions
}

type wglPlatform struct {
	helperWindow                   winapi.HWND
	Extensions                     string
	EXT_swap_control               bool
	EXT_colorspace                 bool
	ARB_multisample                bool
	ARB_framebuffer_sRGB           bool
	EXT_framebuffer_sRGB           bool
	ARB_pixel_format               bool
	ARB_create_context             bool
	ARB_create_context_profile     bool
	EXT_create_context_es2_profile bool
	ARB_create_context_robustness  bool
	ARB_create_context_no_error    bool
	ARB_context_flush_control      bool
	init                           bool
}

var platform wglPlatform

func (p *wglPlatform) Init() (err error) {
	if p.init {
		return nil
	}

	err = p.createHelperWindow()
	if err != nil {
		return
	}

	hdc, err := winapi.GetDC(p.helperWindow)
	if err != nil {
		return
	}

	err = wgl.Init(hdc)
	if err != nil {
		return
	}

	if wgl.GetExtensionsStringARB != nil {
		p.Extensions, _ = wgl.GetExtensionsStringARB(hdc)
	} else if wgl.GetExtensionsStringEXT != nil {
		p.Extensions, _ = wgl.GetExtensionsStringEXT()
	}
	p.loadExtensions()

	// TODO: create base share glContext?

	p.init = true
	return nil
}

func (p *wglPlatform) createHelperWindow() (err error) {
	instance, err := winapi.GetModuleHandle(nil)
	if err != nil {
		return
	}

	cls, _ := syscall.UTF16PtrFromString("WGL_Helper")
	wdc := winapi.WNDCLASSEX{
		Size:      winapi.Sizeof_WNDCLASSEX,
		Style:     winapi.CS_OWNDC,
		WndProc:   winapi.GetDefWindowProc(),
		Instance:  instance,
		ClassName: cls,
	}
	_, err = winapi.RegisterClassEx(&wdc)
	if err != nil {
		return
	}

	p.helperWindow, err = winapi.CreateWindowEx(winapi.WS_EX_OVERLAPPEDWINDOW, cls, cls,
		winapi.WS_CLIPSIBLINGS|winapi.WS_CLIPCHILDREN,
		0, 0, 1, 1, 0, 0,
		instance, nil)

	if err != nil {
		return
	}

	winapi.ShowWindow(p.helperWindow, winapi.SW_HIDE)

	var msg winapi.MSG
	for {
		has, _ := winapi.PeekMessage(&msg, p.helperWindow, 0, 0, winapi.PM_REMOVE)
		if has == winapi.FALSE {
			break
		}
		winapi.TranslateMessage(&msg)
		winapi.DispatchMessage(&msg)
	}

	return nil
}

func (p *wglPlatform) choosePixelFormat(hdc winapi.HDC, format PixelFormat) (pixelFormat winapi.INT, err error) {
	var (
		attrs      [40]int32
		values     [40]int32
		attrsCount int
		addAttr    = func(attr int32) {
			attrs[attrsCount] = attr
			attrsCount++
		}
		findAttrValue = func(attr int32) int {
			for i := 0; i < attrsCount; i++ {
				if attrs[i] == attr {
					return int(values[i])
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
	if p.ARB_multisample {
		addAttr(wgl.WGL_SAMPLES_ARB)
	}

	// TODO: support OpenGL ES
	if p.ARB_framebuffer_sRGB || p.EXT_framebuffer_sRGB {
		addAttr(wgl.WGL_FRAMEBUFFER_SRGB_CAPABLE_ARB)
	}

	if wgl.GetPixelFormatAttribivARB != nil {
		// Get pixel format attributes through "modern" extension

		err = wgl.GetPixelFormatAttribivARB(hdc, 1, 0, []int32{wgl.WGL_NUMBER_PIXEL_FORMATS_ARB}, values[:])
		if err != nil {
			return
		}

		pixelFormatCount := values[0]
		usableConfigs := make([]FBConfig, 0, pixelFormatCount)

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

			if doubleBuffer := findAttrValue(wgl.WGL_DOUBLE_BUFFER_ARB) != 0; doubleBuffer != format.DoubleBuffer {
				continue
			}

			var u FBConfig
			u.Handle = uintptr(pixelFormat)

			u.RedBits = findAttrValue(wgl.WGL_RED_BITS_ARB)
			u.GreenBits = findAttrValue(wgl.WGL_GREEN_BITS_ARB)
			u.BlueBits = findAttrValue(wgl.WGL_BLUE_BITS_ARB)
			u.AlphaBits = findAttrValue(wgl.WGL_ALPHA_BITS_ARB)

			u.DepthBits = findAttrValue(wgl.WGL_DEPTH_BITS_ARB)
			u.StencilBits = findAttrValue(wgl.WGL_STENCIL_BITS_ARB)

			u.AccumRedBits = findAttrValue(wgl.WGL_ACCUM_RED_BITS_ARB)
			u.AccumGreenBits = findAttrValue(wgl.WGL_ACCUM_GREEN_BITS_ARB)
			u.AccumBlueBits = findAttrValue(wgl.WGL_ACCUM_BLUE_BITS_ARB)
			u.AccumAlphaBits = findAttrValue(wgl.WGL_ACCUM_ALPHA_BITS_ARB)

			u.AuxBuffers = findAttrValue(wgl.WGL_AUX_BUFFERS_ARB)

			u.Stereo = findAttrValue(wgl.WGL_STEREO_ARB) != 0
			u.DoubleBuffer = findAttrValue(wgl.WGL_DOUBLE_BUFFER_ARB) != 0

			if p.ARB_multisample {
				u.Samples = findAttrValue(wgl.WGL_SAMPLES_ARB)
			}

			if p.ARB_framebuffer_sRGB || p.EXT_framebuffer_sRGB {
				u.SRGB = findAttrValue(wgl.WGL_FRAMEBUFFER_SRGB_CAPABLE_ARB) != 0
			}

			usableConfigs = append(usableConfigs, u)
		}
		closest := ChooseFBConfig(format, usableConfigs)
		if closest.Handle != 0 {
			return winapi.INT(closest.Handle), nil
		}
	}
	return 0, errors.New("can not choose useful pixel format")
}

func (p *wglPlatform) loadExtensions() {
	p.EXT_swap_control = strings.Contains(p.Extensions, "WGL_EXT_swap_control")
	p.EXT_colorspace = strings.Contains(p.Extensions, "WGL_EXT_colorspace")
	p.ARB_multisample = strings.Contains(p.Extensions, "WGL_ARB_multisample")
	p.ARB_framebuffer_sRGB = strings.Contains(p.Extensions, "WGL_ARB_framebuffer_sRGB")
	p.EXT_framebuffer_sRGB = strings.Contains(p.Extensions, "WGL_EXT_framebuffer_sRGB")
	p.ARB_pixel_format = strings.Contains(p.Extensions, "WGL_ARB_pixel_format")
	p.ARB_create_context = strings.Contains(p.Extensions, "WGL_ARB_create_context")
	p.EXT_create_context_es2_profile = strings.Contains(p.Extensions, "WGL_EXT_create_context_es2_profile")
	p.ARB_create_context_robustness = strings.Contains(p.Extensions, "WGL_ARB_create_context_robustness")
	p.ARB_create_context_no_error = strings.Contains(p.Extensions, "WGL_ARB_create_context_no_error")
	p.ARB_context_flush_control = strings.Contains(p.Extensions, "WGL_ARB_context_flush_control")
}
