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

	// SetChild is the single-content API; it replaces the previous content.
	sv.SetChild(first)
	if sv.Child() != first {
		t.Fatal("SetChild should set the content")
	}
	children := sv.Children()
	if len(children) != 1 || children[0] != first {
		t.Fatalf("content should be the only child, got %v", children)
	}

	sv.SetChild(second)
	if sv.Child() != second {
		t.Fatal("SetChild should replace the previous content")
	}
	children = sv.Children()
	if len(children) != 1 || children[0] != second {
		t.Fatalf("content should be the only child after replace, got %v", children)
	}

	// Clear via SetChild(nil).
	sv.SetChild(nil)
	if sv.Child() != nil {
		t.Fatal("SetChild(nil) should clear the content")
	}
	if len(sv.Children()) != 0 {
		t.Fatal("children should be empty after clear")
	}
}

func TestScrollViewChildrenReflectsContent(t *testing.T) {
	sv := NewScrollView()
	content := &mockWidget{size: geometry.Size{Width: 100, Height: 500}}
	sv.SetChild(content)

	// Children() comes from Widget (not Container): single-content containers
	// expose their content through the common traversal API, which is what
	// hitTest uses to recurse.
	children := sv.Children()
	if len(children) != 1 || children[0] != content {
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

	if len(content.layoutCalls) != 1 {
		t.Fatalf("LayoutVisible should be called once on Arrange, got %d", len(content.layoutCalls))
	}
	if content.layoutCalls[0].Y != 0 {
		t.Fatalf("initial offset should be 0, got %v", content.layoutCalls[0])
	}
	// The content's own rect must be set to the viewport, otherwise the
	// SubPainter clip is empty and nothing draws.
	if got := content.Rect(); got != (geometry.Rect(0, 0, 100, 100)) {
		t.Fatalf("content rect should be the viewport, got %v", got)
	}

	sv.SetScrollY(100)
	if len(content.layoutCalls) != 2 {
		t.Fatalf("LayoutVisible should be called again on SetScrollY, got %d calls", len(content.layoutCalls))
	}
	if content.layoutCalls[1].Y != 100 {
		t.Fatalf("offset should be 100, got %v", content.layoutCalls[1])
	}

	// Clamp: content height 500, viewport 100 → max 400.
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

	// Scrollable mode has no intrinsic size: Loose constraint -> zero size,
	// the parent (a linear box) decides through MainWeight.
	if got := sv.Measure(layout.Constraint{Max: geometry.Size{Width: 100, Height: 100}}); got != (geometry.Size{}) {
		t.Fatalf("elastic measure should report no intrinsic size, got %v", got)
	}
	if sv.MainWeight() != 1 {
		t.Fatalf("ScrollView should default to MainWeight(1), got %v", sv.MainWeight())
	}
	// Tight constraint (window root): c.Min == c.Max -> fills.
	if got := sv.Measure(layout.Tight(geometry.Size{Width: 800, Height: 600})); got != (geometry.Size{Width: 800, Height: 600}) {
		t.Fatalf("elastic measure under Tight should fill, got %v", got)
	}
}

func TestScrollViewSideBySideInLinearBox(t *testing.T) {
	// Regression: two ScrollViews in an HBox previously each reported c.Max
	// (the whole available width), overflowing the linear sum and pushing the
	// second list out of the container. ScrollView's MainWeight(1) splits
	// the main axis; the explicit CrossStretch below fills the cross axis.
	box := NewLinearBox(layout.DirectionHorizontal)
	box.SetCrossAlign(layout.CrossStretch)
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
