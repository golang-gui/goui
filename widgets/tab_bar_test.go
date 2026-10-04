package widgets

import (
	"image"
	"image/color"
	"math"
	"slices"
	"testing"
	"time"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/platform/typography"
	"github.com/golang-gui/goui/style"
	widgetstyle "github.com/golang-gui/goui/widgets/style"
)

// 布局、溢出与页面顺序同步。

func TestAllocateTabWidth(t *testing.T) {
	for _, tc := range []struct {
		count                             int
		available, minimum, maximum, want float32
		overflow                          bool
	}{
		{0, 0, 96, 200, 0, false},
		{1, 500, 96, 200, 200, false},
		{3, 800, 96, 200, 200, false},
		{3, 608, 96, 200, 200, false},
		{3, 458, 96, 200, 150, false},
		{3, 296, 96, 200, 96, false},
		{3, 295, 96, 200, 96, true},
		{3, 296.75, 96, 200, 96.25, false},
		{3, 0, 96, 200, 96, true},
		{1, 95, 96, 200, 96, true},
		{3, 608, 200, 200, 200, false},
		{3, 607, 200, 200, 200, true},
	} {
		width, overflow := allocateTabWidth(tc.count, tc.available, tc.minimum, tc.maximum)
		if width != tc.want || overflow != tc.overflow {
			t.Fatalf("%+v: got width=%g overflow=%v", tc, width, overflow)
		}
	}
}

func TestTabWidthThresholdStableAndReveal(t *testing.T) {
	bar, view, pages := dragTestBar()
	bar.SetTabWidthRange(0, 0) // restore defaults after the fixed-width input fixture
	view.SetCurrent(pages[2])
	for _, available := range []float32{800, 458, 344, 343, 344, 343, 458} {
		for repeat := 0; repeat < 3; repeat++ {
			bar.Arrange(geometry.Rect(0, 0, available, 40))
			wantWidth := float32(112)
			if available >= 344 {
				wantWidth = min(240, (available-8)/3)
			}
			if bar.overflow != (available < 344) {
				t.Fatalf("threshold oscillated at width=%g", available)
			}
			for i, page := range pages {
				got := bar.items[page].Rect()
				if got.Width != wantWidth || got.X != float32(i)*(wantWidth+4)-bar.scroll {
					t.Fatalf("slots disagree with allocation: %v", got)
				}
			}
			last := bar.items[pages[2]].Rect()
			if last.X < 0 || last.X+last.Width > bar.viewportWidth {
				t.Fatal("resize lost current tab visibility")
			}
			if !bar.overflow && bar.scroll != 0 {
				t.Fatal("shrink mode retained scroll")
			}
		}
	}
	bar.Arrange(geometry.Rect(0, 0, 343, 40))
	bar.setScroll(0)
	bar.Arrange(bar.Rect())
	if bar.scroll != 0 {
		t.Fatal("ordinary layout undid manual scroll")
	}
	bar.beginDrag(pages[0], geometry.Point{X: 44, Y: 20})
	bar.Arrange(geometry.Rect(0, 0, 458, 40))
	if bar.dragPage != nil || bar.lifted != nil {
		t.Fatal("resize retained stale drag geometry")
	}
	bar.beginDrag(pages[0], geometry.Point{X: 20, Y: 20})
	bar.SetTabWidthRange(100, 100)
	if bar.dragPage != nil || bar.lifted != nil {
		t.Fatal("range change retained drag")
	}
}

type tabTestIcon struct{}

func (tabTestIcon) Image(int, int, gui.Color) image.Image { return nil }

