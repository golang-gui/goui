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
