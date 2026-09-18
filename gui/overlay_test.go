package gui

import (
	"math"
	"slices"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/events"
)

type overlayMeasureWidget struct {
	WidgetBase
	measurement layout.Measurement
	constraints []layout.Constraint
}

func (w *overlayMeasureWidget) Measure(c layout.Constraint) layout.Measurement {
	w.constraints = append(w.constraints, c)
	return w.measurement.Constrain(w.layoutConstraint(c))
}

func TestOverlayOnlyMeasuresMainChild(t *testing.T) {
	o := NewOverlay()
	main := &overlayMeasureWidget{measurement: layout.MeasuredWithBaseline(geometry.Size{Width: 80, Height: 30}, 22)}
	floating := &overlayMeasureWidget{measurement: layout.Measured(geometry.Size{Width: 400, Height: 300})}
	o.SetChild(main)
	o.AddOverlay(floating)
	c := layout.Loose(geometry.Size{Width: 500, Height: 400})
	if got := o.Measure(c); got != main.measurement {
		t.Fatalf("main measurement/baseline lost: %+v", got)
	}
	if len(floating.constraints) != 0 {
		t.Fatal("floating child must not participate in parent measurement")
	}
	main.SetVisible(false)
	if got := o.Measure(c); got != (layout.Measurement{}) {
		t.Fatalf("hidden main should not affect measurement: %+v", got)
	}
	o.SetChild(nil)
	o.SetMinSize(geometry.Size{Width: 40, Height: 20})
	if got := o.Measure(c).Size; got != (geometry.Size{Width: 40, Height: 20}) {
		t.Fatalf("empty overlay lost its size preference: %+v", got)
	}
	if got := o.Measure(layout.Tight(geometry.Size{Width: 10, Height: 8})).Size; got != (geometry.Size{Width: 10, Height: 8}) {
		t.Fatalf("parent constraint did not win: %+v", got)
	}
	o.SetVisible(false)
	if got := o.Measure(c); got != (layout.Measurement{}) {
		t.Fatalf("hidden overlay measured nonzero: %+v", got)
	}
}

func TestOverlayMeasuresAtFinalAllocationAndHonorsFill(t *testing.T) {
	o := NewOverlay()
	main := &overlayMeasureWidget{measurement: layout.Measured(geometry.Size{Width: 80, Height: 20})}
	floating := &overlayMeasureWidget{measurement: layout.Measured(geometry.Size{Width: 120, Height: 10})}
	floating.SetMinSize(geometry.Size{Height: 30})
	floating.SetMaxSize(geometry.Size{Width: 100, Height: 40})
	o.SetChild(main)
	o.AddOverlay(floating)
	o.Measure(layout.Loose(geometry.Size{Width: 800, Height: 600}))
	var sizes []geometry.Size
	o.ConnectOverlayPosition(floating, func(available, size geometry.Size, position *geometry.Point) {
		sizes = append(sizes, size)
		*position = geometry.Point{X: available.Width - size.Width - 8, Y: 8}
	})
	o.Arrange(geometry.Rect(50, 60, 200, 100))
	if main.Rect() != geometry.Rect(0, 0, 200, 100) {
		t.Fatalf("main must fill in local coordinates: %+v", main.Rect())
	}
	if got := main.constraints[len(main.constraints)-1]; got != layout.Tight(geometry.Size{Width: 200, Height: 100}) {
		t.Fatalf("main not remeasured at its final allocation: %+v", got)
	}
	if floating.Rect() != geometry.Rect(92, 8, 100, 30) {
		t.Fatalf("natural size preferences or positioning lost: %+v", floating.Rect())
	}
	o.Arrange(geometry.Rect(50, 60, 60, 20))
	if floating.Rect() != geometry.Rect(-8, 8, 60, 20) {
		t.Fatalf("resized allocation did not constrain/reposition: %+v", floating.Rect())
	}
	o.SetOverlayFill(floating, true)
	o.Arrange(geometry.Rect(50, 60, 200, 100))
	if floating.Rect() != geometry.Rect(-8, 8, 200, 100) || !o.OverlayFill(floating) {
		t.Fatalf("fill must override self max and retain position query: %+v", floating.Rect())
	}
	if !slices.Equal(sizes, []geometry.Size{{Width: 100, Height: 30}, {Width: 60, Height: 20}, {Width: 200, Height: 100}}) {
		t.Fatalf("queries did not receive final measured sizes: %+v", sizes)
	}
	o.SetOverlayFill(floating, false)
	o.Arrange(geometry.Rect(0, 0, 200, 100))
	if floating.Rect().Size != (geometry.Size{Width: 100, Height: 30}) {
		t.Fatal("disabling fill did not restore natural sizing")
	}
}

