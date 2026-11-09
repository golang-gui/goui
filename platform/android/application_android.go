package android

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"unsafe"

	"github.com/goexlib/cgo"
	native "github.com/golang-gui/goui/platform/android/libs/android"
	"github.com/golang-gui/goui/platform/android/libs/jni"
	"github.com/golang-gui/goui/platform/android/sdk"
	"github.com/golang-gui/goui/platform/internal/androidruntime"
	"github.com/golang-gui/goui/platform/internal/mobileentry"
)

// nativeApplication owns the process bridge and the currently adopted Activity.
// All fields except dispatcher atomics are confined to the native UI thread.
type nativeApplication struct {
	dispatcher    nativeDispatcher
	activity      *activityState
	callbacks     native.ActivityCallbacks
	inputCallback uintptr
	initialized   bool
	bound         *Window
	target        drawingTarget
	onHostReady   func()
}

// activityState borrows system objects only until the matching destroy callback.
type activityState struct {
	native           *native.Activity
	input            uintptr
	started, focused bool
	density          float32
}

var androidApplication nativeApplication

func init() {
	mobileentry.NativeCreate = onCreate
	mobileentry.SetThread(androidruntime.EnterThread)
}

// withNativeThread anchors a callback that Android already delivers on its main
// thread. It never posts to another thread or waits for the UI loop.
func withNativeThread(fn func()) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if syscall.Gettid() != syscall.Getpid() {
		panic("android: native callback outside the system main thread")
	}
	fn()
}

func onCreate(address, _ uintptr, _ uintptr) {
	withNativeThread(func() {
		h := &androidApplication
		n := (*native.Activity)(cgo.Pointer(address))
		if n == nil || n.Callbacks == nil {
			panic("android: invalid native Activity")
		}
		if !h.initialized {
			if err := h.initialize(n); err != nil {
				panic(err)
			}
		}
		if previous := h.activity; previous != nil {
			h.detachActivity(previous)
		}
		a := &activityState{native: n, density: 1}
		h.activity = a
		*n.Callbacks = h.callbacks
		h.configure(a)
		if h.onHostReady != nil {
			h.onHostReady()
		}
		if !mobileentry.Start() {
			panic("android: application main is not registered")
		}
	})
}

func (h *nativeApplication) initialize(n *native.Activity) error {
	env := jni.Env(n.Env)
	if err := sdk.Init(env); err != nil {
		return err
	}
	loader, err := sdk.Activity(n.Object).GetClassLoader(env)
	if err != nil {
		return err
	}
	if loader == 0 {
		return fmt.Errorf("android: Activity class loader unavailable")
	}
	defer env.DeleteLocalRef(jni.Object(loader))
	global := env.NewGlobalRef(jni.Object(loader))
	if err := env.Exception(); err != nil {
		return err
	}
	if global == 0 {
		return fmt.Errorf("android: retain class loader failed")
	}
	if err := androidruntime.Initialize(jni.VM(n.VM), sdk.ClassLoader(global)); err != nil {
		env.DeleteGlobalRef(global)
		return err
	}
	if os.Getenv("TMPDIR") == "" {
		cache, err := activityCacheDirectory(env, sdk.Activity(n.Object))
		if err != nil {
			return err
		}
		if err := os.Setenv("TMPDIR", cache); err != nil {
			return err
		}
	}
	if err := h.dispatcher.initialize(); err != nil {
		return err
	}
	h.initCallbacks()
	h.initialized = true
	return nil
}
func activityCacheDirectory(env jni.Env, activity sdk.Activity) (string, error) {
	file, err := activity.GetCacheDir(env)
	if err != nil {
		return "", err
	}
	if file == 0 {
		return "", fmt.Errorf("android: cache directory unavailable")
	}
	defer env.DeleteLocalRef(jni.Object(file))
	path, err := file.GetAbsolutePath(env)
	if err != nil {
		return "", err
	}
	if !filepath.IsAbs(path) {
		return "", fmt.Errorf("android: invalid cache directory")
	}
	return path, nil
}

