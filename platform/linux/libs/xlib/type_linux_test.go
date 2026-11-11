package xlib

import (
	"testing"
	"unsafe"
)

func TestFocusChangeEventABI(t *testing.T) {
	// Xlib.h 的 LP64 布局；Linux amd64/arm64 均使用此 ABI。
	if unsafe.Sizeof(uintptr(0)) != 8 {
		t.Skip("仅验证支持目标的 LP64 ABI")
	}
	for _, tc := range []struct {
		name      string
		got, want uintptr
	}{
		{"size", unsafe.Sizeof(FocusChangeEvent{}), 48},
		{"type", unsafe.Offsetof(FocusChangeEvent{}.Type), 0},
		{"serial", unsafe.Offsetof(FocusChangeEvent{}.Serial), 8},
		{"send_event", unsafe.Offsetof(FocusChangeEvent{}.SendEvent), 16},
		{"display", unsafe.Offsetof(FocusChangeEvent{}.Display), 24},
		{"window", unsafe.Offsetof(FocusChangeEvent{}.Window), 32},
		{"mode", unsafe.Offsetof(FocusChangeEvent{}.Mode), 40},
		{"detail", unsafe.Offsetof(FocusChangeEvent{}.Detail), 44},
	} {
		if tc.got != tc.want {
			t.Errorf("%s = %d，期望 %d", tc.name, tc.got, tc.want)
		}
	}
	// 按 Xlib 的实际字节位置写入，防止 accessor 错用 CrossingEvent 布局。
	var event Event
	event.Type = FocusOut
	*(*int32)(unsafe.Add(unsafe.Pointer(&event), 40)) = NotifyGrab
	*(*int32)(unsafe.Add(unsafe.Pointer(&event), 44)) = 3
	if got := event.FocusChangeEvent(); got.Type != FocusOut || got.Mode != NotifyGrab || got.Detail != 3 {
		t.Fatalf("焦点事件 union 读取错误：%+v", got)
	}
}
