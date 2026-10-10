package gui

import (
	"image/color"
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
	item.SetLabel("复制")
	item.SetLabel("复制")
	if changes != 3 || item.Label() != "复制" || m.ItemAt(0) != item || item.Action() == nil {
		t.Fatal("label change must notify once and preserve item identity/action")
	}
}

// --- menuContent rendering ---

func TestMenuContentNaturalSize(t *testing.T) {
	m := NewMenu()
	for i := 0; i < 3; i++ {
		m.Append("Item", func() {})
	}
	m.AppendSeparator()

	mc := newMenuContent(m, 0, func(*MenuItem) {})
	size := mc.Measure(layoutLooseFull)

	if size.Width != 160 {
		t.Fatalf("menu body width = %v, want default minimum 160", size.Width)
	}
	if want := float32(2*menuItemPadding + 2*menuContentPadding); mc.naturalW != want {
		t.Fatalf("intrinsic width = %v, want padding-only width %v without typography", mc.naturalW, want)
	}
	// Without typography, rows use their minimum height, plus shell padding.
	want := 3*menuItemMinHeight + menuSeparatorHeight + 2*menuContentPadding
	if size.Height != float32(want) {
		t.Fatalf("natural height = %v, want %v", size.Height, want)
	}
}

func TestMenuContentWidthConstraints(t *testing.T) {
	useTestApplication(t, &application{typo: &styleTypography{}, style: textStyleSheet(10, color.Black)})
	for _, tt := range []struct {
		name  string
		label string
		c     layout.Constraint
		want  float32
	}{
		{"short Chinese", "复制", layout.Unbounded(), 160},
		{"short English", "Copy", layout.Unbounded(), 160},
		// The typography fixture measures each byte as 10 DIP; include row and body padding.
		{"long content", "A very long menu item", layout.Unbounded(), 210 + 16 + 12},
		{"narrow work area", "复制", layout.Loose(geometry.Size{Width: 100, Height: 100}), 100},
		{"parent minimum", "复制", layout.Tight(geometry.Size{Width: 220, Height: 40}), 220},
	} {
		t.Run(tt.name, func(t *testing.T) {
			m := NewMenu()
			m.Append(tt.label, nil)
			mc := newMenuContent(m, 0, nil)
			size := mc.Measure(tt.c).Size
			if size != (geometry.Size{Width: tt.want, Height: 40}) {
				t.Fatalf("body size = %v, want %vx40", size, tt.want)
			}
		})
	}
}

func TestMenuContentSkipsInvisibleItems(t *testing.T) {
	m := NewMenu()
	m.Append("A", func() {})
	hidden := m.Append("B", func() {})
	hidden.SetVisible(false)
	m.Append("C", func() {})

	mc := newMenuContent(m, 0, func(*MenuItem) {})
	size := mc.Measure(layoutLooseFull)

	want := 2*menuItemMinHeight + 2*menuContentPadding
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

func TestPopoverMenuClosesBeforeRunningAction(t *testing.T) {
	owner := &window{}
	p := &popover{owner: owner, visible: true, modal: true}
	owner.SetModalTarget(p)
	pm := &PopoverMenu{popover: p}
	closed := 0
	p.ConnectClosed(func() { closed++ })
	called := false
	pm.activate(NewMenuItem("Open", func() {
		called = true
		if p.Visible() || owner.modalTarget != nil || closed != 1 {
			t.Fatal("menu must hide and release its modal target before the action opens a native dialog")
		}
	}))
	if !called || closed != 1 {
		t.Fatalf("called=%t, closed=%d", called, closed)
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

func TestMenuButtonKeyboardUsesOpenPath(t *testing.T) {
	for _, key := range []events.Key{events.KeySpace, events.KeyEnter} {
		b := NewMenuButton()
		menu := NewMenu()
		menu.Append("Copy", func() {})
		b.SetMenu(menu)
		win := &window{}
		win.SetWidget(b)
		_ = win.DispatchEvent(shortcutPress(events.KeyTab, 0))
		_ = win.DispatchEvent(shortcutPress(key, 0))
		// No native surface is supplied: showing fails, but the real open path
		// must have created the menu and forwarded its model, not just a counter.
		if b.pm == nil || b.pm.Menu() != menu {
			t.Fatal("keyboard bypassed menu open path")
		}
		win.Destroy()
	}
}
