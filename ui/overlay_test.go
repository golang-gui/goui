package ui

import (
	"slices"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/platform/events"
)

func TestOverlayReconcilesMainIndependently(t *testing.T) {
	r := newRoot()
	t.Cleanup(r.unmountWindow)
	build := func(main View, text string) *OverlayView {
		return Overlay(main, OverlayItem(TextInput().Text(text).Name("find")))
	}
	o := r.update(build(Label("main"), "first")).(*gui.Overlay)
	main, floating := o.Child(), o.Children()[1].(*gui.TextInput)
	entry := r.root.state.(*overlayChildren).items[floating]
	handle := entry.position
	for _, child := range []View{nil, Button("new main"), Label("main again")} {
		if got := r.update(build(child, "updated")); got != o {
			t.Fatal("rebuild replaced overlay")
		}
		if floating.Parent() != o || floating.Text() != "updated" {
			t.Fatal("main replacement lost the floating slot")
		}
		state := r.root.state.(*overlayChildren)
		if state.items[floating] != entry || entry.position != handle {
			t.Fatal("rebuild replaced the floating state or signal")
		}
		if children := o.Children(); children[len(children)-1] != floating {
			t.Fatal("main insertion changed stacking order")
		}
	}
	if main.Parent() != nil {
		t.Fatal("replaced main is still attached")
	}
	r.update(Overlay(nil))
	if len(o.Children()) != 0 || floating.Parent() != nil || entry.position != nil {
		t.Fatal("clearing declarations retained widgets or signals")
	}
}

func TestOverlayReconcilesFloatingPrefixAndNilItems(t *testing.T) {
	r := newRoot()
	t.Cleanup(r.unmountWindow)
	builds := 0
	empty := &testCompositionView{builds: &builds}
	items := []*OverlayItemView{OverlayItem(Label("one")), OverlayItem(TextInput()), OverlayItem(Label("three"))}
	v := Overlay(Label("main"), items...)
	items[0] = nil // The caller's slice is not the declaration's storage.
	o := r.update(v).(*gui.Overlay)
	main := o.Child()
	old := o.Children()[1:]
	r.update(Overlay(nil).Child(Label("MAIN")).Overlays(
		nil, OverlayItem(nil), OverlayItem(empty),
		OverlayItem(Label("ONE")), OverlayItem(Label("replacement")), OverlayItem(Label("THREE")),
	))
	children := o.Children()
	if builds != 1 || len(children) != 4 || o.Child() != main || children[1] != old[0] {
		t.Fatal("nil filtering, composition expansion or prefix reuse failed")
	}
	if children[2] == old[1] || children[3] == old[2] || old[1].Parent() != nil || old[2].Parent() != nil {
		t.Fatal("type mismatch must rebuild and detach the tail")
	}
	if children[1].(*gui.Label).Text() != "ONE" {
		t.Fatal("reused content was not updated")
	}
	r.update(Overlay(Label("main")).Overlays(OverlayItem(Label("ignored"))).Overlays())
	if len(o.Children()) != 1 || len(r.root.children) != 1 {
		t.Fatal("Overlays must replace, not append")
	}
}

