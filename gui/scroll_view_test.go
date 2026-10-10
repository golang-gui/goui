package gui

import (
	"math"
	"testing"

	"github.com/goexlib/mathx"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/graphics"
)

func TestScrollViewCoupledBarsAndResize(t *testing.T) {
	for _, size := range []geometry.Size{{Width: 96, Height: 200}, {Width: 200, Height: 96}} {
		sv := NewScrollView()
		sv.SetChild(&mockWidget{size: size})
		sv.Measure(layout.Tight(geometry.Size{Width: 100, Height: 100}))
		sv.Arrange(geometry.Rect(0, 0, 100, 100))
		if !sv.vbar.Visible() || !sv.hbar.Visible() || sv.viewport.Rect() != geometry.Rect(0, 0, 92, 92) {
			t.Fatalf("bar-induced overflow not resolved: size=%v viewport=%v", size, sv.viewport.Rect())
		}
		info := sv.Snapshot()
		if info.MaxScrollX != size.Width-92 || info.MaxScrollY != size.Height-92 {
			t.Fatalf("snapshot ranges disagree with viewport: %+v", info)
		}
		sv.SetScrollX(1000)
		sv.SetScrollY(1000)
		sv.Arrange(geometry.Rect(0, 0, 220, 220))
		if sv.hbar.Visible() || sv.vbar.Visible() || sv.ScrollX() != 0 || sv.ScrollY() != 0 || sv.viewport.Rect() != geometry.Rect(0, 0, 220, 220) {
			t.Fatal("old bars or offsets survived a larger allocation")
		}
	}
}

// responsiveScrollWidget models wrapping without fonts or a desktop. Its
// natural text extent is 400 DIP; each wrapped line/column takes 20 DIP.
type responsiveScrollWidget struct {
	WidgetBase
	horizontal  bool
	length      float32
	constraints []layout.Constraint
}

func (w *responsiveScrollWidget) Measure(c layout.Constraint) layout.Measurement {
	w.constraints = append(w.constraints, c)
	cross := c.Max.Width
	if w.horizontal {
		cross = c.Max.Height
	}
	cross = min(w.length, cross)
	lines := float32(1)
	if cross > 0 {
		lines = float32(math.Ceil(float64(w.length / cross)))
	}
	size := geometry.Size{Width: cross, Height: 20 * lines}
	if w.horizontal {
		size.Width, size.Height = size.Height, size.Width
	}
	return layout.Measured(size).Constrain(c)
}

func TestScrollViewSingleAxisMeasuresActualViewport(t *testing.T) {
	for _, direction := range []ScrollDirection{ScrollVertical, ScrollHorizontal} {
		horizontal := direction == ScrollHorizontal
		sv := NewScrollView()
		sv.SetDirection(direction)
		content := &responsiveScrollWidget{length: 400, horizontal: horizontal}
		sv.SetChild(content)
		size := geometry.Size{Width: 100, Height: 60}
		if horizontal {
			size.Width, size.Height = size.Height, size.Width
		}
		sv.Measure(layout.Tight(size))
		sv.Arrange(geometry.Rectangle{Size: size})
		wantContent, wantArea := geometry.Size{Width: 92, Height: 100}, geometry.Size{Width: 92, Height: 60}
		if horizontal {
			wantContent.Width, wantContent.Height = wantContent.Height, wantContent.Width
			wantArea.Width, wantArea.Height = wantArea.Height, wantArea.Width
		}
		if content.Rect().Size != wantContent || sv.viewport.Rect().Size != wantArea || sv.vbar.Visible() == horizontal || sv.hbar.Visible() != horizontal {
			t.Fatalf("single-axis layout: direction=%v content=%v viewport=%v", direction, content.Rect(), sv.viewport.Rect())
		}
		last := content.constraints[len(content.constraints)-1]
		if !horizontal && (last.Min.Width != 92 || last.Max.Width != 92 || last.Max.Height != layout.Inf) ||
			horizontal && (last.Min.Height != 92 || last.Max.Height != 92 || last.Max.Width != layout.Inf) {
			t.Fatalf("not measured at bar-reduced cross extent: %+v", last)
		}
		// Reveal requests, including an overflowing cross-axis rectangle, may
		// only move the configured axis. The visible end is 100 - 60 = 40.
		sv.ScrollIntoView(geometry.Rect(200, 200, 1, 1))
		if !horizontal && (sv.ScrollX() != 0 || sv.ScrollY() != 40) || horizontal && (sv.ScrollX() != 40 || sv.ScrollY() != 0) {
			t.Fatalf("reveal ignored direction: %g/%g", sv.ScrollX(), sv.ScrollY())
		}
		content.length = 60
		sv.Arrange(geometry.Rectangle{Size: size})
		if sv.hbar.Visible() || sv.vbar.Visible() || sv.ScrollX() != 0 || sv.ScrollY() != 0 {
			t.Fatal("content shrink retained scrollbar or offset")
		}
		// Short content fills only the cross axis, not the scrolling axis.
		want := geometry.Size{Width: 100, Height: 20}
		if horizontal {
			want.Width, want.Height = want.Height, want.Width
		}
		if content.Rect().Size != want {
			t.Fatalf("short content stretched along the scrolling axis: %v", content.Rect())
		}
	}
}

