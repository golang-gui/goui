package gui

import (
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/events"
)

var layoutLooseFull = layout.Constraint{Min: geometry.Size{}, Max: geometry.Size{Width: layout.Inf, Height: layout.Inf}}

// --- Menu model ---

func TestMenuModelAppendAndQuery(t *testing.T) {
	m := NewMenu()
	copy := m.Append("Copy", func() {})
	m.AppendSeparator()
	m.Append("Delete", func() {})

	if got := m.ItemsCount(); got != 3 {
		t.Fatalf("ItemsCount = %d, want 3", got)
	}
	if copy.Separator() {
		t.Fatal("Copy should not be a separator")
	}
	sep := m.ItemAt(1)
	if !sep.Separator() {
		t.Fatal("item 1 should be a separator")
	}
	if m.ItemAt(0).Label() != "Copy" {
		t.Fatalf("item 0 label = %q, want Copy", m.ItemAt(0).Label())
	}
}

func TestMenuItemChangesNotifyModel(t *testing.T) {
	m := NewMenu()
	item := m.Append("Copy", func() {})

	changes := 0
	h := m.ConnectItems(func() { changes++ })
	defer h.Disconnect()

	item.SetEnabled(false)
	if changes != 1 {
		t.Fatalf("SetEnabled should notify model once, got %d", changes)
	}
	item.SetVisible(false)
	if changes != 2 {
		t.Fatalf("SetVisible should notify model once, got %d", changes)
	}
	// No-op changes do not notify.
	item.SetEnabled(false)
	if changes != 2 {
		t.Fatalf("no-op SetEnabled should not notify, got %d", changes)
	}
}

// --- menuContent rendering ---

func TestMenuContentNaturalSize(t *testing.T) {
	m := NewMenu()
	for i := 0; i < 3; i++ {
		m.Append("Item", func() {})
	}
	m.AppendSeparator()

	mc := newMenuContent(m, defaultMaxMenuHeight, func(*MenuItem) {})
	size := mc.Measure(layoutLooseFull)

	if size.Width != minMenuWidth {
		t.Fatalf("natural width = %v, want min %v", size.Width, minMenuWidth)
	}
	// App is nil in tests, so labels measure 0 and rows fall back to the 24px
	// minimum; 3 items + 1 separator.
	want := 3*menuItemMinHeight + menuSeparatorHeight
	if size.Height != float32(want) {
		t.Fatalf("natural height = %v, want %v", size.Height, want)
	}
}

func TestMenuContentSkipsInvisibleItems(t *testing.T) {
	m := NewMenu()
	m.Append("A", func() {})
	hidden := m.Append("B", func() {})
	hidden.SetVisible(false)
	m.Append("C", func() {})

	mc := newMenuContent(m, defaultMaxMenuHeight, func(*MenuItem) {})
	size := mc.Measure(layoutLooseFull)

	want := 2 * menuItemMinHeight
	if size.Height != float32(want) {
		t.Fatalf("invisible item should not contribute height, got %v want %v", size.Height, want)
	}
}

func TestMenuContentCapsAtMaxHeight(t *testing.T) {
	m := NewMenu()
	for i := 0; i < 100; i++ {
		m.Append("Item", func() {})
	}
	mc := newMenuContent(m, 60, func(*MenuItem) {})
	size := mc.Measure(layoutLooseFull)
	if size.Height != 60 {
		t.Fatalf("height should cap at maxHeight 60, got %v", size.Height)
	}
}

// --- menu row activation ---

func TestMenuItemRowActivates(t *testing.T) {
	mi := NewMenuItem("Copy", nil)

	var activated *MenuItem
	row := newMenuItemRow(func(m *MenuItem) { activated = m })
	row.bind(mi)
	row.Arrange(geometry.Rect(0, 0, 120, 24))

	dispatchClick(row, t)
	if activated != mi {
		t.Fatal("click should activate the bound item")
	}
}

func TestMenuItemRowSeparatorNotInteractive(t *testing.T) {
	sep := &MenuItem{separator: true, visible: true}
	var activated *MenuItem
	row := newMenuItemRow(func(m *MenuItem) { activated = m })
	row.bind(sep)
	row.Arrange(geometry.Rect(0, 0, 120, 9))

	dispatchClick(row, t)
	if activated != nil {
		t.Fatal("separator row should not activate")
	}
}

func dispatchClick(row *menuItemRow, t *testing.T) {
	t.Helper()
	down := &eventContext{
		current: row,
		event: events.PointerEvent{
			EventType: events.PointerDown,
			Button:    events.PointerButtonLeft,
			Position:  geometry.Point{X: 10, Y: 10},
		},
	}
	row.click.HandleEvent(down)
	up := &eventContext{
		current: row,
		event: events.PointerEvent{
			EventType: events.PointerUp,
			Button:    events.PointerButtonLeft,
			Position:  geometry.Point{X: 10, Y: 10},
		},
	}
	row.click.HandleEvent(up)
}

// --- PopoverMenu ---

func TestPopoverMenuActivateRunsAction(t *testing.T) {
	var ran string
	mi := NewMenuItem("Copy", func() { ran = "copy" })
	pm := NewPopoverMenu(nil) // no anchor: Hide is a safe no-op
	pm.activate(mi)
	if ran != "copy" {
		t.Fatalf("activate should run the item action, ran=%q", ran)
	}
}

func TestPopoverMenuActivateIgnoresDisabled(t *testing.T) {
	var ran bool
	mi := NewMenuItem("Copy", func() { ran = true })
	mi.SetEnabled(false)
	pm := NewPopoverMenu(nil)
	pm.activate(mi)
	if ran {
		t.Fatal("disabled item must not run its action")
	}
}

func TestPopoverMenuVisibleFalseBeforeShow(t *testing.T) {
	m := NewMenu()
	m.Append("Copy", func() {})
	pm := NewPopoverMenu(newTestWidget())
	pm.SetMenu(m)
	if pm.Menu() != m {
		t.Fatal("Menu() should return the set model")
	}
	if pm.Visible() {
		t.Fatal("not shown popover should not be visible")
	}
}

func TestPopoverMenuShowAtUnmountedAnchorErrors(t *testing.T) {
	// The anchor is not mounted in a window, so ShowAt must fail gracefully
	// (anchorWindow fails) rather than panic — exercising the pre-App path.
	m := NewMenu()
	m.Append("Copy", func() {})
	pm := NewPopoverMenu(newTestWidget())
	pm.SetMenu(m)
	err := pm.ShowAt(geometry.Point{X: 10, Y: 10})
	if err == nil {
		t.Fatal("ShowAt with unmounted anchor should return an error")
	}
}

// --- MenuButton ---

func TestMenuButtonSetMenuNoMenuIsNoop(t *testing.T) {
	b := NewMenuButton()
	b.SetChild(NewLabel("File"))
	b.openMenu() // menu nil → should not panic / do nothing
	if b.pm != nil {
		t.Fatal("openMenu with nil menu should not create a popover")
	}
}

func TestMenuButtonCreatesPopoverOnOpen(t *testing.T) {
	m := NewMenu()
	m.Append("Copy", func() {})
	b := NewMenuButton()
	b.SetChild(NewLabel("File"))
	b.SetMenu(m)
	b.openMenu()
	if b.pm == nil {
		t.Fatal("openMenu with a menu should create a PopoverMenu")
	}
	if b.pm.Menu() != m {
		t.Fatal("MenuButton should forward its menu to the PopoverMenu")
	}
}