func TestOverlayLinearBoxKeepsContentSize(t *testing.T) {
	o := NewOverlay()
	main := newTestWidget()
	o.SetChild(main)
	bar := NewLinearBox(layout.DirectionHorizontal)
	bar.SetSpacing(6)
	bar.SetPadding(4)
	first := newSizedWidget(geometry.Size{Width: 10, Height: 20})
	second := newSizedWidget(geometry.Size{Width: 30, Height: 10})
	bar.AddChild(first)
	bar.AddChild(second)
	bar.SetMainWeight(1) // Overlay is not a LinearBox parent.
	o.AddOverlay(bar)
	o.ConnectOverlayPosition(bar, func(available, size geometry.Size, position *geometry.Point) {
		*position = geometry.Point{X: available.Width - size.Width - 8, Y: 8}
	})
	o.Arrange(geometry.Rect(0, 0, 200, 100))
	if bar.Rect() != geometry.Rect(138, 8, 54, 28) {
		t.Fatalf("linear box should hug its contents: %+v", bar.Rect())
	}
	if first.Rect() != geometry.Rect(4, 4, 10, 20) || second.Rect() != geometry.Rect(20, 9, 30, 10) {
		t.Fatalf("linear child layout changed: first=%+v second=%+v", first.Rect(), second.Rect())
	}
	if Pick(o, geometry.Point{X: 130, Y: 10}) != main || Pick(o, geometry.Point{X: 139, Y: 9}) != bar {
		t.Fatal("outside must reach main, but padding must still hit the box")
	}
}

func TestOverlayPositionSignalsAndLayoutInvalidation(t *testing.T) {
	o := NewOverlay()
	a, b := newTestWidget(), newTestWidget()
	a.SetMinSize(geometry.Size{Width: 20, Height: 10})
	b.SetMinSize(geometry.Size{Width: 5, Height: 5})
	o.AddOverlay(a)
	o.AddOverlay(b)
	win := &window{}
	win.SetWidget(o)
	win.layoutDirty = false
	var calls []string
	handle := o.ConnectOverlayPosition(a, func(_, _ geometry.Size, p *geometry.Point) {
		calls = append(calls, "a-first")
		p.X = 11
	})
	if !win.layoutDirty {
		t.Fatal("connecting a position query must request layout")
	}
	o.ConnectOverlayPosition(a, func(_, _ geometry.Size, p *geometry.Point) {
		calls = append(calls, "a-second")
		p.X += 3
	})
	o.ConnectOverlayPosition(b, func(_, _ geometry.Size, p *geometry.Point) {
		calls = append(calls, "b")
		p.Y = 7
	})
	bounds := geometry.Rect(0, 0, 100, 80)
	o.Arrange(bounds)
	assertStrings(t, calls, []string{"a-first", "a-second", "b"})
	if a.Rect().X != 14 || b.Rect().X != 0 || b.Rect().Y != 7 {
		t.Fatal("position results leaked between children or lost connection order")
	}
	handle.Block()
	o.Arrange(bounds)
	if a.Rect().X != 3 {
		t.Fatal("blocked callback still ran or position was not reset")
	}
	handle.Unblock()
	o.Arrange(bounds)
	if a.Rect().X != 14 {
		t.Fatal("unblocking did not restore the callback")
	}
	handle.Disconnect()
	o.Arrange(bounds)
	if a.Rect().X != 3 {
		t.Fatal("disconnected callback still ran")
	}
	win.layoutDirty = false
	o.SetOverlayFill(a, false)
	if win.layoutDirty {
		t.Fatal("unchanged fill requested layout")
	}
	o.SetOverlayFill(a, true)
	if !win.layoutDirty {
		t.Fatal("changing fill did not request layout")
	}
	calls = nil
	a.SetVisible(false)
	o.Arrange(bounds)
	assertStrings(t, calls, []string{"b"})
	a.SetVisible(true)
	o.Arrange(bounds)
	if a.Rect() != geometry.Rect(3, 0, 100, 80) {
		t.Fatalf("hiding lost queries or fill: %+v", a.Rect())
	}
}

