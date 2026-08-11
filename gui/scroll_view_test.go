package gui

import (
	"testing"

	"github.com/goexlib/mathx"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/events"
)

// --- ScrollView (viewport mode) ---

// mockWidget is a plain widget with a fixed measure size (no typography
// dependency), standing in for a real content tree in tests.
type mockWidget struct {
	WidgetBase
	size geometry.Size
}

func (m *mockWidget) Measure(c layout.Constraint) geometry.Size {
	return m.size
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
	if got := sv.ScrollY(); got != 300 {
		t.Fatalf("SetScrollY(1000) should clamp to 300, got %v", got)
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

	// SetChild is the Bin content API; it replaces the content slot. The
	// scrollbars are structural children that remain regardless.
	sv.SetChild(first)
	if sv.Child() != first {
		t.Fatal("SetChild should set the content")
	}
	if !sliceContains(sv.Children(), first) {
		t.Fatalf("content should be in children, got %v", sv.Children())
	}

	sv.SetChild(second)
	if sv.Child() != second {
		t.Fatal("SetChild should replace the previous content")
	}
	if !sliceContains(sv.Children(), second) {
		t.Fatalf("new content should be in children, got %v", sv.Children())
	}
	if sliceContains(sv.Children(), first) {
		t.Fatal("previous content should be removed after replace")
	}

	// Clear via SetChild(nil): content slot is nil, scrollbars remain.
	sv.SetChild(nil)
	if sv.Child() != nil {
		t.Fatal("SetChild(nil) should clear the content")
	}
	if sliceContains(sv.Children(), first) || sliceContains(sv.Children(), second) {
		t.Fatal("content should not be in children after clear")
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

func TestScrollViewChildrenReflectsContent(t *testing.T) {
	sv := NewScrollView()
	content := &mockWidget{size: geometry.Size{Width: 100, Height: 500}}
	sv.SetChild(content)

	// Children() comes from Widget (not Container): content is reachable for
	// hit-test traversal. The scrollbars are also structural children.
	children := sv.Children()
	if !sliceContains(children, content) {
		t.Fatalf("Children should contain the content, got %v", children)
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
	controller.HandleEvent(&eventContext{event: events.WheelEvent{DeltaY: 5, Mode: events.WheelDeltaPixel}})
	if got.DeltaY != 5 {
		t.Fatalf("ConnectScroll should receive the wheel event, got %+v", got)
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
	if got := sv.ScrollY(); got != 400 {
		t.Fatalf("wheel should clamp to 400, got %v", got)
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

	// Vertical scrollbar is shown (content 500 > viewport 100), so the content
	// area width is 100 - scrollbarWidth = 94.
	areaW := float32(100 - scrollbarWidth)

	if len(content.layoutCalls) != 1 {
		t.Fatalf("LayoutVisible should be called once on Arrange, got %d", len(content.layoutCalls))
	}
	if content.layoutCalls[0].Y != 0 {
		t.Fatalf("initial offset should be 0, got %v", content.layoutCalls[0])
	}
	// The content's own rect must be set to the content area, otherwise the
	// SubPainter clip is empty and nothing draws.
	if got := content.Rect(); got != (geometry.Rect(0, 0, areaW, 100)) {
		t.Fatalf("content rect should be the content area, got %v", got)
	}

	sv.SetScrollY(100)
	if len(content.layoutCalls) != 2 {
		t.Fatalf("LayoutVisible should be called again on SetScrollY, got %d calls", len(content.layoutCalls))
	}
	if content.layoutCalls[1].Y != 100 {
		t.Fatalf("offset should be 100, got %v", content.layoutCalls[1])
	}

	// Clamp: content height 500, content area height 100 → max 400.
	sv.SetScrollY(999)
	if got := content.layoutCalls[len(content.layoutCalls)-1].Y; got != 400 {
		t.Fatalf("offset should clamp to 400, got %v", got)
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
	if info.MaxScrollY != 300 {
		t.Fatalf("maxScrollY should be 300, got %v", info.MaxScrollY)
	}
}

func TestScrollViewElasticMeasure(t *testing.T) {
	sv := NewScrollView()
	sv.SetChild(&mockScrollContent{height: 500})

	// A finite parent height is the viewport height; content height remains
	// cached separately for scrolling.
	if got := sv.Measure(layout.Constraint{Max: geometry.Size{Width: 100, Height: 100}}); got != (geometry.Size{Width: 100, Height: 100}) {
		t.Fatalf("bounded measure should use the viewport height, got %v", got)
	}
	if sv.MainWeight() != 1 {
		t.Fatalf("ScrollView should default to MainWeight(1), got %v", sv.MainWeight())
	}
	// An unbounded scroll axis has no intrinsic viewport height.
	if got := sv.Measure(layout.Constraint{Max: geometry.Size{Width: 100, Height: layout.Inf}}); got != (geometry.Size{Width: 100}) {
		t.Fatalf("unbounded measure should have no intrinsic scroll height, got %v", got)
	}
	// Tight constraint (window root): c.Min == c.Max -> fills.
	if got := sv.Measure(layout.Tight(geometry.Size{Width: 800, Height: 600})); got != (geometry.Size{Width: 800, Height: 600}) {
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
	if got != (geometry.Size{Width: 200, Height: 100}) {
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

	// Content 500 wide, viewport 200, no vertical scroll → maxX = 300.
	sv.SetScrollX(1000)
	if got := sv.ScrollX(); got != 300 {
		t.Fatalf("SetScrollX(1000) should clamp to 300, got %v", got)
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
	if info.MaxScrollX != 300 {
		t.Fatalf("maxScrollX should be 300, got %v", info.MaxScrollX)
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
