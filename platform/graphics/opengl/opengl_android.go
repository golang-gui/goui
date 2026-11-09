package opengl

import (
	"fmt"

	native "github.com/golang-gui/goui/platform/android/libs/android"
	"github.com/golang-gui/goui/platform/android/libs/egl"
	"github.com/golang-gui/goui/platform/android/libs/gles"
	"github.com/golang-gui/goui/platform/common"
)

type eglContext struct {
	window mobileNativeWindow

	// The default display is process-shared; this context never terminates it.
	display          egl.Display
	config           egl.Config
	context          egl.Context
	scratch, surface egl.Surface
	handle           uintptr
	generation       uint64
	visual           int32
	target           common.NativeTarget
	current          bool
}

func newContext(win NativeWindow, share Context, config Config) (Context, error) {
	window, provided := win.(mobileNativeWindow)
	if !provided || !mobileContextSupported(share, config) {
		return nil, common.ErrUnsupported
	}
	c := &eglContext{window: window}
	c.display = egl.GetDisplay(0)
	if c.display == 0 || egl.Initialize(c.display, nil, nil) == 0 {
		return nil, fmt.Errorf("android: initialize EGL: %#x", egl.GetError())
	}
	attrs := []int32{
		egl.SurfaceType, egl.WindowBit | egl.PbufferBit,
		egl.RenderableType, egl.OpenGLES3Bit,
		egl.RedSize, 8, egl.GreenSize, 8, egl.BlueSize, 8, egl.AlphaSize, 8,
		egl.DepthSize, 24, egl.StencilSize, 8,
		egl.None,
	}
	var chosen [1]egl.Config
	count, ok := egl.ChooseConfig(c.display, attrs, chosen[:])
	if ok == 0 {
		return nil, fmt.Errorf("android: select EGL ES3 configuration: %#x", egl.GetError())
	}
	if count != 1 {
		return nil, fmt.Errorf("android: no matching EGL ES3 configuration")
	}
	c.config = chosen[0]
	c.visual, ok = egl.GetConfigAttrib(c.display, c.config, egl.NativeVisualID)
	if ok == 0 {
		return nil, fmt.Errorf("android: EGL native visual unavailable: %#x", egl.GetError())
	}
	c.context = egl.CreateContext(c.display, c.config, 0, []int32{egl.ContextClientVersion, 3, egl.None})
	if c.context == 0 {
		return nil, fmt.Errorf("android: create EGL context: %#x", egl.GetError())
	}
	c.scratch = egl.CreatePbufferSurface(c.display, c.config, []int32{egl.Width, 1, egl.Height, 1, egl.None})
	if c.scratch == 0 {
		err := fmt.Errorf("android: create EGL pbuffer: %#x", egl.GetError())
		c.Destroy()
		return nil, err
	}
	return c, nil
}
func (*eglContext) Name() string { return "OpenGL ES 3" }
func (c *eglContext) dropSurface() {
	if c.surface != 0 {
		egl.DestroySurface(c.display, c.surface)
		c.surface = 0
	}
	if c.handle != 0 {
		native.NativeWindowRelease(c.handle)
		c.handle = 0
	}
	c.generation = 0
}
func (c *eglContext) MakeCurrent() error {
	if c.context == 0 {
		return common.ErrUnavailable
	}
	if c.current {
		return nil
	}
	surface := c.scratch
	if c.window.NativeHandle() != 0 {
		target, err := c.window.AcquireTarget()
		if err != nil {
			return ErrSurfaceUnavailable
		}
		c.target = target
		handle, generation := target.NativeHandle(), target.Generation()
		if c.surface == 0 || c.handle != handle || c.generation != generation {
			c.dropSurface()
			native.NativeWindowAcquire(handle)
			c.handle = handle
			c.generation = generation
			if native.NativeWindowSetBuffersGeometry(handle, 0, 0, c.visual) != 0 {
				_ = c.ClearCurrent()
				c.dropSurface()
				return fmt.Errorf("android: set EGL buffer geometry")
			}
			c.surface = egl.CreateWindowSurface(c.display, c.config, handle, nil)
			if c.surface == 0 {
				err := fmt.Errorf("android: create EGL window surface: %#x", egl.GetError())
				_ = c.ClearCurrent()
				c.dropSurface()
				return err
			}
		}
		surface = c.surface
	}
	if surface == 0 || egl.MakeCurrent(c.display, surface, surface, c.context) == 0 {
		err := fmt.Errorf("android: bind EGL context: %#x", egl.GetError())
		if c.target != nil {
			c.target.Release()
			c.target = nil
		}
		return err
	}
	c.current = true
	return nil
}
func (c *eglContext) ClearCurrent() error {
	var err error
	if c.current {
		gles.Flush()
		if egl.MakeCurrent(c.display, 0, 0, 0) == 0 {
			err = fmt.Errorf("android: unbind EGL: %#x", egl.GetError())
		}
		c.current = false
	}
	if c.target != nil {
		c.target.Release()
		c.target = nil
	}
	return err
}
func (c *eglContext) SwapBuffers() error {
	if !c.current || c.target == nil {
		return ErrSurfaceUnavailable
	}
	if egl.SwapBuffers(c.display, c.surface) == 0 {
		return fmt.Errorf("android: swap EGL: %#x", egl.GetError())
	}
	return nil
}
func (c *eglContext) SwapInterval(value int) error {
	if !c.current {
		return common.ErrUnavailable
	}
	if egl.SwapInterval(c.display, int32(value)) == 0 {
		return fmt.Errorf("android: EGL swap interval: %#x", egl.GetError())
	}
	return nil
}
func (c *eglContext) GetProcAddress(name string) (uintptr, error) {
	address := egl.GetProcAddress(name)
	if address != 0 {
		return address, nil
	}
	return gles.Lookup(name)
}
func (c *eglContext) GetExtensions() string {
	if !c.current {
		return ""
	}
	return gles.GetString(gles.Extensions)
}
func (c *eglContext) Destroy() {
	_ = c.ClearCurrent()
	c.dropSurface()
	if c.scratch != 0 {
		egl.DestroySurface(c.display, c.scratch)
		c.scratch = 0
	}
	if c.context != 0 {
		egl.DestroyContext(c.display, c.context)
		c.context = 0
	}
}