func (h *nativeApplication) activityFor(address uintptr) *activityState {
	a := h.activity
	if a == nil || uintptr(unsafe.Pointer(a.native)) != address {
		return nil
	}
	return a
}
func (h *nativeApplication) initCallbacks() {
	h.callbacks.OnStart = cgo.NewCallback(h.onStart)
	h.callbacks.OnStop = cgo.NewCallback(h.onStop)
	h.callbacks.OnDestroy = cgo.NewCallback(h.onDestroy)
	h.callbacks.OnWindowFocusChanged = cgo.NewCallback(h.onFocus)
	h.callbacks.OnConfigurationChanged = cgo.NewCallback(h.onConfiguration)
	h.callbacks.OnNativeWindowCreated = cgo.NewCallback(h.onWindowCreated)
	h.callbacks.OnNativeWindowResized = cgo.NewCallback(h.onWindowResized)
	h.callbacks.OnNativeWindowRedrawNeeded = cgo.NewCallback(h.onWindowRedraw)
	h.callbacks.OnNativeWindowDestroyed = cgo.NewCallback(h.onWindowDestroyed)
	h.callbacks.OnInputQueueCreated = cgo.NewCallback(h.onInputCreated)
	h.callbacks.OnInputQueueDestroyed = cgo.NewCallback(h.onInputDestroyed)
	h.inputCallback = cgo.NewCallback(h.onInput)
}
func (h *nativeApplication) onStart(address uintptr) {
	withNativeThread(func() {
		if a := h.activityFor(address); a != nil {
			a.started = true
			h.applyVisibility()
		}
	})
}
func (h *nativeApplication) onStop(address uintptr) {
	withNativeThread(func() {
		if a := h.activityFor(address); a != nil {
			a.started = false
			if w := h.bound; w != nil {
				w.Source().Visible(false)
			}
		}
	})
}
func (h *nativeApplication) onFocus(address uintptr, focused int32) {
	withNativeThread(func() {
		if a := h.activityFor(address); a != nil {
			a.focused = focused != 0
			if w := h.bound; w != nil {
				w.Source().Focused(a.focused)
			}
		}
	})
}
func (h *nativeApplication) onConfiguration(address uintptr) {
	withNativeThread(func() {
		if a := h.activityFor(address); a != nil {
			h.configure(a)
		}
	})
}
func (h *nativeApplication) onDestroy(address uintptr) {
	withNativeThread(func() {
		if a := h.activityFor(address); a != nil {
			h.detachActivity(a)
		}
	})
}
func (h *nativeApplication) detachActivity(a *activityState) {
	if h.activity != a {
		return
	}
	h.activity = nil
	a.detachInput()
	h.updateTarget(0, false)
	if w := h.bound; w != nil {
		w.Source().Visible(false)
		w.Source().Focused(false)
	}
}
func (h *nativeApplication) configure(a *activityState) {
	if h.activity != a {
		return
	}
	config := native.ConfigurationNew()
	if config == 0 {
		panic("android: read configuration")
	}
	native.ConfigurationFromAssetManager(config, a.native.AssetManager)
	dpi := native.ConfigurationGetDensity(config)
	native.ConfigurationDelete(config)
	density := float32(1)
	if dpi > 0 && dpi < native.ConfigurationDensityAny {
		density = float32(dpi) / float32(native.ConfigurationDensityDefault)
	}
	if density != a.density {
		a.density = density
		h.updateTarget(h.target.handle, true)
	}
}

func (a *activityState) setVisible(visible bool) (bool, error) {
	env := jni.Env(a.native.Env)
	if env.PushLocalFrame(8) != jni.OK {
		if err := env.Exception(); err != nil {
			return false, err
		}
		return false, fmt.Errorf("android: create Activity local frame")
	}
	defer env.PopLocalFrame(0)
	window, err := sdk.Activity(a.native.Object).GetWindow(env)
	if err != nil {
		return false, err
	}
	if window == 0 {
		return false, fmt.Errorf("android: Activity window unavailable")
	}
	view, err := window.GetDecorView(env)
	if err != nil {
		return false, err
	}
	if view == 0 {
		return false, fmt.Errorf("android: Activity decor view unavailable")
	}
	value := sdk.ViewInvisible
	if visible {
		value = sdk.ViewVisible
	}
	if err := view.SetVisibility(env, value); err != nil {
		return false, err
	}
	actual, err := view.GetVisibility(env)
	return actual == sdk.ViewVisible, err
}
