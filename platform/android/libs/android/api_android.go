package android

import "github.com/goexlib/cgo"

var (
	libandroid                      = cgo.NewLazyLibrary("libandroid.so")
	aNativeWindowGetWidth           = libandroid.NewSymbol("ANativeWindow_getWidth")
	aNativeWindowGetHeight          = libandroid.NewSymbol("ANativeWindow_getHeight")
	aNativeWindowSetBuffersGeometry = libandroid.NewSymbol("ANativeWindow_setBuffersGeometry")
	aNativeWindowUnlockAndPost      = libandroid.NewSymbol("ANativeWindow_unlockAndPost")
	aNativeWindowAcquire            = libandroid.NewSymbol("ANativeWindow_acquire")
	aNativeWindowRelease            = libandroid.NewSymbol("ANativeWindow_release")
	aLooperForThread                = libandroid.NewSymbol("ALooper_forThread")
	aLooperAddFd                    = libandroid.NewSymbol("ALooper_addFd")
	aLooperRemoveFd                 = libandroid.NewSymbol("ALooper_removeFd")
	aConfigurationNew               = libandroid.NewSymbol("AConfiguration_new")
	aConfigurationFromAssetManager  = libandroid.NewSymbol("AConfiguration_fromAssetManager")
	aConfigurationGetDensity        = libandroid.NewSymbol("AConfiguration_getDensity")
	aConfigurationDelete            = libandroid.NewSymbol("AConfiguration_delete")
	aInputQueuePreDispatchEvent     = libandroid.NewSymbol("AInputQueue_preDispatchEvent")
	aInputQueueFinishEvent          = libandroid.NewSymbol("AInputQueue_finishEvent")
	aInputQueueAttachLooper         = libandroid.NewSymbol("AInputQueue_attachLooper")
	aInputQueueDetachLooper         = libandroid.NewSymbol("AInputQueue_detachLooper")
	aNativeWindowLock               = libandroid.NewSymbol("ANativeWindow_lock")
	aInputQueueGetEvent             = libandroid.NewSymbol("AInputQueue_getEvent")
)

func NativeWindowGetWidth(window uintptr) int32 {
	return cgo.CallRet[int32](aNativeWindowGetWidth.Addr(), window)
}

func NativeWindowGetHeight(window uintptr) int32 {
	return cgo.CallRet[int32](aNativeWindowGetHeight.Addr(), window)
}

func NativeWindowSetBuffersGeometry(window uintptr, width, height, format int32) int32 {
	return cgo.CallRet[int32](aNativeWindowSetBuffersGeometry.Addr(), window, width, height, format)
}

func NativeWindowUnlockAndPost(window uintptr) int32 {
	return cgo.CallRet[int32](aNativeWindowUnlockAndPost.Addr(), window)
}

func NativeWindowAcquire(window uintptr) { cgo.Call(aNativeWindowAcquire.Addr(), window) }

func NativeWindowRelease(window uintptr) { cgo.Call(aNativeWindowRelease.Addr(), window) }

func LooperForThread() uintptr { return cgo.CallRet[uintptr](aLooperForThread.Addr()) }

func LooperAddFd(looper uintptr, fd, ident, events int32, callback, data uintptr) int32 {
	return cgo.CallRet[int32](aLooperAddFd.Addr(), looper, fd, ident, events, callback, data)
}

func LooperRemoveFd(looper uintptr, fd int32) int32 {
	return cgo.CallRet[int32](aLooperRemoveFd.Addr(), looper, fd)
}

func ConfigurationNew() uintptr { return cgo.CallRet[uintptr](aConfigurationNew.Addr()) }

func ConfigurationFromAssetManager(config, assets uintptr) {
	cgo.Call(aConfigurationFromAssetManager.Addr(), config, assets)
}

func ConfigurationGetDensity(config uintptr) int32 {
	return cgo.CallRet[int32](aConfigurationGetDensity.Addr(), config)
}

func ConfigurationDelete(config uintptr) { cgo.Call(aConfigurationDelete.Addr(), config) }

func InputQueuePreDispatchEvent(queue, event uintptr) int32 {
	return cgo.CallRet[int32](aInputQueuePreDispatchEvent.Addr(), queue, event)
}

func InputQueueFinishEvent(queue, event uintptr, handled int32) {
	cgo.Call(aInputQueueFinishEvent.Addr(), queue, event, handled)
}

func InputQueueAttachLooper(queue, looper uintptr, ident int32, callback, data uintptr) {
	cgo.Call(aInputQueueAttachLooper.Addr(), queue, looper, ident, callback, data)
}

func InputQueueDetachLooper(queue uintptr) { cgo.Call(aInputQueueDetachLooper.Addr(), queue) }

// NativeWindowLock borrows buffer pixels. dirty may be nil; otherwise Android
// reads and updates the dirty rectangle synchronously.
func NativeWindowLock(window uintptr, buffer *WindowBuffer, dirty *Rect) int32 {
	return cgo.CallRet[int32](aNativeWindowLock.Addr(), window, buffer, dirty)
}

func InputQueueGetEvent(queue uintptr) (event uintptr, status int32) {
	status = cgo.CallRet[int32](aInputQueueGetEvent.Addr(), queue, &event)
	return
}
