package opengl

import (
	"errors"
	"strings"

	"github.com/golang-gui/goui/platform/linux/libs/glx"
	"github.com/golang-gui/goui/platform/linux/libs/xlib"
)

func InitGLX(display xlib.Display) error {
	return platform.Init(display)
}

func ChooseGLXFBConfig(config FBConfig) (native glx.FBConfig, err error) {
	return chooseGLXFBConfig(config)
}

type glxContext struct {
	window xlib.Window
	handle glx.Context
}

type glxNativeWindow interface {
	NativeFBConfig() glx.FBConfig
}

func newContext(win NativeWindow, share Context, _ Config) (_ Context, err error) {
	fbConfig := win.(glxNativeWindow).NativeFBConfig() // TODO: check

	var shareCtx glxContext
	if share != nil {
		shareCtx, _ = share.(glxContext)
	}

	if platform.ARB_create_context {
		// TODO: support `release` `version` or OpenGL ES
		attrs := []int32{
			glx.GLX_CONTEXT_PROFILE_MASK_ARB, glx.GLX_CONTEXT_CORE_PROFILE_BIT_ARB,
			0, 0,
		}
		ctx := glx.CreateContextAttribsARB(platform.display, fbConfig, shareCtx.handle, true, attrs)
		if ctx == 0 {
			return nil, errors.New("create GLX context failed")
		}

		return glxContext{
			window: xlib.Window(win.NativeHandle()),
			handle: ctx,
		}, nil
	}
	return nil, errors.New("can not create GLX context")
}

func (c glxContext) Destroy() {
	if c.handle != 0 {
		glx.DestroyContext(platform.display, c.handle)
	}
}

func (c glxContext) Name() string {
	return "GLX"
}

func (c glxContext) MakeCurrent() error {
	glx.MakeCurrent(platform.display, xlib.Drawable(c.window), c.handle)
	return nil
}

func (c glxContext) ClearCurrent() error {
	glx.MakeCurrent(platform.display, 0, 0)
	return nil
}

func (c glxContext) SwapBuffers() error {
	glx.SwapBuffers(platform.display, xlib.Drawable(c.window))
	return nil
}

func (c glxContext) SwapInterval(v int) error {
	if platform.EXT_swap_control {
		glx.SwapIntervalEXT(platform.display, xlib.Drawable(c.window), v)
	} else if platform.MESA_swap_control {
		glx.SwapIntervalMESA(v)
	} else if platform.SGI_swap_control && v > 0 {
		glx.SwapIntervalSGI(v)
	}
	return nil
}

func (c glxContext) GetProcAddress(name string) (proc uintptr, err error) {
	return glx.GetProcAddress(name)
}

func (c glxContext) GetExtensions() string {
	return platform.Extensions
}

func chooseGLXFBConfig(desired FBConfig) (glx.FBConfig, error) {
	vendor := glx.GetClientString(platform.display, glx.GLX_VENDOR)
	trustWindowBit := vendor != "Chromium"

	nativeConfigs := glx.GetFBConfigs(platform.display, platform.display.DefaultScreen())
	if len(nativeConfigs) == 0 {
		return 0, errors.New("get GLXFBConfigs failed")
	}

	usableConfigs := make([]FBConfig, 0, len(nativeConfigs))
	for _, n := range nativeConfigs {

		if getGLXFBConfigAttrib(n, glx.GLX_RENDER_TYPE)&glx.GLX_RGBA_BIT == 0 {
			continue
		}

		if getGLXFBConfigAttrib(n, glx.GLX_DRAWABLE_TYPE)&glx.GLX_WINDOW_BIT == 0 {
			if trustWindowBit {
				continue
			}
		}

		if doubleBuffer := getGLXFBConfigAttrib(n, glx.GLX_DOUBLEBUFFER) != 0; doubleBuffer != desired.DoubleBuffer {
			continue
		}

		if desired.Transparent {
			// TODO: implement
		}

		var u FBConfig
		u.Handle = uintptr(n)

		u.RedBits = getGLXFBConfigAttrib(n, glx.GLX_RED_SIZE)
		u.GreenBits = getGLXFBConfigAttrib(n, glx.GLX_GREEN_SIZE)
		u.BlueBits = getGLXFBConfigAttrib(n, glx.GLX_BLUE_SIZE)
		u.AlphaBits = getGLXFBConfigAttrib(n, glx.GLX_ALPHA_SIZE)

		u.DepthBits = getGLXFBConfigAttrib(n, glx.GLX_DEPTH_SIZE)
		u.StencilBits = getGLXFBConfigAttrib(n, glx.GLX_STENCIL_SIZE)

		u.AccumRedBits = getGLXFBConfigAttrib(n, glx.GLX_ACCUM_RED_SIZE)
		u.AccumGreenBits = getGLXFBConfigAttrib(n, glx.GLX_ACCUM_GREEN_SIZE)
		u.AccumBlueBits = getGLXFBConfigAttrib(n, glx.GLX_ACCUM_BLUE_SIZE)
		u.AccumAlphaBits = getGLXFBConfigAttrib(n, glx.GLX_ACCUM_ALPHA_SIZE)

		u.AuxBuffers = getGLXFBConfigAttrib(n, glx.GLX_AUX_BUFFERS)

		u.Stereo = getGLXFBConfigAttrib(n, glx.GLX_STEREO) != 0

		if platform.ARB_multisample {
			u.Samples = getGLXFBConfigAttrib(n, glx.GLX_SAMPLES)
		}

		if platform.ARB_framebuffer_sRGB || platform.EXT_framebuffer_sRGB {
			u.SRGB = getGLXFBConfigAttrib(n, glx.GLX_FRAMEBUFFER_SRGB_CAPABLE_ARB) != 0
		}

		usableConfigs = append(usableConfigs, u)
	}

	closest := ChooseFBConfig(desired.PixelFormat, usableConfigs)
	if closest != nil {
		return glx.FBConfig(closest.Handle), nil
	}

	return 0, errors.New("can not choose GLXFBConfig")
}