func TestTabWidthRangeNormalizationAndAccessoryFloor(t *testing.T) {
	if minimum, maximum := NewTabBar().TabWidthRange(); minimum != 112 || maximum != 240 {
		t.Fatalf("constructor defaults: %g..%g", minimum, maximum)
	}
	bar, _, pages := dragTestBar()
	for _, tc := range []struct{ min, max, wantMin, wantMax float32 }{
		{0, 0, 112, 240}, {-1, -1, 112, 240},
		{float32(math.NaN()), float32(math.Inf(1)), 112, 240},
		{240, 100, 240, 240},
	} {
		bar.SetTabWidthRange(tc.min, tc.max)
		if minimum, maximum := bar.TabWidthRange(); minimum != tc.wantMin || maximum != tc.wantMax {
			t.Fatalf("normalization: %g..%g", minimum, maximum)
		}
	}
	pages[0].SetIcon(tabTestIcon{})
	pages[1].SetClosable(false)
	pages[2].SetTitle("A much longer title must not alter equal widths")
	bar.SetTabWidthRange(10, 10)
	bar.Arrange(geometry.Rect(0, 0, 300, 40))
	for _, page := range pages {
		if width := bar.items[page].Rect().Width; width != 70 {
			t.Fatalf("mixed accessories/titles broke common safety floor: %g", width)
		}
	}
	// A single slot wider than an extremely small viewport retains its minimum;
	// arrow rectangles must not overlap or extend outside the host.
	bar.Arrange(geometry.Rect(0, 0, 30, 40))
	if bar.previous.Rect().Width != 15 || bar.next.Rect().X != 15 || bar.viewportWidth != 0 {
		t.Fatal("tiny viewport produced invalid arrow geometry")
	}
	bar.SetTabWidthRange(100, 220)
	if got := bar.Measure(layout.Unbounded()).Width; got != 668 {
		t.Fatalf("preferred strip width did not follow max: %g", got)
	}
}

// 页面同步与边缘滚动。

func TestTabViewMoveAndBarOverflow(t *testing.T) {
	v := NewTabView()
	pages := []*TabPage{NewTabPage("One", nil), NewTabPage("Two", nil), NewTabPage("Three", nil)}
	for _, p := range pages {
		p.SetClosable(true)
		v.AppendPage(p)
	}
	v.SetCurrent(pages[1])
	bar := NewTabBar()
	bar.SetView(v)
	bar.SetReorderable(true)
	bar.Measure(layout.Loose(geometry.Size{Width: 110, Height: 40}))
	bar.Arrange(geometry.Rect(0, 0, 110, 40))
	if !bar.overflow || bar.viewport.Rect().Width != 110-2*tabButtonWidth {
		t.Fatalf("overflow viewport = %+v", bar.viewport.Rect())
	}
	for _, child := range bar.viewport.Children() {
		// Extreme host constraints clip the viewport, not the minimum tab width.
		if child.Rect().Width != 112 {
			t.Fatalf("overflow violated uniform minimum width: %+v", child.Rect())
		}
	}
	var moved []*TabPage
	v.ConnectMoved(func(page *TabPage, from, to int) {
		if from != 2 || to != 0 {
			t.Fatalf("move %d -> %d", from, to)
		}
		moved = append(moved, page)
	})
	v.MovePage(pages[2], 0)
	got := v.Pages()
	if got[0] != pages[2] || got[1] != pages[0] || got[2] != pages[1] || v.Current() != pages[1] || len(moved) != 1 {
		t.Fatalf("unexpected reorder: %v current=%p moved=%v", got, v.Current(), moved)
	}
	info := bar.Snapshot()
	if info.Role != RoleTabBar || len(info.Children) != 5 ||
		info.Children[1].Role != RoleTab || info.Children[1].Text != "Three" ||
		info.Children[2].Text != "One" || info.Children[3].Text != "Two" || !info.Children[3].Selected {
		t.Fatalf("semantic order or selection disagrees with pages: %+v", info)
	}
	bar.SetView(nil)
	if len(bar.items) != 0 {
		t.Fatal("unbinding retained tab rows")
	}
}

func TestTabBarReleaseOutsideViewportCancelsReorder(t *testing.T) {
	v := NewTabView()
	a, b := NewTabPage("One", nil), NewTabPage("Two", nil)
	v.AppendPage(a)
	v.AppendPage(b)
	bar := NewTabBar()
	bar.SetView(v)
	bar.SetReorderable(true)
	bar.Measure(layout.Loose(geometry.Size{Width: 300, Height: 40}))
	bar.Arrange(geometry.Rect(0, 0, 300, 40))
	bar.beginDrag(a, geometry.Point{X: 8, Y: 20})
	bar.endDrag(a, geometry.Point{X: 290, Y: 100})
	if pages := v.Pages(); pages[0] != a || pages[1] != b || bar.dragPage != nil {
		t.Fatalf("outside release reordered pages or kept drag active: %v", pages)
	}
}

