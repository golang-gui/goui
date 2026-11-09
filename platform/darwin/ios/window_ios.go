package ios

import (
	"image"
	"math"
	"sync"
	"sync/atomic"

	"github.com/ebitengine/purego/objc"
	"github.com/golang-gui/goui/platform/common"
	cg "github.com/golang-gui/goui/platform/darwin/frameworks/core_graphics"
	. "github.com/golang-gui/goui/platform/darwin/frameworks/foundation"
	qc "github.com/golang-gui/goui/platform/darwin/frameworks/quartzcore"
	ui "github.com/golang-gui/goui/platform/darwin/frameworks/uikit"
	"github.com/golang-gui/goui/platform/darwin/libs/dispatch"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/internal/mobile"
)

// Window adapts UIKit's content window without exposing a native host protocol.
type Window struct {
	*mobile.Window
	state *nativeState
}

// PrepareEAGLLayer makes the UIKit drawable visible on the native UI thread.
func (w *Window) PrepareEAGLLayer() error {
	if !dispatch.IsMainThread() || w.state.layer.ID == 0 {
		return common.ErrUnavailable
	}
	w.state.layer.SetHidden(false)
	return nil
}

// NativeEAGLLayer borrows the layer under an acquired native target lease.
// It performs no UIKit calls and does not extend the layer's lifetime.
func (w *Window) NativeEAGLLayer() qc.CALayer { return w.state.layer }

// LockGPU acquires the lifecycle gate and reports whether GL work is allowed.
// The caller must call UnlockGPU for either result. This also protects offscreen
// work, which has no native target lease to join when UIKit enters background.
func (w *Window) LockGPU() bool {
	w.state.gpu.Lock()
	return w.state.allowed
}

// UnlockGPU releases the lifecycle gate after all submitted GPU work is drained.
func (w *Window) UnlockGPU() { w.state.gpu.Unlock() }

type nativeState struct {
	// UIKit objects and geometry belong exclusively to the system main thread.
	window        ui.UIWindow
	view          ui.UIView
	controller    ui.UIViewController
	layer         qc.CALayer
	bound         *Window
	width, height int32
	scale         float32
	generation    uint64
	foreground    bool
	wanted        atomic.Bool
	// The GPU gate also covers offscreen work. Background entry drains it first.
	gpu     sync.Mutex
	allowed bool
	image   pendingImage
}

var iosHost nativeState
var contentControllerClass ui.UIViewControllerClass

func initWindow() (err error) {
	h := &iosHost
	contentControllerClass, err = ui.ImplementUIViewController("GoUIContent", ui.UIViewControllerOverride{
		ViewDidLayoutSubviews: func(self ui.UIViewController) {
			onSystemThread(func() {
				objc.SendSuper[uintptr](self.ID, ui.UIViewControllerSel.ViewDidLayoutSubviews)
				h.target(false)
			})
		},
	})
	if err != nil {
		return
	}
	_, err = ui.ImplementUIWindowSceneDelegate("GoUIScene", ui.UIWindowSceneDelegateOverride{
		SceneWillConnectToSessionOptions: func(_ ui.UIWindowSceneDelegate, scene ui.UIScene, _ ui.UISceneSession, _ ui.UISceneConnectionOptions) {
			onSystemThread(func() { h.connect(scene) })
		},
		SceneWillEnterForeground: func(_ ui.UIWindowSceneDelegate, scene ui.UIScene) {
			onSystemThread(func() { h.enterForeground(scene) })
		},
		SceneDidEnterBackground: func(_ ui.UIWindowSceneDelegate, scene ui.UIScene) {
			onSystemThread(func() { h.enterBackground(scene) })
		},
		SceneDidBecomeActive: func(_ ui.UIWindowSceneDelegate, scene ui.UIScene) {
			onSystemThread(func() { h.focus(scene, true) })
		},
		SceneWillResignActive: func(_ ui.UIWindowSceneDelegate, scene ui.UIScene) {
			onSystemThread(func() { h.focus(scene, false) })
		},
		SceneDidDisconnect: func(_ ui.UIWindowSceneDelegate, scene ui.UIScene) {
			onSystemThread(func() { h.disconnect() })
		},
	})
	return
}

func (h *nativeState) enterForeground(_ ui.UIScene) {
	h.foreground = true
	h.gpu.Lock()
	h.allowed = true
	h.gpu.Unlock()
	h.target(true)
	h.visibility()
}

func (h *nativeState) enterBackground(_ ui.UIScene) {
	h.suspend()
	h.foreground = false
	h.visibility()
}

func (h *nativeState) focus(_ ui.UIScene, active bool) {
	if h.bound != nil {
		h.bound.Source().Focused(active && h.window.ID != 0 && h.window.IsKeyWindow())
	}
}

