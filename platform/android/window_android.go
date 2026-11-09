package android

import (
	"image"
	"sync/atomic"

	native "github.com/golang-gui/goui/platform/android/libs/android"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/internal/mobile"
)

// Window owns the logical adapter independently of Activity/target replacement.
type Window struct {
	*mobile.Window
	application *nativeApplication
	loop        *EventLoop
	visible     atomic.Bool
}

// drawingTarget contains borrowed native facts. Generation belongs to the
// logical host and must not restart when Android replaces its Activity.
type drawingTarget struct {
	handle        uintptr
	width, height int32
	generation    uint64
}

func newWindow(loop *EventLoop, handler events.EventHandler) (*Window, error) {
	w := &Window{application: &androidApplication, loop: loop}
	base, err := mobile.NewWindow(loop.EventLoop, handler, windowHost{w})
	if err != nil {
		return nil, err
	}
	w.Window = base
	return w, nil
}
func (w *Window) bind() {
	w.loop.Post(func() {
		if w.Destroyed() {
			return
		}
		h := w.application
		h.bound = w
		h.publishTarget()
		if w.Destroyed() || h.bound != w {
			return
		}
		h.applyVisibility()
		if !w.Destroyed() && h.bound == w && h.activity != nil {
			w.Source().Focused(h.activity.focused)
		}
	})
}

type windowHost struct{ window *Window }

func (a windowHost) SetVisible(value bool) error {
	w := a.window
	w.visible.Store(value)
	w.loop.Deliver(func() {
		if w.application.bound == w {
			w.application.applyVisibility()
		}
	})
	return nil
}
func (a windowHost) Close() {
	w := a.window
	w.loop.Deliver(func() {
		if h := w.application; h.bound == w {
			h.bound = nil
			h.applyVisibility()
		}
	})
}

func (h *nativeApplication) onWindowCreated(address, window uintptr) {
	withNativeThread(func() {
		if h.activityFor(address) == nil {
			return
		}
		if native.NativeWindowSetBuffersGeometry(window, 0, 0, native.WindowFormatRGBX8888) != 0 {
			panic("android: set software buffer format")
		}
		h.updateTarget(window, false)
	})
}
func (h *nativeApplication) onWindowResized(address, window uintptr) {
	withNativeThread(func() {
		if h.activityFor(address) != nil {
			h.updateTarget(window, false)
		}
	})
}
func (h *nativeApplication) onWindowRedraw(address, window uintptr) {
	withNativeThread(func() {
		a := h.activityFor(address)
		if a == nil {
			return
		}
		h.updateTarget(window, false)
		if h.activity == a && h.target.handle == window {
			if w := h.bound; w != nil {
				w.Source().Redraw()
			}
		}
	})
}
func (h *nativeApplication) onWindowDestroyed(address, window uintptr) {
	withNativeThread(func() {
		if h.activityFor(address) != nil && h.target.handle == window {
			h.updateTarget(0, false)
		}
	})
}
func (h *nativeApplication) updateTarget(handle uintptr, force bool) {
	var width, height int32
	if handle != 0 {
		width = native.NativeWindowGetWidth(handle)
		height = native.NativeWindowGetHeight(handle)
	}
	if !force && h.target.handle == handle && h.target.width == width && h.target.height == height {
		return
	}
	h.target = drawingTarget{handle: handle, width: width, height: height, generation: h.target.generation + 1}
	generation := h.target.generation
	if w := h.bound; w != nil {
		w.Source().Revoke()
	}
	if h.target.generation == generation {
		h.publishTarget()
	}
}
func (h *nativeApplication) publishTarget() {
	w, a, target := h.bound, h.activity, h.target
	if w == nil || a == nil || target.handle == 0 || target.width <= 0 || target.height <= 0 {
		return
	}
	w.Source().Attach(target.handle, target.generation, target.width, target.height, a.density, func(img image.Image) error { return present(target.handle, img) })
}
func (h *nativeApplication) applyVisibility() {
	a, w := h.activity, h.bound
	if a == nil {
		return
	}
	shown, err := a.setVisible(w != nil && w.visible.Load())
	if err != nil {
		panic(err)
	}
	if h.activity == a && h.bound == w && w != nil {
		w.Source().Visible(a.started && shown)
	}
}