func TestTabBarEdgeScrollKeepsDragActive(t *testing.T) {
	v := NewTabView()
	for _, title := range []string{"One", "Two", "Three", "Four"} {
		page := NewTabPage(title, nil)
		page.SetClosable(true)
		v.AppendPage(page)
	}
	bar := NewTabBar()
	bar.SetView(v)
	bar.SetReorderable(true)
	bar.Measure(layout.Loose(geometry.Size{Width: 110, Height: 40}))
	bar.Arrange(geometry.Rect(0, 0, 110, 40))
	if !bar.overflow || bar.scroll != 0 {
		t.Fatalf("initial overflow=%v scroll=%g", bar.overflow, bar.scroll)
	}
	page := v.Pages()[0]
	bar.beginDrag(page, geometry.Point{X: bar.viewport.Rect().X + 8, Y: 20})
	bar.pointer = geometry.Point{X: bar.viewport.Rect().X + bar.viewportWidth - 1, Y: 20}
	bar.advanceMotion(100 * time.Millisecond)
	forward := bar.scroll
	if forward <= 0 || bar.dragPage != page {
		t.Fatalf("right edge did not scroll during drag: scroll=%g", forward)
	}
	bar.pointer.X = bar.viewport.Rect().X + 1
	bar.advanceMotion(100 * time.Millisecond)
	if bar.scroll >= forward {
		t.Fatalf("left edge did not reverse scroll: before=%g after=%g", forward, bar.scroll)
	}
	bar.stopDrag()
}

// 输入、排序和关闭按钮。

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
	bar.ConnectDetachRequest(func(*TabDetachRequest, *bool) { requests++ })
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

// 分隔线绘制与状态。

type separatorPainter struct {
	gui.Painter
	rects []geometry.Rectangle
}

func (p *separatorPainter) FillRect(rect geometry.Rectangle, _ graphics.Brush) {
	p.rects = append(p.rects, rect)
}

type separatorApp struct {
	gui.Application
	sheet style.StyleSheet
}

func (*separatorApp) Typography() typography.Context { return nil }

func (a *separatorApp) StyleSheet() style.StyleSheet { return a.sheet }

func TestTabSeparatorStatesAndGeometry(t *testing.T) {
	old := gui.App
	app := &separatorApp{sheet: style.Sheet(append(gui.DefaultStyleRules(), widgetstyle.Rules()...)...)}
	gui.App = app
	defer func() { gui.App = old }()
	bar, view, pages := dragTestBar()
	check := func(want ...geometry.Rectangle) {
		t.Helper()
		p := new(separatorPainter)
		bar.viewport.Paint(p)
		if len(p.rects) != len(want) {
			t.Fatalf("separators=%v want=%v", p.rects, want)
		}
		for i, rect := range want {
			if p.rects[i] != rect {
				t.Fatalf("separator=%v want=%v", p.rects[i], rect)
			}
		}
	}
	check(geometry.Rect(205.5, 12, 1, 16)) // only B|C, never A|B
	if target := gui.Pick(bar, geometry.Point{X: 206, Y: 20}); target != bar.viewport {
		t.Fatal("separator changed picking")
	}
	if len(bar.Snapshot().Children) != 3 {
		t.Fatal("separator became a semantic child")
	}
	d := new(gui.EventDispatcher)
	host := &tabInputHost{root: bar}
	for _, x := range []float32{130, 230} {
		dispatchTabPointer(t, d, host, events.PointerMove, x, 20)
		check() // hovering either neighbor hides their separator
	}
	dispatchTabPointer(t, d, host, events.PointerMove, 350, 20)
	check(geometry.Rect(205.5, 12, 1, 16))
	view.SetCurrent(pages[1])
	check()
	view.SetCurrent(pages[2])
	check(geometry.Rect(101.5, 12, 1, 16))
	bar.beginDrag(pages[0], geometry.Point{X: 20, Y: 20})
	bar.updateDrag(pages[0], geometry.Point{X: 140, Y: 20})
	check()
	bar.cancelDrag()
	check() // also hidden during return animation
	bar.advanceMotion(100 * time.Millisecond)
	bar.advanceMotion(100 * time.Millisecond)
	check(geometry.Rect(101.5, 12, 1, 16))
	view.SetCurrent(pages[0])
	bar.Arrange(geometry.Rect(0, 0, 300, 40))
	bar.setScroll(56)
	bar.Arrange(bar.Rect())
	check(geometry.Rect(149.5, 12, 1, 16))
	bar.Arrange(geometry.Rect(0, 0, 300, 10))
	check(geometry.Rect(149.5, 0, 1, 10)) // short hosts cap the line height
	app.sheet = style.Sheet(style.Name("tab-bar").Part("separator").ForegroundColor(color.Transparent))
	check() // transparent style disables decoration without changing geometry
}