func (h *nativeState) connect(scene ui.UIScene) {
	if h.window.ID != 0 {
		h.disconnect()
	}
	h.window = ui.UIWindowClassId.Alloc().InitWithWindowScene(Cast[ui.UIWindowScene](scene.ID))
	h.controller = contentControllerClass.Alloc().Init()
	h.view = ui.UIViewClassId.Alloc().InitWithFrame(cg.CGRect{})
	h.view.SetBackgroundColor(ui.UIColorClassId.BlackColor())
	h.controller.SetView(h.view)
	h.window.SetRootViewController(h.controller)
	h.layer = Cast[qc.CALayer](qc.CAEAGLLayerClassId.Alloc().Send(NSObjectSel.Init))
	h.layer.SetOpaque(true)
	h.layer.SetHidden(true)
	Cast[qc.CALayer](h.view.Layer()).AddSublayer(h.layer)
	h.width, h.height = 0, 0
	h.window.MakeKeyAndVisible()
	h.target(true)
	h.visibility()
	if iosApplication.onHostReady != nil {
		iosApplication.onHostReady()
	}
}
func (h *nativeState) target(force bool) {
	if h.view.ID == 0 {
		return
	}
	bounds := h.view.Bounds()
	scale := h.window.Screen().Scale()
	width, height := int32(math.Round(bounds.Size.Width*scale)), int32(math.Round(bounds.Size.Height*scale))
	if width <= 0 || height <= 0 || scale <= 0 {
		return
	}
	if !force && width == h.width && height == h.height && float32(scale) == h.scale {
		return
	}
	h.revoke()
	h.width, h.height, h.scale = width, height, float32(scale)
	h.view.SetContentScaleFactor(scale)
	Cast[qc.CALayer](h.view.Layer()).SetContentsScale(scale)
	qc.CATransactionClassId.Begin()
	qc.CATransactionClassId.SetDisableActions(true)
	h.layer.SetFrame(bounds)
	h.layer.SetContentsScale(scale)
	qc.CATransactionClassId.Commit()
	h.publishTarget()
}
func (h *nativeState) publishTarget() {
	if h.bound == nil || h.view.ID == 0 || h.width <= 0 || h.height <= 0 {
		return
	}
	handle, generation := uintptr(h.view.ID), h.generation
	h.bound.Source().Attach(handle, generation, h.width, h.height, h.scale, func(img image.Image) error { return h.present(handle, generation, img) })
}
func (h *nativeState) revoke() {
	if h.bound != nil {
		h.bound.Source().Revoke()
	}
	h.clearImage()
	h.generation++
}
func (h *nativeState) suspend() { h.gpu.Lock(); h.allowed = false; h.gpu.Unlock(); h.revoke() }
func (h *nativeState) visibility() {
	if h.view.ID != 0 {
		h.view.SetHidden(!h.wanted.Load())
	}
	if h.bound != nil {
		h.bound.Source().Visible(h.foreground && h.view.ID != 0 && !h.view.Hidden())
	}
}
func (h *nativeState) disconnect() {
	h.suspend()
	h.foreground = false
	h.visibility()
	if h.window.ID != 0 {
		h.window.SetHidden(true)
		h.window.SetRootViewController(ui.UIViewController{})
	}
	if h.layer.ID != 0 {
		h.layer.RemoveFromSuperlayer()
		h.layer.Release()
	}
	if h.view.ID != 0 {
		h.view.Release()
	}
	if h.controller.ID != 0 {
		h.controller.Release()
	}
	if h.window.ID != 0 {
		h.window.Release()
	}
	h.layer = qc.CALayer{}
	h.view = ui.UIView{}
	h.controller = ui.UIViewController{}
	h.window = ui.UIWindow{}
	h.width, h.height = 0, 0
}
func newWindow(loop *EventLoop, handler events.EventHandler) (*Window, error) {
	w := &Window{state: &iosHost}
	base, err := mobile.NewWindow(loop.EventLoop, handler, windowHost{w})
	if err != nil {
		return nil, err
	}
	w.Window = base
	return w, nil
}

// bind starts observations only after the consumer has returned its handler.
func (w *Window) bind(loop *EventLoop) {
	loop.Post(func() {
		if w.Destroyed() {
			return
		}
		w.state.bound = w
		w.state.publishTarget()
		// Target delivery may synchronously destroy the adapter.
		if !w.Destroyed() {
			w.state.visibility()
			w.Source().Focused(w.state.window.ID != 0 && w.state.window.IsKeyWindow())
		}
	})
}

type windowHost struct{ window *Window }

func (a windowHost) SetVisible(value bool) error {
	w := a.window
	w.state.wanted.Store(value)
	iosApplication.loop.Load().Deliver(w.state.visibility)
	return nil
}
func (a windowHost) Close() {
	w := a.window
	iosApplication.loop.Load().Deliver(func() {
		if w.state.bound == w {
			w.state.bound = nil
			w.state.wanted.Store(false)
			w.state.clearImage()
			if w.state.view.ID != 0 {
				Cast[qc.CALayer](w.state.view.Layer()).SetContents(0)
			}
			w.state.visibility()
		}
	})
}
