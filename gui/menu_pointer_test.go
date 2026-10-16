package gui

import (
	"image/color"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/style"
)

func TestMenuPointerAndKeyboardShareOneActiveItem(t *testing.T) {
	normal, highlight := color.NRGBA{R: 255, G: 255, B: 255, A: 255}, color.NRGBA{B: 220, A: 255}
	useTestApplication(t, &application{typo: &styleTypography{}, style: style.Sheet(
		style.Name(styleNameMenuItemText).FontSize(12),
		style.Name(styleNameMenuItemTextDisabled).FontSize(12),
		style.Name(styleNameMenuItem).BackgroundColor(normal),
		style.Name(styleNameMenuItem).State(style.Hovered).BackgroundColor(highlight),
		style.Name(styleNameMenuItem).State(style.Pressed).BackgroundColor(highlight),
	)})
	m := NewMenu()
	first, second := m.Append("First", nil), m.Append("Second", nil)
	disabled := m.Append("Disabled", nil)
	disabled.SetEnabled(false)
	m.AppendSeparator()
	m.Append("Last", nil)
	var activated *MenuItem
	content := newMenuContent(m, 0, func(item *MenuItem) { activated = item })
	win := &window{}
	win.SetWidget(content)
	defer win.Destroy()
	size := content.Measure(layout.Unbounded()).Size
	content.Arrange(geometry.Rect(0, 0, size.Width, size.Height))
	row := func(index int) *menuItemRow {
		t.Helper()
		r, ok := content.list.items[index].(*menuItemRow)
		if !ok {
			t.Fatalf("row %d was not realized", index)
		}
		return r
	}
	point := func(index int, text bool) geometry.Point {
		r := row(index)
		bounds := r.Snapshot().Bounds
		p := geometry.Point{X: bounds.X + 2, Y: bounds.Y + bounds.Height/2}
		var want Widget = r
		if text {
			bounds = r.label.Snapshot().Bounds
			p = geometry.Point{X: bounds.X + bounds.Width/2, Y: bounds.Y + bounds.Height/2}
			want = r.label
		}
		if got := Pick(content, p); got != want {
			t.Fatalf("point did not hit intended target: %T, want %T", got, want)
		}
		return p
	}
	assertActive := func(want *MenuItem) {
		t.Helper()
		painted, selected := 0, 0
		for _, widget := range content.list.items {
			r := widget.(*menuItemRow)
			background, ok := r.resolvedStyle().BackgroundColor()
			lit := ok && background != nil && graphics.ColorOf(background) == graphics.ColorOf(highlight)
			if lit {
				painted++
			}
			if r.Snapshot().Selected {
				selected++
			}
			if lit != (want != nil && r.mi == want) || lit != r.Snapshot().Selected {
				t.Errorf("row %q: painted=%t selected=%t, want active=%v", r.mi.Label(), lit, r.Snapshot().Selected, want)
			}
		}
		count := 0
		if want != nil {
			count = 1
		}
		if content.active != want || painted != count || selected != count {
			t.Fatalf("active=%v, painted=%d selected=%d; want %v/%d", content.active, painted, selected, want, count)
		}
	}
	move := func(index int, text bool) {
		t.Helper()
		if err := win.DispatchEvent(events.PointerEvent{EventType: events.PointerMove, Position: point(index, text)}); err != nil {
			t.Fatal(err)
		}
	}
	press := func(key events.Key) {
		t.Helper()
		if err := win.DispatchEvent(shortcutPress(key, 0)); err != nil {
			t.Fatal(err)
		}
	}

	move(0, false)
	assertActive(first)
	if err := win.DispatchEvent(events.PointerEvent{EventType: events.PointerDown, Button: events.PointerButtonLeft, Position: point(0, false)}); err != nil {
		t.Fatal(err)
	}
	press(events.KeyEnd)
	assertActive(m.ItemAt(4))
	if err := win.DispatchEvent(events.PointerEvent{EventType: events.PointerUp, Button: events.PointerButtonLeft, Position: point(0, false)}); err != nil {
		t.Fatal(err)
	}
	// A click at the stationary pointer also takes over keyboard navigation.
	if err := win.DispatchEvent(events.PointerEvent{EventType: events.PointerDown, Button: events.PointerButtonLeft, Position: point(0, false)}); err != nil {
		t.Fatal(err)
	}
	assertActive(first)
	if err := win.DispatchEvent(events.PointerEvent{EventType: events.PointerUp, Button: events.PointerButtonLeft, Position: point(0, false)}); err != nil {
		t.Fatal(err)
	}
	// Jump straight from one row's padding to another row's child Label.
	move(1, true)
	assertActive(second)
	for range 100 {
		move(0, true)
		assertActive(first)
		move(1, true)
		assertActive(second)
	}
	press(events.KeyHome)
	assertActive(first)
	// The pointer is still inside Second's Label: no crossing event is emitted.
	move(1, true)
	assertActive(second)
	move(1, false)
	assertActive(second)
	move(1, true)
	assertActive(second)
	move(2, true)
	assertActive(nil)
	move(3, false)
	assertActive(nil)
	move(0, true)
	assertActive(first)
	if err := win.DispatchEvent(events.PointerEvent{EventType: events.PointerLeave}); err != nil {
		t.Fatal(err)
	}
	// A native Enter has crossing notifications but no ordinary Motion event.
	if err := win.DispatchEvent(events.PointerEvent{EventType: events.PointerEnter, Position: point(1, true)}); err != nil {
		t.Fatal(err)
	}
	assertActive(second)
	press(events.KeyEnter)
	if activated != second {
		t.Fatal("keyboard activation ignored the pointer's active item")
	}
}
