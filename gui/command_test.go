package gui

import (
	"slices"
	"testing"

	"github.com/golang-gui/goui/platform/events"
)

func TestCommandActivationAndEnabledNotifications(t *testing.T) {
	c := NewCommand()
	var states []bool
	c.ConnectEnabled(func(v bool) { states = append(states, v) })
	n := 0
	h := c.ConnectActivate(func() { n++ })
	c.Activate()
	c.SetEnabled(false)
	c.SetEnabled(false)
	c.Activate()
	c.SetEnabled(true)
	c.Activate()
	h.Disconnect()
	c.Activate()
	if n != 2 || !slices.Equal(states, []bool{false, true}) {
		t.Fatalf("激活或通知错误：n=%d states=%v", n, states)
	}
	c = NewCommand()
	first, stale := 0, 0
	c.ConnectActivate(func() { first++; c.SetEnabled(false); c.SetEnabled(true) })
	c.ConnectActivate(func() { stale++ })
	c.Activate()
	if first != 1 || stale != 0 {
		t.Fatal("禁用再恢复后执行了旧激活的后续回调")
	}
}

func TestCommandShortcutAssociations(t *testing.T) {
	a, b := NewCommand(), NewCommand()
	changesA, changesB := 0, 0
	a.ConnectShortcuts(func() { changesA++ })
	b.ConnectShortcuts(func() { changesB++ })
	x := NewShortcut(KeyGesture{Key: KeyS, Modifiers: ModPrimary})
	y := NewShortcut(KeyGesture{Key: KeyF5})
	x.SetCommand(a)
	x.SetCommand(a)
	y.SetCommand(a)
	got := a.Shortcuts()
	got[0] = nil
	if changesA != 2 || !slices.Equal(a.Shortcuts(), []*Shortcut{x, y}) {
		t.Fatal("重复关联或只读列表错误")
	}
	x.SetGesture(x.Gesture())
	x.SetEnabled(false)
	x.SetAutoRepeat(true)
	if changesA != 2 {
		t.Fatal("不相关属性触发了快捷键配置通知")
	}
	x.SetGesture(KeyGesture{Key: KeyS, Modifiers: ModPrimary | ModShift})
	x.SetCommand(b)
	if changesA != 4 || changesB != 1 || !slices.Equal(a.Shortcuts(), []*Shortcut{y}) || !slices.Equal(b.Shortcuts(), []*Shortcut{x}) {
		t.Fatal("手势更新或关联移动错误")
	}
	x.SetCommand(nil)
	y.SetCommand(nil)
	if len(a.Shortcuts()) != 0 || len(b.Shortcuts()) != 0 {
		t.Fatal("解除关联未释放反向引用")
	}
}

func TestCommandShortcutDispatchAndIndependentEnablement(t *testing.T) {
	w := testWindowWithSurface(&window{surfaceBase: surfaceBase{root: newTestWidget()}})
	c := NewCommand()
	n, fallback := 0, 0
	c.ConnectActivate(func() { n++ })
	s := NewShortcut(KeyGesture{Key: KeyF5})
	s.SetCommand(c)
	other := NewShortcut(s.Gesture())
	other.ConnectActivate(func() { fallback++ })
	w.Shortcuts().AddShortcut(s)
	w.Shortcuts().AddShortcut(other)
	press := func() {
		t.Helper()
		if err := w.DispatchEvent(shortcutPress(events.KeyF5, 0)); err != nil {
			t.Fatal(err)
		}
	}
	press()
	c.SetEnabled(false)
	press()
	c.SetEnabled(true)
	s.SetEnabled(false)
	press()
	if n != 1 || fallback != 2 || !c.Enabled() {
		t.Fatalf("操作与入口禁用混淆：n=%d fallback=%d", n, fallback)
	}
	s.SetEnabled(true)
	h := s.ConnectActivate(func() { c.SetEnabled(false); c.SetEnabled(true) })
	press()
	h.Disconnect()
	if n != 1 {
		t.Fatal("快捷键通知使旧操作失效后仍然执行")
	}
	replacement := NewCommand()
	h = s.ConnectActivate(func() { s.SetCommand(replacement) })
	press()
	h.Disconnect()
	if n != 1 || len(c.Shortcuts()) != 0 {
		t.Fatal("关联替换后执行了旧操作或残留反向关系")
	}
}

func TestKeyGestureDisplayText(t *testing.T) {
	for _, tc := range []struct {
		os   string
		g    KeyGesture
		want string
	}{
		{"windows", KeyGesture{Key: KeyS, Modifiers: ModPrimary}, "Ctrl+S"},
		{"linux", KeyGesture{Key: KeyS, Modifiers: ModPrimary | ModControl | ModShift}, "Ctrl+Shift+S"},
		{"darwin", KeyGesture{Key: KeyS, Modifiers: ModPrimary | ModOption | ModShift}, "⌥⇧⌘S"},
		{"darwin", KeyGesture{Key: KeyPrimary, Modifiers: ModPrimary}, "⌘"},
		{"windows", KeyGesture{Key: KeyWin, Modifiers: ModWin}, "Win"},
		{"linux", KeyGesture{Key: KeyF24}, "F24"},
		{"linux", KeyGesture{Key: KeyNumpad0}, "Num 0"},
		{"windows", KeyGesture{Key: KeyC, Modifiers: ModCommand}, ""},
		{"darwin", KeyGesture{Key: KeyWin}, ""},
		{"linux", KeyGesture{}, ""},
		{"linux", KeyGesture{Key: Key(9999)}, ""},
	} {
		t.Run(tc.os+tc.want, func(t *testing.T) {
			if got := tc.g.displayText(tc.os); got != tc.want {
				t.Fatalf("展示文字=%q，预期=%q", got, tc.want)
			}
		})
	}
}
