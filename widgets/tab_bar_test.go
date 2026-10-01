package widgets

import (
	"slices"
	"testing"
	"time"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/events"
)

func dragTestBar() (*TabBar, *TabView, []*TabPage) {
	view := NewTabView()
	pages := []*TabPage{NewTabPage("A", nil), NewTabPage("B", nil), NewTabPage("C", nil)}
	for _, page := range pages {
		page.SetClosable(true)
		view.AppendPage(page)
	}
	bar := NewTabBar()
	bar.SetView(view)
	bar.SetReorderable(true)
	// Fixed equal slots isolate input geometry from font/desktop measurement.
	bar.SetTabWidthRange(100, 100)
	bar.Measure(layout.Loose(geometry.Size{Width: 400, Height: 40}))
	bar.Arrange(geometry.Rect(0, 0, 400, 40))
	return bar, view, pages
}

type tabInputHost struct {
	root, focus gui.Widget
}

func (h *tabInputHost) Widget() gui.Widget                 { return h.root }
func (h *tabInputHost) FocusedWidget() gui.Widget          { return h.focus }
func (h *tabInputHost) SetFocusedWidget(w gui.Widget) bool { h.focus = w; return true }

func dispatchTabPointer(t *testing.T, d *gui.EventDispatcher, host *tabInputHost, kind events.EventType, x, y float32) {
	t.Helper()
	buttons := events.PointerButtonLeftDown
	if kind == events.PointerUp {
		buttons = 0
	}
	if err := d.DispatchEvent(host, events.PointerEvent{
		EventType: kind, Position: geometry.Point{X: x, Y: y},
		Button: events.PointerButtonLeft, Buttons: buttons,
	}); err != nil {
		t.Fatal(err)
	}
}

func TestTabDragFollowsPointerSlidesAndCommitsOnce(t *testing.T) {
	bar, view, pages := dragTestBar()
	view.SetCurrent(pages[2])
	moves := 0
	view.ConnectMoved(func(page *TabPage, from, to int) {
		moves++
		if page != pages[0] || from != 0 || to != 1 {
			t.Fatalf("unexpected move %s %d -> %d", page.Title(), from, to)
		}
	})
	d := new(gui.EventDispatcher)
	host := &tabInputHost{root: bar}
	dispatchTabPointer(t, d, host, events.PointerDown, 20, 20)
	dispatchTabPointer(t, d, host, events.PointerMove, 30, 20) // cross the drag threshold
	dispatchTabPointer(t, d, host, events.PointerMove, 140, 20)
	if !slices.Equal(view.Pages(), pages) || moves != 0 || view.Current() != pages[2] {
		t.Fatal("drag preview changed page order or selection")
	}
	a, b := bar.items[pages[0]], bar.items[pages[1]]
	if a.Rect().X != 120 || b.Rect().X != 104 || b.motion.target != 0 {
		t.Fatalf("drag must preserve its 20 DIP grab offset and begin neighbor slide: A=%v B=%v target=%g", a.Rect(), b.Rect(), b.motion.target)
	}
	bar.advanceMotion(70 * time.Millisecond)
	if a.Rect().X != 120 || b.Rect().X <= 0 || b.Rect().X >= 104 {
		t.Fatalf("neighbor jumped or dragged item lagged: A=%v B=%v", a.Rect(), b.Rect())
	}
	// The lifted item owns the overlapping hit area. Snapshot describes the
	// same back-to-front order and animated bounds used for drawing and picking.
	if target := gui.Pick(bar, geometry.Point{X: 140, Y: 20}); target != a.body {
		t.Fatalf("overlap did not hit raised tab body: %T", target)
	}
	info := bar.Snapshot()
	if info.Children[2].Text != "A" || info.Children[2].Bounds.X != 120 {
		t.Fatalf("snapshot lost paint order or animated bounds: %+v", info.Children)
	}
	dispatchTabPointer(t, d, host, events.PointerUp, 140, 20)
	if moves != 1 || !slices.Equal(view.Pages(), []*TabPage{pages[1], pages[0], pages[2]}) {
		t.Fatalf("release did not commit exactly once: moves=%d pages=%v", moves, view.Pages())
	}
	bar.advanceMotion(100 * time.Millisecond)
	bar.advanceMotion(100 * time.Millisecond)
	if a.Rect().X != 104 || b.Rect().X != 0 || bar.lifted != nil || bar.moving() || view.Current() != pages[2] {
		t.Fatal("drop did not settle or changed the retained selection")
	}
}

