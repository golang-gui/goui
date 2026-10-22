package widgets

import (
	"slices"
	"testing"

	"github.com/golang-gui/goui/gui"
)

// 页面管理与关闭请求。

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

// 关闭回调安全。

func TestTabCloseRequestCallbacksCannotUseRemovedPage(t *testing.T) {
	view := NewTabView()
	page := NewTabPage("page", nil)
	page.SetClosable(true)
	view.AppendPage(page)
	view.ConnectCloseRequest(view.RemovePage)
	later := 0
	view.ConnectCloseRequest(func(*TabPage) { later++ })
	view.RequestClose(page)
	if later != 0 || len(view.Pages()) != 0 {
		t.Fatal("later handler used removed page")
	}
}
