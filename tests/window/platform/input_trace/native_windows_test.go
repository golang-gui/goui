package main

import (
	"testing"
	"unsafe"

	"github.com/golang-gui/goui/platform/windows/sdk/winapi"
)

func TestWindowsHookMessageLayout(t *testing.T) {
	// Windows x64 的 SDK 布局；回调直接借用原生内存，字段不能偏移。
	var c winapi.CWPSTRUCT
	if unsafe.Sizeof(c) != 32 || unsafe.Offsetof(c.LParam) != 0 || unsafe.Offsetof(c.WParam) != 8 || unsafe.Offsetof(c.Message) != 16 || unsafe.Offsetof(c.Hwnd) != 24 {
		t.Fatalf("CWPSTRUCT 布局错误：size=%d message=%d hwnd=%d", unsafe.Sizeof(c), unsafe.Offsetof(c.Message), unsafe.Offsetof(c.Hwnd))
	}
	var m winapi.MSG
	if unsafe.Sizeof(m) != 48 || unsafe.Offsetof(m.Message) != 8 || unsafe.Offsetof(m.WParam) != 16 || unsafe.Offsetof(m.LParam) != 24 || unsafe.Offsetof(m.Time) != 32 || unsafe.Offsetof(m.Pt) != 36 {
		t.Fatalf("MSG 布局错误：size=%d message=%d wp=%d lp=%d time=%d pt=%d", unsafe.Sizeof(m), unsafe.Offsetof(m.Message), unsafe.Offsetof(m.WParam), unsafe.Offsetof(m.LParam), unsafe.Offsetof(m.Time), unsafe.Offsetof(m.Pt))
	}
	if size := unsafe.Sizeof(winapi.INPUT_MESSAGE_SOURCE{}); size != 8 {
		t.Fatalf("INPUT_MESSAGE_SOURCE 大小=%d，期望 8", size)
	}
}