// 上下文菜单、目标与生命周期。

func TestTabContextMenuQueriesTargetInOrderAndMayReuseModel(t *testing.T) {
	view := NewTabView()
	a, b := NewTabPage("A", nil), NewTabPage("B", nil)
	view.AppendPage(a)
	view.AppendPage(b)
	bar := NewTabBar()
	bar.SetView(view)
	menu := gui.NewMenu()
	menu.Append("Close B", func() { view.RequestClose(b) })
	var order []int
	first := bar.ConnectContextMenu(func(page *TabPage, result *gui.MenuModel) {
		if page != b || *result != nil {
			t.Fatal("query did not start with the requested inactive page and nil model")
		}
		order = append(order, 1)
		*result = menu
	})
	second := bar.ConnectContextMenu(func(page *TabPage, result *gui.MenuModel) {
		if page != b || *result != menu {
			t.Fatal("later handler did not receive previous result")
		}
		order = append(order, 2)
		*result = nil
	})
	if bar.menuForPage(b) != nil || !slices.Equal(order, []int{1, 2}) || view.Current() != a {
		t.Fatal("query order, cancellation or selection changed")
	}
	second.Disconnect()
	if bar.menuForPage(b) != menu || bar.menuForPage(b) != menu {
		t.Fatal("a shared menu model could not be reused")
	}
	first.Disconnect()
	if bar.menuForPage(b) != nil {
		t.Fatal("disconnected handler remained active")
	}
}

func TestTabContextMenuBarsSharingViewHaveIndependentQueries(t *testing.T) {
	bar, view, pages := dragTestBar()
	other := NewTabBar()
	other.SetView(view)
	first, second := gui.NewMenu(), gui.NewMenu()
	first.Append("first", nil)
	second.Append("second", nil)
	menus := []gui.MenuModel{first, second}
	calls := [2]int{}
	bar.ConnectContextMenu(func(page *TabPage, result *gui.MenuModel) {
		if page != pages[1] {
			t.Fatal("first bar queried the wrong page")
		}
		calls[0]++
		*result = menus[0]
	})
	other.ConnectContextMenu(func(page *TabPage, result *gui.MenuModel) {
		if page != pages[1] {
			t.Fatal("second bar queried the wrong page")
		}
		calls[1]++
		*result = menus[1]
	})
	if bar.menuForPage(pages[1]) != menus[0] || calls != [2]int{1, 0} {
		t.Fatal("first bar invoked or used another bar's provider")
	}
	if other.menuForPage(pages[1]) != menus[1] || calls != [2]int{1, 1} || view.Current() != pages[0] {
		t.Fatal("second bar shared another bar's menu or changed selection")
	}
	bar.SetView(nil)
	if bar.menuForPage(pages[1]) != nil || other.menuForPage(pages[1]) != menus[1] || calls != [2]int{1, 2} {
		t.Fatal("unbinding one bar invalidated another bar's provider")
	}
}

