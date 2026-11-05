package ios

import (
	"fmt"
	"image"
	"math"
	"runtime"
	"sync"
	"sync/atomic"

	"github.com/ebitengine/purego/objc"
	"github.com/goexlib/cgo"
	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform/common"
	"github.com/golang-gui/goui/platform/darwin/frameworks"
	cf "github.com/golang-gui/goui/platform/darwin/frameworks/core_foundation"
	cg "github.com/golang-gui/goui/platform/darwin/frameworks/core_graphics"
	. "github.com/golang-gui/goui/platform/darwin/frameworks/foundation"
	qc "github.com/golang-gui/goui/platform/darwin/frameworks/quartzcore"
	ui "github.com/golang-gui/goui/platform/darwin/frameworks/uikit"
	"github.com/golang-gui/goui/platform/darwin/libs/dispatch"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/platform/internal/mobile"
	"github.com/golang-gui/goui/platform/internal/mobileentry"
)

// EventLoop owns the Go application queue; UIKit owns the native main run loop.
type EventLoop struct{ *mobile.EventLoop }

// Window adapts UIKit's content window without exposing a native host protocol.
type Window struct {
	*mobile.Window
	state *nativeState
}

// PrepareEAGLLayer makes the UIKit-owned drawable layer visible. Call this
// before acquiring a drawing target or GPU lock: it may wait for the main thread.
func (w *Window) PrepareEAGLLayer() error {
	var result error
	if err := w.state.syncMain(func() {
		if w.state.layer.ID == 0 {
			result = common.ErrUnavailable
			return
		}
		w.state.layer.SetHidden(false)
	}); err != nil {
		return err
	}
	return result
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
	running       atomic.Bool
	queueMu       sync.Mutex
	tasks         []func()
	scheduled     bool
	callback      uintptr
	// The GPU gate also covers offscreen work. Background entry drains it first.
	gpu             sync.Mutex
	allowed         bool
	imageMu         sync.Mutex
	pending         cg.CGImageRef
	imageGeneration uint64
	imageHandle     uintptr
	imageScheduled  bool
}

var iosHost nativeState