func TestScrollViewDirectionOffsetsWheelAndSnapshot(t *testing.T) {
	sv := NewScrollView()
	sv.SetChild(&mockWidget{size: geometry.Size{Width: 500, Height: 500}})
	sv.Arrange(geometry.Rect(0, 0, 100, 100))
	sv.SetScrollX(20)
	sv.SetScrollY(30)
	sv.SetDirection(ScrollVertical)
	if sv.ScrollX() != 0 || sv.ScrollY() != 30 {
		t.Fatal("direction change did not reset just the disabled offset")
	}
	sv.Arrange(sv.Rect())
	sv.SetScrollX(50)
	win := &window{}
	win.SetWidget(sv)
	defer win.Destroy()
	_ = win.DispatchEvent(events.WheelEvent{Position: geometry.Point{X: 20, Y: 20}, DeltaX: 40, DeltaY: 10, Mode: events.WheelDeltaPixel})
	info := sv.Snapshot()
	if sv.ScrollX() != 0 || sv.ScrollY() != 40 || info.MaxScrollX != 0 || sv.hbar.Visible() {
		t.Fatal("vertical mode accepted horizontal input or exposed a disabled range")
	}
	sv.SetDirection(ScrollHorizontal)
	sv.Arrange(sv.Rect())
	sv.SetScrollY(50)
	_ = win.DispatchEvent(events.WheelEvent{Position: geometry.Point{X: 20, Y: 20}, DeltaY: 10, Modifiers: events.ModifierShift, Mode: events.WheelDeltaPixel})
	info = sv.Snapshot()
	if sv.ScrollX() != 10 || sv.ScrollY() != 0 || info.MaxScrollY != 0 || sv.vbar.Visible() {
		t.Fatal("horizontal mode did not honor Shift-wheel or exposed a disabled range")
	}
	sv.SetDirection(255)
	if sv.Direction() != ScrollBoth {
		t.Fatal("invalid direction was not normalized")
	}
}

func TestScrollViewUnboundedCrossAxisAndOwnMaximum(t *testing.T) {
	sv := NewScrollView()
	sv.SetDirection(ScrollVertical)
	content := &responsiveScrollWidget{length: 400}
	sv.SetChild(content)
	if got := sv.Measure(layout.Unbounded()).Size; got != (geometry.Size{Width: 400}) {
		t.Fatalf("unbounded viewport invented a cross size: %v", got)
	}
	sv.SetMaxSize(geometry.Size{Width: 100})
	if got := sv.Measure(layout.Loose(geometry.Size{Width: 200, Height: 60})).Width; got != 100 || content.constraints[len(content.constraints)-1].Max.Width != 100 {
		t.Fatalf("own maximum did not reach content measurement: width=%g", got)
	}
}

func TestScrollViewTinyAllocationAndClear(t *testing.T) {
	sv := NewScrollView()
	sv.SetChild(&mockWidget{size: geometry.Size{Width: 200, Height: 200}})
	for _, size := range []geometry.Size{{}, {Width: 3, Height: 4}, {Width: 8, Height: 8}} {
		sv.Arrange(geometry.Rectangle{Size: size})
		for _, r := range []geometry.Rectangle{sv.viewport.Rect(), sv.vbar.Rect(), sv.hbar.Rect()} {
			if r.X < 0 || r.Y < 0 || r.Width < 0 || r.Height < 0 || r.X+r.Width > size.Width || r.Y+r.Height > size.Height {
				t.Fatalf("structural allocation outside tiny viewport %v: %v", size, r)
			}
		}
	}
	sv.SetChild(nil)
	sv.Arrange(sv.Rect())
	info := sv.Snapshot()
	if info.MaxScrollX != 0 || info.MaxScrollY != 0 || sv.hbar.Visible() || sv.vbar.Visible() {
		t.Fatal("cleared child retained its extent")
	}
}

type extentScrollContent struct {
	WidgetBase
	size      geometry.Size
	discover  geometry.Size
	viewports []geometry.Size
}

func (w *extentScrollContent) ContentSize() geometry.Size { return w.size }
func (w *extentScrollContent) LayoutVisible(viewport geometry.Size, offset geometry.Point) {
	w.viewports = append(w.viewports, viewport)
	if w.discover != (geometry.Size{}) {
		w.size, w.discover = w.discover, geometry.Size{}
	}
}

func TestScrollViewVirtualExtentAndDisabledAxis(t *testing.T) {
	sv := NewScrollView()
	content := &extentScrollContent{discover: geometry.Size{Width: 96, Height: 200}}
	sv.SetChild(content)
	sv.Measure(layout.Tight(geometry.Size{Width: 100, Height: 100}))
	sv.Arrange(geometry.Rect(0, 0, 100, 100))
	if len(content.viewports) != 2 || content.Rect() != geometry.Rect(0, 0, 92, 92) || content.viewports[1] != (geometry.Size{Width: 92, Height: 92}) {
		t.Fatalf("lazy extent did not update the actual viewport: %v / %v", content.viewports, content.Rect())
	}
	sv.SetDirection(ScrollVertical)
	sv.Arrange(sv.Rect())
	if content.ContentSize().Width != 96 || sv.Snapshot().MaxScrollX != 0 || sv.hbar.Visible() || content.Rect() != geometry.Rect(0, 0, 92, 100) {
		t.Fatal("host changed virtual content policy instead of disabling its scroll axis")
	}
}

// --- ScrollView (viewport mode) ---

// mockWidget is a plain widget with a fixed measure size (no typography
// dependency), standing in for a real content tree in tests.
type mockWidget struct {
	WidgetBase
	size geometry.Size
}

