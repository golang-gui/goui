package ui

import (
	"testing"

	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/platform/events"
)

func TestShortcutReconciliation(t *testing.T) {
	r := newRoot()
	t.Cleanup(r.unmountWindow)
	w := &overlayInputWindow{testWindow: newTestWindow()}
	old, current := 0, 0
	build := func(fn func(), enabled bool) *LabelView {
		return Label("shortcut").Shortcuts(Shortcut(KeyF5).Enabled(enabled).OnActivate(fn))
	}
	r.updateWindow(w, build(func() { old++ }, true))
	press := func() {
		t.Helper()
		if err := w.DispatchEvent(events.KeyEvent{EventType: events.KeyDown, Key: events.KeyF5}); err != nil {
			t.Fatal(err)
		}
	}
	if old != 0 {
		t.Fatal("mount ran callback")
	}
	press()
	for i := 0; i < 3; i++ {
		r.updateWindow(w, build(func() { current++ }, true))
	}
	press()
	if old != 1 || current != 1 {
		t.Fatalf("stale or duplicated callback: %d %d", old, current)
	}
	r.updateWindow(w, build(func() { current++ }, false))
	press()
	if current != 1 {
		t.Fatal("disabled binding fired")
	}
	r.updateWindow(w, Label("shortcut"))
	press()
	if current != 1 {
		t.Fatal("removed declaration fired")
	}
	if len(w.Widget().EventControllers()) != 0 {
		t.Fatal("empty declarative controller retained")
	}
	r.updateWindow(w, build(func() { r.unmountWindow() }, true))
	press() // reentrant unmount must not invoke a stale callback or panic.
}

func TestWindowShortcutBindingsPreserveImperativeEntries(t *testing.T) {
	w := newTestWindow()
	m := &windowMount{window: w}
	defer m.disconnect()
	imperative := gui.NewShortcut(gui.KeyGesture{Key: gui.KeyF6})
	w.Shortcuts().AddShortcut(imperative)
	n := 0
	v := Window("keys").Shortcuts(Shortcut(KeyF5).OnActivate(func() { n++ }))
	if err := m.applyWindowProperties(v); err != nil {
		t.Fatal(err)
	}
	first := m.shortcuts.entries[0].shortcut
	if err := m.applyWindowProperties(v); err != nil {
		t.Fatal(err)
	}
	if len(m.shortcuts.entries) != 1 || m.shortcuts.entries[0].shortcut != first || n != 0 {
		t.Fatal("rebuild recreated binding or called callback")
	}
	if err := m.applyWindowProperties(Window("keys")); err != nil {
		t.Fatal(err)
	}
	// Route through the real dispatcher, placing the window controller on
	// this headless test root only to exercise connection cleanup.
	root := gui.NewLabel("keys")
	root.AddEventController(w.Shortcuts())
	w.SetWidget(root)
	d := gui.EventDispatcher{}
	imperative.ConnectActivate(func() { n++ })
	_ = d.DispatchEvent(w, events.KeyEvent{EventType: events.KeyDown, Key: events.KeyF5})
	if n != 0 {
		t.Fatal("declarative entry survived removal")
	}
	_ = d.DispatchEvent(w, events.KeyEvent{EventType: events.KeyDown, Key: events.KeyF6})
	if n != 1 {
		t.Fatal("UI removed an imperative entry")
	}
}