func TestTabDragEscapeRestoresWithoutCommit(t *testing.T) {
	bar, view, pages := dragTestBar()
	d := new(gui.EventDispatcher)
	host := &tabInputHost{root: bar}
	dispatchTabPointer(t, d, host, events.PointerDown, 20, 20)
	dispatchTabPointer(t, d, host, events.PointerMove, 30, 20)
	dispatchTabPointer(t, d, host, events.PointerMove, 140, 20)
	if err := d.DispatchEvent(host, events.KeyEvent{EventType: events.KeyDown, Key: events.KeyEscape}); err != nil {
		t.Fatal(err)
	}
	dispatchTabPointer(t, d, host, events.PointerUp, 140, 20)
	bar.advanceMotion(100 * time.Millisecond)
	bar.advanceMotion(100 * time.Millisecond)
	if !slices.Equal(view.Pages(), pages) || bar.items[pages[0]].Rect().X != 0 || bar.lifted != nil {
		t.Fatal("Escape failed to restore original tab positions")
	}
}

func TestTabDragReverseAndExternalRemoval(t *testing.T) {
	bar, view, pages := dragTestBar()
	bar.beginDrag(pages[0], geometry.Point{X: 20, Y: 20})
	bar.updateDrag(pages[0], geometry.Point{X: 140, Y: 20})
	bar.advanceMotion(40 * time.Millisecond)
	x := bar.items[pages[1]].Rect().X
	bar.updateDrag(pages[0], geometry.Point{X: 140, Y: 20})
	if bar.insertAt != 1 {
		t.Fatal("stationary pointer changed slots while neighbor was animating")
	}
	bar.updateDrag(pages[0], geometry.Point{X: 40, Y: 20})
	if bar.insertAt != 0 || bar.items[pages[1]].Rect().X != x {
		t.Fatal("reversing direction jumped the animated neighbor")
	}
	bar.advanceMotion(40 * time.Millisecond)
	if bar.items[pages[1]].Rect().X <= x {
		t.Fatal("neighbor did not animate back")
	}
	view.RemovePage(pages[0])
	if bar.dragPage != nil || bar.lifted != nil || bar.moving() {
		t.Fatal("removing dragged page left a live transition")
	}
}

func TestTabDropCallbackCanUnbindBar(t *testing.T) {
	bar, view, pages := dragTestBar()
	view.ConnectMoved(func(*TabPage, int, int) { bar.SetView(nil) })
	bar.beginDrag(pages[0], geometry.Point{X: 20, Y: 20})
	bar.updateDrag(pages[0], geometry.Point{X: 140, Y: 20})
	bar.endDrag(pages[0], geometry.Point{X: 140, Y: 20})
	if bar.View() != nil || len(bar.items) != 0 || bar.dragPage != nil || bar.lifted != nil || bar.moving() {
		t.Fatal("drop callback retained the old binding or drag state")
	}
}

func TestTabBarBlankHitPathIsPassive(t *testing.T) {
	bar, _, pages := dragTestBar()
	for _, x := range []float32{102, 206} {
		if target := gui.Pick(bar, geometry.Point{X: x, Y: 20}); target != bar.viewport {
			t.Fatalf("4 DIP gap hit a tab at x=%g: %T", x, target)
		}
	}
	target := gui.Pick(bar, geometry.Point{X: 350, Y: 20})
	if target != bar.viewport {
		t.Fatalf("blank area hit an item: %T", target)
	}
	// HeaderBar already tests its conservative controller policy in gui. This
	// asserts the composition supplied to that policy: no focus or pointer input
	// controllers on the blank path, while actual tabs remain interactive.
	for w := target; w != nil; w = w.Parent() {
		if w.Focusable() {
			t.Fatalf("blank path includes focusable %T", w)
		}
		for _, controller := range w.EventControllers() {
			if _, ok := controller.(*gui.KeyEventController); !ok {
				t.Fatalf("blank path consumes pointer input through %T", controller)
			}
		}
	}
	if !bar.items[pages[0]].body.Focusable() {
		t.Fatal("tab body lost keyboard focus")
	}
	d := new(gui.EventDispatcher)
	host := &tabInputHost{root: bar}
	closed := 0
	bar.view.ConnectCloseRequest(func(page *TabPage) {
		closed++
		if page != pages[1] {
			t.Fatal("wrong close target")
		}
	})
	// An inactive tab discloses close only after hover has been laid out.
	dispatchTabPointer(t, d, host, events.PointerMove, 188, 20)
	bar.Arrange(bar.Rect())
	dispatchTabPointer(t, d, host, events.PointerDown, 188, 20)
	dispatchTabPointer(t, d, host, events.PointerUp, 188, 20)
	if closed != 1 || bar.view.Current() != pages[0] || bar.dragPage != nil {
		t.Fatal("close target selected or dragged the tab")
	}
}

