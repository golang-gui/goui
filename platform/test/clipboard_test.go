package test

import (
	"os"
	"testing"
	"time"

	"github.com/golang-gui/goui/platform"
)

// TestClipboard exercises a same-process SetText -> RequestText round-trip.
//
// The callback is expected synchronously: on x11 we own the CLIPBOARD selection
// so RequestText short-circuits; win32/cocoa read the system clipboard directly.
// This does not cover the cross-process async selection path (see
// TestClipboardPasteFromApp).
//
// Note: this overwrites the real system clipboard on win32/cocoa.
func TestClipboard(t *testing.T) {
	skipWithoutDisplay(t)

	plat, err := getPlatform()
	if err != nil {
		t.Fatal(err)
	}

	var clip platform.Clipboard
	runOnMainThread(func() {
		clip, err = plat.NewClipboard()
	})
	if err != nil {
		t.Fatal(err)
	}
	if clip == nil {
		t.Fatal("NewClipboard returned nil")
	}

	cases := []struct {
		name string
		text string
	}{
		{"ASCII", "Hello, clipboard!"},
		{"UTF8", "你好，剪贴板 🧧 çà"},
		{"Multiline", "line one\nline two\ttabbed"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok, called, setErr := clipboardRoundTrip(clip, tc.text)
			if setErr != nil {
				t.Fatalf("SetText(%q) failed: %v", tc.text, setErr)
			}
			if !called {
				t.Fatal("RequestText callback was not invoked synchronously for a same-process round-trip")
			}
			if !ok {
				t.Fatalf("RequestText reported no text after SetText(%q)", tc.text)
			}
			if got != tc.text {
				t.Fatalf("clipboard round-trip mismatch: set %q, got %q", tc.text, got)
			}
		})
	}

	t.Run("Overwrite", func(t *testing.T) {
		runOnMainThread(func() {
			err = clip.SetText("first value")
		})
		if err != nil {
			t.Fatalf("first SetText failed: %v", err)
		}
		got, ok, called, setErr := clipboardRoundTrip(clip, "second value")
		if setErr != nil {
			t.Fatalf("second SetText failed: %v", setErr)
		}
		if !called || !ok || got != "second value" {
			t.Fatalf("overwrite failed: got %q ok=%v called=%v", got, ok, called)
		}
	})
}

// TestClipboardPasteFromApp reads text copied in another application. It drives
// the event loop so the x11 SelectionNotify can be delivered (the path the
// same-process round-trip skips), with a safety timeout.
func TestClipboardPasteFromApp(t *testing.T) {
	if os.Getenv("GOUI_TEST_CLIPBOARD") == "" {
		t.Skip("set GOUI_TEST_CLIPBOARD=1 and copy some text in another app to verify RequestText")
	}
	skipWithoutDisplay(t)

	plat, err := getPlatform()
	if err != nil {
		t.Fatal(err)
	}

	var (
		clip platform.Clipboard
		loop platform.EventLoop
	)
	runOnMainThread(func() {
		if clip, err = plat.NewClipboard(); err != nil {
			return
		}
		loop, err = plat.NewEventLoop()
	})
	if err != nil {
		t.Fatal(err)
	}
	defer runOnMainThread(loop.Destroy)

	var (
		got    string
		gotOK  bool
		called bool
	)
	loop.Post(func() {
		clip.RequestText(func(text string, ok bool) {
			got, gotOK, called = text, ok, true
			loop.Quit()
		})
	})
	// Quit is safe to call cross-thread; bounds the wait if no owner responds.
	go func() {
		time.Sleep(10 * time.Second)
		loop.Quit()
	}()
	runOnMainThread(loop.Run)

	if !called {
		t.Fatal("RequestText callback was not invoked within the timeout (no clipboard owner responded?)")
	}
	if !gotOK {
		t.Fatal("clipboard reported no text; copy some text in another app first")
	}
	t.Logf("clipboard text from another app: %q", got)
}

// clipboardRoundTrip sets text then requests it back on the platform thread,
// capturing the (synchronous) callback result. Assertions are left to the
// caller's goroutine.
func clipboardRoundTrip(clip platform.Clipboard, text string) (got string, gotOK, called bool, setErr error) {
	runOnMainThread(func() {
		if setErr = clip.SetText(text); setErr != nil {
			return
		}
		clip.RequestText(func(t string, ok bool) {
			got, gotOK, called = t, ok, true
		})
	})
	return
}