func init()                    { mobileentry.NativeRun = runNative }
func onSystemThread(fn func()) { runtime.LockOSThread(); defer runtime.UnlockOSThread(); fn() }
func (h *nativeState) post(fn func()) error {
	if !h.running.Load() {
		return common.ErrUnavailable
	}
	h.queueMu.Lock()
	defer h.queueMu.Unlock()
	h.tasks = append(h.tasks, fn)
	if !h.scheduled {
		h.scheduled = true
		dispatch.MainAsync(h.callback)
	}
	return nil
}
func (h *nativeState) syncMain(fn func()) error {
	if dispatch.IsMainThread() {
		fn()
		return nil
	}
	done := make(chan struct{})
	if err := h.post(func() { defer close(done); fn() }); err != nil {
		return err
	}
	<-done
	return nil
}
func runNative() int {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if !dispatch.IsMainThread() {
		panic("ios: native entry must run on the system main thread")
	}
	if err := frameworks.InitIOS(); err != nil {
		panic(err)
	}
	h := &iosHost
	h.callback = cgo.NewCallback(func(uintptr) {
		onSystemThread(func() {
			h.queueMu.Lock()
			tasks := h.tasks
			h.tasks = nil
			h.scheduled = false
			h.queueMu.Unlock()
			AutoReleasePool(func() {
				for _, task := range tasks {
					task()
				}
			})
		})
	})
	if err := h.registerClasses(); err != nil {
		panic(err)
	}
	h.running.Store(true)
	var result int
	AutoReleasePool(func() {
		name := NSStringClassId.Alloc().InitWithBytes([]byte("GoUIDelegate"), NSUTF8StringEncoding)
		defer name.Release()
		result = int(ui.UIApplicationMain(0, 0, NSString{}, name))
	})
	return result
}
func (h *nativeState) registerClasses() error {
	register := func(name, super string, methods []objc.MethodDef) error {
		var protocols []*objc.Protocol
		protocol := ""
		if name == "GoUIScene" {
			protocol = "UIWindowSceneDelegate"
		}
		if name == "GoUIDelegate" {
			protocol = "UIApplicationDelegate"
		}
		if protocol != "" {
			p := objc.GetProtocol(protocol)
			if p == nil {
				return fmt.Errorf("ios: missing protocol %s", protocol)
			}
			protocols = []*objc.Protocol{p}
		}
		_, err := objc.RegisterClass(name, objc.GetClass(super), protocols, nil, methods)
		return err
	}
	layout := objc.RegisterName("viewDidLayoutSubviews")
	if err := register("GoUIContent", "UIViewController", []objc.MethodDef{{Cmd: layout, Fn: func(self objc.ID, cmd objc.SEL) {
		onSystemThread(func() { objc.SendSuper[uintptr](self, cmd); h.target(false) })
	}}}); err != nil {
		return err
	}
	if err := register("GoUIDelegate", "UIResponder", []objc.MethodDef{{Cmd: objc.RegisterName("application:didFinishLaunchingWithOptions:"), Fn: func(objc.ID, objc.SEL, objc.ID, objc.ID) bool { return mobileentry.Start() }}}); err != nil {
		return err
	}
	return register("GoUIScene", "UIResponder", []objc.MethodDef{
		{Cmd: objc.RegisterName("scene:willConnectToSession:options:"), Fn: func(_ objc.ID, _ objc.SEL, scene objc.ID, _ objc.ID, _ objc.ID) {
			onSystemThread(func() { h.connect(scene) })
		}},
		{Cmd: objc.RegisterName("sceneWillEnterForeground:"), Fn: func(objc.ID, objc.SEL, objc.ID) {
			onSystemThread(func() {
				h.foreground = true
				h.gpu.Lock()
				h.allowed = true
				h.gpu.Unlock()
				h.target(true)
				h.visibility()
			})
		}},
		{Cmd: objc.RegisterName("sceneDidEnterBackground:"), Fn: func(objc.ID, objc.SEL, objc.ID) {
			onSystemThread(func() { h.suspend(); h.foreground = false; h.visibility() })
		}},
		{Cmd: objc.RegisterName("sceneDidBecomeActive:"), Fn: func(objc.ID, objc.SEL, objc.ID) {
			onSystemThread(func() {
				if h.bound != nil {
					h.bound.Source().Focused(h.window.IsKeyWindow())
				}
			})
		}},
		{Cmd: objc.RegisterName("sceneWillResignActive:"), Fn: func(objc.ID, objc.SEL, objc.ID) {
			onSystemThread(func() {
				if h.bound != nil {
					h.bound.Source().Focused(false)
				}
			})
		}},
		{Cmd: objc.RegisterName("sceneDidDisconnect:"), Fn: func(objc.ID, objc.SEL, objc.ID) { onSystemThread(func() { h.disconnect() }) }},
	})
}
func (h *nativeState) connect(scene objc.ID) {
	if h.window.ID != 0 {
		h.disconnect()
	}
	h.window = ui.UIWindowClassId.Alloc().InitWithWindowScene(scene)
	class := ui.UIViewControllerClass{}
	class.Class = objc.GetClass("GoUIContent")
	h.controller = Cast[ui.UIViewController](class.Alloc().Send(NSObjectSel.Init))
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
func (h *nativeState) clearImage() {
	h.imageMu.Lock()
	if h.pending != 0 {
		cg.CGImageRelease(h.pending)
		h.pending = 0
	}
	h.imageMu.Unlock()
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
func newWindow(loop *EventLoop, size geometry.Size, handler events.EventHandler, options common.WindowOptions) (*Window, error) {
	w := &Window{state: &iosHost}
	base, err := mobile.NewWindow(loop.EventLoop, size, handler, options, windowHost{w})
	if err != nil {
		return nil, err
	}
	w.Window = base
	if err = w.state.post(func() { w.state.bound = w; w.state.publishTarget(); w.state.visibility() }); err != nil {
		return nil, err
	}
	return w, nil
}

type windowHost struct{ window *Window }

func (a windowHost) SetVisible(value bool) error {
	w := a.window
	w.state.wanted.Store(value)
	return w.state.post(w.state.visibility)
}
func (a windowHost) Close() {
	w := a.window
	_ = w.state.post(func() {
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
func (h *nativeState) present(handle uintptr, generation uint64, img image.Image) error {
	bitmap, ok := graphics.ToBitmap(img, graphics.PixelFormatRGBA)
	if !ok {
		bitmap = graphics.CopyToBitmap(img, graphics.PixelFormatRGBA, nil)
	}
	data := cf.CFDataCreate(0, bitmap.Pixels)
	if data == 0 {
		return fmt.Errorf("ios: allocate image data")
	}
	defer cf.CFRelease(cf.CFTypeRef(data))
	provider := cg.CGDataProviderCreateWithCFData(data)
	if provider == 0 {
		return fmt.Errorf("ios: allocate image provider")
	}
	defer cg.CGDataProviderRelease(provider)
	space := cg.CGColorSpaceCreateDeviceRGB()
	if space == 0 {
		return fmt.Errorf("ios: allocate color space")
	}
	defer cg.CGColorSpaceRelease(space)
	// Premultiplied RGBA bytes, big-endian component order. CFData copied the input.
	image := cg.CGImageCreate(uint(bitmap.Width), uint(bitmap.Height), 8, 32, uint(bitmap.Stride), space, cg.CGBitmapInfo(0x4001), provider, nil, false, 0)
	if image == 0 {
		return fmt.Errorf("ios: create image")
	}
	h.imageMu.Lock()
	if h.pending != 0 {
		cg.CGImageRelease(h.pending)
	}
	h.pending = image
	h.imageHandle = handle
	h.imageGeneration = generation
	schedule := !h.imageScheduled
	h.imageScheduled = true
	h.imageMu.Unlock()
	if schedule {
		if err := h.post(h.commitImage); err != nil {
			h.imageMu.Lock()
			h.imageScheduled = false
			if h.pending != 0 {
				cg.CGImageRelease(h.pending)
				h.pending = 0
			}
			h.imageMu.Unlock()
			return err
		}
	}
	return nil
}
func (h *nativeState) commitImage() {
	h.imageMu.Lock()
	img, handle, generation := h.pending, h.imageHandle, h.imageGeneration
	h.pending = 0
	h.imageScheduled = false
	h.imageMu.Unlock()
	if img == 0 {
		return
	}
	defer cg.CGImageRelease(img)
	if h.bound != nil && h.view.ID != 0 && uintptr(h.view.ID) == handle && h.generation == generation {
		qc.CATransactionClassId.Begin()
		qc.CATransactionClassId.SetDisableActions(true)
		Cast[qc.CALayer](h.view.Layer()).SetContents(objc.ID(img))
		qc.CATransactionClassId.Commit()
	}
}