func TestOverlayPositionFillAndVisibilityUpdates(t *testing.T) {
	r := newRoot()
	t.Cleanup(r.unmountWindow)
	oldCalls, newCalls := 0, 0
	oldPosition := func(_, _ geometry.Size, p *geometry.Point) { oldCalls++; p.X = 7 }
	newPosition := func(available, size geometry.Size, p *geometry.Point) {
		newCalls++
		*p = geometry.Point{X: available.Width - size.Width - 8, Y: 9}
	}
	content := func() *BoxView { return HBox().MinSize(40, 20).MaxSize(60, 30) }
	o := r.update(Overlay(nil, OverlayItem(content()).OnPosition(oldPosition))).(*gui.Overlay)
	child := o.Children()[0]
	entry := r.root.state.(*overlayChildren).items[child]
	handle := entry.position
	o.Arrange(geometry.Rect(20, 30, 200, 100))
	if child.Rect() != geometry.Rect(7, 0, 40, 20) || oldCalls != 1 {
		t.Fatal("default content sizing/initial callback failed")
	}
	r.update(Overlay(nil, OverlayItem(content()).Fill(true).OnPosition(oldPosition).OnPosition(newPosition)))
	o.Arrange(geometry.Rect(20, 30, 200, 100))
	if child.Rect() != geometry.Rect(-8, 9, 200, 100) || oldCalls != 1 || newCalls != 1 {
		t.Fatal("fill did not use GUI constraints or callback was not replaced")
	}
	r.update(Overlay(nil, OverlayItem(content()).OnPosition(newPosition)))
	o.Arrange(geometry.Rect(20, 30, 200, 100))
	if child.Rect() != geometry.Rect(152, 9, 40, 20) || o.OverlayFill(child) {
		t.Fatal("omitted Fill did not restore natural sizing")
	}
	// No UI rebuild is needed for resizing.
	o.Arrange(geometry.Rect(20, 30, 300, 150))
	if child.Rect() != geometry.Rect(252, 9, 40, 20) {
		t.Fatal("resizing did not query placement again")
	}
	r.update(Overlay(nil, OverlayItem(content().Visible(false)).OnPosition(newPosition)))
	before := newCalls
	o.Arrange(geometry.Rect(0, 0, 100, 80))
	if newCalls != before || child.Visible() || o.Snapshot().Children[0].Visible {
		t.Fatal("hidden content still queried placement or snapshot lost visibility")
	}
	r.update(Overlay(nil, OverlayItem(content()).Fill(true).Fill(false).OnPosition(newPosition).OnPosition(nil)))
	o.Arrange(geometry.Rect(0, 0, 100, 80))
	if child.Rect() != geometry.Rect(0, 0, 40, 20) || !child.Visible() || newCalls != before {
		t.Fatal("explicit callback reset/default visibility failed")
	}
	r.update(Overlay(nil, OverlayItem(content()).OnPosition(newPosition)))
	r.update(Overlay(nil, OverlayItem(content())))
	o.Arrange(geometry.Rect(0, 0, 100, 80))
	if child.Rect().Pos != (geometry.Point{}) || newCalls != before {
		t.Fatal("omitted callback retained stale placement")
	}
	if o.Children()[0] != child || entry.position != handle {
		t.Fatal("configuration/visibility updates recreated child or connection")
	}
}

type overlayDisconnectProbe struct {
	signal.Handle
	calls int
}

func (h *overlayDisconnectProbe) Disconnect() { h.calls++; h.Handle.Disconnect() }

func TestOverlayReleaseOwnsEveryNodeAndSignal(t *testing.T) {
	for _, destroying := range []bool{false, true} {
		t.Run(map[bool]string{false: "detach", true: "window-destroy"}[destroying], func(t *testing.T) {
			r := newRoot()
			trackers := []*lifecycleTracker{{}, {}, {}}
			child := func(i int) View { return &lifecycleView{tracker: trackers[i]} }
			o := r.update(Overlay(child(0), OverlayItem(
				Overlay(child(1), OverlayItem(child(2))),
			))).(*gui.Overlay)
			outerState := r.root.state.(*overlayChildren)
			innerState := r.root.children[1].nodes[0].state.(*overlayChildren)
			var probes []*overlayDisconnectProbe
			for _, state := range []*overlayChildren{outerState, innerState} {
				entry := state.items[state.overlay.Children()[1]]
				probe := &overlayDisconnectProbe{Handle: entry.position}
				entry.position = probe
				probes = append(probes, probe)
			}
			outerChildren := o.Children()
			inner := outerChildren[1].(*gui.Overlay)
			innerChildren := inner.Children()
			if destroying {
				r.unmountForWindowDestroy()
				if !slices.Equal(o.Children(), outerChildren) || !slices.Equal(inner.Children(), innerChildren) {
					t.Fatal("UI detached widgets before Window could destroy them")
				}
			} else {
				r.update(Label("replacement"))
				if len(o.Children()) != 0 || len(inner.Children()) != 0 {
					t.Fatal("unmounted overlay retained its main/floating children")
				}
				for _, widget := range append(outerChildren, innerChildren...) {
					if widget.Parent() != nil {
						t.Fatal("released child still has a parent")
					}
				}
			}
			r.unmountWindow()
			for _, tracker := range trackers {
				if tracker.mounts != 1 || tracker.unmounts != 1 {
					t.Fatalf("child was not released exactly once: %+v", tracker)
				}
			}
			for _, probe := range probes {
				if probe.calls != 1 {
					t.Fatalf("position connection disconnected %d times", probe.calls)
				}
			}
			if outerState.items != nil || innerState.items != nil {
				t.Fatal("unmount retained slot state")
			}
		})
	}
}