func TestOverlayWindowResizeAndContentChanges(t *testing.T) {
	o := NewOverlay()
	o.SetChild(newTestWidget())
	floating := newTestWidget()
	floating.SetMinSize(geometry.Size{Width: 60, Height: 20})
	o.AddOverlay(floating)
	queries := 0
	margin := float32(8)
	handle := o.ConnectOverlayPosition(floating, func(available, size geometry.Size, p *geometry.Point) {
		queries++
		*p = geometry.Point{X: available.Width - size.Width - margin, Y: margin}
	})
	win := &window{rootBase: rootBase{painter: new(testGraphicsPainter)}}
	win.SetWidget(o)
	paint := func() {
		t.Helper()
		if err := win.DispatchEvent(events.PaintEvent{}); err != nil {
			t.Fatal(err)
		}
	}
	resize := func(width, height float32) {
		t.Helper()
		if err := win.DispatchEvent(events.SizeEvent{Width: width, Height: height, PixelWidth: 2 * width, PixelHeight: 2 * height}); err != nil {
			t.Fatal(err)
		}
		paint()
	}
	resize(300, 200)
	if floating.Rect() != geometry.Rect(232, 8, 60, 20) {
		t.Fatalf("wrong initial placement: %+v", floating.Rect())
	}
	paint()
	if queries != 1 {
		t.Fatal("unchanged paint should not repeat layout queries")
	}
	resize(180, 100)
	if floating.Rect() != geometry.Rect(112, 8, 60, 20) {
		t.Fatalf("window resize did not reposition overlay: %+v", floating.Rect())
	}
	floating.SetMinSize(geometry.Size{Width: 80, Height: 30})
	paint()
	if floating.Rect() != geometry.Rect(92, 8, 80, 30) {
		t.Fatalf("content size change did not remeasure/reposition: %+v", floating.Rect())
	}
	margin = 12
	o.RequestLayout()
	paint()
	if floating.Rect() != geometry.Rect(88, 12, 80, 30) {
		t.Fatalf("captured state change did not take effect: %+v", floating.Rect())
	}
	handle.Disconnect()
	o.RequestLayout()
	paint()
	if floating.Rect() != geometry.Rect(0, 0, 80, 30) {
		t.Fatal("disconnection did not restore default placement")
	}
}

func TestOverlayRemoveAndReparent(t *testing.T) {
	o := NewOverlay()
	main, floating := newTestWidget(), newTestWidget()
	o.SetChild(main)
	o.AddOverlay(floating)
	o.SetOverlayFill(floating, true)
	calls := 0
	handle := o.ConnectOverlayPosition(floating, func(_, _ geometry.Size, _ *geometry.Point) { calls++ })
	o.RemoveOverlay(main)
	if o.Child() != main {
		t.Fatal("RemoveOverlay removed the main child")
	}
	o.RemoveOverlay(floating)
	if floating.Parent() != nil || floating.destroyed || o.OverlayFill(floating) {
		t.Fatal("remove must detach, clear fill and not destroy")
	}
	o.AddOverlay(floating)
	o.Arrange(geometry.Rect(0, 0, 100, 80))
	if calls != 0 || o.OverlayFill(floating) {
		t.Fatal("re-adding retained the previous attachment's settings")
	}
	handle.Disconnect()

	other := NewLinearBox(layout.DirectionHorizontal)
	other.AddChild(floating)
	other.AddChild(main)
	if o.Child() != nil || o.OverlayFill(floating) {
		t.Fatal("reparented children still reported as owned")
	}
	o.Measure(layout.Unbounded())
	if len(o.overlays) != 0 || o.child != nil || len(o.Children()) != 0 {
		t.Fatal("layout did not discard reparented metadata")
	}
	o.AddOverlay(floating)
	o.SetOverlayFill(floating, true)
	o.SetChild(floating) // Promote without making two copies in the tree.
	if o.Child() != floating || len(o.Children()) != 1 || o.OverlayFill(floating) {
		t.Fatal("floating-to-main promotion failed")
	}
	o.RemoveChild(floating)
	if o.Child() != nil || len(o.Children()) != 0 {
		t.Fatal("RemoveChild did not clear the main slot")
	}
}

