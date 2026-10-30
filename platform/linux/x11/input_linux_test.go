package x11

import (
	"testing"

	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/linux/libs/xlib"
)

func TestX11AltGraphState(t *testing.T) {
	previous := platform
	t.Cleanup(func() { platform = previous })
	// 用户可以将 Level3 映射至其他槽位，不能假定总是 Mod5。
	platform = &Platform{altGraphMasks: [4]uint32{xlib.Mod3Mask}}
	if got := modifiersFromState(xlib.ControlMask | xlib.Mod3Mask); got != events.ModifierControl|events.ModifierAltGraph {
		t.Fatalf("Control+Level3 = %v", got)
	}
	if got := modifiersFromState(xlib.Mod5Mask); got != 0 {
		t.Fatalf("unmapped Mod5 treated as AltGraph: %v", got)
	}
	key, location := keyFromKeysym(xlib.XK_ISO_Level3_Shift, 0, 0)
	if key != events.KeyAltGraph || location != events.KeyLocationStandard {
		t.Fatalf("Level3 key = %v/%v", key, location)
	}
	w := &Window{}
	if got := w.keyModifiers(events.KeyDown, key, 92, 0); got != events.ModifierAltGraph {
		t.Fatalf("Level3 press = %v", got)
	}
	if got := w.keyModifiers(events.KeyUp, key, 92, xlib.Mod3Mask); got != 0 {
		t.Fatalf("Level3 release = %v", got)
	}
}

func TestLogicalX11Letters(t *testing.T) {
	for _, tt := range []struct {
		sym  xlib.KeySym
		want events.Key
	}{
		{'a', events.KeyA}, {'A', events.KeyA}, {'q', events.KeyQ},
		{'z', events.KeyZ}, {'y', events.KeyY}, {0x6c6, events.KeyUnknown},
	} {
		key, location := keyFromKeysym(tt.sym, 0, 0)
		if key != tt.want || location != events.KeyLocationStandard {
			t.Fatalf("keysym %x: %v/%v", tt.sym, key, location)
		}
	}
	for group := uint32(0); group < 4; group++ {
		if got := keyboardGroup(group<<13 | xlib.ShiftMask | xlib.LockMask | xlib.ControlMask | xlib.Mod5Mask); got != group {
			t.Fatalf("group = %d, want %d", got, group)
		}
	}
}

func TestX11AltGraphFollowsEventGroup(t *testing.T) {
	previous := platform
	t.Cleanup(func() { platform = previous })
	// us,cn(altgr-pinyin) 中 Alt_R 贡献 Mod1，但仅在第二组表示 Level3；
	// 独立 Level3 键仍贡献 Mod5。
	platform = &Platform{altGraphMasks: [4]uint32{xlib.Mod5Mask, xlib.Mod1Mask | xlib.Mod5Mask}}
	for _, tc := range []struct {
		state uint32
		want  events.Modifiers
	}{
		{xlib.Mod1Mask, events.ModifierAlt},
		{xlib.Mod5Mask, events.ModifierAltGraph},
		{1<<13 | xlib.Mod1Mask, events.ModifierAlt | events.ModifierAltGraph},
		{1<<13 | xlib.Mod5Mask, events.ModifierAltGraph},
	} {
		if got := modifiersFromState(tc.state); got != tc.want {
			t.Fatalf("state=%x got=%v want=%v", tc.state, got, tc.want)
		}
	}
	w := &Window{}
	platform.modifierMasks[108] = xlib.Mod1Mask
	w.trackKey(108, true, false)
	if got := w.keyModifiers(events.KeyDown, events.KeyAltGraph, 108, 1<<13); got != events.ModifierAlt|events.ModifierAltGraph {
		t.Fatalf("Level3 on Mod1 down: %v", got)
	}
	w.trackKey(108, false, false)
	if got := w.keyModifiers(events.KeyUp, events.KeyAltGraph, 108, 1<<13|xlib.Mod1Mask); got != 0 {
		t.Fatalf("Level3 on Mod1 release retained a contribution: %v", got)
	}
}

