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
	sv.SetContent(&mockWidget{size: geometry.Size{Width: 100, Height: 500}})

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
	sv.SetContent(content)

	sv.Measure(layout.Constraint{Max: geometry.Size{Width: 100, Height: 100}})
	sv.Arrange(geometry.Rect(0, 0, 100, 100))

	sv.SetScrollY(40)
	// Content should be arranged at -scrollY.
	if got := content.Rect().Y; got != -40 {
		t.Fatalf("content Y should be -40 after scrolling, got %v", got)
	}
}

func TestScrollViewSetContentReplaces(t *testing.T) {
	sv := NewScrollView()
	first := &mockWidget{size: geometry.Size{Width: 100, Height: 500}}
	second := &mockWidget{size: geometry.Size{Width: 100, Height: 500}}

	// SetContent is the single-content API; it replaces the previous content.
	sv.SetContent(first)
	if sv.Content() != first {
		t.Fatal("SetContent should set the content")
	}
	children := sv.Children()
	if len(children) != 1 || children[0] != first {
		t.Fatalf("content should be the only child, got %v", children)
	}

	sv.SetContent(second)
	if sv.Content() != second {
		t.Fatal("SetContent should replace the previous content")
	}
	children = sv.Children()
	if len(children) != 1 || children[0] != second {
		t.Fatalf("content should be the only child after replace, got %v", children)
	}

	// Clear via SetContent(nil).
	sv.SetContent(nil)
	if sv.Content() != nil {
		t.Fatal("SetContent(nil) should clear the content")
	}
	if len(sv.Children()) != 0 {
		t.Fatal("children should be empty after clear")
	}
}

func TestScrollViewChildrenReflectsContent(t *testing.T) {
	sv := NewScrollView()
	content := &mockWidget{size: geometry.Size{Width: 100, Height: 500}}
	sv.SetContent(content)

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
	sv.SetContent(&mockWidget{size: geometry.Size{Width: 100, Height: 50}})

	sv.Measure(layout.Constraint{Max: geometry.Size{Width: 100, Height: 100}})
	sv.Arrange(geometry.Rect(0, 0, 100, 100))
	if sv.Scrollable() {
		t.Fatal("short content should not be scrollable")
	}

	sv.SetContent(&mockWidget{size: geometry.Size{Width: 100, Height: 500}})
	sv.Measure(layout.Constraint{Max: geometry.Size{Width: 100, Height: 100}})
	sv.Arrange(geometry.Rect(0, 0, 100, 100))
	if !sv.Scrollable() {
		t.Fatal("tall content should be scrollable")
	}
}

func TestScrollViewWheel(t *testing.T) {
	sv := NewScrollView()
	sv.SetContent(&mockWidget{size: geometry.Size{Width: 100, Height: 500}})
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
	sv.SetContent(content)

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

// --- ListView ---

func TestListViewVisibleRange(t *testing.T) {
	lv := NewListView()
	lv.SetItemCount(100)
	lv.SetItemHeight(20)

	// viewport 100 tall at offset 0 → items 0..4
	lv.LayoutVisible(geometry.Size{Width: 100, Height: 100}, geometry.Point{})
	got := lv.VisibleIndexes()
	if len(got) != 5 || got[0] != 0 || got[4] != 4 {
		t.Fatalf("visible at offset 0 should be [0..4], got %v", got)
	}

	// offset 50 → items 2..7 (50/20=2, (50+100)/20=7)
	lv.LayoutVisible(geometry.Size{Width: 100, Height: 100}, geometry.Point{Y: 50})
	got = lv.VisibleIndexes()
	if len(got) != 6 || got[0] != 2 || got[5] != 7 {
		t.Fatalf("visible at offset 50 should be [2..7], got %v", got)
	}

	// offset beyond end → clamped to last items (total height 2000,
	// max offset 1900; use 3000 to exceed)
	lv.LayoutVisible(geometry.Size{Width: 100, Height: 100}, geometry.Point{Y: 3000})
	got = lv.VisibleIndexes()
	if len(got) == 0 || got[len(got)-1] != 99 {
		t.Fatalf("visible at offset 3000 should end at 99, got %v", got)
	}
	if got[0] != 95 {
		t.Fatalf("visible at offset 3000 should start at 95, got %v", got)
	}
}

func TestListViewItemReuse(t *testing.T) {
	lv := NewListView()
	lv.SetItemCount(100)
	lv.SetItemHeight(20)
	created := 0
	lv.SetRenderItem(func(i int) Widget {
		created++
		return NewLabel("item")
	})

	// First scroll range creates 5 items.
	lv.LayoutVisible(geometry.Size{Width: 100, Height: 100}, geometry.Point{})
	if created != 5 {
		t.Fatalf("first layout should create 5 items, got %d", created)
	}

	// Scroll down one row: items 1..5; item 0 detached, item 5 created.
	lv.LayoutVisible(geometry.Size{Width: 100, Height: 100}, geometry.Point{Y: 20})
	if created != 6 {
		t.Fatalf("scroll down one row should create 1 new item, got %d", created)
	}
	if _, ok := lv.items[0]; !ok {
		t.Fatal("item 0 should remain cached after scrolling out")
	}

	// Scroll back up: item 0 reused, no new creation.
	lv.LayoutVisible(geometry.Size{Width: 100, Height: 100}, geometry.Point{})
	if created != 6 {
		t.Fatalf("scroll back up should reuse cached item, created=%d", created)
	}
	if _, ok := lv.items[0]; !ok {
		t.Fatal("item 0 should be re-attached")
	}
}

func TestListViewEmpty(t *testing.T) {
	lv := NewListView()
	lv.LayoutVisible(geometry.Size{Width: 100, Height: 100}, geometry.Point{})
	if got := lv.VisibleIndexes(); got != nil {
		t.Fatalf("empty list should have no visible items, got %v", got)
	}

	lv.SetItemCount(10)
	lv.SetItemHeight(0) // zero height → no visible items
	lv.LayoutVisible(geometry.Size{Width: 100, Height: 100}, geometry.Point{})
	if got := lv.VisibleIndexes(); got != nil {
		t.Fatalf("zero item height should have no visible items, got %v", got)
	}
}

func TestListViewHitTestItems(t *testing.T) {
	lv := NewListView()
	lv.SetItemCount(10)
	lv.SetItemHeight(20)
	lv.SetRenderItem(func(i int) Widget {
		return &mockWidget{size: geometry.Size{Width: 100, Height: 20}}
	})
	lv.Arrange(geometry.Rect(0, 0, 100, 100))
	lv.LayoutVisible(geometry.Size{Width: 100, Height: 100}, geometry.Point{})

	// ListView satisfies Container, so hitTest recurses into its items.
	item := lv.items[2]
	if item == nil {
		t.Fatal("item 2 should be created")
	}
	target := hitTest(lv, geometry.Point{X: 50, Y: 2*20 + 10})
	if target != item {
		t.Fatalf("hitTest should return item 2, got %v", target)
	}
}