type mockPaintWidget struct {
	mockWidget
}

func (m *mockPaintWidget) Paint(p Painter) {
	p.FillRect(geometry.Rect(0, 0, m.Rect().Width, m.Rect().Height), graphics.RGB(1, 2, 3))
}

func (m *mockWidget) Measure(c layout.Constraint) layout.Measurement {
	return layout.Measured(m.size)
}

func (m *mockWidget) Arrange(rect geometry.Rectangle) {
	m.WidgetBase.Arrange(rect)
}

func TestScrollViewClamp(t *testing.T) {
	sv := NewScrollView()
	sv.SetChild(&mockWidget{size: geometry.Size{Width: 100, Height: 500}})

	// Measure gives the viewport the parent constraint; content is unbounded.
	sv.Measure(layout.Constraint{Max: geometry.Size{Width: 100, Height: 200}})
	sv.Arrange(geometry.Rect(0, 0, 100, 200))
	if sv.contentHeight != 500 {
		t.Fatalf("content height should be 500, got %v", sv.contentHeight)
	}

	sv.SetScrollY(1000)
	// The vertical bar reduces width below 100, requiring a horizontal bar too.
	if got := sv.ScrollY(); got != 308 {
		t.Fatalf("SetScrollY(1000) should clamp to 308, got %v", got)
	}

	sv.SetScrollY(-50)
	if got := sv.ScrollY(); got != 0 {
		t.Fatalf("SetScrollY(-50) should clamp to 0, got %v", got)
	}
}

func TestScrollViewContentOffset(t *testing.T) {
	sv := NewScrollView()
	content := &mockWidget{size: geometry.Size{Width: 100, Height: 500}}
	sv.SetChild(content)

	sv.Measure(layout.Constraint{Max: geometry.Size{Width: 100, Height: 100}})
	sv.Arrange(geometry.Rect(0, 0, 100, 100))

	sv.SetScrollY(40)
	// Content should be arranged at -scrollY.
	if got := content.Rect().Y; got != -40 {
		t.Fatalf("content Y should be -40 after scrolling, got %v", got)
	}
}

func TestScrollViewSetChildReplaces(t *testing.T) {
	sv := NewScrollView()
	first := &mockWidget{size: geometry.Size{Width: 100, Height: 500}}
	second := &mockWidget{size: geometry.Size{Width: 100, Height: 500}}

	// SetChild is the Bin content API; it replaces the content inside the
	// structural viewport. The viewport and scrollbars remain regardless.
	sv.SetChild(first)
	if sv.Child() != first {
		t.Fatal("SetChild should set the content")
	}
	if first.Parent() != sv.viewport || !sliceContains(sv.viewport.Children(), first) {
		t.Fatalf("content should be mounted under the viewport, parent=%v children=%v", first.Parent(), sv.viewport.Children())
	}

	sv.SetChild(second)
	if sv.Child() != second {
		t.Fatal("SetChild should replace the previous content")
	}
	if second.Parent() != sv.viewport || !sliceContains(sv.viewport.Children(), second) {
		t.Fatalf("new content should be mounted under the viewport, parent=%v children=%v", second.Parent(), sv.viewport.Children())
	}
	if first.Parent() != nil || sliceContains(sv.viewport.Children(), first) {
		t.Fatal("previous content should be removed after replace")
	}

	// Clear via SetChild(nil): content slot is nil, scrollbars remain.
	sv.SetChild(nil)
	if sv.Child() != nil {
		t.Fatal("SetChild(nil) should clear the content")
	}
	if len(sv.viewport.Children()) != 0 || second.Parent() != nil {
		t.Fatal("viewport should not retain content after clear")
	}
}

func sliceContains(ws []Widget, w Widget) bool {
	for _, c := range ws {
		if c == w {
			return true
		}
	}
	return false
}

func TestScrollViewChildrenContainStructuralViewport(t *testing.T) {
	sv := NewScrollView()
	content := &mockWidget{size: geometry.Size{Width: 100, Height: 500}}
	sv.SetChild(content)

	// The real tree includes a viewport between ScrollView and content so paint
	// clipping and hit testing use the same structural boundary.
	children := sv.Children()
	if !sliceContains(children, sv.viewport) || sliceContains(children, content) {
		t.Fatalf("ScrollView children should contain the viewport, not content directly: %v", children)
	}
	if content.Parent() != sv.viewport || !sliceContains(sv.viewport.Children(), content) {
		t.Fatalf("viewport should contain content, parent=%v children=%v", content.Parent(), sv.viewport.Children())
	}
}

