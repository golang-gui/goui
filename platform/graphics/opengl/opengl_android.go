package opengl

import (
	"fmt"
	"github.com/goexlib/cgo"
	native "github.com/golang-gui/goui/platform/android/libs/android"
	"github.com/golang-gui/goui/platform/android/libs/egl"
	"github.com/golang-gui/goui/platform/common"
)

var gles = cgo.NewLazyLibrary("libGLESv3.so")
var glFlush = gles.NewSymbol("glFlush")
var glGetString = gles.NewSymbol("glGetString")

type eglContext struct {
	window mobileNativeWindow

	display, config, context uintptr
	scratch, surface, handle uintptr
	generation               uint64
	visual                   int32
	target                   common.NativeTarget
	current                  bool
}

func newContext(win NativeWindow, share Context, config Config) (Context, error) {
	window, provided := win.(mobileNativeWindow)
	if !provided || !mobileContextSupported(share, config) {
		return nil, common.ErrUnsupported
	}
	c := &eglContext{window: window}
	c.display = egl.GetDisplay(0)
	if c.display == 0 || egl.Initialize(c.display) == 0 {
		return nil, fmt.Errorf("android: initialize EGL: %#x", egl.Error())
	}
	attrs := []int32{0x3033, 5, 0x3040, 0x40, 0x3024, 8, 0x3023, 8, 0x3022, 8, 0x3021, 8, 0x3025, 24, 0x3026, 8, 0x3038}
	chosen, count, ok := egl.ChooseConfig(c.display, attrs)
	if ok == 0 || count != 1 {
		return nil, fmt.Errorf("android: select EGL ES3 configuration: %#x", egl.Error())
	}
	c.config = chosen
	c.visual, ok = egl.ConfigAttrib(c.display, c.config, 0x302e)
	if ok == 0 {
		return nil, fmt.Errorf("android: EGL native visual unavailable")
	}
	c.context = egl.CreateContext(c.display, c.config, []int32{0x3098, 3, 0x3038})
	c.scratch = egl.CreatePbuffer(c.display, c.config, []int32{0x3057, 1, 0x3056, 1, 0x3038})
	if c.context == 0 || c.scratch == 0 {
		c.Destroy()
		return nil, fmt.Errorf("android: create EGL context: %#x", egl.Error())
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
		native.WindowRelease(c.handle)
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
			native.WindowAcquire(handle)
			c.handle = handle
			c.generation = generation
			if native.WindowGeometry(handle, 0, 0, c.visual) != 0 {
				c.ClearCurrent()
				return fmt.Errorf("android: set EGL buffer geometry")
			}
			c.surface = egl.CreateWindow(c.display, c.config, handle)
		}
		surface = c.surface
	}
	if surface == 0 || egl.MakeCurrent(c.display, surface, surface, c.context) == 0 {
		if c.target != nil {
			c.target.Release()
			c.target = nil
		}
		return fmt.Errorf("android: bind EGL context: %#x", egl.Error())
	}
	c.current = true
	return nil
}
func (c *eglContext) ClearCurrent() error {
	var err error
	if c.current {
		cgo.Call(glFlush.Addr())
		if egl.MakeCurrent(c.display, 0, 0, 0) == 0 {
			err = fmt.Errorf("android: unbind EGL: %#x", egl.Error())
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
		return fmt.Errorf("android: swap EGL: %#x", egl.Error())
	}
	return nil
}
func (c *eglContext) SwapInterval(value int) error {
	if !c.current {
		return common.ErrUnavailable
	}
	if egl.SwapInterval(c.display, int32(value)) == 0 {
		return fmt.Errorf("android: EGL swap interval: %#x", egl.Error())
	}
	return nil
}
func (c *eglContext) GetProcAddress(name string) (uintptr, error) {
	address := egl.Proc(name)
	if address != 0 {
		return address, nil
	}
	if err := gles.Load(); err != nil {
		return 0, err
	}
	return cgo.Dlsym(gles.Handle(), name)
}
func (c *eglContext) GetExtensions() string {
	if !c.current {
		return ""
	}
	return cgo.GoString(cgo.Pointer(cgo.CallRet[uintptr](glGetString.Addr(), uint32(0x1f03))))
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
