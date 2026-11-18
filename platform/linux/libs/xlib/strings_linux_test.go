package xlib

import (
	"runtime"
	"strings"
	"testing"

	"github.com/goexlib/cgo"
)

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
