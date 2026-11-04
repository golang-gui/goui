// Package bitmap binds the public Android software bitmap NDK interface.
package bitmap

import (
	"runtime"
	"unsafe"

	"github.com/goexlib/cgo"
	"github.com/golang-gui/goui/platform/android/libs/jni"
)

type Info struct {
	Width, Height, Stride uint32
	Format                int32
	Flags                 uint32
}

const RGBA8888 = 1

var library = cgo.NewLazyLibrary("libjnigraphics.so")
var getInfo = library.NewSymbol("AndroidBitmap_getInfo")
var lockPixels = library.NewSymbol("AndroidBitmap_lockPixels")
var unlockPixels = library.NewSymbol("AndroidBitmap_unlockPixels")

func GetInfo(env jni.Env, obj jni.Object) (Info, int32) {
	var out Info
	var pin runtime.Pinner
	pin.Pin(&out)
	defer pin.Unpin()
	status := cgo.CallRet[int32](getInfo.Addr(), uintptr(env), uintptr(obj), uintptr(unsafe.Pointer(&out)))
	return out, status
}
func LockPixels(env jni.Env, obj jni.Object) (uintptr, int32) {
	var out uintptr
	var pin runtime.Pinner
	pin.Pin(&out)
	defer pin.Unpin()
	status := cgo.CallRet[int32](lockPixels.Addr(), uintptr(env), uintptr(obj), uintptr(unsafe.Pointer(&out)))
	return out, status
}
func UnlockPixels(env jni.Env, obj jni.Object) int32 {
	return cgo.CallRet[int32](unlockPixels.Addr(), uintptr(env), uintptr(obj))
}
