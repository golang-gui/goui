package android

import (
	"testing"
	"unsafe"
)

func TestNativeActivityARM64Layout(t *testing.T) {
	if unsafe.Sizeof(uintptr(0)) != 8 {
		t.Skip("当前只验收 64 位 Android ABI")
	}
	var a Activity
	var b WindowBuffer
	if unsafe.Sizeof(a) != 80 || unsafe.Offsetof(a.SDKVersion) != 48 || unsafe.Offsetof(a.Instance) != 56 || unsafe.Sizeof(ActivityCallbacks{}) != 128 || unsafe.Sizeof(b) != 48 || unsafe.Offsetof(b.Bits) != 16 || unsafe.Sizeof(Rect{}) != 16 {
		t.Fatalf("NDK 公共布局不匹配：activity=%d callbacks=%d buffer=%d", unsafe.Sizeof(a), unsafe.Sizeof(ActivityCallbacks{}), unsafe.Sizeof(b))
	}
}
