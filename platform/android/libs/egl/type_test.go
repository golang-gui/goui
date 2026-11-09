package egl

import (
	"testing"
	"unsafe"
)

func TestEGLArgumentStorage(t *testing.T) {
	if unsafe.Sizeof(Boolean(0)) != 4 || unsafe.Sizeof(Config(0)) != unsafe.Sizeof(uintptr(0)) {
		t.Fatal("EGL 类型宽度错误")
	}
	if attributes(nil) != nil || attributes(make([]int32, 0, 4)) != nil || configPointer(nil) != nil {
		t.Fatal("空属性/输出必须传原生 NULL")
	}
	values := []int32{None}
	if attributes(values) != &values[0] {
		t.Fatal("丢失属性存储身份")
	}
	configs := make([]Config, 1)
	*configPointer(configs) = 7
	if configs[0] != 7 {
		t.Fatal("输出未指向调用方存储")
	}
}
