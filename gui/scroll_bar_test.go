package gui

import (
	"math"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/style"
)

func approxEq(a, b float32) bool {
	return math.Abs(float64(a-b)) < 0.01
}

func newVerticalBar(t *testing.T, max float32) *ScrollBar {
	t.Helper()
	b := NewScrollBar(layout.DirectionVertical)
	b.Arrange(geometry.Rect(0, 0, scrollbarWidth, 200))
	b.SetMax(max)
	return b
}

func TestScrollBarThumbGeometry(t *testing.T) {
	b := newVerticalBar(t, 400) // content = 600, viewport = 200

	// thumbLen = viewport² / content = 200² / 600 ≈ 66.67
	if got := b.thumbLen(); !approxEq(got, 66.6667) {
		t.Fatalf("thumbLen: want 66.67, got %v", got)
	}

	// At value=0, thumb starts at 0.
	b.SetValue(0)
	if got := b.thumbStart(); !approxEq(got, 0) {
		t.Fatalf("thumbStart at 0: want 0, got %v", got)
	}

	// At value=max, thumb starts at mainLen - thumbLen ≈ 133.33.
	b.SetValue(400)
	if got := b.thumbStart(); !approxEq(got, 133.3333) {
		t.Fatalf("thumbStart at max: want 133.33, got %v", got)
	}

	// At value=200 (mid), thumb starts at (200-66.67)*200/400 ≈ 66.67.
	b.SetValue(200)
	if got := b.thumbStart(); !approxEq(got, 66.6667) {
		t.Fatalf("thumbStart at mid: want 66.67, got %v", got)
	}
}

func TestScrollBarThumbRect(t *testing.T) {
	b := newVerticalBar(t, 400)
	b.SetValue(100)
	rect := b.thumbRect()
	if rect.X != 0 || rect.Width != scrollbarWidth {
		t.Fatalf("vertical thumb rect X/Width: want (0, %v), got (%v, %v)", scrollbarWidth, rect.X, rect.Width)
	}
	if !approxEq(rect.Y, b.thumbStart()) || !approxEq(rect.Height, b.thumbLen()) {
		t.Fatalf("thumb rect Y/H mismatch: got %+v", rect)
	}
}

func TestScrollBarHorizontalGeometry(t *testing.T) {
	b := NewScrollBar(layout.DirectionHorizontal)
	b.Arrange(geometry.Rect(0, 0, 200, scrollbarWidth))
	b.SetMax(400)
	b.SetValue(200)

	// Same math as vertical, just along X.
	rect := b.thumbRect()
	if rect.Y != 0 || rect.Height != scrollbarWidth {
		t.Fatalf("horizontal thumb rect Y/Height: want (0, %v), got (%v, %v)", scrollbarWidth, rect.Y, rect.Height)
	}
	if !approxEq(rect.X, 66.6667) || !approxEq(rect.Width, 66.6667) {
		t.Fatalf("horizontal thumb X/W: want 66.67, got (%v, %v)", rect.X, rect.Width)
	}
}

func TestScrollBarDragThumb(t *testing.T) {
	b := newVerticalBar(t, 400)
	b.SetValue(0)

	var got float32
	b.ConnectChange(func(v float32) { got = v })

	// Press on the thumb at y=30 (within [0, 66.67)).
	down := &eventContext{
		current: b,
		event: events.PointerEvent{
			EventType: events.PointerDown,
			Button:    events.PointerButtonLeft,
			Position:  geometry.Point{X: 0, Y: 30},
		},
	}
	b.drag.HandleEvent(down)
	if !b.drag.Dragging() {
		t.Fatal("drag should be active after press on thumb")
	}
	// grabOffset = 30 (press point - thumbStart(0)).
	// First emit from begin: start = 30 - 30 = 0 → value = 0.
	if !approxEq(got, 0) {
		t.Fatalf("begin should emit value 0, got %v", got)
	}

	// Move to y=100: start = 100 - 30 = 70 → value = 70 * 400 / 133.33 ≈ 210.
	move := &eventContext{
		current: b,
		event: events.PointerEvent{
			EventType: events.PointerMove,
			Button:    events.PointerButtonLeft,
			Position:  geometry.Point{X: 0, Y: 100},
		},
	}
	b.drag.HandleEvent(move)
	if !approxEq(got, 210) {
		t.Fatalf("update should emit value 210, got %v", got)
	}

	// Release.
	up := &eventContext{
		current: b,
		event: events.PointerEvent{
			EventType: events.PointerUp,
			Button:    events.PointerButtonLeft,
			Position:  geometry.Point{X: 0, Y: 100},
		},
	}
	b.drag.HandleEvent(up)
	if b.drag.Dragging() {
		t.Fatal("drag should end after pointer up")
	}
}

func TestScrollBarDragTroughJumps(t *testing.T) {
	b := newVerticalBar(t, 400)
	b.SetValue(0)

	var got float32
	b.ConnectChange(func(v float32) { got = v })

	// Press on the trough at y=100 (below the thumb [0, 66.67)).
	down := &eventContext{
		current: b,
		event: events.PointerEvent{
			EventType: events.PointerDown,
			Button:    events.PointerButtonLeft,
			Position:  geometry.Point{X: 0, Y: 100},
		},
	}
	b.drag.HandleEvent(down)

	// Jump: grabOffset = thumbLen/2 ≈ 33.33.
	// start = 100 - 33.33 = 66.67 → value = 66.67 * 400 / 133.33 ≈ 200.
	// (Thumb centers on the press point: thumbStart(200) = 66.67, center = 100.)
	if !approxEq(got, 200) {
		t.Fatalf("trough press should jump to value 200, got %v", got)
	}
}