func TestTabContextMenuAndCloseRejectInvalidatedPages(t *testing.T) {
	for _, change := range []string{"remove", "transfer"} {
		t.Run(change, func(t *testing.T) {
			view, target := NewTabView(), NewTabView()
			page := NewTabPage("page", nil)
			page.SetClosable(true)
			view.AppendPage(page)
			bar := NewTabBar()
			bar.SetView(view)
			owner := gui.NewPopover(nil, nil)
			owner.SetWidget(view)
			defer owner.Destroy()
			invalidate := func() {
				switch change {
				case "remove":
					view.RemovePage(page)
				case "transfer":
					if err := view.TransferPage(page, target, 0); err != nil {
						t.Fatal(err)
					}
				}
			}
			later := 0
			bar.ConnectContextMenu(func(_ *TabPage, result *gui.MenuModel) {
				menu := gui.NewMenu()
				menu.Append("unused", nil)
				*result = menu
				invalidate()
			})
			bar.ConnectContextMenu(func(*TabPage, *gui.MenuModel) { later++ })
			if bar.menuForPage(page) != nil || later != 0 {
				t.Fatal("query returned a stale menu or invoked a handler after invalidation")
			}
			view.ConnectCloseRequest(func(*TabPage) { later++ })
			view.RequestClose(page)
			if later != 0 {
				t.Fatal("stale page sent a close request")
			}
		})
	}
}

func TestTabContextMenuPointerUsesActualTabNotCurrentOrBlank(t *testing.T) {
	bar, view, pages := dragTestBar()
	var queries []*TabPage
	bar.ConnectContextMenu(func(page *TabPage, _ *gui.MenuModel) { queries = append(queries, page) })
	closed := 0
	view.ConnectCloseRequest(func(*TabPage) { closed++ })
	d := new(gui.EventDispatcher)
	host := &tabInputHost{root: bar}
	for _, x := range []float32{130, 102, 350, 85} {
		if err := d.DispatchEvent(host, events.PointerEvent{
			EventType: events.PointerDown, Button: events.PointerButtonRight,
			Position: geometry.Point{X: x, Y: 20}, Buttons: events.PointerButtonRightDown,
		}); err != nil {
			t.Fatal(err)
		}
		if err := d.DispatchEvent(host, events.PointerEvent{
			EventType: events.PointerUp, Button: events.PointerButtonRight,
			Position: geometry.Point{X: x, Y: 20},
		}); err != nil {
			t.Fatal(err)
		}
	}
	// B body, gaps/blank ignored, then A's visible close button.
	if !slices.Equal(queries, []*TabPage{pages[1], pages[0]}) || view.Current() != pages[0] ||
		closed != 0 || bar.dragPage != nil || bar.contextMenu != nil {
		t.Fatal("secondary click selected/closed/dragged a page or consumed blank caption space")
	}
	if info := bar.Snapshot(); len(info.Children) != 3 || info.Children[1].Text != "B" || info.Children[1].Selected {
		t.Fatal("context query changed tab snapshot")
	}
}

func TestTabContextMenuPresentationFailureAndRebinding(t *testing.T) {
	bar, _, pages := dragTestBar()
	menu := gui.NewMenu()
	menu.Append("action", nil)
	bar.ConnectContextMenu(func(_ *TabPage, result *gui.MenuModel) { *result = menu })
	errors := 0
	bar.ConnectContextMenuError(func(err error) {
		if err == nil {
			t.Fatal("nil presentation error")
		}
		errors++
	})
	// An unmounted anchor cannot create a native popup; no desktop is involved.
	bar.showContextMenu(pages[1], geometry.Point{X: 130, Y: 20})
	if errors != 1 || bar.menuPage != nil || bar.contextMenu.Visible() || bar.contextMenu.Menu() != nil {
		t.Fatal("presentation failure retained page/model or was not reported once")
	}
	bar.ConnectContextMenu(func(*TabPage, *gui.MenuModel) { bar.SetView(nil) })
	later := 0
	bar.ConnectContextMenu(func(*TabPage, *gui.MenuModel) { later++ })
	bar.showContextMenu(pages[1], geometry.Point{X: 130, Y: 20})
	if errors != 1 || bar.View() != nil || bar.menuPage != nil || later != 0 {
		t.Fatal("query continued after callback unbound its bar")
	}
}

