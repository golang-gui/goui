//go:build android || ios

package mobile

import (
	"fmt"
	"image"
	"runtime"
	"sync"
	"sync/atomic"
	"unsafe"

	ffi "github.com/goexlib/cgo"
	"github.com/golang-gui/goui/platform/common"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/graphics"
)

// This private ABI belongs to the separately compiled platform host, not the
// runtime base. host.h asserts the same 32-byte packet layout on native ARM64.
type nativePacket struct {
	Kind, Width, Height int32
	Scale               float32
	Handle              uintptr
	Generation          uint64
}

type nativeWindow struct {
	window                    *Window
	mu                        sync.Mutex
	current                   *surfaceTarget
	lastGeneration            uint64
	bind, visibility, present uintptr
	closed                    atomic.Bool
	pixels                    []byte // GOUI thread only; never retained by native presentation
}

var nativeContent *nativeWindow // GOUI thread only

func init() {
	connectNative = func(w *Window) error {
		if nativeContent != nil {
			return common.ErrUnsupported
		}
		n := &nativeWindow{window: w}
		for name, out := range map[string]*uintptr{"goui_host_bind": &n.bind, "goui_host_visible": &n.visibility, "goui_host_present": &n.present} {
			address, err := ffi.Dlsym(ffi.RTLD_DEFAULT, name)
			if err != nil {
				return fmt.Errorf("mobile host %s: %w", name, err)
			}
			*out = address
		}
		callback := ffi.NewCallback(func(address uintptr) uintptr {
			packet := *(*nativePacket)(ffi.Pointer(address))
			n.receive(packet)
			return 0
		})
		w.native = n
		if result := ffi.CallRet[int32](n.bind, callback, uint32(1)); result != 0 {
			return fmt.Errorf("mobile host ABI rejected: %d", result)
		}
		nativeContent = n
		return nil
	}
}

func (n *nativeWindow) setVisible(visible bool) error {
	value := int32(0)
	if visible {
		value = 1
	}
	if result := ffi.CallRet[int32](n.visibility, value); result != 0 {
		return fmt.Errorf("mobile visibility request: %d", result)
	}
	return nil
}
func (n *nativeWindow) close() {
	n.closed.Store(true)
	n.mu.Lock()
	if n.current != nil {
		n.current.revoke()
		n.current = nil
	}
	n.mu.Unlock()
	n.pixels = nil
	ffi.Call(n.bind, uintptr(0), uint32(1))
}

func (n *nativeWindow) receive(p nativePacket) {
	if n.closed.Load() {
		return
	}
	w := n.window
	switch p.Kind {
	case 1: // native target or dimensions changed
		n.mu.Lock()
		if n.closed.Load() {
			n.mu.Unlock()
			return
		}
		if p.Handle == 0 || p.Generation <= n.lastGeneration {
			n.mu.Unlock()
			return
		}
		n.lastGeneration = p.Generation
		if n.current != nil {
			n.current.revoke()
		}
		target := &surfaceTarget{handle: p.Handle, generation: p.Generation, alive: true}
		target.present = func(img image.Image) error { return n.draw(target, img) }
		n.current = target
		n.mu.Unlock()
		w.loop.Post(func() { w.attach(target, p.Width, p.Height, p.Scale) })
	case 2: // revoke synchronously; GUI notification can follow later
		n.mu.Lock()
		target := n.current
		if target == nil || target.generation != p.Generation {
			n.mu.Unlock()
			return
		}
		target.revoke()
		n.current = nil
		n.mu.Unlock()
		if target != nil {
			w.loop.Post(func() { w.detach(target) })
		}
	case 3:
		w.loop.Post(func() { w.setVisible(p.Width != 0) })
	case 4:
		w.loop.Post(func() { w.emit(events.FocusEvent{Focused: p.Width != 0}) })
	case 5: // Android's synchronous native redraw boundary
		done := make(chan struct{})
		w.loop.Post(func() {
			defer close(done)
			if !w.destroyed && w.visible && w.SurfaceState().Ready {
				w.paintDirty = false
				w.emit(events.PaintEvent{})
			}
		})
		select {
		case <-done:
		case <-w.loop.quit:
		}
	}
}

func (n *nativeWindow) draw(target *surfaceTarget, img image.Image) error {
	bitmap, ok := graphics.ToBitmap(img, graphics.PixelFormatRGBA)
	if !ok || bitmap.Stride != bitmap.Width*4 {
		bitmap = graphics.CopyToBitmap(img, graphics.PixelFormatRGBA, n.pixels)
		n.pixels = bitmap.Pixels
	}
	if len(bitmap.Pixels) == 0 {
		return common.ErrUnavailable
	}
	if bitmap.Stride < bitmap.Width*4 || int64(bitmap.Stride) > 2147483647 || int64(bitmap.Height-1)*int64(bitmap.Stride)+int64(bitmap.Width)*4 > int64(len(bitmap.Pixels)) {
		return fmt.Errorf("mobile: invalid bitmap storage")
	}
	var pin runtime.Pinner
	pin.Pin(&bitmap.Pixels[0])
	defer pin.Unpin()
	result := ffi.CallRet[int32](n.present, target.handle, target.generation, uintptr(unsafe.Pointer(&bitmap.Pixels[0])), int32(bitmap.Width), int32(bitmap.Height), int32(bitmap.Stride))
	runtime.KeepAlive(bitmap)
	if result == -1 {
		return common.ErrUnavailable
	}
	if result != 0 {
		return fmt.Errorf("mobile present: %d", result)
	}
	return nil
}