func TestScrollViewViewportDrivesPaintClipAndHitTesting(t *testing.T) {
	sv := NewScrollView()
	content := &mockPaintWidget{mockWidget: mockWidget{
		size: geometry.Size{Width: 100, Height: 500},
	}}
	sv.SetChild(content)
	sv.Measure(layout.Constraint{Max: geometry.Size{Width: 100, Height: 100}})
	sv.Arrange(geometry.Rect(10, 20, 100, 100))
	sv.SetScrollY(40)

	wantViewport := geometry.Rect(0, 0, 100-scrollbarWidth, 100-scrollbarWidth)
	if sv.viewport.Rect() != wantViewport {
		t.Fatalf("unexpected viewport rect: got %v want %v", sv.viewport.Rect(), wantViewport)
	}

	backend := new(recordingPainterBackend)
	paintWidget(sv, newPainter(backend, geometry.Rect(0, 0, 200, 200), 1))
	if len(backend.fills) != 1 {
		t.Fatalf("expected one content fill, got %d", len(backend.fills))
	}
	fill := backend.fills[0]
	if fill.clip != geometry.Rect(10, 20, 100-scrollbarWidth, 100-scrollbarWidth) {
		t.Fatalf("content must be clipped by the viewport, got %v", fill.clip)
	}
	if fill.transform != geometry.Translate(10, -20) {
		t.Fatalf("unexpected scrolled content transform: %v", fill.transform)
	}

	// Scrollbars are later siblings than the viewport, so reverse-order hit
	// testing reaches the vertical bar before any content beneath it.
	if target := hitTest(sv, geometry.Point{X: 109, Y: 25}); target != sv.vbar {
		t.Fatalf("point over the scrollbar should hit vbar, got %T", target)
	}
}

func TestWheelEventControllerPublic(t *testing.T) {
	// WheelEventController is a public, reusable controller (like
	// MotionEventController): connect via signal, drive via HandleEvent.
	controller := NewWheelEventController()
	var got events.WheelEvent
	controller.ConnectScroll(func(_ EventContext, e events.WheelEvent) {
		got = e
	})
	ctx := &eventContext{event: events.WheelEvent{DeltaY: 5, Mode: events.WheelDeltaPixel}}
	controller.HandleEvent(ctx)
	if got.DeltaY != 5 {
		t.Fatalf("ConnectScroll should receive the wheel event, got %+v", got)
	}
	if ctx.PropagationStopped() {
		t.Fatal("a reusable wheel controller must leave consumption to its listener")
	}
}

// 通过窗口正常派发路径验证嵌套滚动：消费依据是请求轴的范围，不是本次偏移
// 是否改变。内层已到边界或小于整数 DIP 的输入也不能意外移动外层。
func TestScrollViewNestedWheelPropagation(t *testing.T) {
	for _, tc := range []struct {
		name      string
		direction ScrollDirection
		content   geometry.Size
		initial   geometry.Point
		delta     geometry.Point
		modifiers events.Modifiers
		wantInner geometry.Point
		wantOuter geometry.Point
		propagate bool
	}{
		{name: "vertical", content: geometry.Size{Width: 50, Height: 500}, delta: geometry.Point{Y: 12}, wantInner: geometry.Point{Y: 12}},
		{name: "bottom", content: geometry.Size{Width: 50, Height: 500}, initial: geometry.Point{Y: 400}, delta: geometry.Point{Y: 12}, wantInner: geometry.Point{Y: 400}},
		{name: "top", content: geometry.Size{Width: 50, Height: 500}, delta: geometry.Point{Y: -12}},
		{name: "fractional", content: geometry.Size{Width: 50, Height: 500}, delta: geometry.Point{Y: 0.2}},
		{name: "horizontal", content: geometry.Size{Width: 500, Height: 50}, delta: geometry.Point{X: 12}, wantInner: geometry.Point{X: 12}},
		{name: "right", content: geometry.Size{Width: 500, Height: 50}, initial: geometry.Point{X: 400}, delta: geometry.Point{X: 12}, wantInner: geometry.Point{X: 400}},
		{name: "left", content: geometry.Size{Width: 500, Height: 50}, delta: geometry.Point{X: -12}},
		{name: "shift", content: geometry.Size{Width: 500, Height: 50}, delta: geometry.Point{Y: 12}, modifiers: events.ModifierShift, wantInner: geometry.Point{X: 12}},
		{name: "both axes", content: geometry.Size{Width: 500, Height: 500}, delta: geometry.Point{X: 8, Y: 12}, wantInner: geometry.Point{X: 8, Y: 12}},
		{name: "one matching axis consumes the event", direction: ScrollVertical, content: geometry.Size{Width: 500, Height: 500}, delta: geometry.Point{X: 8, Y: 12}, wantInner: geometry.Point{Y: 12}},
		{name: "short", content: geometry.Size{Width: 50, Height: 50}, delta: geometry.Point{Y: 12}, wantOuter: geometry.Point{Y: 12}, propagate: true},
		{name: "empty", delta: geometry.Point{Y: 12}, wantOuter: geometry.Point{Y: 12}, propagate: true},
		{name: "no vertical range", content: geometry.Size{Width: 500, Height: 50}, delta: geometry.Point{Y: 12}, wantOuter: geometry.Point{Y: 12}, propagate: true},
		{name: "disabled vertical axis", direction: ScrollHorizontal, content: geometry.Size{Width: 500, Height: 500}, delta: geometry.Point{Y: 12}, wantOuter: geometry.Point{Y: 12}, propagate: true},
		{name: "disabled horizontal axis", direction: ScrollVertical, content: geometry.Size{Width: 500, Height: 500}, delta: geometry.Point{Y: 12}, modifiers: events.ModifierShift, wantOuter: geometry.Point{X: 12}, propagate: true},
		{name: "zero", content: geometry.Size{Width: 500, Height: 500}, propagate: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			inner := NewScrollView()
			inner.SetDirection(tc.direction)
			if tc.content != (geometry.Size{}) {
				inner.SetChild(&mockWidget{size: tc.content})
			}
			content := &mockWidget{size: geometry.Size{Width: 600, Height: 600}}
			content.AddChild(content, inner)
			outer := NewScrollView()
			outer.SetChild(content)
			win := &window{}
			win.SetWidget(outer)
			defer win.Destroy()
			outer.Arrange(geometry.Rect(0, 0, 300, 300))
			outer.SetScrollX(20)
			outer.SetScrollY(20)
			inner.Arrange(geometry.Rect(40, 40, 100, 100))
			inner.SetScrollX(tc.initial.X)
			inner.SetScrollY(tc.initial.Y)
			outerReceived := false
			outer.wheel.ConnectScroll(func(EventContext, events.WheelEvent) { outerReceived = true })
			if err := win.DispatchEvent(events.WheelEvent{
				Position: geometry.Point{X: 50, Y: 50}, DeltaX: tc.delta.X, DeltaY: tc.delta.Y,
				Mode: events.WheelDeltaPixel, Modifiers: tc.modifiers,
			}); err != nil {
				t.Fatal(err)
			}
			if got := (geometry.Point{X: inner.ScrollX(), Y: inner.ScrollY()}); got != tc.wantInner {
				t.Fatalf("inner offset = %v, want %v", got, tc.wantInner)
			}
			wantOuter := geometry.Point{X: 20 + tc.wantOuter.X, Y: 20 + tc.wantOuter.Y}
			if got := (geometry.Point{X: outer.ScrollX(), Y: outer.ScrollY()}); got != wantOuter || outerReceived != tc.propagate {
				t.Fatalf("outer offset = %v, want %v; received = %v, want %v", got, wantOuter, outerReceived, tc.propagate)
			}
		})
	}
}

