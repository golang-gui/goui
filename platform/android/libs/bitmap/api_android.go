package bitmap

import (
	"github.com/goexlib/cgo"
	"github.com/golang-gui/goui/platform/android/libs/jni"
)

var (
	libjnigraphics            = cgo.NewLazyLibrary("libjnigraphics.so")
	androidBitmapGetInfo      = libjnigraphics.NewSymbol("AndroidBitmap_getInfo")
	androidBitmapLockPixels   = libjnigraphics.NewSymbol("AndroidBitmap_lockPixels")
	androidBitmapUnlockPixels = libjnigraphics.NewSymbol("AndroidBitmap_unlockPixels")
)

func GetInfo(env jni.Env, obj jni.Object) (out Info, status int32) {
	status = cgo.CallRet[int32](androidBitmapGetInfo.Addr(), env, obj, &out)
	return
}

// LockPixels borrows native memory until UnlockPixels; the caller retains obj.
func LockPixels(env jni.Env, obj jni.Object) (out uintptr, status int32) {
	status = cgo.CallRet[int32](androidBitmapLockPixels.Addr(), env, obj, &out)
	return
}
func UnlockPixels(env jni.Env, obj jni.Object) int32 {
	return cgo.CallRet[int32](androidBitmapUnlockPixels.Addr(), env, obj)
}
