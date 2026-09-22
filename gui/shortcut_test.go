package gui

import (
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform"
	"github.com/golang-gui/goui/platform/events"
)

func TestShortcutModalSuspendsOwnerText(t *testing.T) {
	editor, win, _ := newEditorFixture(t, "")
	probe := &shortcutIMEProbe{enabled: true}
	win.inputMethod = probe
	win.SetModalTarget(&shortcutModal{})
	if probe.enabled || probe.resets != 1 {
		t.Fatal("owner IME not suspended")
	}
	handled := false
	e := shortcutPress(events.KeyQ, 0)
	e.Handled = &handled
	_ = win.DispatchEvent(e)
	if !handled {
		t.Fatal("modal input allowed owner default")
	}
	win.onInputMethod(platform.InputMethodResult{Kind: platform.InputMethodCommit, Text: "leak"})
	if editor.Model().Text() != "" {
		t.Fatal("modal input edited owner")
	}
	win.SetModalTarget(nil)
	if !probe.enabled {
		t.Fatal("IME not restored after modal closes")
	}
	win.onInputMethod(platform.InputMethodResult{Kind: platform.InputMethodCommit, Text: "ok"})
	if editor.Model().Text() != "ok" {
		t.Fatal("owner text not restored")
	}
}

type shortcutIMEProbe struct {
	enabled bool
	resets  int
}

func (p *shortcutIMEProbe) SetEnabled(v bool)               { p.enabled = v }
func (p *shortcutIMEProbe) SetCaretRect(geometry.Rectangle) {}
func (p *shortcutIMEProbe) Reset()                          { p.resets++ }
func (p *shortcutIMEProbe) Destroy()                        {}

func TestShortcutNativeDefaultAndTextPriority(t *testing.T) {
	for _, mods := range []events.Modifiers{0, events.ModifierOption, events.ModifierAltGraph, events.ModifierControl | events.ModifierAlt} {
		editor, win, _ := newEditorFixture(t, "")
		gestureMods := KeyModifiers(0)
		if mods == events.ModifierOption {
			gestureMods = ModOption
		}
		if mods == events.ModifierAltGraph {
			gestureMods = ModAltGraph
		}
		if mods == events.ModifierControl|events.ModifierAlt {
			gestureMods = ModControl | ModAlt
		}
		s := NewShortcut(KeyGesture{Key: KeyQ, Modifiers: gestureMods})
		count := 0
		s.ConnectActivate(func() { count++ })
		win.Shortcuts().AddShortcut(s)
		handled := false
		e := shortcutPress(events.KeyQ, mods)
		e.Handled = &handled
		_ = win.DispatchEvent(e)
		if count != 0 || handled {
			t.Fatal("text input was consumed by a window shortcut")
		}
		win.onInputMethod(platform.InputMethodResult{Kind: platform.InputMethodCommit, Text: "@"})
		if editor.Model().Text() != "@" {
			t.Fatal("native text was not committed")
		}
	}
	_, win, _ := newEditorFixture(t, "")
	count := 0
	s := NewShortcut(KeyGesture{Key: KeyQ})
	handled := false
	s.ConnectActivate(func() {
		if !handled {
			t.Error("native default not canceled before callback")
		}
		count++
	})
	win.Shortcuts().AddShortcut(s)
	win.Shortcuts().SetPhase(PhaseCapture)
	e := shortcutPress(events.KeyQ, 0)
	e.Handled = &handled
	_ = win.DispatchEvent(e)
	if !handled || count != 1 {
		t.Fatal("capture did not cancel native text default")
	}
	e.Repeat = true
	handled = false
	_ = win.DispatchEvent(e)
	if !handled || count != 1 {
		t.Fatal("suppressed repeat leaked to native text default")
	}
}

func shortcutPress(key events.Key, mods events.Modifiers) events.KeyEvent {
	return events.KeyEvent{EventType: events.KeyDown, Key: key, Modifiers: mods}
}

func TestShortcutMatchingAndRepeat(t *testing.T) {
	w := &window{root: newTestWidget()}
	s := NewShortcut(KeyGesture{Key: KeyS, Modifiers: ModPrimary})
	n := 0
	s.ConnectActivate(func() { n++ })
	w.Shortcuts().AddShortcut(s)
	w.Shortcuts().AddShortcut(s) // idempotent registration
	mods, _ := ModPrimary.Resolve()
	e := shortcutPress(events.KeyS, mods)
	_ = w.DispatchEvent(e)
	e.Repeat = true
	_ = w.DispatchEvent(e)
	if n != 1 {
		t.Fatalf("default repeats: %d", n)
	}
	s.SetAutoRepeat(true)
	_ = w.DispatchEvent(e)
	if n != 2 {
		t.Fatalf("enabled repeats: %d", n)
	}
	e.Modifiers |= events.ModifierShift
	_ = w.DispatchEvent(e)
	if n != 2 {
		t.Fatal("extra modifiers matched")
	}
	e.Modifiers = mods
	e.EventType = events.KeyUp
	_ = w.DispatchEvent(e)
	s.SetEnabled(false)
	e.EventType = events.KeyDown
	_ = w.DispatchEvent(e)
	if n != 2 {
		t.Fatal("disabled or key-up fired")
	}
	s.SetEnabled(true)
	w.Shortcuts().RemoveShortcut(s)
	_ = w.DispatchEvent(e)
	if n != 2 {
		t.Fatal("removed shortcut fired")
	}
	if (KeyGesture{Key: KeyUnknown}).Matches(shortcutPress(events.KeyUnknown, 0)) {
		t.Fatal("unknown key matched")
	}
	primary, _ := KeyPrimary.Resolve()
	if !(KeyGesture{Key: KeyPrimary}).Matches(shortcutPress(primary, mods)) {
		t.Fatal("modifier key could not match its own press")
	}
}