func TestShortcutDeclarationUpdates(t *testing.T) {
	r := newRoot()
	t.Cleanup(r.unmountWindow)
	w := &overlayInputWindow{testWindow: newTestWindow()}
	n := 0
	activate := func() { n++ }
	update := func(bindings ...*ShortcutView) {
		t.Helper()
		r.updateWindow(w, Label("shortcut").Shortcuts(bindings...))
	}
	press := func(modifiers events.Modifiers, repeat bool, want int, consumed bool) {
		t.Helper()
		handled := false
		err := w.DispatchEvent(events.KeyEvent{
			EventType: events.KeyDown, Key: events.KeyF5,
			Modifiers: modifiers, Repeat: repeat, Handled: &handled,
		})
		if err != nil {
			t.Fatal(err)
		}
		if n != want || handled != consumed {
			t.Fatalf("activations=%d, consumed=%v; want %d, %v", n, handled, want, consumed)
		}
	}

	update(Shortcut(KeyF5))
	press(0, false, 0, false) // No callback: leave the key available.
	update(Shortcut(KeyF5).OnActivate(activate))
	press(0, false, 1, true) // Enabled by default, without modifiers.
	press(0, true, 1, true)  // Suppress repeats without falling through.
	press(events.ModifierShift, false, 1, false)

	// Replacing the descriptor updates the existing registration. Modifiers
	// replaces rather than merges the earlier combination.
	update(Shortcut(KeyF5).Modifiers(ModControl).Modifiers(ModShift).OnActivate(activate))
	press(0, false, 1, false)
	press(events.ModifierControl|events.ModifierShift, false, 1, false)
	press(events.ModifierShift, false, 2, true)

	update(Shortcut(KeyF5).Modifiers(ModShift).Modifiers(0).AutoRepeat(true).OnActivate(activate))
	press(events.ModifierShift, false, 2, false)
	press(0, true, 3, true)
	update(Shortcut(KeyF5).OnActivate(activate))
	press(0, true, 3, true) // Rebuilding without AutoRepeat restores its default.
	update(Shortcut(KeyF5).Enabled(false).OnActivate(activate))
	press(0, false, 3, false)

	update(nil, Shortcut(KeyF5).OnActivate(activate), nil)
	press(0, false, 4, true)
	update(nil)
	press(0, false, 4, false)
	update()
	if len(w.Widget().EventControllers()) != 0 {
		t.Fatal("removed shortcuts retained a controller")
	}
}

func TestShortcutDescriptorsReconcileByPosition(t *testing.T) {
	controller := gui.NewShortcutController()
	var bindings shortcutBindings
	t.Cleanup(bindings.clear)
	first := Shortcut(KeyF5).OnActivate(func() {})
	second := Shortcut(KeyF6).OnActivate(func() {})
	bindings.update(controller, []*ShortcutView{first, second})
	entry := bindings.entries[0].shortcut
	bindings.update(controller, []*ShortcutView{nil, second})
	if len(bindings.entries) != 1 || bindings.entries[0].shortcut != entry {
		t.Fatal("descriptor identity replaced positional reconciliation")
	}
	if entry.Gesture().Key != gui.KeyF6 {
		t.Fatal("reused registration retained its old key")
	}
	bindings.update(controller, nil)
	if len(bindings.entries) != 0 {
		t.Fatal("removed declarations retained registrations")
	}
}