func TestTabContextMenuEmptyModelAndDragDoNotCreatePopup(t *testing.T) {
	bar, _, pages := dragTestBar()
	calls := 0
	bar.ConnectContextMenu(func(_ *TabPage, result *gui.MenuModel) {
		calls++
		*result = gui.NewMenu()
	})
	bar.showContextMenu(pages[0], geometry.Point{})
	bar.beginDrag(pages[0], geometry.Point{X: 20, Y: 20})
	bar.showContextMenu(pages[0], geometry.Point{})
	if calls != 1 || bar.contextMenu != nil {
		t.Fatal("empty model created popup or active drag started a query")
	}
}

func TestTabContextMenuNestedQuerySupersedesOuterQuery(t *testing.T) {
	bar, _, pages := dragTestBar()
	queries, failures := 0, 0
	bar.ConnectContextMenu(func(page *TabPage, result *gui.MenuModel) {
		queries++
		menu := gui.NewMenu()
		menu.Append(page.Title(), nil)
		*result = menu
		if page == pages[0] {
			bar.showContextMenu(pages[1], geometry.Point{})
		}
	})
	bar.ConnectContextMenuError(func(error) { failures++ })
	bar.showContextMenu(pages[0], geometry.Point{})
	if queries != 2 || failures != 1 || bar.menuPage != nil || bar.contextMenu.Menu() != nil {
		t.Fatal("outer query replaced or presented after the nested query")
	}
}

func TestTabContextMenuCoordinatesAndKeyboardTarget(t *testing.T) {
	bar, view, pages := dragTestBar()
	bar.viewport.Arrange(geometry.Rect(24, 0, 200, 40))
	item := bar.items[pages[1]]
	item.Arrange(geometry.Rect(-30, 0, 100, 40))
	if got := item.contextMenuPosition(geometry.Point{X: 16, Y: 12}); got != (geometry.Point{X: 10, Y: 12}) {
		t.Fatalf("scrolled tab-local point not converted to bar DIP: %v", got)
	}
	owner := gui.NewPopover(nil, nil)
	owner.SetWidget(bar)
	defer owner.Destroy()
	host := owner.(gui.EventTarget)
	host.SetFocusedWidget(item.body)
	var requested *TabPage
	bar.ConnectContextMenu(func(page *TabPage, _ *gui.MenuModel) { requested = page })
	d := new(gui.EventDispatcher)
	if err := d.DispatchEvent(host, events.FocusEvent{Focused: true}); err != nil {
		t.Fatal(err)
	}
	if err := d.DispatchEvent(host, events.KeyEvent{EventType: events.KeyDown, Key: events.KeyF10, Modifiers: events.ModifierShift}); err != nil {
		t.Fatal(err)
	}
	if requested != pages[1] || view.Current() != pages[0] {
		t.Fatal("keyboard menu used selection instead of focused tab")
	}
}

func TestTabContextMenuQueryCannotOutliveBarMount(t *testing.T) {
	bar, _, pages := dragTestBar()
	owner := gui.NewPopover(nil, nil)
	owner.SetWidget(bar)
	defer owner.Destroy()
	bar.ConnectContextMenu(func(_ *TabPage, result *gui.MenuModel) {
		menu := gui.NewMenu()
		menu.Append("unused", nil)
		*result = menu
		owner.SetWidget(nil)
	})
	later := 0
	bar.ConnectContextMenu(func(*TabPage, *gui.MenuModel) { later++ })
	failures := 0
	bar.ConnectContextMenuError(func(error) { failures++ })
	bar.showContextMenu(pages[0], geometry.Point{})
	if bar.Root() != nil || bar.contextMenu != nil || failures != 0 || later != 0 {
		t.Fatal("unmounted bar continued the old query")
	}
}
