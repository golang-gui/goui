package android

import (
	"fmt"
	"image"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"unsafe"

	"github.com/goexlib/cgo"
	"github.com/golang-gui/goui/core/geometry"
	native "github.com/golang-gui/goui/platform/android/libs/android"
	"github.com/golang-gui/goui/platform/android/libs/jni"
	"github.com/golang-gui/goui/platform/android/libs/libc"
	"github.com/golang-gui/goui/platform/common"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/platform/internal/androidruntime"
	"github.com/golang-gui/goui/platform/internal/mobile"
	"github.com/golang-gui/goui/platform/internal/mobileentry"
)

// EventLoop owns the application Go queue. Android retains its system looper.
type EventLoop struct{ *mobile.EventLoop }

// Window owns the Android adapter and composes the shared Go window state.
type Window struct {
	*mobile.Window
	state *nativeState
}

// nativeState's Activity fields are confined to Android's system main thread.
// Only the command queue and desired visibility are accessed from GOUI threads.
type nativeState struct {
	activity               *native.Activity
	window                 uintptr
	width, height          int32
	generation             uint64
	density                float32
	started, initialized   bool
	input                  uintptr
	bound                  *Window
	wanted                 atomic.Bool
	mu                     sync.Mutex
	fd                     int32
	tasks                  []func()
	callbacks              native.ActivityCallbacks
	command, inputCallback uintptr
}

var androidHost = nativeState{fd: -1, density: 1}

func init() {
	mobileentry.NativeCreate = onCreate
	mobileentry.SetThread(androidruntime.EnterThread)
}
func onSystemThread(fn func()) { runtime.LockOSThread(); defer runtime.UnlockOSThread(); fn() }
func (h *nativeState) post(fn func()) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.fd < 0 {
		return common.ErrUnavailable
	}
	h.tasks = append(h.tasks, fn)
	libc.Signal(h.fd)
	return nil
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
			if h.bound != nil {
				h.bound.Source().Focused(focus != 0)
			}
		})
	})
	h.callbacks.OnNativeWindowCreated = cgo.NewCallback(func(_ uintptr, w uintptr) { wrap(func() { native.WindowGeometry(w, 0, 0, 2); h.target(w, false) }) })
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
			h.mu.Lock()
			fd := h.fd
			h.fd = -1
			h.tasks = nil
			h.mu.Unlock()
			if fd >= 0 {
				native.LooperRemoveFd(native.LooperForThread(), fd)
				libc.Close(fd)
			}
			if h.input != 0 {
				native.InputDetach(h.input)
				h.input = 0
			}
			h.activity = nil
			h.started = false
		})
	})
	h.inputCallback = cgo.NewCallback(func(int32, int32, uintptr) int32 {
		wrap(func() {
			for h.input != 0 {
				event, status := native.InputEvent(h.input)
				if status < 0 {
					break
				}
				if native.InputPreDispatch(h.input, event) != 0 {
					continue
				}
				native.InputFinish(h.input, event, 0)
			}
		})
		return 1
	})
	h.callbacks.OnInputQueueCreated = cgo.NewCallback(func(_ uintptr, q uintptr) {
		wrap(func() { h.input = q; native.InputAttach(q, native.LooperForThread(), -2, h.inputCallback, 0) })
	})
	h.callbacks.OnInputQueueDestroyed = cgo.NewCallback(func(_ uintptr, q uintptr) {
		wrap(func() {
			native.InputDetach(q)
			if h.input == q {
				h.input = 0
			}
		})
	})
	h.command = cgo.NewCallback(func(fd int32, _ int32, _ uintptr) int32 {
		wrap(func() {
			libc.Drain(fd)
			h.mu.Lock()
			tasks := h.tasks
			h.tasks = nil
			h.mu.Unlock()
			for _, task := range tasks {
				task()
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
			class := env.ObjectClass(jni.Object(a.Object))
			get := env.Method(class, "getClassLoader", "()Ljava/lang/ClassLoader;", false)
			loader := env.CallObject(jni.Object(a.Object), get)
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
			h.initialized = true
		}
		*a.Callbacks = h.callbacks
		h.mu.Lock()
		h.fd = libc.EventFD()
		fd := h.fd
		h.mu.Unlock()
		if fd < 0 || native.LooperAddFd(native.LooperForThread(), fd, -2, 1, h.command, 0) < 0 {
			panic("android: create native dispatcher")
		}
		h.configure()
		if !mobileentry.Start() {
			panic("android: application main is not registered")
		}
	})
}

func activityCacheDirectory(env jni.Env, activity jni.Object) (string, error) {
	class := env.ObjectClass(activity)
	if err := env.Exception(); err != nil {
		return "", err
	}
	if class == 0 {
		return "", fmt.Errorf("android: Activity class unavailable")
	}
	defer env.DeleteLocalRef(class)
	method := env.Method(class, "getCacheDir", "()Ljava/io/File;", false)
	if err := env.Exception(); err != nil {
		return "", err
	}
	file := env.CallObject(activity, method)
	if err := env.Exception(); err != nil {
		return "", err
	}
	if file == 0 {
		return "", fmt.Errorf("android: cache directory unavailable")
	}
	defer env.DeleteLocalRef(file)
	fileClass := env.ObjectClass(file)
	if err := env.Exception(); err != nil {
		return "", err
	}
	if fileClass == 0 {
		return "", fmt.Errorf("android: File class unavailable")
	}
	defer env.DeleteLocalRef(fileClass)
	absolute := env.Method(fileClass, "getAbsolutePath", "()Ljava/lang/String;", false)
	if err := env.Exception(); err != nil {
		return "", err
	}
	text := env.CallObject(file, absolute)
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
	native.ConfigurationFromAssets(config, h.activity.AssetManager)
	dpi := native.ConfigurationDensity(config)
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
		width = native.WindowWidth(window)
		height = native.WindowHeight(window)
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
	defer env.PopLocalFrame()
	object := jni.Object(h.activity.Object)
	class := env.ObjectClass(object)
	win := env.CallObject(object, env.Method(class, "getWindow", "()Landroid/view/Window;", false))
	wc := env.ObjectClass(win)
	decor := env.CallObject(win, env.Method(wc, "getDecorView", "()Landroid/view/View;", false))
	vc := env.ObjectClass(decor)
	visibility := int32(4)
	if h.wanted.Load() {
		visibility = 0
	}
	env.CallVoid(decor, env.Method(vc, "setVisibility", "(I)V", false), jni.Int(visibility))
	shown := env.CallInt(decor, env.Method(vc, "getVisibility", "()I", false)) == 0
	if err := env.Exception(); err != nil {
		panic(err)
	}
	if h.bound != nil {
		h.bound.Source().Visible(h.started && shown)
	}
}
func newWindow(loop *EventLoop, size geometry.Size, handler events.EventHandler, options common.WindowOptions) (*Window, error) {
	w := &Window{state: &androidHost}
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
	if native.WindowLock(window, &buffer) != 0 {
		return common.ErrUnavailable
	}
	defer func() {
		if native.WindowUnlockAndPost(window) != 0 && result == nil {
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
