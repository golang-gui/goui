package jni

import (
	"math"
	"testing"
	"unsafe"
)

func TestJNIValueLayout(t *testing.T) {
	if Bool(false) != 0 || Bool(true) != 1 {
		t.Fatal("jboolean 的 false/true 必须编码为 0/1")
	}
	if unsafe.Sizeof(Value(0)) != 8 || uint64(Int(-1)) != 0xffffffff || uint64(Ref(Object(7))) != 7 || uint64(Float(-1.25)) != uint64(math.Float32bits(-1.25)) {
		t.Fatal("jvalue 的整数、引用或浮点位布局错误")
	}
	if unsafe.Sizeof(uintptr(0)) == 8 && (unsafe.Sizeof(AttachArgs{}) != 24 || unsafe.Offsetof(AttachArgs{}.Name) != 8) {
		t.Fatal("JavaVMAttachArgs 64 位布局错误")
	}
}
