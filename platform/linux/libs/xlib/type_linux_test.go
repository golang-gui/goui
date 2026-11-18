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

// Linux amd64/arm64 的 LP64 XErrorEvent 布局，不是线上协议错误包布局。
func TestErrorEventABI(t *testing.T) {
	if unsafe.Sizeof(uintptr(0)) != 8 {
		t.Skip("LP64 ABI assertions")
	}
	for _, tc := range []struct {
		name      string
		got, want uintptr
	}{
		{"size", unsafe.Sizeof(ErrorEvent{}), 40},
		{"display", unsafe.Offsetof(ErrorEvent{}.Display), 8},
		{"resource", unsafe.Offsetof(ErrorEvent{}.ResourceID), 16},
		{"serial", unsafe.Offsetof(ErrorEvent{}.Serial), 24},
		{"error", unsafe.Offsetof(ErrorEvent{}.ErrorCode), 32},
		{"request", unsafe.Offsetof(ErrorEvent{}.RequestCode), 33},
		{"minor", unsafe.Offsetof(ErrorEvent{}.MinorCode), 34},
	} {
		if tc.got != tc.want {
			t.Errorf("%s = %d, want %d", tc.name, tc.got, tc.want)
		}
	}
}

func TestXIMDestroyCallbackWithoutInputMethod(t *testing.T) {
	if XIM(0).SetDestroyCallback(XIMCallback{}) {
		t.Fatal("missing XIM accepted a destroy callback")
	}
}

// Linux amd64/arm64 共用的 LP64 Xlib.h 布局，不连接显示服务器。
func TestXIMCallbackABI(t *testing.T) {
	if unsafe.Sizeof(uintptr(0)) != 8 {
		t.Skip("LP64 ABI assertions")
	}
	checks := []struct {
		name      string
		got, want uintptr
	}{
		{"styles size", unsafe.Sizeof(XIMStyles{}), 16},
		{"styles pointer", unsafe.Offsetof(XIMStyles{}.SupportedStyles), 8},
		{"callback size", unsafe.Sizeof(XIMCallback{}), 16},
		{"text size", unsafe.Sizeof(XIMText{}), 32},
		{"text feedback", unsafe.Offsetof(XIMText{}.Feedback), 8},
		{"text encoding", unsafe.Offsetof(XIMText{}.EncodingIsWchar), 16},
		{"text string", unsafe.Offsetof(XIMText{}.String), 24},
		{"draw size", unsafe.Sizeof(XIMPreeditDrawCallbackStruct{}), 24},
		{"draw text", unsafe.Offsetof(XIMPreeditDrawCallbackStruct{}.Text), 16},
		{"caret size", unsafe.Sizeof(XIMPreeditCaretCallbackStruct{}), 12},
	}
	for _, check := range checks {
		if check.got != check.want {
			t.Errorf("%s: %d want %d", check.name, check.got, check.want)
		}
	}
}