func TestX11RepeatState(t *testing.T) {
	w := &Window{}
	if w.trackKey(38, true, false) || !w.trackKey(38, true, false) || w.trackKey(38, false, false) || w.trackKey(38, true, false) {
		t.Fatal("incorrect down/repeat/up state")
	}
	var up, down xlib.Event
	u, d := up.KeyEvent(), down.KeyEvent()
	u.Type, u.KeyCode, u.Time, u.Window = xlib.KeyRelease, 38, 100, 1
	d.Type, d.KeyCode, d.Time, d.Window = xlib.KeyPress, 38, 100, 1
	if !isRepeatPair(&up, &down) {
		t.Fatal("core repeat pair not recognized")
	}
	d.Time++
	if isRepeatPair(&up, &down) {
		t.Fatal("real up/down collapsed")
	}
	d.Time = u.Time
	d.Window++
	if isRepeatPair(&up, &down) {
		t.Fatal("cross-window pair collapsed")
	}
}

func TestX11RepeatThroughXIM(t *testing.T) {
	type step struct {
		down, filtered, repeat, pressed bool
	}
	for _, tc := range []struct {
		name  string
		steps []step
	}{
		{"forward before physical release", []step{
			{true, true, false, false}, // 原始按下发送给 IM 服务器
			{true, false, false, true}, // 首次转发按下，不是重复
			{true, true, false, true},  // 原始自动重复发送给 IM
			{true, false, true, true},  // 转发的自动重复
			{false, true, false, true}, // 原始释放不能提前清空状态
			{false, false, false, false},
			{true, false, false, true}, // 下次独立按下
		}},
		{"forward after physical release", []step{
			{true, true, false, false},
			{false, true, false, false},
			{true, false, false, true},
			{false, false, false, false},
		}},
		{"consumed without forwarding", []step{
			{true, true, false, false},
			{true, true, false, false},
			{false, true, false, false},
			{true, false, false, true},
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w, other := &Window{}, &Window{}
			for i, s := range tc.steps {
				got := w.trackKey(71, s.down, s.filtered) // 物理 F5
				if got != s.repeat || w.pressedKeys[71] != s.pressed {
					t.Fatalf("step %d: repeat=%v pressed=%v, want %v/%v", i, got, w.pressedKeys[71], s.repeat, s.pressed)
				}
			}
			if other.trackKey(71, true, false) {
				t.Fatal("another window inherited pressed state")
			}
			if w.trackKey(0, true, false) || w.trackKey(256, true, false) || w.pressedKeys[0] {
				t.Fatal("IME commit keycode or invalid code affected repeat tracking")
			}
		})
	}
}

func TestX11RepeatClearsOnFocusOut(t *testing.T) {
	previousPlatform, previousWindows := platform, windowMap
	t.Cleanup(func() { platform, windowMap = previousPlatform, previousWindows })
	w := &Window{} // 无原生句柄，下方焦点分发仅验证逻辑
	platform, windowMap = nil, map[xlib.Window]*Window{1: w}
	w.trackKey(71, true, false)
	var event xlib.Event
	event.Type = xlib.FocusOut
	event.AnyEvent().Window = 1
	handleEvent(event)
	if w.modifierKeys != [32]byte{} || w.pressedKeys[71] || w.trackKey(71, true, false) {
		t.Fatal("first press after focus loss was treated as repeat")
	}
}

