package android

import (
	"fmt"
	"image"
	"os"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"syscall"
	"unsafe"

	"github.com/goexlib/cgo"
	native "github.com/golang-gui/goui/platform/android/libs/android"
	"github.com/golang-gui/goui/platform/android/libs/jni"
	"github.com/golang-gui/goui/platform/common"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/platform/internal/androidruntime"
	"github.com/golang-gui/goui/platform/internal/eventfd"
	"github.com/golang-gui/goui/platform/internal/mobile"
	"github.com/golang-gui/goui/platform/internal/mobileentry"
)

// EventLoop dispatches Go work through Android's system main looper.
type EventLoop struct{ *mobile.EventLoop }

// Window owns the Android adapter and composes the shared Go window state.
type Window struct {
	*mobile.Window
	state *nativeState
}

// Activity fields belong to Android's system main thread. The process dispatcher
// survives Activity replacement; its queue is owned by the shared EventLoop.
type nativeState struct {
	activity               *native.Activity
	window                 uintptr
	width, height          int32
	generation             uint64
	density                float32
	started, initialized   bool
	input                  uintptr
	focused                bool
	bound                  *Window
	onHostReady            func()
	wanted                 atomic.Bool
	wakeup                 *eventfd.EventFD
	loop                   atomic.Pointer[mobile.EventLoop]
	thread                 atomic.Int64
	callbacks              native.ActivityCallbacks
	command, inputCallback uintptr
}

var androidHost = nativeState{density: 1}

func init() {
	mobileentry.NativeCreate = onCreate
	mobileentry.SetThread(androidruntime.EnterThread)
}
func onSystemThread(fn func()) { runtime.LockOSThread(); defer runtime.UnlockOSThread(); fn() }

func (h *nativeState) isCurrent() bool { return h.thread.Load() == int64(syscall.Gettid()) }

