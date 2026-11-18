package xlib

import (
	"testing"
	"unsafe"
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