func TestScrollViewScrollable(t *testing.T) {
	sv := NewScrollView()
	sv.SetChild(&mockWidget{size: geometry.Size{Width: 100, Height: 50}})

	sv.Measure(layout.Constraint{Max: geometry.Size{Width: 100, Height: 100}})
	sv.Arrange(geometry.Rect(0, 0, 100, 100))
	if sv.Scrollable() {
		t.Fatal("short content should not be scrollable")
	}

	sv.SetChild(&mockWidget{size: geometry.Size{Width: 100, Height: 500}})
	sv.Measure(layout.Constraint{Max: geometry.Size{Width: 100, Height: 100}})
	sv.Arrange(geometry.Rect(0, 0, 100, 100))
	if !sv.Scrollable() {
		t.Fatal("tall content should be scrollable")
	}
}

func TestScrollViewWheel(t *testing.T) {
	sv := NewScrollView()
	sv.SetChild(&mockWidget{size: geometry.Size{Width: 100, Height: 500}})
	sv.Measure(layout.Constraint{Max: geometry.Size{Width: 100, Height: 100}})
	sv.Arrange(geometry.Rect(0, 0, 100, 100))

	// Pixel mode: DeltaY directly translates to scroll.
	sv.wheel.HandleEvent(&eventContext{event: events.WheelEvent{DeltaY: 30, Mode: events.WheelDeltaPixel}})
	if got := sv.ScrollY(); got != 30 {
		t.Fatalf("pixel wheel should scroll 30, got %v", got)
	}

	// Line mode: DeltaY * lineHeight, rounded to whole pixels.
	before := sv.ScrollY()
	sv.wheel.HandleEvent(&eventContext{event: events.WheelEvent{DeltaY: 2, Mode: events.WheelDeltaLine}})
	want := mathx.Round(before + 2*sv.lineHeightPx())
	if got := sv.ScrollY(); got != want {
		t.Fatalf("line wheel should scroll to %v, got %v", want, got)
	}
	// Scroll offsets are always whole pixels (text bitmaps stay crisp).
	if sv.ScrollY() != mathx.Round(sv.ScrollY()) {
		t.Fatalf("scrollY should be a whole pixel, got %v", sv.ScrollY())
	}

	// Clamp at the end.
	sv.wheel.HandleEvent(&eventContext{event: events.WheelEvent{DeltaY: 10000, Mode: events.WheelDeltaPixel}})
	if got := sv.ScrollY(); got != 408 {
		t.Fatalf("wheel should clamp to 408 with both bars, got %v", got)
	}
}

// --- ScrollView (Scrollable mode) ---

// mockScrollContent is a minimal Scrollable for tests.
type mockScrollContent struct {
	WidgetBase
	height      float32
	layoutCalls []geometry.Point
}

func (m *mockScrollContent) ContentSize() geometry.Size {
	return geometry.Size{Width: 100, Height: m.height}
}

func (m *mockScrollContent) LayoutVisible(viewport geometry.Size, offset geometry.Point) {
	m.layoutCalls = append(m.layoutCalls, offset)
}

