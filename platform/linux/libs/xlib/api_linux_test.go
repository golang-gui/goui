package xlib

import (
	"runtime"
	"strings"
	"testing"
	"unsafe"

	"github.com/goexlib/cgo"
)

func TestWindowAttributesABI(t *testing.T) {
	var attrs WindowAttributes
	if unsafe.Sizeof(attrs) != 136 || unsafe.Offsetof(attrs.Visual) != 24 ||
		unsafe.Offsetof(attrs.MapState) != 92 || unsafe.Offsetof(attrs.Screen) != 128 {
		t.Fatalf("XWindowAttributes LP64 layout: size=%d mapState=%d screen=%d",
			unsafe.Sizeof(attrs), unsafe.Offsetof(attrs.MapState), unsafe.Offsetof(attrs.Screen))
	}
}

func TestMappingEventABI(t *testing.T) {
	var mapEvent MapEvent
	var unmapEvent UnmapEvent
	if unsafe.Sizeof(mapEvent) != 56 || unsafe.Sizeof(unmapEvent) != 56 ||
		unsafe.Offsetof(mapEvent.Window) != 40 || unsafe.Offsetof(unmapEvent.Window) != 40 {
		t.Fatal("mapping event must distinguish recipient Event from target Window")
	}
}

func TestKeyboardMappingEventABI(t *testing.T) {
	// Xlib.h 的 LP64 XMappingEvent 布局，适用 Linux amd64/arm64。
	var event MappingEvent
	if unsafe.Sizeof(event) != 56 || unsafe.Offsetof(event.Display) != 24 ||
		unsafe.Offsetof(event.Window) != 32 || unsafe.Offsetof(event.Request) != 40 ||
		unsafe.Offsetof(event.FirstKeycode) != 44 || unsafe.Offsetof(event.Count) != 48 {
		t.Fatal("XMappingEvent ABI does not match Xlib.h")
	}
	var raw Event
	raw.MappingEvent().FirstKeycode = 38
	if (*MappingEvent)(unsafe.Pointer(&raw)).FirstKeycode != 38 {
		t.Fatal("MappingEvent accessor does not address native event storage")
	}
}

func TestKeymapEventABI(t *testing.T) {
	// Xlib.h 的 LP64 XKeymapEvent 布局，适用 Linux amd64/arm64。
	var e KeymapEvent
	if unsafe.Sizeof(e) != 72 || unsafe.Offsetof(e.Window) != 32 || unsafe.Offsetof(e.KeyVector) != 40 {
		t.Fatalf("XKeymapEvent layout: size=%d window=%d vector=%d", unsafe.Sizeof(e), unsafe.Offsetof(e.Window), unsafe.Offsetof(e.KeyVector))
	}
}

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

func TestResourceDatabaseCopiesTemporaryStrings(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	want := strings.Repeat("dynamic-value", 100)
	db := GetStringDatabase("goui.test: " + want + "\n")
	if db == 0 {
		t.Fatal("cannot create resource database")
	}
	defer db.Destroy()
	runtime.GC()
	for range 4 {
		if got, ok := db.GetResource(strings.Join([]string{"goui", "test"}, "."), "Goui.Test"); !ok || got != want {
			t.Fatal("resource did not survive temporary input", got, ok)
		}
		runtime.GC()
	}
}

func TestResourceDatabaseEmptyString(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	// 新 X server 可能没有 RESOURCE_MANAGER；应传入空 C 字符串，不能给 XrmGetStringDatabase 传 NULL。
	db := GetStringDatabase("")
	defer db.Destroy()
	if value, ok := db.GetResource("Xcursor.theme", "Xcursor.Theme"); ok || value != "" {
		t.Fatalf("empty database returned %q, %v", value, ok)
	}
}

func TestXIMOptionNamesAreTerminated(t *testing.T) {
	for _, name := range []string{xnInputStyle, xnClientWindow, xnFocusWindow, xnPreeditAttributes, xnSpotLocation} {
		if name[len(name)-1] != 0 {
			t.Fatal("option name is not terminated", name)
		}
		pointer := cgo.CStringTemp(name)
		if got := cgo.GoString(pointer); got != name[:len(name)-1] {
			t.Fatal(got, name)
		}
		runtime.KeepAlive(pointer)
	}
}
