// Package android binds libandroid's public NativeActivity, window and looper APIs.
package android

import (
	"github.com/goexlib/cgo"
	"runtime"
	"unsafe"
)

type Activity struct {
	Callbacks                          *ActivityCallbacks
	VM, Env, Object                    uintptr
	InternalDataPath, ExternalDataPath uintptr
	SDKVersion                         int32
	_                                  int32
	Instance, AssetManager, OBBPath    uintptr
}
type ActivityCallbacks struct {
	OnStart, OnResume, OnSaveInstanceState, OnPause, OnStop, OnDestroy uintptr
	OnWindowFocusChanged, OnNativeWindowCreated, OnNativeWindowResized uintptr
	OnNativeWindowRedrawNeeded, OnNativeWindowDestroyed                uintptr
	OnInputQueueCreated, OnInputQueueDestroyed, OnContentRectChanged   uintptr
	OnConfigurationChanged, OnLowMemory                                uintptr
}
type WindowBuffer struct {
	Width, Height, Stride, Format int32
	Bits                          uintptr
	Reserved                      [6]uint32
}

var library = cgo.NewLazyLibrary("libandroid.so")

var fnWindowWidth = library.NewSymbol("ANativeWindow_getWidth")

func WindowWidth(window uintptr) int32 { return cgo.CallRet[int32](fnWindowWidth.Addr(), window) }

var fnWindowHeight = library.NewSymbol("ANativeWindow_getHeight")

func WindowHeight(window uintptr) int32 { return cgo.CallRet[int32](fnWindowHeight.Addr(), window) }

var fnWindowGeometry = library.NewSymbol("ANativeWindow_setBuffersGeometry")

func WindowGeometry(window uintptr, width, height, format int32) int32 {
	return cgo.CallRet[int32](fnWindowGeometry.Addr(), window, width, height, format)
}

var fnWindowUnlockAndPost = library.NewSymbol("ANativeWindow_unlockAndPost")

func WindowUnlockAndPost(window uintptr) int32 {
	return cgo.CallRet[int32](fnWindowUnlockAndPost.Addr(), window)
}

var fnLooperForThread = library.NewSymbol("ALooper_forThread")

func LooperForThread() uintptr { return cgo.CallRet[uintptr](fnLooperForThread.Addr()) }

var fnLooperAddFd = library.NewSymbol("ALooper_addFd")

func LooperAddFd(looper uintptr, fd, ident, events int32, callback, data uintptr) int32 {
	return cgo.CallRet[int32](fnLooperAddFd.Addr(), looper, fd, ident, events, callback, data)
}

var fnLooperRemoveFd = library.NewSymbol("ALooper_removeFd")

func LooperRemoveFd(looper uintptr, fd int32) int32 {
	return cgo.CallRet[int32](fnLooperRemoveFd.Addr(), looper, fd)
}

var fnConfigurationNew = library.NewSymbol("AConfiguration_new")

func ConfigurationNew() uintptr { return cgo.CallRet[uintptr](fnConfigurationNew.Addr()) }

var fnConfigurationFromAssets = library.NewSymbol("AConfiguration_fromAssetManager")

func ConfigurationFromAssets(config, assets uintptr) {
	cgo.Call(fnConfigurationFromAssets.Addr(), config, assets)
}

var fnConfigurationDensity = library.NewSymbol("AConfiguration_getDensity")

func ConfigurationDensity(config uintptr) int32 {
	return cgo.CallRet[int32](fnConfigurationDensity.Addr(), config)
}

var fnConfigurationDelete = library.NewSymbol("AConfiguration_delete")

func ConfigurationDelete(config uintptr) { cgo.Call(fnConfigurationDelete.Addr(), config) }

var fnInputPreDispatch = library.NewSymbol("AInputQueue_preDispatchEvent")

func InputPreDispatch(queue, event uintptr) int32 {
	return cgo.CallRet[int32](fnInputPreDispatch.Addr(), queue, event)
}

var fnInputFinish = library.NewSymbol("AInputQueue_finishEvent")

func InputFinish(queue, event uintptr, handled int32) {
	cgo.Call(fnInputFinish.Addr(), queue, event, handled)
}

var fnInputAttach = library.NewSymbol("AInputQueue_attachLooper")

func InputAttach(queue, looper uintptr, ident int32, callback, data uintptr) {
	cgo.Call(fnInputAttach.Addr(), queue, looper, ident, callback, data)
}

var fnInputDetach = library.NewSymbol("AInputQueue_detachLooper")

func InputDetach(queue uintptr) { cgo.Call(fnInputDetach.Addr(), queue) }

var windowLock = library.NewSymbol("ANativeWindow_lock")

func WindowLock(window uintptr, buffer *WindowBuffer) int32 {
	var pin runtime.Pinner
	pin.Pin(buffer)
	defer pin.Unpin()
	return cgo.CallRet[int32](windowLock.Addr(), window, uintptr(unsafe.Pointer(buffer)), uintptr(0))
}

var getInput = library.NewSymbol("AInputQueue_getEvent")

func InputEvent(queue uintptr) (uintptr, int32) {
	var event uintptr
	var pin runtime.Pinner
	pin.Pin(&event)
	defer pin.Unpin()
	status := cgo.CallRet[int32](getInput.Addr(), queue, uintptr(unsafe.Pointer(&event)))
	return event, status
}

var acquire = library.NewSymbol("ANativeWindow_acquire")
var release = library.NewSymbol("ANativeWindow_release")

func WindowAcquire(window uintptr) { cgo.Call(acquire.Addr(), window) }
func WindowRelease(window uintptr) { cgo.Call(release.Addr(), window) }