func TestScrollViewScrollableContentMode(t *testing.T) {
	sv := NewScrollView()
	content := &mockScrollContent{height: 500}
	sv.SetChild(content)

	sv.Measure(layout.Constraint{Max: geometry.Size{Width: 100, Height: 100}})
	sv.Arrange(geometry.Rect(0, 0, 100, 100))

	// The vertical bar reduces width below the content's fixed 100, so both
	// bars occupy space. Scrollable still controls its own width/wrapping policy.
	areaW := float32(100 - scrollbarWidth)

	if len(content.layoutCalls) != 1 {
		t.Fatalf("LayoutVisible should be called once on Arrange, got %d", len(content.layoutCalls))
	}
	if content.layoutCalls[0].Y != 0 {
		t.Fatalf("initial offset should be 0, got %v", content.layoutCalls[0])
	}
	// The content's own rect is the structural viewport area; its descendants
	// are laid out virtually for the requested offset.
	if got := content.Rect(); got != (geometry.Rect(0, 0, areaW, areaW)) {
		t.Fatalf("content rect should be the content area, got %v", got)
	}

	sv.SetScrollY(100)
	if len(content.layoutCalls) != 2 {
		t.Fatalf("LayoutVisible should be called again on SetScrollY, got %d calls", len(content.layoutCalls))
	}
	if content.layoutCalls[1].Y != 100 {
		t.Fatalf("offset should be 100, got %v", content.layoutCalls[1])
	}

	// Clamp: content height 500, content area height 92 → max 408.
	sv.SetScrollY(999)
	if got := content.layoutCalls[len(content.layoutCalls)-1].Y; got != 408 {
		t.Fatalf("offset should clamp to 408, got %v", got)
	}
}

func TestScrollViewSnapshot(t *testing.T) {
	sv := NewScrollView()
	sv.SetChild(newTestWidget())
	sv.Measure(layout.Constraint{Max: geometry.Size{Width: 100, Height: 100}})
	sv.Arrange(geometry.Rect(0, 0, 100, 100))

	info := sv.Snapshot()
	if info.Role != RoleScrollView {
		t.Fatalf("role should be scrollview, got %q", info.Role)
	}
	if info.ScrollY != 0 {
		t.Fatalf("scrollY should be 0, got %v", info.ScrollY)
	}
	if len(info.Children) != 1 {
		t.Fatalf("viewport content should appear in the snapshot, got %d children", len(info.Children))
	}

	// Scrollable content: MaxScrollY reflects the range.
	content := &mockWidget{size: geometry.Size{Width: 100, Height: 400}}
	sv.SetChild(content)
	sv.Measure(layout.Constraint{Max: geometry.Size{Width: 100, Height: 100}})
	sv.Arrange(geometry.Rect(0, 0, 100, 100))
	sv.SetScrollY(50)
	info = sv.Snapshot()
	if info.ScrollY != 50 {
		t.Fatalf("scrollY should be 50, got %v", info.ScrollY)
	}
	if info.MaxScrollY != 308 {
		t.Fatalf("maxScrollY should be 308 with both bars, got %v", info.MaxScrollY)
	}
}

func TestScrollViewElasticMeasure(t *testing.T) {
	sv := NewScrollView()
	sv.SetChild(&mockScrollContent{height: 500})

	// A finite parent height is the viewport height; content height remains
	// cached separately for scrolling.
	if got := sv.Measure(layout.Constraint{Max: geometry.Size{Width: 100, Height: 100}}); got.Size != (geometry.Size{Width: 100, Height: 100}) {
		t.Fatalf("bounded measure should use the viewport height, got %v", got)
	}
	if sv.MainWeight() != 1 {
		t.Fatalf("ScrollView should default to MainWeight(1), got %v", sv.MainWeight())
	}
	// An unbounded scroll axis has no intrinsic viewport height.
	if got := sv.Measure(layout.Constraint{Max: geometry.Size{Width: 100, Height: layout.Inf}}); got.Size != (geometry.Size{Width: 100}) {
		t.Fatalf("unbounded measure should have no intrinsic scroll height, got %v", got)
	}
	// Tight constraint (window root): c.Min == c.Max -> fills.
	if got := sv.Measure(layout.Tight(geometry.Size{Width: 800, Height: 600})); got.Size != (geometry.Size{Width: 800, Height: 600}) {
		t.Fatalf("elastic measure under Tight should fill, got %v", got)
	}
}

func TestScrollViewMeasureCombinesChildSizeWithParentConstraint(t *testing.T) {
	sv := NewScrollView()
	sv.SetChild(&mockScrollContent{height: 500})

	got := sv.Measure(layout.Constraint{
		Min: geometry.Size{Width: 120, Height: 20},
		Max: geometry.Size{Width: 200, Height: 100},
	})
	// Scrollable content fills the cross axis, so width is the parent's max
	// width, clamped to the constraint bounds.
	if got.Size != (geometry.Size{Width: 200, Height: 100}) {
		t.Fatalf("Measure should fill the cross axis and apply both parent bounds, got %v", got)
	}
}

func TestScrollViewFillsCrossAxisByDefault(t *testing.T) {
	box := NewLinearBox(layout.DirectionHorizontal)
	sv := NewScrollView()
	sv.SetChild(&mockScrollContent{height: 500})
	box.AddChild(sv)

	box.Arrange(geometry.Rect(0, 0, 800, 600))
	if got := sv.Rect(); got != geometry.Rect(0, 0, 800, 600) {
		t.Fatalf("single ScrollView should fill the viewport by default, got %v", got)
	}
}

func TestScrollViewFillsCrossAxisInVerticalLinearBox(t *testing.T) {
	box := NewLinearBox(layout.DirectionVertical)
	box.SetCrossAlign(layout.CrossStretch)
	sv := NewScrollView()
	sv.SetChild(&mockScrollContent{height: 500})
	box.AddChild(sv)

	box.Arrange(geometry.Rect(0, 0, 800, 600))
	if got := sv.Rect(); got != geometry.Rect(0, 0, 800, 600) {
		t.Fatalf("ScrollView should fill the cross axis in a VBox, got %v", got)
	}
}

