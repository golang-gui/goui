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

type menuInputHost struct {
	root, focus gui.Widget
}

func (h *menuInputHost) Widget() gui.Widget                 { return h.root }
func (h *menuInputHost) FocusedWidget() gui.Widget          { return h.focus }
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