func TestOverlayRejectsInvalidChildren(t *testing.T) {
	o := NewOverlay()
	main, floating, foreign := newTestWidget(), newTestWidget(), newTestWidget()
	o.SetChild(main)
	o.AddOverlay(floating)
	parent := NewLinearBox(layout.DirectionVertical)
	parent.AddChild(o)
	dead := newTestWidget()
	dead.destroy(dead)
	for _, invalid := range []Widget{o, parent, dead} {
		o.SetChild(invalid)
		o.AddOverlay(invalid)
	}
	o.AddOverlay(nil)
	o.AddOverlay(main)
	o.AddOverlay(floating)
	if o.Child() != main || !slices.Equal(o.Children(), []Widget{main, floating}) {
		t.Fatal("invalid/duplicate add changed the tree")
	}
	for _, invalid := range []Widget{nil, main, foreign} {
		o.SetOverlayFill(invalid, true)
		if o.OverlayFill(invalid) {
			t.Fatal("invalid child acquired fill")
		}
		h := o.ConnectOverlayPosition(invalid, func(_, _ geometry.Size, _ *geometry.Point) { t.Fatal("invalid query ran") })
		h.Block()
		h.Unblock()
		h.Disconnect()
	}
	o.ConnectOverlayPosition(floating, nil).Disconnect()
	o.Arrange(geometry.Rect(0, 0, 100, 80))
}

func TestOverlayTreeOrderPaintingClippingAndSnapshot(t *testing.T) {
	o := NewOverlay()
	first := newPainterTestWidget(fillAt(1))
	second := newPainterTestWidget(fillAt(2))
	main := newPainterTestWidget(fillAt(3))
	for _, child := range []Widget{first, second} {
		child.SetMinSize(geometry.Size{Width: 30, Height: 20})
		o.AddOverlay(child)
	}
	main.SetID("main")
	first.SetID("first")
	second.SetID("second")
	o.SetChild(main) // Main must be below children added earlier.
	o.ConnectOverlayPosition(first, func(_, _ geometry.Size, p *geometry.Point) { p.X = -10 })
	o.ConnectOverlayPosition(second, func(_, _ geometry.Size, p *geometry.Point) { p.X, p.Y = 90, 70 })
	o.Arrange(geometry.Rect(10, 20, 100, 80))
	backend := new(recordingPainterBackend)
	paintWidget(o, newPainter(backend, geometry.Rect(0, 0, 200, 200), 1))
	if len(backend.fills) != 3 {
		t.Fatalf("unexpected paint count: %d", len(backend.fills))
	}
	for i, x := range []float32{3, 1, 2} {
		if backend.fills[i].rect.X != x {
			t.Fatal("paint order differs from the canonical child order")
		}
	}
	if backend.fills[1].clip != geometry.Rect(10, 20, 20, 20) || backend.fills[2].clip != geometry.Rect(100, 90, 10, 10) {
		t.Fatalf("ordinary tree clipping did not apply: %+v", backend.fills)
	}
	info := o.Snapshot()
	if info.Role != RoleBox {
		t.Fatalf("overlay must report a generic container: %q", info.Role)
	}
	if len(info.Children) != 3 || info.Children[0].ID != "main" || info.Children[1].ID != "first" || info.Children[2].ID != "second" {
		t.Fatalf("unexpected snapshot children: %+v", info.Children)
	}
	if info.Children[2].Bounds != geometry.Rect(100, 90, 30, 20) {
		t.Fatalf("snapshot must report absolute bounds: %+v", info.Children[2].Bounds)
	}
	replacement := newTestWidget()
	o.SetChild(replacement)
	if !slices.Equal(o.Children(), []Widget{replacement, first, second}) || main.Parent() != nil {
		t.Fatal("replacing main changed overlay order")
	}
}

func TestOverlayDispatchesToFloatingContentOrUnderlyingChild(t *testing.T) {
	o := NewOverlay()
	main := newTestWidget()
	mainClicks, buttonClicks := 0, 0
	mainClick := NewClickEventController()
	mainClick.ConnectClicked(func(EventContext) { mainClicks++ })
	main.AddEventController(mainClick)
	wheelEvents := 0
	wheel := NewWheelEventController()
	wheel.ConnectScroll(func(EventContext, events.WheelEvent) { wheelEvents++ })
	main.AddEventController(wheel)
	o.SetChild(main)
	bar := NewLinearBox(layout.DirectionHorizontal)
	bar.SetPadding(8)
	button := NewButton()
	button.SetMinSize(geometry.Size{Width: 30, Height: 20})
	button.ConnectClicked(func() { buttonClicks++ })
	bar.AddChild(button)
	o.AddOverlay(bar)
	o.ConnectOverlayPosition(bar, func(_, _ geometry.Size, p *geometry.Point) { p.X, p.Y = 100, 10 })
	win := &window{}
	win.SetWidget(o)
	o.Arrange(geometry.Rect(0, 0, 300, 200))
	overlayTestClick(t, win, geometry.Point{X: 10, Y: 10})
	overlayTestClick(t, win, button.windowRect().Center())
	overlayTestClick(t, win, geometry.Point{X: 101, Y: 11}) // Padding hits bar, not main.
	if mainClicks != 1 || buttonClicks != 1 {
		t.Fatalf("unexpected clicks: main=%d button=%d", mainClicks, buttonClicks)
	}
	for _, point := range []geometry.Point{{X: 10, Y: 10}, {X: 101, Y: 11}} {
		if err := win.DispatchEvent(events.WheelEvent{Position: point, DeltaY: 1}); err != nil {
			t.Fatal(err)
		}
	}
	if wheelEvents != 1 {
		t.Fatal("wheel was forwarded to an underlying sibling after an unhandled overlay hit")
	}
	bar.SetVisible(false)
	overlayTestClick(t, win, geometry.Point{X: 101, Y: 11})
	if mainClicks != 2 {
		t.Fatal("hidden bar still intercepted input")
	}
}

