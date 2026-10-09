package cocoa

import (
	"runtime"
	"testing"

	"github.com/golang-gui/goui/platform/darwin/frameworks/appkit"
	"github.com/golang-gui/goui/platform/darwin/frameworks/foundation"
	"golang.org/x/text/language"
)

// Foundation locale objects need no NSApplication, window or desktop session.
// AppKit initialization here only loads existing NSArray method bindings.
func TestSettingsLanguageNative(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err := foundation.InitFoundation(); err != nil {
		t.Fatal(err)
	}
	if err := appkit.InitAppKit(); err != nil {
		t.Fatal(err)
	}
	foundation.AutoReleasePool(func() {
		got, err := (Settings{}).Language()
		if err != nil {
			t.Fatal(err)
		}
		if tag, err := language.Parse(got); err != nil || tag == language.Und {
			t.Fatalf("invalid native language %q: %v", got, err)
		}
		again, err := (Settings{}).Language()
		if err != nil || again != got {
			t.Fatalf("unstable language: %q, %v; first %q", again, err, got)
		}
		t.Logf("native preferred UI language: %s", got)
	})
}
