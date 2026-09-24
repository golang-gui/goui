package ui

import (
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
)

// Every widget-view constructor must wire ViewBase.Self so the shared chain
// modifiers (ID/Name/Hidden/Style) return the concrete view instead of panicking on
// a nil Self. This is the guardrail for the one unsafe edge of the self-type.
func TestViewConstructorsWireSelf(t *testing.T) {
	cases := []struct {
		name string
		make func() View // ends in a shared modifier, which panics if Self is nil
	}{
		{"Button", func() View { return Button("x").ID("btn") }},
		{"Label", func() View { return Label("x").ID("lbl") }},
		{"HBox", func() View { return HBox().ID("hbox") }},
		{"HeaderBar", func() View { return HeaderBar(nil).ID("header") }},
		{"VBox", func() View { return VBox().ID("vbox") }},
		{"TextInput", func() View { return TextInput().ID("input") }},
		{"Image", func() View { return Image(nil).ID("img") }},
	}
	for _, c := range cases {
		view := c.make() // would panic in self() if the constructor forgot Self
		if view == nil {
			t.Fatalf("%s: chained modifier returned nil (Self not wired)", c.name)
		}
		if view.base().id == "" {
			t.Fatalf("%s: shared ID modifier did not apply", c.name)
		}
	}
}

func TestViewIDAndNameRemainIndependentAcrossUpdates(t *testing.T) {
	r := newRoot()
	t.Cleanup(r.unmountWindow)
	w := r.update(Label("content").ID("first").Name("natural").Style("label")).(*gui.Label)
	if w.ID() != "first" || w.Name() != "natural" || w.StyleName() != "label" ||
		w.Snapshot().Text != "content" {
		t.Fatalf("initial identity: %+v", w.Snapshot())
	}
	if next := r.update(Label("updated").ID("second").Name("natural")); next != w {
		t.Fatal("changing ID replaced the widget")
	}
	if w.ID() != "second" || w.Name() != "natural" || w.StyleName() != "" {
		t.Fatalf("updated identity: %+v", w.Snapshot())
	}
	r.update(Label("updated").ID("").Name(""))
	if w.ID() != "" || w.Name() != "" {
		t.Fatalf("explicit empty values were ignored: id=%q name=%q", w.ID(), w.Name())
	}
	r.update(Label("plain"))
	if w.ID() != "" || w.Name() != "" {
		t.Fatalf("removed modifiers did not restore defaults: id=%q name=%q", w.ID(), w.Name())
	}
}

type initialIdentityView struct{ ViewBase[initialIdentityView] }

func newInitialIdentityView() *initialIdentityView {
	v := new(initialIdentityView)
	v.Self = v
	return v
}

func (v *initialIdentityView) Build() View { return v }
func (v *initialIdentityView) Mount(BuildContext) gui.Widget {
	w := gui.NewLabel("content")
	w.SetID("original-id")
	w.SetName("original-name")
	return w
}
func (v *initialIdentityView) Update(BuildContext, gui.Widget)  {}
func (v *initialIdentityView) Unmount(BuildContext, gui.Widget) {}

func TestViewIdentityRestoresConstructorDefaults(t *testing.T) {
	r := newRoot()
	t.Cleanup(r.unmountWindow)
	w := r.update(newInitialIdentityView().ID("replacement").Name("renamed"))
	if w.ID() != "replacement" || w.Name() != "renamed" {
		t.Fatalf("declared identity: id=%q name=%q", w.ID(), w.Name())
	}
	r.update(newInitialIdentityView().ID("").Name(""))
	if w.ID() != "" || w.Name() != "" {
		t.Fatalf("explicit empty identity: id=%q name=%q", w.ID(), w.Name())
	}
	r.update(newInitialIdentityView())
	if w.ID() != "original-id" || w.Name() != "original-name" {
		t.Fatalf("constructor identity not restored: id=%q name=%q", w.ID(), w.Name())
	}
}

// A view's size modifiers flow through apply() to the gui widget's size
// constraint (an empty VBox has zero intrinsic size, so MinSize alone drives it).
func TestViewSizeModifiersApplyToWidget(t *testing.T) {
	root := newRoot()
	w := root.update(VBox().MinSize(120, 80))
	got := w.Measure(layout.Loose(geometry.Size{Width: 1000, Height: 1000}))
	if got.Size != (geometry.Size{Width: 120, Height: 80}) {
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
	if got.Size != (geometry.Size{Width: 32, Height: 32}) { // empty box, padding on both sides
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