func TestScrollViewCanHugContentWidthInVerticalLinearBox(t *testing.T) {
	box := NewLinearBox(layout.DirectionVertical)
	sv := NewScrollView()
	sv.SetChild(&mockScrollContent{height: 500})
	box.AddChild(sv)

	box.Arrange(geometry.Rect(0, 0, 800, 600))
	// Scrollable content fills the cross axis regardless of CrossAlign, so the
	// viewport spans the full 800 width.
	if got := sv.Rect(); got != geometry.Rect(0, 0, 800, 600) {
		t.Fatalf("ScrollView should fill the cross axis with Scrollable content, got %v", got)
	}
}

func TestScrollViewViewportModeUsesParentHeight(t *testing.T) {
	box := NewLinearBox(layout.DirectionVertical)
	sv := NewScrollView()
	content := &mockWidget{size: geometry.Size{Width: 100, Height: 1000}}
	sv.SetChild(content)
	box.AddChild(sv)

	box.Arrange(geometry.Rect(0, 0, 800, 600))
	if got := sv.Rect(); got != geometry.Rect(0, 0, 100, 600) {
		t.Fatalf("ScrollView should preserve content width and use the parent's height, got %v", got)
	}
	if got := content.Rect(); got != geometry.Rect(0, 0, 100, 1000) {
		t.Fatalf("content should retain its full scrollable height, got %v", got)
	}
}

func TestScrollViewSideBySideInLinearBox(t *testing.T) {
	// Regression: two ScrollViews in an HBox previously each reported c.Max
	// (the whole available width), overflowing the linear sum and pushing the
	// second list out of the container. ScrollView's MainWeight(1) splits the
	// main axis and its viewport capability fills the cross axis by default.
	box := NewLinearBox(layout.DirectionHorizontal)
	left := NewScrollView()
	left.SetChild(&mockScrollContent{height: 500})
	right := NewScrollView()
	right.SetChild(&mockScrollContent{height: 500})
	box.AddChild(left)
	box.AddChild(right)

	box.Arrange(geometry.Rect(0, 0, 800, 600))
	if left.Rect() != geometry.Rect(0, 0, 400, 600) {
		t.Fatalf("left ScrollView should take half, got %v", left.Rect())
	}
	if right.Rect() != geometry.Rect(400, 0, 400, 600) {
		t.Fatalf("right ScrollView should take half, got %v", right.Rect())
	}
}

func TestScrollViewStackedInVerticalLinearBox(t *testing.T) {
	box := NewLinearBox(layout.DirectionVertical)
	top := NewScrollView()
	top.SetChild(&mockScrollContent{height: 500})
	bottom := NewScrollView()
	bottom.SetChild(&mockScrollContent{height: 500})
	box.AddChild(top)
	box.AddChild(bottom)

	box.Arrange(geometry.Rect(0, 0, 800, 600))
	// Scrollable content fills the cross axis, so the stacked viewports span
	// the full 800 width and split the height via main-axis weight.
	if top.Rect() != geometry.Rect(0, 0, 800, 300) {
		t.Fatalf("top ScrollView should fill the width and take half the height, got %v", top.Rect())
	}
	if bottom.Rect() != geometry.Rect(0, 300, 800, 300) {
		t.Fatalf("bottom ScrollView should fill the width and take half the height, got %v", bottom.Rect())
	}
}

// --- ScrollView slice 2: horizontal scroll + scrollbar interaction ---

func TestScrollViewScrollXClamp(t *testing.T) {
	sv := NewScrollView()
	sv.SetChild(&mockWidget{size: geometry.Size{Width: 500, Height: 100}})

	sv.Measure(layout.Constraint{Max: geometry.Size{Width: 200, Height: 100}})
	sv.Arrange(geometry.Rect(0, 0, 200, 100))

	// The horizontal bar reduces height below 100, requiring a vertical bar.
	sv.SetScrollX(1000)
	if got := sv.ScrollX(); got != 308 {
		t.Fatalf("SetScrollX(1000) should clamp to 308, got %v", got)
	}
	sv.SetScrollX(-50)
	if got := sv.ScrollX(); got != 0 {
		t.Fatalf("SetScrollX(-50) should clamp to 0, got %v", got)
	}
}

func TestScrollViewContentOffsetX(t *testing.T) {
	sv := NewScrollView()
	content := &mockWidget{size: geometry.Size{Width: 500, Height: 100}}
	sv.SetChild(content)

	sv.Measure(layout.Constraint{Max: geometry.Size{Width: 200, Height: 100}})
	sv.Arrange(geometry.Rect(0, 0, 200, 100))
	sv.SetScrollX(50)

	if got := content.Rect().X; got != -50 {
		t.Fatalf("content X should be -50 after horizontal scroll, got %v", got)
	}
}

func TestScrollViewHorizontalWheel(t *testing.T) {
	sv := NewScrollView()
	sv.SetChild(&mockWidget{size: geometry.Size{Width: 500, Height: 100}})

	sv.Measure(layout.Constraint{Max: geometry.Size{Width: 200, Height: 100}})
	sv.Arrange(geometry.Rect(0, 0, 200, 100))

	// Native horizontal wheel (DeltaX).
	sv.wheel.HandleEvent(&eventContext{event: events.WheelEvent{DeltaX: 30, Mode: events.WheelDeltaPixel}})
	if got := sv.ScrollX(); got != 30 {
		t.Fatalf("horizontal wheel should scroll X by 30, got %v", got)
	}

	// Shift+vertical → horizontal.
	sv.wheel.HandleEvent(&eventContext{event: events.WheelEvent{DeltaY: 20, Mode: events.WheelDeltaPixel, Modifiers: events.ModifierShift}})
	if got := sv.ScrollX(); got != 50 {
		t.Fatalf("Shift+wheel should scroll X by 20 more, got %v", got)
	}
}

