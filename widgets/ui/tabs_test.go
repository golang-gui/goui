package ui

import (
	"testing"

	"github.com/golang-gui/goui/gui"
	baseui "github.com/golang-gui/goui/ui"
	"github.com/golang-gui/goui/widgets"
)

type tabContext struct {
	state    any
	children map[baseui.Bin]gui.Widget
	after    []func()
	lists    map[baseui.Container][]*tabTestNode
}

type tabTestNode struct {
	id      string
	widget  gui.Widget
	view    *TabPageView
	context *tabContext
}

func TestTabBarWidthRangeReconcilesAndResets(t *testing.T) {
	c := new(tabContext)
	v := TabBar("documents").TabWidthRange(120, 240)
	bar := v.Mount(c).(*widgets.TabBar)
	v.Update(c, bar)
	if minimum, maximum := bar.TabWidthRange(); minimum != 120 || maximum != 240 {
		t.Fatalf("custom range not applied: %g..%g", minimum, maximum)
	}
	v = TabBar("documents").TabWidthRange(180, 180)
	v.Update(c, bar)
	if minimum, maximum := bar.TabWidthRange(); minimum != 180 || maximum != 180 {
		t.Fatal("fixed-width update not applied")
	}
	v = TabBar("documents")
	v.Update(c, bar)
	if minimum, maximum := bar.TabWidthRange(); minimum != 112 || maximum != 240 {
		t.Fatal("omission retained stale width range")
	}
}

func (c *tabContext) State() any                       { return c.state }
func (c *tabContext) Coordinator() *baseui.Coordinator { return nil }
func (c *tabContext) SetState(v any)                   { c.state = v }
func (c *tabContext) AfterUpdate(fn func())            { c.after = append(c.after, fn) }
func (c *tabContext) UpdateChild(target baseui.Bin, child baseui.View) gui.Widget {
	if c.children == nil {
		c.children = make(map[baseui.Bin]gui.Widget)
	}
	if child == nil {
		delete(c.children, target)
		target.SetChild(nil)
		return nil
	}
	widget := c.children[target]
	if widget == nil {
		widget = gui.NewLabel("mounted")
		c.children[target] = widget
		target.SetChild(widget)
	}
	return widget
}

// This adapter-level double executes page bindings; generic reconciliation and
// node transfer use the real root in ui's tests, not this simplified double.
func (c *tabContext) UpdateChildren(target baseui.Container, views []baseui.View) []gui.Widget {
	if c.lists == nil {
		c.lists = make(map[baseui.Container][]*tabTestNode)
	}
	old := c.lists[target]
	var next []*tabTestNode
	result := make([]gui.Widget, len(views))
	for i, declaration := range views {
		view := declaration.(*TabPageView)
		var n *tabTestNode
		for _, candidate := range old {
			if candidate.id == view.id {
				n = candidate
				break
			}
		}
		if n == nil {
			n = &tabTestNode{id: view.id, context: new(tabContext)}
			n.widget = view.Mount(n.context)
			n.widget.SetID(view.id)
			target.AddChild(n.widget)
		}
		n.view = view
		view.Update(n.context, n.widget)
		next = append(next, n)
		result[i] = n.widget
	}
	for _, n := range old {
		found := false
		for _, live := range next {
			if live == n {
				found = true
			}
		}
		if !found {
			n.view.Unmount(n.context, n.widget)
			target.RemoveChild(n.widget)
		}
	}
	c.lists[target] = next
	var sibling gui.Widget
	for i := len(next) - 1; i >= 0; i-- {
		target.MoveChildBefore(next[i].widget, sibling)
		sibling = next[i].widget
	}
	return result
}

func TestDeclarativeTabViewRejectsUncoordinatedTransfersInEitherDirection(t *testing.T) {
	ctx := new(tabContext)
	declaration := TabView(TabPage("page", baseui.Label("retained")))
	view := declaration.Mount(ctx).(*widgets.TabView)
	declaration.Update(ctx, view)
	page := view.Pages()[0]
	child := page.Child()
	plain := widgets.NewTabView()
	if view.TransferPage(page, plain, 0) == nil || page.Parent() != view || page.Child() != child {
		t.Fatal("UI page escaped without coordinated ownership")
	}
	other := widgets.NewTabPage("plain", nil)
	plain.AppendPage(other)
	if plain.TransferPage(other, view, 0) == nil || other.Parent() != plain {
		t.Fatal("imperative page entered declarative target without ownership")
	}
	declaration.Unmount(ctx, view)
}

func TestTabViewIDKeepsPageAndChildAcrossReorder(t *testing.T) {
	c := new(tabContext)
	v := TabView(TabPage("a", baseui.Label("A")).Title("Alpha"), TabPage("b", baseui.Label("B")).Title("Beta").Closable(true))
	view := v.Mount(c).(*widgets.TabView)
	v.Update(c, view)
	first := view.Pages()
	childA, childB := first[0].Child(), first[1].Child()
	if childA == nil || childB == nil {
		t.Fatal("pages were not mounted")
	}
	selected, closed, moved := "", "", ""
	v = TabView(TabPage("b", baseui.Label("B2")).Title("Beta2").Closable(true), TabPage("a", baseui.Label("A2")).Title("Alpha2")).
		OnCurrent(func(key string) { selected = key }).
		OnCloseRequest(func(key string) { closed = key }).
		OnMoved(func(key string, _, _ int) { moved = key })
	v.Update(c, view)
	second := view.Pages()
	if second[0] != first[1] || second[1] != first[0] || second[0].Child() != childB || second[1].Child() != childA {
		t.Fatal("reorder replaced an identified page or its content")
	}
	if second[0].Title() != "Beta2" || second[1].Title() != "Alpha2" {
		t.Fatal("reused pages did not receive metadata updates")
	}
	view.SetCurrent(second[0])
	view.RequestClose(second[0])
	view.MovePage(second[0], 1)
	if selected != "b" || closed != "b" || moved != "b" {
		t.Fatalf("callbacks used stale IDs: current=%q close=%q move=%q", selected, closed, moved)
	}
	v.Unmount(c, view)
}

func TestTabViewDuplicateIDRejectsBeforeMutation(t *testing.T) {
	c := new(tabContext)
	v := TabView(TabPage("a", baseui.Label("A")))
	view := v.Mount(c).(*widgets.TabView)
	v.Update(c, view)
	page := view.Pages()[0]
	v = TabView(TabPage("a", baseui.Label("first")), TabPage("a", baseui.Label("second")))
	defer func() {
		if recover() == nil {
			t.Fatal("duplicate ID did not panic")
		}
		if len(view.Pages()) != 1 || view.Pages()[0] != page || page.Title() != "" {
			t.Fatal("duplicate ID changed the mounted view")
		}
	}()
	v.Update(c, view)
}