func TestScrollBarDragClampsAtEdges(t *testing.T) {
	b := newVerticalBar(t, 400)
	b.SetValue(0)

	var got float32
	b.ConnectChange(func(v float32) { got = v })

	// Press on thumb at top, then drag far beyond the trough.
	down := &eventContext{
		current: b,
		event: events.PointerEvent{
			EventType: events.PointerDown,
			Button:    events.PointerButtonLeft,
			Position:  geometry.Point{X: 0, Y: 10},
		},
	}
	b.drag.HandleEvent(down)

	// Drag way past the bottom: start clamps to mainLen - thumbLen → value = max.
	move := &eventContext{
		current: b,
		event: events.PointerEvent{
			EventType: events.PointerMove,
			Button:    events.PointerButtonLeft,
			Position:  geometry.Point{X: 0, Y: 10000},
		},
	}
	b.drag.HandleEvent(move)
	if !approxEq(got, 400) {
		t.Fatalf("drag past bottom should clamp to max (400), got %v", got)
	}

	// Drag way past the top: start clamps to 0 → value = 0.
	move2 := &eventContext{
		current: b,
		event: events.PointerEvent{
			EventType: events.PointerMove,
			Button:    events.PointerButtonLeft,
			Position:  geometry.Point{X: 0, Y: -10000},
		},
	}
	b.drag.HandleEvent(move2)
	if !approxEq(got, 0) {
		t.Fatalf("drag past top should clamp to 0, got %v", got)
	}
}

func TestScrollBarNotScrollableNoDrag(t *testing.T) {
	b := NewScrollBar(layout.DirectionVertical)
	b.Arrange(geometry.Rect(0, 0, scrollbarWidth, 200))
	b.SetMax(0) // not scrollable

	var got float32
	got = -1 // sentinel
	b.ConnectChange(func(v float32) { got = v })

	down := &eventContext{
		current: b,
		event: events.PointerEvent{
			EventType: events.PointerDown,
			Button:    events.PointerButtonLeft,
			Position:  geometry.Point{X: 0, Y: 30},
		},
	}
	b.drag.HandleEvent(down)
	// max <= 0 → onDragBegin returns without emitting; drag state is set by the
	// controller but no change is emitted. (The owning ScrollView hides the bar
	// in this case, so hit-test never reaches it.)
	if got != -1 {
		t.Fatalf("non-scrollable bar should not emit change, got %v", got)
	}
}

func TestScrollBarStyleParts(t *testing.T) {
	b := newVerticalBar(t, 400)

	// Trough part resolves under "scroll-view" with the default rules.
	trough := b.troughStyle()
	bg, ok := trough.BackgroundColor()
	if !ok {
		t.Fatal("trough should have a background color")
	}
	r, g, bl, a := bg.RGBA()
	if r != 0 || g != 0 || bl != 0 || a != 24*0x101 {
		t.Fatalf("trough bg: want rgba(0,0,0,24), got r=%d g=%d b=%d a=%d", r, g, bl, a)
	}

	// Thumb normal state.
	thumb := b.thumbStyle()
	bg, ok = thumb.BackgroundColor()
	if !ok {
		t.Fatal("thumb should have a background color")
	}
	r, g, bl, a = bg.RGBA()
	if r != 140*0x101 || g != 140*0x101 || bl != 140*0x101 {
		t.Fatalf("thumb normal bg: want rgb(140,140,140), got r=%d g=%d b=%d a=%d", r, g, bl, a)
	}

	// Thumb pressed state (dragging).
	down := &eventContext{
		current: b,
		event: events.PointerEvent{
			EventType: events.PointerDown,
			Button:    events.PointerButtonLeft,
			Position:  geometry.Point{X: 0, Y: 10},
		},
	}
	b.drag.HandleEvent(down)
	if b.thumbState() != style.Pressed {
		t.Fatalf("thumb state should be Pressed while dragging, got %v", b.thumbState())
	}
}

func TestScrollBarStyleNameOverride(t *testing.T) {
	b := newVerticalBar(t, 400)
	b.SetStyleName("my-scroll")

	// The bar resolves under the pushed-down name, not "scroll-view".
	if b.styleName() != "my-scroll" {
		t.Fatalf("styleName should be the override, got %q", b.styleName())
	}
	// No rules for "my-scroll" → unset background (ResolveStyle returns empty
	// style merged over the "widget" default chain; the part rules do not
	// exist, so BackgroundColor is unset/transparent).
	_ = b.troughStyle() // should not panic; resolves against empty sheet.
}

func TestScrollBarSnapshot(t *testing.T) {
	b := newVerticalBar(t, 400)
	info := b.Snapshot()
	if info.Role != RoleScrollBar {
		t.Fatalf("role should be scrollbar, got %q", info.Role)
	}
}