func getGLXFBConfigAttrib(config glx.FBConfig, attr int) (value int) {
	value, _ = glx.GetFBConfigAttrib(platform.display, config, attr)
	return
}

type glxPlatform struct {
	init    bool
	display xlib.Display
	major   int
	minor   int

	Extensions string

	SGI_swap_control               bool
	EXT_swap_control               bool
	MESA_swap_control              bool
	ARB_multisample                bool
	ARB_framebuffer_sRGB           bool
	EXT_framebuffer_sRGB           bool
	ARB_create_context             bool
	ARB_create_context_profile     bool
	ARB_create_context_robustness  bool
	EXT_create_context_es2_profile bool
	ARB_create_context_no_error    bool
	ARB_context_flush_control      bool
}

var platform glxPlatform

func (p *glxPlatform) Init(display xlib.Display) (err error) {
	if p.init {
		return nil
	}

	p.display = display

	if support, _, _ := glx.QueryExtension(display); !support {
		return errors.New("GLX extension not found")
	}

	if ok, major, minor := glx.QueryVersion(display); !ok {
		return errors.New("query GLX version failed")
	} else {
		p.major, p.minor = major, minor
	}

	p.Extensions = glx.QueryExtensionsString(p.display, p.display.DefaultScreen())
	p.loadExtensions()

	p.init = true
	return nil
}

func (p *glxPlatform) loadExtensions() {
	p.EXT_swap_control = strings.Contains(p.Extensions, "GLX_EXT_swap_control")
	p.SGI_swap_control = strings.Contains(p.Extensions, "GLX_SGI_swap_control")
	p.MESA_swap_control = strings.Contains(p.Extensions, "GLX_MESA_swap_control")
	p.ARB_multisample = strings.Contains(p.Extensions, "GLX_ARB_multisample")
	p.ARB_framebuffer_sRGB = strings.Contains(p.Extensions, "GLX_ARB_framebuffer_sRGB")
	p.EXT_framebuffer_sRGB = strings.Contains(p.Extensions, "GLX_EXT_framebuffer_sRGB")
	p.ARB_create_context = strings.Contains(p.Extensions, "GLX_ARB_create_context")
	p.ARB_create_context_robustness = strings.Contains(p.Extensions, "GLX_ARB_create_context_robustness")
	p.ARB_create_context_profile = strings.Contains(p.Extensions, "GLX_ARB_create_context_profile")
	p.EXT_create_context_es2_profile = strings.Contains(p.Extensions, "GLX_EXT_create_context_es2_profile")
	p.ARB_create_context_no_error = strings.Contains(p.Extensions, "GLX_ARB_create_context_no_error")
	p.ARB_context_flush_control = strings.Contains(p.Extensions, "GLX_ARB_context_flush_control")
}