func TestOverlayNestedConfirmationLayer(t *testing.T) {
	o := NewOverlay()
	main := newTestWidget()
	o.SetChild(main)
	confirm := NewOverlay()
	mask, panel := newTestWidget(), newTestWidget()
	panel.SetMinSize(geometry.Size{Width: 120, Height: 60})
	confirm.SetChild(mask)
	confirm.AddOverlay(panel)
	confirm.ConnectOverlayPosition(panel, func(available, size geometry.Size, p *geometry.Point) {
		*p = geometry.Point{X: (available.Width - size.Width) / 2, Y: (available.Height - size.Height) / 2}
	})
	o.AddOverlay(confirm)
	o.SetOverlayFill(confirm, true)
	counts := make(map[Widget]int)
	for _, child := range []Widget{main, mask, panel} {
		click := NewClickEventController()
		click.ConnectClicked(func(EventContext) { counts[child]++ })
		child.AddEventController(click)
	}
	win := &window{}
	win.SetWidget(o)
	o.Arrange(geometry.Rect(0, 0, 400, 300))
	if confirm.Rect() != geometry.Rect(0, 0, 400, 300) || mask.Rect() != confirm.Rect() || panel.Rect() != geometry.Rect(140, 120, 120, 60) {
		t.Fatalf("wrong nested layout: confirm=%+v mask=%+v panel=%+v", confirm.Rect(), mask.Rect(), panel.Rect())
	}
	info := o.Snapshot()
	if len(info.Children) != 2 || info.Children[1].Role != RoleBox || len(info.Children[1].Children) != 2 {
		t.Fatalf("nested overlay lost its semantic subtree: %+v", info)
	}
	if got := info.Children[1].Children[1].Bounds; got != geometry.Rect(140, 120, 120, 60) {
		t.Fatalf("nested panel snapshot has wrong bounds: %+v", got)
	}
	overlayTestClick(t, win, geometry.Point{X: 10, Y: 10})
	overlayTestClick(t, win, geometry.Point{X: 200, Y: 150})
	if counts[main] != 0 || counts[mask] != 1 || counts[panel] != 1 {
		t.Fatalf("panel clicks must not bubble to sibling mask or main: %+v", counts)
	}
	confirm.SetVisible(false)
	overlayTestClick(t, win, geometry.Point{X: 200, Y: 150})
	if counts[main] != 1 {
		t.Fatal("hiding confirmation did not reveal underlying content")
	}
	confirm.SetVisible(true)
	o.Arrange(geometry.Rect(0, 0, 200, 120))
	if panel.Rect() != geometry.Rect(40, 30, 120, 60) {
		t.Fatalf("confirmation did not recenter after resize: %+v", panel.Rect())
	}
}

func TestOverlaySnapshotWithoutMainChild(t *testing.T) {
	o := NewOverlay()
	if info := o.Snapshot(); info.Role != RoleBox || len(info.Children) != 0 {
		t.Fatalf("unexpected empty overlay snapshot: %+v", info)
	}
	button := NewButton()
	button.SetID("floating")
	o.AddOverlay(button)
	o.ConnectOverlayPosition(button, func(_, _ geometry.Size, p *geometry.Point) {
		*p = geometry.Point{X: 15, Y: 25}
	})
	o.Arrange(geometry.Rect(10, 20, 200, 100))
	info := o.Snapshot()
	if len(info.Children) != 1 || info.Children[0].ID != "floating" || info.Children[0].Role != RoleButton {
		t.Fatalf("floating content lost its semantics without a main child: %+v", info)
	}
	if info.Children[0].Bounds.Pos != (geometry.Point{X: 25, Y: 45}) {
		t.Fatalf("floating snapshot must use window coordinates: %+v", info.Children[0].Bounds)
	}
	o.RemoveOverlay(button)
	if info := o.Snapshot(); len(info.Children) != 0 {
		t.Fatalf("detached floating content remains in snapshot: %+v", info)
	}
}

