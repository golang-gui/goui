package opengl

import (
	"fmt"
	"runtime"

	"github.com/goexlib/cgo"
	"github.com/golang-gui/goui/platform/common"
	gl "github.com/golang-gui/goui/platform/darwin/frameworks/opengles"
	qc "github.com/golang-gui/goui/platform/darwin/frameworks/quartzcore"
)

// eaglNativeWindow supplies UIKit-owned geometry and lifecycle synchronization.
// Context creation prepares the layer before borrowing any target. Subsequent
// layer queries require a target lease and must never wait for the UIKit thread.
type eaglNativeWindow interface {
	mobileNativeWindow
	PrepareEAGLLayer() error
	NativeEAGLLayer() qc.CALayer
	LockGPU() bool
	UnlockGPU()
}

// eaglContext and its GL objects belong to the caller's locked Go render thread.
// UIKit owns the drawable layer; target leases exclude changes to that layer.
type eaglContext struct {
	window            eaglNativeWindow
	context           gl.EAGLContext
	scratch, drawable [3]uint32
	generation        uint64
	target            common.NativeTarget
	current           bool
}

func newContext(win NativeWindow, share Context, config Config) (Context, error) {
	window, ok := win.(eaglNativeWindow)
	if !ok || !mobileContextSupported(share, config) {
		return nil, common.ErrUnsupported
	}
	if err := window.PrepareEAGLLayer(); err != nil {
		return nil, err
	}
	allowed := window.LockGPU()
	defer window.UnlockGPU()
	if !allowed {
		return nil, common.ErrUnavailable
	}
	context := gl.EAGLContextClassId.Alloc().InitWithAPI(3)
	if context.ID == 0 {
		return nil, fmt.Errorf("ios: create EAGL ES3 context")
	}
	return &eaglContext{window: window, context: context}, nil
}
func (*eaglContext) Name() string { return "OpenGL ES 3" }
func deleteFramebuffer(ids *[3]uint32) {
	var pin runtime.Pinner
	pin.Pin(ids)
	defer pin.Unpin()
	if ids[0] != 0 {
		gl.DeleteFramebuffers(1, &ids[0])
	}
	if ids[1] != 0 {
		gl.DeleteRenderbuffers(2, &ids[1])
	}
	*ids = [3]uint32{}
}
func (c *eaglContext) framebuffer(ids *[3]uint32, layer qc.CALayer) (result error) {
	defer func() {
		if result != nil {
			deleteFramebuffer(ids)
		}
	}()
	deleteFramebuffer(ids)
	var pin runtime.Pinner
	pin.Pin(ids)
	defer pin.Unpin()
	gl.GenFramebuffers(1, &ids[0])
	gl.BindFramebuffer(0x8d40, ids[0])
	gl.GenRenderbuffers(2, &ids[1])
	gl.BindRenderbuffer(0x8d41, ids[1])
	if layer.ID != 0 {
		if !c.context.RenderbufferStorage(0x8d41, layer.ID) {
			return fmt.Errorf("ios: EAGL drawable storage")
		}
	} else {
		gl.RenderbufferStorage(0x8d41, 0x8058, 1, 1)
	}
	var size [2]int32
	pin.Pin(&size)
	gl.GetRenderbufferParameteriv(0x8d41, 0x8d42, &size[0])
	gl.GetRenderbufferParameteriv(0x8d41, 0x8d43, &size[1])
	gl.FramebufferRenderbuffer(0x8d40, 0x8ce0, 0x8d41, ids[1])
	gl.BindRenderbuffer(0x8d41, ids[2])
	gl.RenderbufferStorage(0x8d41, 0x88f0, size[0], size[1])
	gl.FramebufferRenderbuffer(0x8d40, 0x821a, 0x8d41, ids[2])
	if status := gl.CheckFramebufferStatus(0x8d40); status != 0x8cd5 {
		return fmt.Errorf("ios: incomplete framebuffer %#x", status)
	}
	return nil
}
func (c *eaglContext) MakeCurrent() (result error) {
	if c.current {
		return nil
	}
	if c.context.ID == 0 {
		return common.ErrUnavailable
	}
	if !c.window.LockGPU() {
		c.window.UnlockGPU()
		return ErrSurfaceUnavailable
	}
	if !gl.EAGLContextClassId.SetCurrentContext(c.context) {
		c.window.UnlockGPU()
		return fmt.Errorf("ios: bind EAGL context")
	}
	c.current = true
	defer func() {
		if result != nil {
			_ = c.ClearCurrent()
		}
	}()
	if c.window.NativeHandle() == 0 {
		if c.scratch[0] == 0 {
			if err := c.framebuffer(&c.scratch, qc.CALayer{}); err != nil {
				return err
			}
		}
		gl.BindFramebuffer(0x8d40, c.scratch[0])
		return nil
	}
	target, err := c.window.AcquireTarget()
	if err != nil {
		return ErrSurfaceUnavailable
	}
	c.target = target
	if c.generation != target.Generation() || c.drawable[0] == 0 {
		if err = c.framebuffer(&c.drawable, c.window.NativeEAGLLayer()); err != nil {
			return err
		}
		c.generation = target.Generation()
	}
	gl.BindFramebuffer(0x8d40, c.drawable[0])
	return nil
}
func (c *eaglContext) ClearCurrent() error {
	if !c.current {
		return nil
	}
	gl.Finish()
	ok := gl.EAGLContextClassId.SetCurrentContext(gl.EAGLContext{})
	if c.target != nil {
		c.target.Release()
		c.target = nil
	}
	c.current = false
	c.window.UnlockGPU()
	if !ok {
		return fmt.Errorf("ios: unbind EAGL context")
	}
	return nil
}
func (c *eaglContext) SwapBuffers() error {
	if !c.current || c.target == nil {
		return ErrSurfaceUnavailable
	}
	gl.BindRenderbuffer(0x8d41, c.drawable[1])
	if !c.context.PresentRenderbuffer(0x8d41) {
		return fmt.Errorf("ios: present EAGL renderbuffer")
	}
	return nil
}
func (*eaglContext) SwapInterval(int) error                      { return common.ErrUnsupported }
func (*eaglContext) GetProcAddress(name string) (uintptr, error) { return gl.GetProcAddress(name) }
func (c *eaglContext) GetExtensions() string {
	if !c.current {
		return ""
	}
	return cgo.GoString(gl.GetString(0x1f03))
}
func (c *eaglContext) Destroy() {
	_ = c.ClearCurrent()
	allowed := c.window.LockGPU()
	defer c.window.UnlockGPU()
	if c.context.ID == 0 {
		return
	}
	if allowed && gl.EAGLContextClassId.SetCurrentContext(c.context) {
		deleteFramebuffer(&c.scratch)
		deleteFramebuffer(&c.drawable)
		gl.Finish()
		gl.EAGLContextClassId.SetCurrentContext(gl.EAGLContext{})
	}
	c.context.Release()
	c.context = gl.EAGLContext{}
}