func TestX11SidedModifiers(t *testing.T) {
	previous := platform
	t.Cleanup(func() { platform = previous })
	for _, tc := range []struct {
		name     string
		key      events.Key
		mask     uint32
		modifier events.Modifiers
	}{
		{"Shift", events.KeyShift, xlib.ShiftMask, events.ModifierShift},
		{"Control", events.KeyControl, xlib.ControlMask, events.ModifierControl},
		{"Alt", events.KeyAlt, xlib.Mod1Mask, events.ModifierAlt},
		{"Super", events.KeySuper, xlib.Mod4Mask, events.ModifierSuper},
		{"AltGraph", events.KeyAltGraph, xlib.Mod3Mask, events.ModifierAltGraph},
	} {
		t.Run(tc.name, func(t *testing.T) {
			platform = &Platform{altGraphMasks: [4]uint32{xlib.Mod3Mask}}
			// 使用任意 keycode，验证状态来自原生映射，不依赖美式位置或固定左右键。
			platform.modifierMasks[42] = tc.mask
			platform.modifierMasks[109] = tc.mask
			for _, first := range []uint32{42, 109} {
				other := uint32(151) - first
				w := &Window{}
				for i, s := range []struct {
					code   uint32
					down   bool
					before uint32
					want   events.Modifiers
				}{
					{first, true, 0, tc.modifier},
					{other, true, tc.mask, tc.modifier},
					{first, false, tc.mask, tc.modifier},
					{other, false, tc.mask, 0},
				} {
					w.trackKey(s.code, s.down, false)
					typ := events.KeyUp
					if s.down {
						typ = events.KeyDown
					}
					if got := w.keyModifiers(typ, tc.key, s.code, s.before); got != s.want {
						t.Fatalf("first=%d step=%d: modifiers=%v want=%v", first, i, got, s.want)
					}
				}
			}
		})
	}
}

func TestX11ModifierSnapshotAndFiltering(t *testing.T) {
	previousPlatform, previousWindows := platform, windowMap
	t.Cleanup(func() { platform, windowMap = previousPlatform, previousWindows })
	platform = &Platform{}
	platform.modifierMasks[37], platform.modifierMasks[105] = xlib.ControlMask, xlib.ControlMask
	w, other := &Window{}, &Window{}
	windowMap = map[xlib.Window]*Window{1: w, 2: other}
	// Xlib KeymapNotify 不含窗口 ID；焦点／进入快照表示获得焦点前已按住两侧 Control。
	var snapshot xlib.Event
	snapshot.Type = xlib.KeymapNotify
	snapshot.KeymapEvent().KeyVector[37/8] |= 1 << (37 % 8)
	snapshot.KeymapEvent().KeyVector[105/8] |= 1 << (105 % 8)
	snapshot.KeymapEvent().KeyVector[71/8] |= 1 << (71 % 8) // 等待转发的 XIM F5
	handleEvent(snapshot)
	if w.modifierKeys != other.modifierKeys || w.modifierKeys != snapshot.KeymapEvent().KeyVector {
		t.Fatal("keyboard-wide native snapshot not restored")
	}
	if w.trackKey(71, true, false) {
		t.Fatal("snapshot turned first forwarded F5 into repeat")
	}
	w.trackKey(37, false, true) // 被过滤的原始释放不能改变状态。
	if got := w.keyModifiers(events.KeyDown, events.KeyA, 38, xlib.ControlMask); got != events.ModifierControl {
		t.Fatalf("filtered event changed modifier state: %v", got)
	}
	w.trackKey(37, false, false)
	if got := w.keyModifiers(events.KeyUp, events.KeyControl, 37, xlib.ControlMask); got != events.ModifierControl {
		t.Fatalf("released left Control lost held right Control: %v", got)
	}
	w.trackKey(105, false, false)
	if got := w.keyModifiers(events.KeyUp, events.KeyControl, 105, xlib.ControlMask); got != 0 {
		t.Fatalf("last release left stale Control: %v", got)
	}
	var focus xlib.Event
	focus.Type, focus.AnyEvent().Window = xlib.FocusOut, 2
	handleEvent(focus)
	if other.modifierKeys != [32]byte{} {
		t.Fatal("lost focus retained modifier keys")
	}
	// 后续快照替换旧状态，避免保留在其他应用中已经释放的按键。
	snapshot.KeymapEvent().KeyVector = [32]byte{}
	handleEvent(snapshot)
	if w.modifierKeys != [32]byte{} || other.modifierKeys != [32]byte{} {
		t.Fatal("empty snapshot did not replace modifier state")
	}
}
