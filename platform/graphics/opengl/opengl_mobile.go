//go:build android || ios

package opengl

import (
	"fmt"

	"github.com/goexlib/cgo"
	"github.com/golang-gui/goui/platform/common"
)

type nativeTargetSource interface {
	NativeWindow
	AcquireTarget() (common.NativeTarget, error)
}

// Platform-native context adaptation lives in the separately compiled host;
// Go owns its lifetime and all rendering. No native compiler runs at consumption.
type mobileContext struct {
	window                                             nativeTargetSource
	object                                             uintptr
	create, bind, clear, swap, interval, proc, destroy uintptr
	target                                             common.NativeTarget
	current                                            bool
}

func newContext(win NativeWindow, share Context, config Config) (Context, error) {
	window, ok := win.(nativeTargetSource)
	if !ok || share != nil || config.PixelFormat.Samples > 0 || config.PixelFormat.Stereo || config.PixelFormat.SRGB {
		return nil, common.ErrUnsupported
	}
	c := &mobileContext{window: window}
	for name, out := range map[string]*uintptr{
		"goui_host_gles_create": &c.create, "goui_host_gles_bind": &c.bind,
		"goui_host_gles_clear": &c.clear, "goui_host_gles_swap": &c.swap,
		"goui_host_gles_interval": &c.interval, "goui_host_gles_proc": &c.proc,
		"goui_host_gles_destroy": &c.destroy,
	} {
		address, err := cgo.Dlsym(cgo.RTLD_DEFAULT, name)
		if err != nil {
			return nil, fmt.Errorf("mobile GLES host %s: %w", name, err)
		}
		*out = address
	}
	c.object = cgo.CallRet[uintptr](c.create)
	if c.object == 0 {
		return nil, fmt.Errorf("mobile: create OpenGL ES 3 context failed")
	}
	return c, nil
}

func (*mobileContext) Name() string { return "OpenGL ES 3" }

func (c *mobileContext) MakeCurrent() error {
	if c.object == 0 {
		return common.ErrUnavailable
	}
	if c.current {
		return nil
	}
	var handle uintptr
	var generation uint64
	if c.window.NativeHandle() != 0 {
		target, err := c.window.AcquireTarget()
		if err != nil {
			return ErrSurfaceUnavailable
		}
		c.target = target
		handle = target.NativeHandle()
		generation = target.Generation()
	}
	// With no target, bind private scratch storage for initialization/cleanup.
	// A successful window bind holds the target through ClearCurrent.
	if result := cgo.CallRet[int32](c.bind, c.object, handle, generation); result != 0 {
		cgo.CallRet[int32](c.clear, c.object)
		if c.target != nil {
			c.target.Release()
			c.target = nil
		}
		if result == -1 {
			return ErrSurfaceUnavailable
		}
		return fmt.Errorf("mobile GLES bind: %d", result)
	}
	c.current = true
	return nil
}

func (c *mobileContext) ClearCurrent() error {
	if !c.current {
		return nil
	}
	result := cgo.CallRet[int32](c.clear, c.object)
	c.current = false
	if c.target != nil {
		c.target.Release()
		c.target = nil
	}
	if result != 0 {
		return fmt.Errorf("mobile GLES clear: %d", result)
	}
	return nil
}

func (c *mobileContext) SwapBuffers() error {
	if !c.current || c.target == nil {
		return ErrSurfaceUnavailable
	}
	result := cgo.CallRet[int32](c.swap, c.object)
	if result == -1 {
		return ErrSurfaceUnavailable
	}
	if result != 0 {
		return fmt.Errorf("mobile GLES swap: %d", result)
	}
	return nil
}

func (c *mobileContext) SwapInterval(interval int) error {
	if !c.current {
		return common.ErrUnavailable
	}
	result := cgo.CallRet[int32](c.interval, c.object, int32(interval))
	if result == -4 {
		return common.ErrUnsupported
	}
	if result != 0 {
		return fmt.Errorf("mobile GLES interval: %d", result)
	}
	return nil
}

func (c *mobileContext) GetProcAddress(name string) (uintptr, error) {
	address := cgo.CallRet[uintptr](c.proc, cgo.CStringTemp(name))
	if address == 0 {
		return 0, fmt.Errorf("mobile GLES symbol %q is unavailable", name)
	}
	return address, nil
}

func (c *mobileContext) GetExtensions() string {
	if !c.current {
		return ""
	}
	address, err := c.GetProcAddress("glGetString")
	if err != nil {
		return ""
	}
	return cgo.GoString(cgo.Pointer(cgo.CallRet[uintptr](address, uint32(0x1F03))))
}

func (c *mobileContext) Destroy() {
	if c.object == 0 {
		return
	}
	_ = c.ClearCurrent()
	cgo.Call(c.destroy, c.object)
	c.object = 0
}
