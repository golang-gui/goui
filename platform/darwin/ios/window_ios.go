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

// nativeState owns the current Scene's UIKit objects and drawing target. All
// native objects and geometry are main-thread-only. The logical Window can
// outlive a Scene; generation rejects submissions from its previous target.
type nativeState struct {
	scene         ui.UIWindowScene // Retained from connect until disconnect.
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

	// The GPU gate also covers offscreen work, which has no target lease.
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
				if h.controller.ID == self.ID {
					h.target(false)
				}
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
			onSystemThread(func() {
				if h.ownsScene(scene) {
					h.disconnect()
				}
			})
		},
	})
	return
}

func (h *nativeState) ownsScene(scene ui.UIScene) bool {
	return scene.ID != 0 && scene.ID == h.scene.ID
}

func (h *nativeState) enterForeground(scene ui.UIScene) {
	if !h.ownsScene(scene) {
		return
	}
	h.foreground = true
	h.gpu.Lock()
	h.allowed = true
	h.gpu.Unlock()
	h.target(true)
	// Target delivery may disconnect or replace this Scene synchronously.
	if h.ownsScene(scene) {
		h.visibility()
	}
}

func (h *nativeState) enterBackground(scene ui.UIScene) {
	if !h.ownsScene(scene) {
		return
	}
	h.foreground = false
	h.suspend()
	if h.ownsScene(scene) {
		h.visibility()
	}
}

func (h *nativeState) focus(scene ui.UIScene, active bool) {
	if h.ownsScene(scene) && h.bound != nil {
		h.bound.Source().Focused(active && h.window.ID != 0 && h.window.IsKeyWindow())
	}
}

func (h *nativeState) connect(scene ui.UIScene) {
	if scene.ID == 0 || h.ownsScene(scene) {
		return
	}
	if h.scene.ID != 0 {
		h.disconnect()
		if h.scene.ID != 0 {
			return
		}
	}
	h.scene = Cast[ui.UIWindowScene](scene.ID)
	h.scene.Retain()
	h.window = ui.UIWindowClassId.Alloc().InitWithWindowScene(h.scene)
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
	if !h.ownsScene(scene) {
		return
	}
	h.target(true)
	if !h.ownsScene(scene) {
		return
	}
	h.visibility()
	if h.ownsScene(scene) && iosApplication.onHostReady != nil {
		iosApplication.onHostReady()
	}
}

func (h *nativeState) target(force bool) {
	if h.view.ID == 0 || h.layer.ID == 0 {
		return
	}
	scene, view := h.scene, h.view
	bounds := view.Bounds()
	scale := h.window.Screen().Scale()
	width, height := int32(math.Round(bounds.Size.Width*scale)), int32(math.Round(bounds.Size.Height*scale))
	if width <= 0 || height <= 0 || scale <= 0 {
		return
	}
	if !force && width == h.width && height == h.height && float32(scale) == h.scale {
		return
	}
	generation := h.generation + 1
	h.revoke()
	// Revoke delivers SurfaceEvent synchronously after Run. A callback may
	// detach this View or publish another target before returning here.
	if h.scene.ID != scene.ID || h.view.ID != view.ID || h.generation != generation {
		return
	}
	h.width, h.height, h.scale = width, height, float32(scale)
	view.SetContentScaleFactor(scale)
	Cast[qc.CALayer](view.Layer()).SetContentsScale(scale)
	qc.CATransactionClassId.Begin()
	qc.CATransactionClassId.SetDisableActions(true)
	h.layer.SetFrame(bounds)
	h.layer.SetContentsScale(scale)
	qc.CATransactionClassId.Commit()
	h.publishTarget()
}

func (h *nativeState) publishTarget() {
	if h.bound == nil || h.bound.Destroyed() || h.view.ID == 0 || h.width <= 0 || h.height <= 0 {
		return
	}
	handle, generation := uintptr(h.view.ID), h.generation
	h.bound.Source().Attach(handle, generation, h.width, h.height, h.scale, func(img image.Image) error { return h.present(handle, generation, img) })
}

func (h *nativeState) revoke() {
	h.generation++
	h.clearImage()
	if h.bound != nil {
		h.bound.Source().Revoke()
	}
}

func (h *nativeState) suspend() {
	h.gpu.Lock()
	h.allowed = false
	h.gpu.Unlock()
	h.revoke()
}

func (h *nativeState) visibility() {
	if h.view.ID != 0 {
		h.view.SetHidden(!h.wanted.Load())
	}
	if h.bound != nil {
		h.bound.Source().Visible(h.foreground && h.view.ID != 0 && !h.view.Hidden())
	}
}

func (h *nativeState) disconnect() {
	h.gpu.Lock()
	h.allowed = false
	h.gpu.Unlock()
	// Detach identity before notifying consumers. Late native callbacks and
	// reentrant delivery cannot release a newly connected Scene's objects.
	scene, window, controller, view, layer := h.scene, h.window, h.controller, h.view, h.layer
	h.scene, h.window, h.controller, h.view, h.layer = ui.UIWindowScene{}, ui.UIWindow{}, ui.UIViewController{}, ui.UIView{}, qc.CALayer{}
	h.width, h.height, h.foreground = 0, 0, false
	h.revoke()
	if h.scene.ID == 0 {
		h.visibility()
	}
	if window.ID != 0 {
		window.SetHidden(true)
		window.SetRootViewController(ui.UIViewController{})
	}
	if layer.ID != 0 {
		layer.RemoveFromSuperlayer()
		layer.Release()
	}
	if view.ID != 0 {
		view.Release()
	}
	if controller.ID != 0 {
		controller.Release()
	}
	if window.ID != 0 {
		window.Release()
	}
	if scene.ID != 0 {
		scene.Release()
	}
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
		if !w.Destroyed() && w.state.bound == w {
			w.state.visibility()
			if !w.Destroyed() && w.state.bound == w {
				w.Source().Focused(w.state.window.ID != 0 && w.state.window.IsKeyWindow())
			}
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
