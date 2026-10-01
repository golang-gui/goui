package ui

import (
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/events"
	baseui "github.com/golang-gui/goui/ui"
	"github.com/golang-gui/goui/widgets"
)

// 声明协调、身份与移交。

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

func (c *tabContext) State() any { return c.state }

func (c *tabContext) Coordinator() *baseui.Coordinator { return nil }

func (c *tabContext) SetState(v any) { c.state = v }

func (c *tabContext) AfterUpdate(fn func()) { c.after = append(c.after, fn) }

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

// 菜单构建与信号连接管理。

type menuInputHost struct {
	root, focus gui.Widget
}

func (h *menuInputHost) Widget() gui.Widget { return h.root }

func (h *menuInputHost) FocusedWidget() gui.Widget { return h.focus }

func (h *menuInputHost) SetFocusedWidget(w gui.Widget) bool { h.focus = w; return true }

func requestTabMenu(t *testing.T, bar *widgets.TabBar) {
	t.Helper()
	host := &menuInputHost{root: bar}
	d := new(gui.EventDispatcher)
	for _, kind := range []events.EventType{events.PointerDown, events.PointerUp} {
		if err := d.DispatchEvent(host, events.PointerEvent{
			EventType: kind, Button: events.PointerButtonRight, Position: geometry.Point{X: 20, Y: 20},
		}); err != nil {
			t.Fatal(err)
		}
	}
}

func TestTabBarContextMenuUsesLatestBuilderAndDisconnects(t *testing.T) {
	ctx := new(tabContext)
	builds, actions := 0, 0
	declaration := TabBar("documents").ContextMenu(func(id string) []*baseui.MenuItemView {
		builds++
		return []*baseui.MenuItemView{baseui.MenuItem("Old "+id, func() { actions++ })}
	})
	bar := declaration.Mount(ctx).(*widgets.TabBar)
	declaration.Update(ctx, bar)
	if builds != 0 || actions != 0 {
		t.Fatal("reconciliation executed menu builder or action")
	}
	view := widgets.NewTabView()
	page := widgets.NewTabPage("page", nil)
	page.SetID("page")
	view.AppendPage(page)
	bar.SetView(view)
	bar.SetTabWidthRange(100, 100)
	bar.Measure(layout.Loose(geometry.Size{Width: 200, Height: 40}))
	bar.Arrange(geometry.Rect(0, 0, 200, 40))
	var menu gui.MenuModel
	bar.ConnectContextMenu(func(_ *widgets.TabPage, result *gui.MenuModel) { menu = *result })
	requestTabMenu(t, bar)
	if builds != 1 || actions != 0 || menu.ItemAt(0).Label() != "Old page" {
		t.Fatal("first input did not build exactly one target-specific menu")
	}
	old := menu
	declaration = TabBar("documents").ContextMenu(func(id string) []*baseui.MenuItemView {
		builds++
		return []*baseui.MenuItemView{baseui.MenuItem("New "+id, func() { actions += 10 }), baseui.MenuSeparator()}
	})
	declaration.Update(ctx, bar)
	if builds != 1 {
		t.Fatal("update eagerly executed builder")
	}
	requestTabMenu(t, bar)
	if builds != 2 || menu == old || menu.ItemsCount() != 2 || menu.ItemAt(0).Label() != "New page" {
		t.Fatal("rebuild kept stale builder/menu or duplicated signal connections")
	}
	menu.ItemAt(0).Action()()
	if actions != 10 {
		t.Fatal("newly opened menu retained old action")
	}
	declaration = TabBar("documents")
	declaration.Update(ctx, bar)
	requestTabMenu(t, bar)
	if menu != nil || builds != 2 {
		t.Fatal("omitted menu builder remained active")
	}
	// Restore a builder, then unmount and rebind the imperative bar: the
	// disconnected declaration must not build another menu.
	declaration = TabBar("documents").ContextMenu(func(string) []*baseui.MenuItemView {
		builds++
		return []*baseui.MenuItemView{baseui.MenuItem("unmounted", nil)}
	})
	declaration.Update(ctx, bar)
	declaration.Unmount(ctx, bar)
	bar.SetView(view)
	bar.Arrange(geometry.Rect(0, 0, 200, 40))
	requestTabMenu(t, bar)
	if menu != nil || builds != 2 {
		t.Fatal("unmount retained declarative query connection")
	}
}

func TestTabBarContextMenuErrorBindingUpdatesAndDisconnects(t *testing.T) {
	ctx := new(tabContext)
	first, second := 0, 0
	declaration := TabBar("documents").OnContextMenuError(func(error) { first++ })
	bar := declaration.Mount(ctx).(*widgets.TabBar)
	declaration.Update(ctx, bar)
	view := widgets.NewTabView()
	view.AppendPage(widgets.NewTabPage("page", nil))
	bar.ConnectContextMenu(func(_ *widgets.TabPage, result *gui.MenuModel) {
		menu := gui.NewMenu()
		menu.Append("item", nil)
		*result = menu
	})
	bar.SetView(view)
	bar.SetTabWidthRange(100, 100)
	bar.Measure(layout.Loose(geometry.Size{Width: 200, Height: 40}))
	bar.Arrange(geometry.Rect(0, 0, 200, 40))
	requestTabMenu(t, bar) // unmounted anchor: deterministic error, no desktop
	declaration = TabBar("documents").OnContextMenuError(func(error) { second++ })
	declaration.Update(ctx, bar)
	requestTabMenu(t, bar)
	if first != 1 || second != 1 {
		t.Fatal("error handler did not update or was called more than once")
	}
	declaration.Unmount(ctx, bar)
	bar.SetView(view)
	bar.Arrange(geometry.Rect(0, 0, 200, 40))
	requestTabMenu(t, bar)
	if first != 1 || second != 1 {
		t.Fatal("unmounted error handler remained connected")
	}
}

func TestTabBarContextMenuDeclarationsAreIndependentForSharedView(t *testing.T) {
	view := widgets.NewTabView()
	page := widgets.NewTabPage("page", nil)
	page.SetID("page")
	view.AppendPage(page)
	var builds [2]int
	var bars [2]*widgets.TabBar
	var menus [2]gui.MenuModel
	for i := range bars {
		ctx := new(tabContext)
		declaration := TabBar("documents").ContextMenu(func(id string) []*baseui.MenuItemView {
			if id != "page" {
				t.Fatalf("unexpected page ID %q", id)
			}
			builds[i]++
			return []*baseui.MenuItemView{baseui.MenuItem([]string{"first", "second"}[i], nil)}
		})
		bars[i] = declaration.Mount(ctx).(*widgets.TabBar)
		declaration.Update(ctx, bars[i])
		bars[i].SetView(view)
		bars[i].SetTabWidthRange(100, 100)
		bars[i].Measure(layout.Loose(geometry.Size{Width: 200, Height: 40}))
		bars[i].Arrange(geometry.Rect(0, 0, 200, 40))
		bars[i].ConnectContextMenu(func(_ *widgets.TabPage, result *gui.MenuModel) { menus[i] = *result })
	}
	requestTabMenu(t, bars[0])
	if builds != [2]int{1, 0} || menus[0] == nil || menus[0].ItemAt(0).Label() != "first" || menus[1] != nil {
		t.Fatal("first bar used another declaration's menu")
	}
	requestTabMenu(t, bars[1])
	if builds != [2]int{1, 1} || menus[1] == nil || menus[1].ItemAt(0).Label() != "second" {
		t.Fatal("second bar used another declaration's menu")
	}
}