func TestTabCloseDisclosureKeepsGeometryAndSnapshot(t *testing.T) {
	bar, view, pages := dragTestBar()
	a, b := bar.items[pages[0]], bar.items[pages[1]]
	if !a.close.Visible() || b.close.Visible() || b.close.Snapshot().Visible {
		t.Fatal("only the selected tab should initially disclose close")
	}
	before := b.Measure(layout.Unbounded())
	labelRect := b.body.label.Rect()
	if target := gui.Pick(bar, geometry.Point{X: 188, Y: 20}); target != b.body {
		t.Fatalf("hidden close space must hit the body: %T", target)
	}
	d := new(gui.EventDispatcher)
	host := &tabInputHost{root: bar}
	dispatchTabPointer(t, d, host, events.PointerMove, 130, 20)
	bar.Arrange(bar.Rect())
	if !b.close.Visible() || !b.close.Snapshot().Visible || b.Measure(layout.Unbounded()) != before || b.body.label.Rect() != labelRect {
		t.Fatal("hover disclosure changed geometry or lost snapshot visibility")
	}
	if target := gui.Pick(bar, geometry.Point{X: 179, Y: 20}); target != b.close {
		t.Fatalf("disclosed close has no hit target: %T", target)
	}
	dispatchTabPointer(t, d, host, events.PointerMove, 350, 20)
	bar.Arrange(bar.Rect())
	if b.close.Visible() || b.Measure(layout.Unbounded()) != before || b.body.label.Rect() != labelRect {
		t.Fatal("leaving the inactive tab failed to hide close without shifting text")
	}
	view.SetCurrent(pages[1])
	bar.Arrange(bar.Rect())
	if !b.close.Visible() || a.close.Visible() {
		t.Fatal("selection failed to transfer close disclosure")
	}
	pages[1].SetClosable(false)
	bar.Arrange(bar.Rect())
	if b.close.Visible() || b.body.Measure(layout.Unbounded()).Width+28 != a.body.Measure(layout.Unbounded()).Width {
		t.Fatal("non-closable tab retained button or reserved close space")
	}
	pages[1].SetClosable(true)
	bar.Arrange(bar.Rect())
	if !b.close.Visible() || b.Measure(layout.Unbounded()) != before {
		t.Fatal("restoring closability did not restore button/geometry")
	}
}

func TestTabHiddenClosePressSelectsEvenAcrossFrame(t *testing.T) {
	bar, view, pages := dragTestBar()
	d := new(gui.EventDispatcher)
	host := &tabInputHost{root: bar}
	closed := 0
	view.ConnectCloseRequest(func(*TabPage) { closed++ })
	// No preceding hover frame: the user has not seen a close affordance.
	dispatchTabPointer(t, d, host, events.PointerDown, 188, 20)
	bar.Arrange(bar.Rect())
	if bar.items[pages[1]].close.Visible() {
		t.Fatal("disclosed a close target under an ongoing body press")
	}
	dispatchTabPointer(t, d, host, events.PointerUp, 188, 20)
	bar.Arrange(bar.Rect())
	if closed != 0 || view.Current() != pages[1] || !bar.items[pages[1]].close.Visible() {
		t.Fatal("hidden close area must select, never close")
	}
}

func TestTabCloseDisclosurePreservesCloseFocusOnly(t *testing.T) {
	bar, _, pages := dragTestBar()
	b := bar.items[pages[1]]
	d := new(gui.EventDispatcher)
	host := &tabInputHost{root: bar}
	focus := func(widget gui.Widget) {
		host.SetFocusedWidget(widget)
		if err := d.DispatchEvent(host, events.FocusEvent{Focused: true}); err != nil {
			t.Fatal(err)
		}
		bar.Arrange(bar.Rect())
	}
	focus(b.body)
	if b.close.Visible() || !b.body.Focused() || !b.ContainsFocus() {
		t.Fatal("inactive body focus must remain without disclosing close")
	}
	// Only focus a visible button, as a keyboard user would after disclosure.
	dispatchTabPointer(t, d, host, events.PointerMove, 130, 20)
	bar.Arrange(bar.Rect())
	focus(b.close)
	dispatchTabPointer(t, d, host, events.PointerMove, 350, 20)
	bar.Arrange(bar.Rect())
	if !b.close.Visible() || !b.close.Focused() {
		t.Fatal("leaving hover hid the focused close button")
	}
	focus(b.body) // focus remains within the same item
	if b.close.Visible() || b.close.Snapshot().Visible {
		t.Fatal("moving focus back to body did not hide inactive close")
	}
}