func (h *nativeState) wake() {
	if err := h.wakeup.Signal(); err != nil {
		panic(fmt.Errorf("android: wake system main looper: %w", err))
	}
}
func (h *nativeState) initCallbacks() {
	wrap := func(fn func()) { onSystemThread(fn) }
	h.callbacks.OnStart = cgo.NewCallback(func(uintptr) { wrap(func() { h.started = true; h.visibility() }) })
	h.callbacks.OnStop = cgo.NewCallback(func(uintptr) {
		wrap(func() {
			h.started = false
			if h.bound != nil {
				h.bound.Source().Visible(false)
			}
		})
	})
	h.callbacks.OnWindowFocusChanged = cgo.NewCallback(func(_ uintptr, focus int32) {
		wrap(func() {
			h.focused = focus != 0
			if h.bound != nil {
				h.bound.Source().Focused(h.focused)
			}
		})
	})
	h.callbacks.OnNativeWindowCreated = cgo.NewCallback(func(_ uintptr, w uintptr) {
		wrap(func() {
			native.NativeWindowSetBuffersGeometry(w, 0, 0, native.WindowFormatRGBX8888)
			h.target(w, false)
		})
	})
	h.callbacks.OnNativeWindowResized = cgo.NewCallback(func(_ uintptr, w uintptr) { wrap(func() { h.target(w, false) }) })
	h.callbacks.OnNativeWindowRedrawNeeded = cgo.NewCallback(func(_ uintptr, w uintptr) {
		wrap(func() {
			h.target(w, false)
			if h.bound != nil {
				h.bound.Source().Redraw()
			}
		})
	})
	h.callbacks.OnNativeWindowDestroyed = cgo.NewCallback(func(_ uintptr, w uintptr) {
		wrap(func() {
			if h.window == w {
				h.target(0, false)
			}
		})
	})
	h.callbacks.OnConfigurationChanged = cgo.NewCallback(func(uintptr) { wrap(func() { h.configure() }) })
	h.callbacks.OnDestroy = cgo.NewCallback(func(uintptr) {
		wrap(func() {
			h.target(0, false)
			if h.bound != nil {
				h.bound.Source().Visible(false)
			}
			if h.input != 0 {
				native.InputQueueDetachLooper(h.input)
				h.input = 0
			}
			h.focused = false
			h.activity = nil
			h.started = false
		})
	})
	h.inputCallback = cgo.NewCallback(func(int32, int32, uintptr) int32 {
		wrap(func() {
			for h.input != 0 {
				event, status := native.InputQueueGetEvent(h.input)
				if status < 0 {
					break
				}
				if native.InputQueuePreDispatchEvent(h.input, event) != 0 {
					continue
				}
				native.InputQueueFinishEvent(h.input, event, 0)
			}
		})
		return 1
	})
	h.callbacks.OnInputQueueCreated = cgo.NewCallback(func(_ uintptr, q uintptr) {
		wrap(func() {
			h.input = q
			native.InputQueueAttachLooper(q, native.LooperForThread(), native.LooperPollCallback, h.inputCallback, 0)
		})
	})
	h.callbacks.OnInputQueueDestroyed = cgo.NewCallback(func(_ uintptr, q uintptr) {
		wrap(func() {
			native.InputQueueDetachLooper(q)
			if h.input == q {
				h.input = 0
			}
		})
	})
	h.command = cgo.NewCallback(func(_ int32, _ int32, _ uintptr) int32 {
		wrap(func() {
			if err := h.wakeup.Drain(); err != nil {
				panic(fmt.Errorf("android: drain main looper wakeup: %w", err))
			}
			if loop := h.loop.Load(); loop != nil {
				loop.Dispatch()
			}
		})
		return 1
	})
}
func onCreate(address, saved, size uintptr) {
	onSystemThread(func() {
		h := &androidHost
		h.activity = (*native.Activity)(cgo.Pointer(address))
		h.started = false
		a := h.activity
		if !h.initialized {
			env := jni.Env(a.Env)
			class := env.GetObjectClass(jni.Object(a.Object))
			get := env.GetMethodID(class, "getClassLoader", "()Ljava/lang/ClassLoader;")
			loader := env.CallObjectMethodA(jni.Object(a.Object), get)
			if err := env.Exception(); err != nil || loader == 0 {
				panic(fmt.Sprintf("android: class loader: %v", err))
			}
			global := env.NewGlobalRef(loader)
			env.DeleteLocalRef(loader)
			env.DeleteLocalRef(class)
			if global == 0 {
				panic("android: retain class loader")
			}
			androidruntime.Initialize(jni.VM(a.VM), global)
			// Android does not provide a writable Unix /tmp. Use the actual
			// Context cache directory before application main calls os.TempDir.
			if os.Getenv("TMPDIR") == "" {
				cache, err := activityCacheDirectory(env, jni.Object(a.Object))
				if err != nil {
					panic(err)
				}
				if err = os.Setenv("TMPDIR", cache); err != nil {
					panic(err)
				}
			}
			h.initCallbacks()
			h.thread.Store(int64(syscall.Gettid()))
			var err error
			h.wakeup, err = eventfd.New()
			if err != nil {
				panic(fmt.Errorf("android: create native dispatcher: %w", err))
			}
			if native.LooperAddFd(native.LooperForThread(), int32(h.wakeup.FD()), native.LooperPollCallback, native.LooperEventInput, h.command, 0) < 0 {
				_ = h.wakeup.Close()
				h.wakeup = nil
				panic("android: register native dispatcher")
			}
			h.initialized = true
		}
		*a.Callbacks = h.callbacks
		h.configure()
		if h.onHostReady != nil {
			h.onHostReady()
		}
		if !mobileentry.Start() {
			panic("android: application main is not registered")
		}
	})
}

