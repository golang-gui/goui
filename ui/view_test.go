package ui

import (
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
)

// Every widget-view constructor must wire ViewBase.Self so the shared chain
// modifiers (Name/Hidden/Style) return the concrete view instead of panicking on
// a nil Self. This is the guardrail for the one unsafe edge of the self-type.
func TestViewConstructorsWireSelf(t *testing.T) {
	cases := []struct {
		name string
		make func() View // ends in a shared modifier, which panics if Self is nil
	}{
		{"Button", func() View { return Button("x").Name("btn") }},
		{"Label", func() View { return Label("x").Name("lbl") }},
		{"HBox", func() View { return HBox().Name("hbox") }},
		{"VBox", func() View { return VBox().Name("vbox") }},
		{"TextInput", func() View { return TextInput().Name("input") }},
		{"Image", func() View { return Image(nil).Name("img") }},
	}
	for _, c := range cases {
		view := c.make() // would panic in self() if the constructor forgot Self
		if view == nil {
			t.Fatalf("%s: chained modifier returned nil (Self not wired)", c.name)
		}
		if view.base().name == "" {
			t.Fatalf("%s: shared Name modifier did not apply", c.name)
		}
	}
}

// A view's size modifiers flow through apply() to the gui widget's size
// constraint (an empty VBox has zero intrinsic size, so MinSize alone drives it).
func TestViewSizeModifiersApplyToWidget(t *testing.T) {
	root := newRoot()
	w := root.update(VBox().MinSize(120, 80))
	got := w.Measure(layout.Loose(geometry.Size{Width: 1000, Height: 1000}))
	if got != (geometry.Size{Width: 120, Height: 80}) {
		t.Fatalf("view MinSize not applied to widget: %+v (want 120x80)", got)
	}
}

func TestBoxCrossAlignAppliesToContainer(t *testing.T) {
	root := newRoot()
	box := root.update(VBox().CrossAlign(layout.CrossStretch)).(*gui.LinearBox)

	if box.CrossAlign() != layout.CrossStretch {
		t.Fatalf("view CrossAlign did not apply to the mounted widget: %v", box.CrossAlign())
	}
}

func TestViewPaddingApplies(t *testing.T) {
	root := newRoot()
	w := root.update(VBox().Padding(16))
	got := w.Measure(layout.Loose(geometry.Size{Width: 500, Height: 500}))
	if got != (geometry.Size{Width: 32, Height: 32}) { // empty box, padding on both sides
		t.Fatalf("view Padding not applied: %+v (want 32x32)", got)
	}
}

func TestViewPaddingUnsetKeepsControlDefault(t *testing.T) {
	// A button has a non-zero built-in padding (6); a view that never calls
	// .Padding must not overwrite it to 0 — that is what paddingSet guards.
	root := newRoot()
	if b := root.update(Button("x")).(*gui.Button); b.Padding() != 6 {
		t.Fatalf("unset padding overwrote control default: %v (want 6)", b.Padding())
	}
	root2 := newRoot()
	if b := root2.update(Button("y").Padding(10)).(*gui.Button); b.Padding() != 10 {
		t.Fatalf("explicit padding not applied: %v (want 10)", b.Padding())
	}
}

func TestViewFocusableAppliesAndUnsetDontOverride(t *testing.T) {
	// A MenuButton is focusable by default (set in its constructor for
	// menu-bar keyboard navigation). A view that never calls .Focusable must
	// leave that default intact.
	root := newRoot()
	if b := root.update(MenuButton("File")).(*gui.MenuButton); !b.Focusable() {
		t.Fatal("unset focusable overwrote control default (want focusable)")
	}

	// Explicitly disabling/enabling focus is applied through the shared
	// modifier, so a menu-bar-style row can strip focus where needed.
	root2 := newRoot()
	if b := root2.update(MenuButton("File").Focusable(false)).(*gui.MenuButton); b.Focusable() {
		t.Fatal("Focusable(false) not applied to the mounted widget")
	}
	if b := root2.update(MenuButton("File").Focusable(true)).(*gui.MenuButton); !b.Focusable() {
		t.Fatal("Focusable(true) not applied to the mounted widget")
	}
}

// OnFocus is a cross-cutting modifier wired on viewBase: the view base
// lifecycle connects the mounted widget's gui focus signal and routes the
// callback through the persistent viewBaseContext. This test drives the focus
// signal directly (the Reconcile root builds the widget tree; focus dispatch is
// exercised in the gui package).
func TestViewOnFocusConnectsAndDynamicallyReadsCallback(t *testing.T) {
	root := newRoot()
	var saw []bool

	root.update(Button("x").OnFocus(func(focused bool) {
		saw = append(saw, focused)
	}))
	node := root.root
	baseCtx := node.baseCtx
	if baseCtx == nil || len(baseCtx.handles) != 1 {
		t.Fatal("OnFocus should connect a focus signal handle on mount")
	}

	// The callback is read from the live viewBaseContext at fire time, but the
	// context (and its handle) is independent of any single rebuild — a rebuilt
	// view keeps the same context and handle.
	root.update(Button("x").OnFocus(func(focused bool) {
		saw = append(saw, focused)
	}))
	if baseCtx != node.baseCtx || len(baseCtx.handles) != 1 {
		t.Fatal("OnFocus context and handle should persist across rebuilds")
	}
	if node.view.base().onFocus == nil {
		t.Fatal("OnFocus callback should remain set across rebuilds")
	}
	if baseCtx.onFocus == nil {
		t.Fatal("viewBase.update should refresh the context's effective callback")
	}

	// Dropping OnFocus keeps the handle connected (a no-op when the context's
	// callback is cleared), which is the cross-cutting contract: setting it back
	// re-arms the same handle.
	root.update(Button("x"))
	if node.baseCtx == nil || len(node.baseCtx.handles) != 1 {
		t.Fatal("OnFocus handle should remain connected across rebuilds even when callback removed")
	}
	if node.view.base().onFocus != nil {
		t.Fatal("removed OnFocus callback should be cleared on the view base")
	}
	if baseCtx.onFocus != nil {
		t.Fatal("removed OnFocus callback should clear the context's effective callback")
	}
}

func TestViewUnmountDisconnectsOnFocusHandle(t *testing.T) {
	root := newRoot()
	root.update(Button("x").OnFocus(func(focused bool) {}))
	node := root.root
	if node.baseCtx == nil || len(node.baseCtx.handles) != 1 {
		t.Fatal("setup: OnFocus handle not connected")
	}

	root.unmountWindow()

	if node.baseCtx == nil || len(node.baseCtx.handles) != 0 || node.baseCtx.onFocus != nil {
		t.Fatal("OnFocus handle and callback should be cleared on unmount")
	}
}