func TestInactiveTabCloseHidesAfterDrag(t *testing.T) {
	for _, ending := range []string{"commit", "escape", "outside"} {
		t.Run(ending, func(t *testing.T) {
			bar, view, pages := dragTestBar()
			item := bar.items[pages[1]]
			d := new(gui.EventDispatcher)
			host := &tabInputHost{root: bar}
			closed := 0
			view.ConnectCloseRequest(func(*TabPage) { closed++ })
			dispatchTabPointer(t, d, host, events.PointerMove, 130, 20)
			bar.Arrange(bar.Rect())
			dispatchTabPointer(t, d, host, events.PointerDown, 130, 20)
			// The test host stores focus; deliver its focus notification through
			// the dispatcher, just as a real host does when focus changes.
			if err := d.DispatchEvent(host, events.FocusEvent{Focused: true}); err != nil {
				t.Fatal(err)
			}
			dispatchTabPointer(t, d, host, events.PointerMove, 120, 20)
			dispatchTabPointer(t, d, host, events.PointerMove, 30, 20)
			if bar.dragPage != pages[1] {
				t.Fatal("fixture did not start an inactive tab drag")
			}
			switch ending {
			case "escape":
				if err := d.DispatchEvent(host, events.KeyEvent{EventType: events.KeyDown, Key: events.KeyEscape}); err != nil {
					t.Fatal(err)
				}
				dispatchTabPointer(t, d, host, events.PointerUp, 30, 20)
			case "outside":
				dispatchTabPointer(t, d, host, events.PointerMove, 30, 70)
				dispatchTabPointer(t, d, host, events.PointerUp, 30, 70)
			default:
				dispatchTabPointer(t, d, host, events.PointerUp, 30, 20)
			}
			bar.advanceMotion(100 * time.Millisecond)
			bar.advanceMotion(100 * time.Millisecond)
			if err := d.DispatchEvent(host, events.PointerEvent{EventType: events.PointerMove, Position: geometry.Point{X: 350, Y: 20}}); err != nil {
				t.Fatal(err)
			}
			bar.Arrange(bar.Rect())
			want := pages
			if ending == "commit" {
				want = []*TabPage{pages[1], pages[0], pages[2]}
			}
			if !slices.Equal(view.Pages(), want) || view.Current() != pages[0] || closed != 0 {
				t.Fatal("drag changed selection, closed a page or committed the wrong order")
			}
			if !item.body.Focused() || !item.ContainsFocus() || host.FocusedWidget() != item.body {
				t.Fatal("hiding close must not clear tab focus")
			}
			if item.hovered || item.close.Visible() || item.close.Snapshot().Visible {
				t.Fatal("inactive close stayed visible after drag and pointer leave")
			}
			point := geometry.Point{X: item.Rect().X + 84, Y: 20}
			if target := gui.Pick(bar, point); target != item.body {
				t.Fatalf("hidden close area intercepted input: %T", target)
			}
		})
	}
}

func TestTabHiddenCloseSpaceCanStartReorder(t *testing.T) {
	bar, view, pages := dragTestBar()
	d := new(gui.EventDispatcher)
	host := &tabInputHost{root: bar}
	closed := 0
	view.ConnectCloseRequest(func(*TabPage) { closed++ })
	dispatchTabPointer(t, d, host, events.PointerDown, 188, 20)
	bar.Arrange(bar.Rect())
	dispatchTabPointer(t, d, host, events.PointerMove, 178, 20)
	bar.Arrange(bar.Rect())
	dispatchTabPointer(t, d, host, events.PointerMove, 30, 20)
	dispatchTabPointer(t, d, host, events.PointerUp, 30, 20)
	if closed != 0 || view.Current() != pages[0] || !slices.Equal(view.Pages(), []*TabPage{pages[1], pages[0], pages[2]}) {
		t.Fatal("hidden close space did not behave as a draggable tab body")
	}
}

