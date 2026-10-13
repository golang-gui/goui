package main

import (
	"fmt"
	"unsafe"

	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/platform/linux/libs/xlib"
)

func checkNative(window gui.Window, skip bool) error {
	d := xlib.OpenDisplay("")
	if d == 0 {
		return fmt.Errorf("open independent X11 observation display failed")
	}
	defer d.Close()
	w := xlib.Window(window.PlatformWindow().NativeHandle())
	var attrs xlib.WindowAttributes
	if d.GetWindowAttributes(w, &attrs) == 0 || attrs.OverrideRedirect != 0 {
		return fmt.Errorf("taskbar window is not WM-managed")
	}
	var typ xlib.Atom
	var format int32
	var count, remaining uint
	var data *byte
	if d.GetWindowProperty(w, d.InternAtom("_NET_WM_STATE", false), 0, 64, false, xlib.AtomAtom,
		&typ, &format, &count, &remaining, &data) != 0 {
		return fmt.Errorf("read native state failed")
	}
	if data != nil {
		defer xlib.Free(data)
	}
	if count != 0 && (data == nil || typ != xlib.AtomAtom || format != 32 || remaining != 0) {
		return fmt.Errorf("invalid native state property")
	}
	atom := d.InternAtom("_NET_WM_STATE_SKIP_TASKBAR", false)
	found := false
	if count != 0 {
		for _, state := range unsafe.Slice((*xlib.Atom)(unsafe.Pointer(data)), int(count)) {
			found = found || state == atom
		}
	}
	if found != skip {
		return fmt.Errorf("observed taskbar state=%v, want %v", found, skip)
	}
	return nil
}