func activityCacheDirectory(env jni.Env, activity jni.Object) (string, error) {
	class := env.GetObjectClass(activity)
	if err := env.Exception(); err != nil {
		return "", err
	}
	if class == 0 {
		return "", fmt.Errorf("android: Activity class unavailable")
	}
	defer env.DeleteLocalRef(class)
	method := env.GetMethodID(class, "getCacheDir", "()Ljava/io/File;")
	if err := env.Exception(); err != nil {
		return "", err
	}
	file := env.CallObjectMethodA(activity, method)
	if err := env.Exception(); err != nil {
		return "", err
	}
	if file == 0 {
		return "", fmt.Errorf("android: cache directory unavailable")
	}
	defer env.DeleteLocalRef(file)
	fileClass := env.GetObjectClass(file)
	if err := env.Exception(); err != nil {
		return "", err
	}
	if fileClass == 0 {
		return "", fmt.Errorf("android: File class unavailable")
	}
	defer env.DeleteLocalRef(fileClass)
	absolute := env.GetMethodID(fileClass, "getAbsolutePath", "()Ljava/lang/String;")
	if err := env.Exception(); err != nil {
		return "", err
	}
	text := env.CallObjectMethodA(file, absolute)
	if err := env.Exception(); err != nil {
		return "", err
	}
	if text == 0 {
		return "", fmt.Errorf("android: cache path unavailable")
	}
	defer env.DeleteLocalRef(text)
	path := env.String(text)
	if err := env.Exception(); err != nil {
		return "", err
	}
	if !filepath.IsAbs(path) {
		return "", fmt.Errorf("android: invalid cache directory")
	}
	return path, nil
}
func (h *nativeState) configure() {
	config := native.ConfigurationNew()
	if config == 0 {
		panic("android: read configuration")
	}
	native.ConfigurationFromAssetManager(config, h.activity.AssetManager)
	dpi := native.ConfigurationGetDensity(config)
	native.ConfigurationDelete(config)
	density := float32(1)
	if dpi > 0 && dpi < 0xfffe {
		density = float32(dpi) / 160
	}
	if density != h.density {
		h.density = density
		h.target(h.window, true)
	}
}
func (h *nativeState) target(window uintptr, force bool) {
	var width, height int32
	if window != 0 {
		width = native.NativeWindowGetWidth(window)
		height = native.NativeWindowGetHeight(window)
	}
	if !force && window == h.window && width == h.width && height == h.height {
		return
	}
	if h.bound != nil {
		h.bound.Source().Revoke()
	}
	h.window, h.width, h.height = window, width, height
	h.generation++
	h.publishTarget()
}
func (h *nativeState) publishTarget() {
	if h.bound == nil || h.window == 0 || h.width <= 0 || h.height <= 0 {
		return
	}
	handle := h.window
	h.bound.Source().Attach(handle, h.generation, h.width, h.height, h.density, func(img image.Image) error { return present(handle, img) })
}
func (h *nativeState) visibility() {
	if h.activity == nil {
		return
	}
	env := jni.Env(h.activity.Env)
	if env.PushLocalFrame(12) < 0 {
		panic("android: JNI local frame")
	}
	defer env.PopLocalFrame(0)
	object := jni.Object(h.activity.Object)
	class := env.GetObjectClass(object)
	win := env.CallObjectMethodA(object, env.GetMethodID(class, "getWindow", "()Landroid/view/Window;"))
	wc := env.GetObjectClass(win)
	decor := env.CallObjectMethodA(win, env.GetMethodID(wc, "getDecorView", "()Landroid/view/View;"))
	vc := env.GetObjectClass(decor)
	visibility := int32(4)
	if h.wanted.Load() {
		visibility = 0
	}
	env.CallVoidMethodA(decor, env.GetMethodID(vc, "setVisibility", "(I)V"), jni.Int(visibility))
	shown := env.CallIntMethodA(decor, env.GetMethodID(vc, "getVisibility", "()I")) == 0
	if err := env.Exception(); err != nil {
		panic(err)
	}
	if h.bound != nil {
		h.bound.Source().Visible(h.started && shown)
	}
}
func newWindow(loop *EventLoop, handler events.EventHandler) (*Window, error) {
	w := &Window{state: &androidHost}
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
			w.Source().Focused(w.state.focused)
		}
	})
}

type windowHost struct{ window *Window }

func (a windowHost) SetVisible(value bool) error {
	w := a.window
	w.state.wanted.Store(value)
	w.state.loop.Load().Deliver(w.state.visibility)
	return nil
}
func (a windowHost) Close() {
	w := a.window
	w.state.loop.Load().Deliver(func() {
		if w.state.bound == w {
			w.state.bound = nil
			w.state.wanted.Store(false)
			w.state.visibility()
		}
	})
}
func present(window uintptr, img image.Image) (result error) {
	pixels, ok := graphics.ToBitmap(img, graphics.PixelFormatRGBA)
	if !ok {
		pixels = graphics.CopyToBitmap(img, graphics.PixelFormatRGBA, nil)
	}
	var buffer native.WindowBuffer
	if native.NativeWindowLock(window, &buffer, nil) != 0 {
		return common.ErrUnavailable
	}
	defer func() {
		if native.NativeWindowUnlockAndPost(window) != 0 && result == nil {
			result = common.ErrUnavailable
		}
	}()
	if buffer.Bits == 0 || buffer.Width <= 0 || buffer.Height <= 0 || int64(buffer.Stride)*int64(buffer.Height) > int64(^uint(0)>>1)/4 || buffer.Stride < buffer.Width || (buffer.Format != 1 && buffer.Format != 2) {
		return fmt.Errorf("android: invalid native window buffer")
	}
	dst := unsafe.Slice((*byte)(cgo.Pointer(buffer.Bits)), int(buffer.Stride)*int(buffer.Height)*4)
	columns, rows := min(pixels.Width, int(buffer.Width)), min(pixels.Height, int(buffer.Height))
	for y := 0; y < int(buffer.Height); y++ {
		row := dst[y*int(buffer.Stride)*4:][:int(buffer.Width)*4]
		clear(row)
		if y < rows {
			copy(row[:columns*4], pixels.Pixels[y*pixels.Stride:][:columns*4])
		}
		for x := 0; x < int(buffer.Width); x++ {
			row[x*4+3] = 255
		}
	}
	return nil
}
