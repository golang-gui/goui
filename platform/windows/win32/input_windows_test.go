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
	// 只读取当前线程的键盘状态，不需要真实 HWND 或桌面。
	// 原生 TranslateMessage 路径由窗口例程验证。
	dispatchMessage(&winapi.MSG{Hwnd: hwnd, Message: winapi.WM_KEYDOWN, WParam: 'S'})
	if calls != 1 {
		t.Fatalf("key delivered %d times", calls)
	}
}

func TestLogicalWindowsLetters(t *testing.T) {
	// wParam 已按当前 Windows 键盘布局翻译；改变物理扫描码，确认 GOUI 不撤销布局映射。
	for _, tt := range []struct {
		vk   int
		scan uintptr
		want events.Key
	}{
		{'A', 0x10, events.KeyA}, // 法语布局的 A 位于美式 Q 的位置。
		{'Q', 0x1e, events.KeyQ},
		{'Y', 0x2c, events.KeyY}, // 德语布局的 Y 位于美式 Z 的位置。
		{'Z', 0x15, events.KeyZ},
	} {
		key, location := keyFromVirtualKey(tt.vk, winapi.LPARAM(tt.scan<<16))
		if key != tt.want || location != events.KeyLocationStandard {
			t.Fatalf("VK %d scan %x: %v/%v", tt.vk, tt.scan, key, location)
		}
	}
}

// 只验证映射与状态，不需要原生窗口或桌面会话。
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
			// 每次调用都是独立的原生消息快照，覆盖获得焦点前按住及在其他窗口释放的按键。
			state := func(vk int) winapi.SHORT {
				if vk == tc.left && sides[0] || vk == tc.right && sides[1] {
					return -32768
				}
				return 1 // 只有低位切换标志不表示按下。
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