func TestShortcutModifierSidesRemainEquivalent(t *testing.T) {
	w := &window{root: newTestWidget()}
	count := 0
	s := NewShortcut(KeyGesture{Key: KeyControl})
	s.ConnectActivate(func() { count++ })
	w.Shortcuts().AddShortcut(s)
	for _, location := range []events.KeyLocation{events.KeyLocationLeft, events.KeyLocationRight} {
		e := shortcutPress(events.KeyControl, events.ModifierControl)
		e.Location = location
		_ = w.DispatchEvent(e)
		e.EventType = events.KeyUp
		_ = w.DispatchEvent(e)
	}
	if count != 2 {
		t.Fatalf("side-neutral Control binding activated %d times, want 2", count)
	}
	// S has its own standard location; the held modifier's side must not be
	// inferred from it. The native aggregate bit survives the first release.
	w.Shortcuts().Clear()
	s = NewShortcut(KeyGesture{Key: KeyS, Modifiers: ModControl})
	s.ConnectActivate(func() { count++ })
	w.Shortcuts().AddShortcut(s)
	_ = w.DispatchEvent(shortcutPress(events.KeyS, events.ModifierControl))
	_ = w.DispatchEvent(shortcutPress(events.KeyS, 0))
	if count != 3 {
		t.Fatalf("aggregate Control+S matched incorrectly: count=%d", count)
	}
}

func TestShortcutPropagationPriority(t *testing.T) {
	root, child := newTestWidget(), newTestWidget()
	root.AddChild(child)
	w := &window{root: root, rootBase: rootBase{focusedWidget: child}}
	local, parent := NewShortcutController(), NewShortcutController()
	child.AddEventController(local)
	root.AddEventController(parent)
	var got string
	add := func(c *ShortcutController, name string) *Shortcut {
		s := NewShortcut(KeyGesture{Key: KeyF5})
		s.ConnectActivate(func() { got = name })
		c.AddShortcut(s)
		return s
	}
	first := add(local, "local")
	second := add(local, "second")
	add(parent, "parent")
	add(w.Shortcuts(), "window")
	press := func(want string) {
		t.Helper()
		got = ""
		_ = w.DispatchEvent(shortcutPress(events.KeyF5, 0))
		if got != want {
			t.Fatalf("got %q want %q", got, want)
		}
	}
	press("local")
	first.SetEnabled(false)
	press("second")
	second.SetEnabled(false)
	press("parent")
	parent.SetPhase(PhaseTarget)
	press("window")
	w.Shortcuts().SetPhase(PhaseCapture)
	first.SetEnabled(true)
	press("window")
}

func TestShortcutEditorFirstAndCaptureOverride(t *testing.T) {
	editor, w, _ := newEditorFixture(t, "abc")
	n := 0
	s := NewShortcut(KeyGesture{Key: KeyA, Modifiers: ModPrimary})
	s.ConnectActivate(func() { n++ })
	w.Shortcuts().AddShortcut(s)
	editorKey(t, w, events.KeyA, textCommandModifier())
	if n != 0 || editor.Selection() != (TextSelection{0, 3}) {
		t.Fatal("window shortcut stole editor selection")
	}
	w.Shortcuts().SetPhase(PhaseCapture)
	editorKey(t, w, events.KeyA, textCommandModifier())
	if n != 1 {
		t.Fatal("explicit capture did not override")
	}
}

func TestShortcutLifetimeAndModal(t *testing.T) {
	w := &window{root: newTestWidget()}
	s := NewShortcut(KeyGesture{Key: KeyF5})
	later := 0
	s.ConnectActivate(func() { w.Destroy() })
	s.ConnectActivate(func() { later++ })
	w.Shortcuts().AddShortcut(s)
	_ = w.DispatchEvent(shortcutPress(events.KeyF5, 0))
	if later != 0 {
		t.Fatal("callback continued after window destruction")
	}
	w.Shortcuts().AddShortcut(NewShortcut(KeyGesture{Key: KeyF5}))
	if len(w.Shortcuts().shortcuts) != 0 {
		t.Fatal("destroyed window accepted binding")
	}
	w = &window{root: newTestWidget()}
	s = NewShortcut(KeyGesture{Key: KeyF5})
	s.ConnectActivate(func() { later++ })
	w.Shortcuts().AddShortcut(s)
	modal := &shortcutModal{}
	w.SetModalTarget(modal)
	_ = w.DispatchEvent(shortcutPress(events.KeyF5, 0))
	if modal.keys != 1 || later != 0 {
		t.Fatal("modal keyboard escaped to owner")
	}
}

type shortcutModal struct{ keys int }

func (m *shortcutModal) DispatchEvent(events.Event) error { m.keys++; return nil }
func (m *shortcutModal) RequestDismiss()                  {}