func TestOverlaySnapshotOcclusionDoesNotDisableContent(t *testing.T) {
	o := NewOverlay()
	button := NewButton()
	clicks := 0
	button.ConnectClicked(func() { clicks++ })
	o.SetChild(button)
	mask := newTestWidget() // Even without a controller, the mask blocks picking.
	o.AddOverlay(mask)
	o.SetOverlayFill(mask, true)
	win := &window{}
	win.SetWidget(o)
	o.Arrange(geometry.Rect(0, 0, 200, 100))
	for _, visible := range []bool{true, false, true} {
		mask.SetVisible(visible)
		info := o.Snapshot()
		if len(info.Children) != 2 || info.Children[1].Visible != visible {
			t.Fatalf("visibility must not remove or reorder snapshot nodes: %+v", info)
		}
		underlying := info.Children[0]
		if underlying.Role != RoleButton || !underlying.Visible || !underlying.Enabled || !slices.Contains(underlying.Actions, ActionClick) {
			t.Fatalf("occlusion changed the button's semantics: %+v", underlying)
		}
		if underlying.Bounds != geometry.Rect(0, 0, 200, 100) {
			t.Fatalf("snapshot bounds must not be reduced by occlusion: %+v", underlying.Bounds)
		}
		before := clicks
		overlayTestClick(t, win, underlying.Bounds.Center())
		want := before
		if !visible {
			want++
		}
		if clicks != want {
			t.Fatalf("mask visible=%v: clicks=%d, want %d", visible, clicks, want)
		}
	}
}

func TestOverlayLifecycleAndDestroyedQueries(t *testing.T) {
	var calls []string
	main := newLifecycleWidget("main", &calls)
	floating := newLifecycleWidget("floating", &calls)
	o := NewOverlay()
	o.AddOverlay(floating)
	o.SetChild(main)
	win := &window{}
	win.SetWidget(o)
	assertStrings(t, calls, []string{"main mount", "floating mount"})
	calls = nil
	o.RemoveOverlay(floating)
	assertStrings(t, calls, []string{"floating unmount"})
	if floating.destroyed {
		t.Fatal("unmount destroyed floating child")
	}
	o.AddOverlay(floating)
	floating.SetFocusable(true)
	if !win.SetFocusedWidget(floating) {
		t.Fatal("floating child not mounted in the ordinary focus tree")
	}
	o.ConnectOverlayPosition(floating, func(_, _ geometry.Size, _ *geometry.Point) { win.Destroy() })
	o.ConnectOverlayPosition(floating, func(_, _ geometry.Size, _ *geometry.Point) { t.Fatal("query ran after destruction") })
	o.Arrange(geometry.Rect(0, 0, 100, 80))
	if !o.destroyed || !main.destroyed || !floating.destroyed || win.FocusedWidget() != nil {
		t.Fatal("window did not clean up the entire ordinary widget tree")
	}
	if floating.Rect() != (geometry.Rectangle{}) {
		t.Fatal("floating child was arranged after destruction")
	}
}

func TestOverlayNonFinitePosition(t *testing.T) {
	o := NewOverlay()
	child := newTestWidget()
	o.AddOverlay(child)
	o.ConnectOverlayPosition(child, func(_, _ geometry.Size, p *geometry.Point) {
		*p = geometry.Point{X: float32(math.NaN()), Y: float32(math.Inf(1))}
	})
	o.Arrange(geometry.Rect(0, 0, 100, 80))
	if child.Rect().Pos != (geometry.Point{}) {
		t.Fatalf("non-finite coordinates reached Arrange: %+v", child.Rect())
	}
}

func overlayTestClick(t *testing.T, win Window, point geometry.Point) {
	t.Helper()
	for _, kind := range []events.EventType{events.PointerDown, events.PointerUp} {
		if err := win.DispatchEvent(events.PointerEvent{
			EventType: kind,
			Position:  point,
			Button:    events.PointerButtonLeft,
		}); err != nil {
			t.Fatal(err)
		}
	}
}