func TestShortcutCommandRefOwnershipAndModes(t *testing.T) {
	r := newRoot()
	defer r.unmountWindow()
	w := &overlayInputWindow{testWindow: newTestWindow()}
	first, second := gui.NewCommand(), gui.NewCommand()
	a, b, direct := 0, 0, 0
	first.ConnectActivate(func() { a++ })
	second.ConnectActivate(func() { b++ })
	update := func(s *ShortcutView) {
		t.Helper()
		r.updateWindow(w, Label("editor").Shortcuts(s))
	}
	press := func() {
		t.Helper()
		if err := w.DispatchEvent(events.KeyEvent{EventType: events.KeyDown, Key: events.KeyF5}); err != nil {
			t.Fatal(err)
		}
	}
	update(Shortcut(KeyF5).CommandRef(first))
	press()
	update(Shortcut(KeyF5).CommandRef(second))
	press()
	if a != 1 || b != 1 || len(first.Shortcuts()) != 0 || len(second.Shortcuts()) != 1 {
		t.Fatal("直接对象未执行或换对象未解绑旧关联")
	}
	second.SetEnabled(false)
	press()
	if b != 1 {
		t.Fatal("外部禁用没有限制快捷键")
	}
	update(Shortcut(KeyF5).CommandRef(second).OnActivate(func() { direct++ }))
	press()
	if direct != 1 || len(second.Shortcuts()) != 0 {
		t.Fatal("直接回调没有替换对象来源")
	}
	update(Shortcut(KeyF5).CommandRef(first).Command("missing"))
	press()
	if a != 1 || len(first.Shortcuts()) != 0 {
		t.Fatal("ID 来源没有替换直接对象")
	}
	update(Shortcut(KeyF5).Command("missing").CommandRef(first))
	press()
	if a != 2 {
		t.Fatal("对象来源没有替换缺失 ID")
	}
	update(Shortcut(KeyF5).CommandRef(nil))
	press()
	if a != 2 || len(first.Shortcuts()) != 0 {
		t.Fatal("空对象没有清除操作")
	}
	update(Shortcut(KeyF5).CommandRef(first))
	r.unmountWindow()
	if len(first.Shortcuts()) != 0 || !first.Enabled() || second.Enabled() {
		t.Fatal("卸载未解绑自有 Shortcut，或修改了外部 Enabled")
	}
	first.Activate()
	if a != 3 {
		t.Fatal("卸载清除了外部激活处理者")
	}
}

func TestWindowShortcutCommandRef(t *testing.T) {
	w := newTestWindow()
	m := &windowMount{window: w}
	defer m.disconnect()
	command := gui.NewCommand()
	if err := m.applyWindowProperties(Window("keys").Shortcuts(Shortcut(KeyF5).CommandRef(command))); err != nil {
		t.Fatal(err)
	}
	if len(command.Shortcuts()) != 1 || command.Shortcuts()[0].Command() != command {
		t.Fatal("窗口快捷键在无作用域时未借用外部命令")
	}
	m.disconnect()
	if len(command.Shortcuts()) != 0 || !command.Enabled() {
		t.Fatal("窗口解绑影响了外部命令")
	}
}

func TestShortcutCommandRefCanUnmountDuringReplacement(t *testing.T) {
	r := newRoot()
	defer r.unmountWindow()
	first, second := gui.NewCommand(), gui.NewCommand()
	view := func(c *gui.Command) View {
		return Label("editor").Shortcuts(Shortcut(KeyF5).CommandRef(c), Shortcut(KeyF6).CommandRef(c))
	}
	r.update(view(first))
	first.ConnectShortcuts(func() { r.unmountWindow() })
	r.update(view(second))
	if r.root != nil || len(first.Shortcuts()) != 0 || len(second.Shortcuts()) != 0 || !first.Enabled() || !second.Enabled() {
		t.Fatal("通知重入卸载留下关联或改变外部命令")
	}
}

// 退出事件循环不等于销毁原生 Window，声明层仍须解除自己创建的关联。
func TestWindowCommandRefReleasedWhenAppStops(t *testing.T) {
	command := gui.NewCommand()
	fake := newWindowTestApplication()
	rt := newApp(fake, func() RootView {
		return Window("main").Content(Label("editor")).Shortcuts(Shortcut(KeyF5).CommandRef(command))
	})
	defer rt.destroyAll()
	if err := rt.rebuild(); err != nil {
		t.Fatal(err)
	}
	imperative := gui.NewShortcut(gui.KeyGesture{Key: gui.KeyF6})
	imperative.SetCommand(command)
	fake.windows[0].Shortcuts().AddShortcut(imperative)
	defer imperative.SetCommand(nil)
	if len(command.Shortcuts()) != 2 {
		t.Fatal("未建立窗口声明和外部关联")
	}
	rt.stop()
	if !command.Enabled() || len(command.Shortcuts()) != 1 || command.Shortcuts()[0] != imperative || fake.windows[0].destroyed {
		t.Fatal("App.stop 留下声明层关联、清除了外部关联或销毁了 GUI 窗口")
	}
}