// Headless host: inputs still enter Window.DispatchEvent and the real GUI
// dispatcher/controllers; never invoke a button callback directly.
type overlayInputWindow struct {
	*testWindow
	dispatcher gui.EventDispatcher
}

func (w *overlayInputWindow) DispatchEvent(event events.Event) error {
	return w.dispatcher.DispatchEvent(w, event)
}

func TestOverlayBindingSnapshotAndPointerRouting(t *testing.T) {
	r := newRoot()
	t.Cleanup(r.unmountWindow)
	win := &overlayInputWindow{testWindow: newTestWindow()}
	background, floating, panel := 0, 0, 0
	build := func(show bool) *OverlayView {
		return Overlay(
			Button().OnClick(func() { background++ }),
			OverlayItem(Button().MinSize(40, 20).Name("find").OnClick(func() { floating++ })).
				OnPosition(func(_, _ geometry.Size, p *geometry.Point) { p.X, p.Y = 150, 10 }),
			OverlayItem(Overlay(
				HBox().Name("mask"),
				OverlayItem(Button().MinSize(80, 40).Name("panel").OnClick(func() { panel++ })).
					OnPosition(func(available, size geometry.Size, p *geometry.Point) {
						p.X, p.Y = (available.Width-size.Width)/2, (available.Height-size.Height)/2
					}),
			).Visible(show)).Fill(true),
		)
	}
	o := r.updateWindow(win, build(false)).(*gui.Overlay)
	o.Arrange(geometry.Rect(0, 0, 300, 200))
	info := o.Snapshot()
	if info.Role != gui.RoleBox || len(info.Children) != 3 || info.Children[1].Role != gui.RoleButton || info.Children[2].Visible {
		t.Fatalf("binding added wrappers or changed GUI semantics: %+v", info)
	}
	click := func(point geometry.Point) {
		t.Helper()
		for _, kind := range []events.EventType{events.PointerDown, events.PointerUp} {
			if err := win.DispatchEvent(events.PointerEvent{EventType: kind, Button: events.PointerButtonLeft, Position: point}); err != nil {
				t.Fatal(err)
			}
		}
	}
	click(geometry.Point{X: 10, Y: 10})
	click(info.Children[1].Bounds.Center())
	if background != 1 || floating != 1 {
		t.Fatal("floating content/outside input routing failed")
	}
	r.updateWindow(win, build(true))
	o.Arrange(geometry.Rect(0, 0, 300, 200))
	info = o.Snapshot()
	confirm := info.Children[2]
	if confirm.Role != gui.RoleBox || len(confirm.Children) != 2 || confirm.Bounds != geometry.Rect(0, 0, 300, 200) {
		t.Fatalf("nested full-size confirmation lost its tree: %+v", confirm)
	}
	click(geometry.Point{X: 10, Y: 10})
	click(confirm.Children[1].Bounds.Center())
	if background != 1 || floating != 1 || panel != 1 {
		t.Fatal("mask/panel did not block underlying siblings")
	}
	r.updateWindow(win, build(false))
	click(geometry.Point{X: 10, Y: 10})
	if background != 2 {
		t.Fatal("hiding confirmation did not restore background input")
	}
}