func TestScrollViewScrollBarVisibility(t *testing.T) {
	sv := NewScrollView()
	content := &mockWidget{size: geometry.Size{Width: 500, Height: 500}}
	sv.SetChild(content)

	sv.Measure(layout.Constraint{Max: geometry.Size{Width: 200, Height: 200}})
	sv.Arrange(geometry.Rect(0, 0, 200, 200))

	// Both axes scrollable → both bars visible.
	if !sv.vbar.Visible() {
		t.Fatal("vertical bar should be visible when content is taller")
	}
	if !sv.hbar.Visible() {
		t.Fatal("horizontal bar should be visible when content is wider")
	}

	// Short content that fits → no bars.
	sv.SetChild(&mockWidget{size: geometry.Size{Width: 100, Height: 100}})
	sv.Measure(layout.Constraint{Max: geometry.Size{Width: 200, Height: 200}})
	sv.Arrange(geometry.Rect(0, 0, 200, 200))
	if sv.vbar.Visible() {
		t.Fatal("vertical bar should be hidden when content fits")
	}
	if sv.hbar.Visible() {
		t.Fatal("horizontal bar should be hidden when content fits")
	}
}

func TestScrollViewScrollBarDragUpdatesScroll(t *testing.T) {
	sv := NewScrollView()
	sv.SetChild(&mockWidget{size: geometry.Size{Width: 100, Height: 500}})

	sv.Measure(layout.Constraint{Max: geometry.Size{Width: 100, Height: 200}})
	sv.Arrange(geometry.Rect(0, 0, 100, 200))

	// maxY = 500 - 200 = 300. The vbar is arranged in the right column.
	// Press on the thumb and drag down; the scroll offset should follow.
	before := sv.ScrollY()
	bar := sv.vbar

	down := &eventContext{
		current: bar,
		event: events.PointerEvent{
			EventType: events.PointerDown,
			Button:    events.PointerButtonLeft,
			Position:  geometry.Point{X: bar.Rect().X, Y: bar.Rect().Y + 5},
		},
	}
	bar.drag.HandleEvent(down)

	move := &eventContext{
		current: bar,
		event: events.PointerEvent{
			EventType: events.PointerMove,
			Button:    events.PointerButtonLeft,
			Position:  geometry.Point{X: bar.Rect().X, Y: bar.Rect().Y + 80},
		},
	}
	bar.drag.HandleEvent(move)

	if sv.ScrollY() <= before {
		t.Fatalf("dragging the thumb down should increase scrollY, before=%v after=%v", before, sv.ScrollY())
	}

	up := &eventContext{
		current: bar,
		event: events.PointerEvent{
			EventType: events.PointerUp,
			Button:    events.PointerButtonLeft,
			Position:  geometry.Point{X: bar.Rect().X, Y: bar.Rect().Y + 80},
		},
	}
	bar.drag.HandleEvent(up)
	if bar.drag.Dragging() {
		t.Fatal("drag should end after pointer up")
	}
}

func TestScrollViewStyleNamePushDown(t *testing.T) {
	sv := NewScrollView()
	sv.SetChild(&mockWidget{size: geometry.Size{Width: 100, Height: 500}})
	sv.SetStyleName("my-scroll")

	if sv.vbar.StyleName() != "my-scroll" {
		t.Fatalf("vbar should inherit ScrollView StyleName, got %q", sv.vbar.StyleName())
	}
	if sv.hbar.StyleName() != "my-scroll" {
		t.Fatalf("hbar should inherit ScrollView StyleName, got %q", sv.hbar.StyleName())
	}
}

func TestScrollViewSnapshotScrollX(t *testing.T) {
	sv := NewScrollView()
	sv.SetChild(&mockWidget{size: geometry.Size{Width: 500, Height: 100}})

	sv.Measure(layout.Constraint{Max: geometry.Size{Width: 200, Height: 100}})
	sv.Arrange(geometry.Rect(0, 0, 200, 100))
	sv.SetScrollX(50)

	info := sv.Snapshot()
	if info.ScrollX != 50 {
		t.Fatalf("scrollX should be 50, got %v", info.ScrollX)
	}
	if info.MaxScrollX != 308 {
		t.Fatalf("maxScrollX should be 308 with both bars, got %v", info.MaxScrollX)
	}
	// Snapshot children filter out scrollbars.
	for _, c := range info.Children {
		if c.Role == RoleScrollBar {
			t.Fatal("snapshot should not include scrollbar children")
		}
	}
}

func TestScrollViewSnapshotFiltersScrollBars(t *testing.T) {
	sv := NewScrollView()
	sv.SetChild(&mockWidget{size: geometry.Size{Width: 100, Height: 500}})
	sv.Measure(layout.Constraint{Max: geometry.Size{Width: 100, Height: 100}})
	sv.Arrange(geometry.Rect(0, 0, 100, 100))

	info := sv.Snapshot()
	// Only the content child, not the scrollbars.
	if len(info.Children) != 1 {
		t.Fatalf("snapshot should have 1 child (content), got %d", len(info.Children))
	}
}
