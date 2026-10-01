package widgets

import (
	"slices"
	"testing"
	"time"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
)

func TestTabViewSelectionRemovalAndCloseRequest(t *testing.T) {
	v := NewTabView()
	a, b, c := NewTabPage("A", gui.NewLabel("a")), NewTabPage("B", gui.NewLabel("b")), NewTabPage("C", gui.NewLabel("c"))
	var selected []*TabPage
	v.ConnectCurrent(func(page *TabPage) { selected = append(selected, page) })
	v.AppendPage(a)
	v.AppendPage(b)
	v.AppendPage(c)
	if v.Current() != a || !a.Visible() || b.Visible() || c.Visible() {
		t.Fatal("first page must be the sole visible selection")
	}
	v.SetCurrent(b)
	if v.Current() != b || a.Visible() || !b.Visible() || !b.Child().Visible() {
		t.Fatal("selection must hide only the inactive page container")
	}
	requests := 0
	v.ConnectCloseRequest(func(page *TabPage) {
		if page != b {
			t.Fatalf("requested %p, want B", page)
		}
		requests++
	})
	v.RequestClose(b)
	if requests != 0 || v.Current() != b {
		t.Fatal("non-closable page sent a request or changed selection")
	}
	b.SetClosable(true)
	v.RequestClose(b)
	if requests != 1 || len(v.Pages()) != 3 {
		t.Fatal("close request should not remove the page")
	}
	v.RemovePage(b)
	if v.Current() != c || b.Parent() != nil || b.Child() == nil || !b.Visible() {
		t.Fatal("removing current must select right neighbor and detach intact page")
	}
	v.RemovePage(c)
	if v.Current() != a {
		t.Fatal("last page removal should select left neighbor")
	}
	v.RemovePage(a)
	if v.Current() != nil || len(v.Pages()) != 0 {
		t.Fatal("last removal should leave empty view")
	}
	if len(selected) != 5 || selected[0] != a || selected[1] != b || selected[2] != c || selected[3] != a || selected[4] != nil {
		t.Fatalf("unexpected current transitions: %v", selected)
	}
}

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

func TestTabPageIndicesMapToRelativeSiblingOrder(t *testing.T) {
	v := NewTabView()
	a, b, c, d := NewTabPage("A", nil), NewTabPage("B", nil), NewTabPage("C", nil), NewTabPage("D", nil)
	v.AppendPage(a)
	v.AppendPage(c)
	v.InsertPage(1, b)
	v.InsertPage(0, d)
	check := func(want ...*TabPage) {
		t.Helper()
		if !slices.Equal(v.Pages(), want) {
			t.Fatalf("page order=%v want=%v", v.Pages(), want)
		}
		info := v.Snapshot()
		if string(info.Role) != "tabview" || len(info.Children) != len(want) {
			t.Fatalf("snapshot=%+v", info)
		}
		for i, p := range want {
			child := info.Children[i]
			if string(child.Role) != "tabpanel" || child.Text != p.Title() || child.Selected != (p == a) {
				t.Fatalf("page snapshot=%+v", child)
			}
		}
	}
	check(d, a, b, c)
	v.MovePage(a, 3) // forward target index must account for removal
	check(d, b, c, a)
	v.MovePage(c, 0)
	check(c, d, b, a)
	v.MovePage(d, 2)
	check(c, b, d, a)
	if v.Current() != a {
		t.Fatal("reorder changed selected page")
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