func TestTabSpacingMeasureOverflowAndReveal(t *testing.T) {
	bar, view, pages := dragTestBar()
	if width := bar.Measure(layout.Unbounded()).Width; width != 308 {
		t.Fatalf("three 100 DIP tabs plus two 4 DIP gaps measured %g", width)
	}
	// The gaps alone cause overflow: they must count in both measurement and
	// the effective scroll extent, with no extra gap after the last tab.
	bar.Arrange(geometry.Rect(0, 0, 300, 40))
	if !bar.overflow || bar.contentWidth != 308 || bar.viewportWidth != 252 {
		t.Fatalf("spacing lost in overflow: content=%g viewport=%g", bar.contentWidth, bar.viewportWidth)
	}
	view.SetCurrent(pages[2])
	bar.Arrange(geometry.Rect(0, 0, 300, 40))
	last := bar.items[pages[2]].Rect()
	if bar.scroll != 56 || last.X+last.Width != 252 {
		t.Fatalf("last tab reveal left trailing space or clipped its edge: scroll=%g last=%v", bar.scroll, last)
	}
	bar.Arrange(geometry.Rect(0, 0, 308, 40))
	if bar.overflow || bar.scroll != 0 {
		t.Fatal("exact fit should have neither arrows nor scroll offset")
	}
	for i, child := range bar.Snapshot().Children {
		if child.Bounds.X != float32(i*104) || child.Bounds.Width != 100 {
			t.Fatalf("snapshot lost spaced slot %d: %v", i, child.Bounds)
		}
	}
	view.RemovePage(pages[2])
	view.RemovePage(pages[1])
	if width := bar.Measure(layout.Unbounded()).Width; width != 100 {
		t.Fatalf("single tab acquired a trailing gap: %g", width)
	}
	view.RemovePage(pages[0])
	if width := bar.Measure(layout.Unbounded()).Width; width != 0 {
		t.Fatalf("empty strip measured %g", width)
	}
}

func TestSingleTabWithoutTransferStaysLocal(t *testing.T) {
	bar, view, pages := dragTestBar()
	view.RemovePage(pages[2])
	view.RemovePage(pages[1])
	bar.Arrange(bar.Rect())
	errors, requests := 0, 0
	bar.ConnectTransferError(func(error) { errors++ })
	bar.ConnectDetachRequest(func(*TabDetachRequest) { requests++ })
	d := new(gui.EventDispatcher)
	host := &tabInputHost{root: bar}
	dispatchTabPointer(t, d, host, events.PointerDown, 20, 20)
	dispatchTabPointer(t, d, host, events.PointerMove, 40, 20)
	dispatchTabPointer(t, d, host, events.PointerMove, 140, 70)
	if bar.dragPage != pages[0] || bar.nativeDrag != nil || errors != 0 || requests != 0 {
		t.Fatalf("single tab left local drag: page=%v errors=%d requests=%d", bar.dragPage, errors, requests)
	}
	if bar.items[pages[0]].Rect().X <= 0 {
		t.Fatal("single tab did not follow pointer")
	}
	dispatchTabPointer(t, d, host, events.PointerUp, 140, 70)
	for range 20 {
		bar.advanceMotion(50 * time.Millisecond)
	}
	if bar.dragPage != nil || bar.items[pages[0]].Rect().X != 0 || view.Current() != pages[0] || len(view.Pages()) != 1 {
		t.Fatal("single tab did not return to its original slot")
	}
}

func TestTabSpacingDragThresholdWithCompressedWidths(t *testing.T) {
	bar, _, pages := dragTestBar()
	bar.SetTabWidthRange(60, 140)
	bar.Arrange(geometry.Rect(0, 0, 308, 40)) // shrink all three slots to 100
	bar.beginDrag(pages[0], geometry.Point{X: 20, Y: 20})
	// B starts at 104, with midpoint 154. A's right edge must cross that
	// midpoint before swapping, not the old no-gap midpoint of 150.
	bar.updateDrag(pages[0], geometry.Point{X: 74, Y: 20})
	if bar.insertAt != 0 {
		t.Fatal("swapped before crossing the spaced neighbor midpoint")
	}
	bar.updateDrag(pages[0], geometry.Point{X: 75, Y: 20})
	if bar.insertAt != 1 || bar.items[pages[1]].motion.target != 0 {
		t.Fatal("did not swap after crossing the neighbor midpoint")
	}
	// B now targets x=0, midpoint 50; A stays in slot 1 until its left
	// edge crosses that midpoint in the reverse direction.
	bar.updateDrag(pages[0], geometry.Point{X: 70, Y: 20})
	if bar.insertAt != 1 {
		t.Fatal("reverse threshold included the wrong gap")
	}
	bar.updateDrag(pages[0], geometry.Point{X: 69, Y: 20})
	if bar.insertAt != 0 || bar.items[pages[1]].motion.target != 104 {
		t.Fatal("reverse crossing failed to restore spaced slots")
	}
	bar.cancelDrag()
}
