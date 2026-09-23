package win32

import (
	"runtime"
	"testing"

	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/windows/sdk/winapi"
)

func TestConsumedQueuedKey(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	const hwnd winapi.HWND = 1
	previous := windowMap[hwnd]
	t.Cleanup(func() {
		if previous != nil {
			windowMap[hwnd] = previous
		} else {
			delete(windowMap, hwnd)
		}
	})
	calls := 0
	w := &Window{hwnd: hwnd, onEvent: func(event events.Event) {
		e := event.(events.KeyEvent)
		if e.EventType != events.KeyDown || e.Key != events.KeyS || e.Handled == nil {
			t.Fatalf("unexpected key: %+v", e)
		}
		calls++
		e.PreventDefault()
	}}
	windowMap[hwnd] = w
	// No real HWND/desktop is needed: only the calling thread's keyboard state
	// is read. The native TranslateMessage path is covered by the window case.
	dispatchMessage(&winapi.MSG{Hwnd: hwnd, Message: winapi.WM_KEYDOWN, WParam: 'S'})
	if calls != 1 {
		t.Fatalf("key delivered %d times", calls)
	}
}

func TestLogicalWindowsLetters(t *testing.T) {
	// wParam is already translated by the active Windows keyboard layout.
	// Vary physical scan positions to ensure GOUI does not undo that mapping.
	for _, tt := range []struct {
		vk   int
		scan uintptr
		want events.Key
	}{
		{'A', 0x10, events.KeyA}, // French A at US Q position.
		{'Q', 0x1e, events.KeyQ},
		{'Y', 0x2c, events.KeyY}, // German Y at US Z position.
		{'Z', 0x15, events.KeyZ},
	} {
		key, location := keyFromVirtualKey(tt.vk, winapi.LPARAM(tt.scan<<16))
		if key != tt.want || location != events.KeyLocationStandard {
			t.Fatalf("VK %d scan %x: %v/%v", tt.vk, tt.scan, key, location)
		}
	}
}

// Pure mapping/state tests: no native window or desktop session is required.
func TestWindowsLogoKey(t *testing.T) {
	for _, tt := range []struct {
		vk       int
		location events.KeyLocation
	}{
		{winapi.VK_LWIN, events.KeyLocationLeft},
		{winapi.VK_RWIN, events.KeyLocationRight},
	} {
		key, location := keyFromVirtualKey(tt.vk, 0)
		if key != events.KeyWin || location != tt.location {
			t.Fatalf("VK %#x: key=%v location=%v; want Win location=%v", tt.vk, key, location, tt.location)
		}
		pressed := map[int]bool{winapi.VK_LCONTROL: true, tt.vk: true}
		state := func(vk int) winapi.SHORT {
			if pressed[vk] {
				return -32768
			}
			return 0
		}
		if got := keyModifiers(state); got != events.ModifierControl|events.ModifierWin {
			t.Fatalf("key down modifiers: %v", got)
		}
		delete(pressed, tt.vk)
		if got := keyModifiers(state); got != events.ModifierControl {
			t.Fatalf("key up modifiers: %v", got)
		}
	}
	if got := keyModifiers(func(int) winapi.SHORT { return -32768 }); got != events.ModifierShift|events.ModifierControl|events.ModifierAlt|events.ModifierWin {
		t.Fatalf("Windows must only report its native modifiers: %v", got)
	}
}

func TestWindowsSidedModifiers(t *testing.T) {
	for _, tc := range []struct {
		left, right int
		key         events.Key
		modifier    events.Modifiers
	}{
		{winapi.VK_LSHIFT, winapi.VK_RSHIFT, events.KeyShift, events.ModifierShift},
		{winapi.VK_LCONTROL, winapi.VK_RCONTROL, events.KeyControl, events.ModifierControl},
		{winapi.VK_LMENU, winapi.VK_RMENU, events.KeyAlt, events.ModifierAlt},
		{winapi.VK_LWIN, winapi.VK_RWIN, events.KeyWin, events.ModifierWin},
	} {
		for i, sides := range [][2]bool{{true, false}, {true, true}, {false, true}, {false, false}, {true, false}} {
			// Each invocation is an independent native message snapshot, including
			// keys held before focus and keys released while another window owned it.
			state := func(vk int) winapi.SHORT {
				if vk == tc.left && sides[0] || vk == tc.right && sides[1] {
					return -32768
				}
				return 1 // low toggle bit alone does not mean pressed
			}
			want := events.Modifiers(0)
			if sides[0] || sides[1] {
				want = tc.modifier
			}
			if got := keyModifiers(state); got != want {
				t.Fatalf("key=%v step=%d modifiers=%v want=%v", tc.key, i, got, want)
			}
		}
		for i, vk := range []int{tc.left, tc.right} {
			key, location := keyFromVirtualKey(vk, 0)
			if key != tc.key || location != events.KeyLocation(int(events.KeyLocationLeft)+i) {
				t.Fatalf("VK=%x key/location=%v/%v", vk, key, location)
			}
		}
	}
}
