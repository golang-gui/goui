package ui

import (
	"reflect"
	"slices"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/gui"
	baseui "github.com/golang-gui/goui/ui"
	"github.com/golang-gui/goui/widgets"
)

type treeBindingModel struct {
	*widgets.TreeStore[string]
	connections, deliveries int
}

func (m *treeBindingModel) ConnectItems(fn func()) signal.Handle {
	m.connections++
	return m.TreeStore.ConnectItems(func() { m.deliveries++; fn() })
}

func newTreeBindingModel() *treeBindingModel {
	m := &treeBindingModel{TreeStore: widgets.NewTreeStore[string]()}
	m.Append("", "root", "Root")
	m.Append("root", "child", "Child")
	return m
}

func TestTreeViewAdapterOnlyInstallsChangedModelPointers(t *testing.T) {
	a, b := newTreeBindingModel(), newTreeBindingModel()
	ctx := new(treeBindingContext)
	v := TreeView[string](a, func(_ widgets.TreeRow, text string) baseui.View { return baseui.Label(text) })
	tree := v.Mount(ctx).(*widgets.TreeView)
	defer tree.SetModel(nil)
	defer v.Unmount(ctx, tree)
	v.Update(ctx, tree)
	if a.connections != 1 {
		t.Fatal("first declaration installed more than once")
	}
	delegate := tree.Delegate()
	tree.SetExpanded("root", true)
	tree.SetSelection([]string{"child"})
	tree.SetCurrent("child")
	tree.LayoutVisible(geometry.Size{Width: 200, Height: 100}, geometry.Point{})
	v = TreeView[string](a, func(_ widgets.TreeRow, text string) baseui.View { return baseui.Label("new:" + text) })
	v.Update(ctx, tree)
	if a.connections != 1 || tree.Delegate() != delegate || !tree.Expanded("root") || tree.Current() != "child" || !slices.Equal(tree.Selection(), []string{"child"}) {
		t.Fatal("uncontrolled rebuild reinstalled model/delegate or reset user state")
	}
	a.Set("child", "Changed")
	if a.deliveries != 1 {
		t.Fatal("same-model rebuild leaked subscriptions")
	}
	// 新实例内容相同，也必须执行安装。先重置，再应用受控展开、选择和 Current。
	b.Set("child", "Changed")
	callbacks := 0
	v = TreeView[string](b, nil).Expanded([]string{"root"}).Selection([]string{"child"}).Current("child").OnSelection(func([]string) { callbacks++ })
	v.Update(ctx, tree)
	if b.connections != 1 || tree.Model() != b || tree.Delegate() != delegate || !tree.Expanded("root") || tree.Current() != "child" || !slices.Equal(tree.Selection(), []string{"child"}) || callbacks != 0 {
		t.Fatal("replacement did not apply controlled state in order, or sent update feedback")
	}
	a.Set("root", "old")
	b.Set("root", "new")
	if a.deliveries != 1 || b.deliveries != 1 {
		t.Fatal("replacement subscription ownership is incorrect")
	}
	v = TreeView[string](nil, nil)
	v.Update(ctx, tree)
	b.Set("root", "detached")
	if tree.Model() != nil || b.deliveries != 1 || len(tree.ExpandedIDs()) != 0 || len(tree.Selection()) != 0 || tree.Current() != "" {
		t.Fatal("nil declaration retained model or state")
	}
}

type treeBindingValue struct {
	*treeBindingModel
	payload any
}

func TestTreeViewValueModelsReinstallWithoutComparison(t *testing.T) {
	for _, payload := range []any{1, []int{1}, map[string]int{"x": 1}, func() {}} {
		m := treeBindingValue{newTreeBindingModel(), payload}
		ctx := new(treeBindingContext)
		v := TreeView[string](m, nil)
		tree := v.Mount(ctx).(*widgets.TreeView)
		v.Update(ctx, tree)
		if m.connections != 1 {
			t.Fatal("first declaration installed more than once")
		}
		tree.SetExpanded("root", true)
		tree.SetSelection([]string{"child"})
		tree.SetCurrent("child")
		v.Update(ctx, tree)
		if m.connections != 2 || tree.Expanded("root") || len(tree.Selection()) != 0 || tree.Current() != "" {
			t.Fatal("value model was treated as stable identity")
		}
		m.Set("root", "changed")
		if m.deliveries != 1 {
			t.Fatal("old subscription leaked")
		}
		v.Unmount(ctx, tree)
		tree.SetModel(nil)
	}
}

// 绑定级替身只负责普通行的 Mount/Update/Unmount。通用协调、延迟完成和跨 Root
// 迁移由 ui/list_view_test.go 使用真实 root 验证，不在此重复实现协调算法。
type treeBindingContext struct {
	tabContext
	rows     map[baseui.Container]baseui.WidgetView
	widgets  map[baseui.Container]gui.Widget
	contexts map[baseui.Container]*tabContext
}

func (c *treeBindingContext) UpdateChildren(target baseui.Container, views []baseui.View) []gui.Widget {
	if c.rows == nil {
		c.rows = make(map[baseui.Container]baseui.WidgetView)
		c.widgets = make(map[baseui.Container]gui.Widget)
		c.contexts = make(map[baseui.Container]*tabContext)
	}
	var next baseui.WidgetView
	if len(views) > 0 && views[0] != nil {
		next = views[0].Build().(baseui.WidgetView)
	}
	old, widget := c.rows[target], c.widgets[target]
	if old != nil && (next == nil || reflect.TypeOf(old) != reflect.TypeOf(next)) {
		old.Unmount(c.contexts[target], widget)
		target.RemoveChild(widget)
		delete(c.rows, target)
		delete(c.widgets, target)
		delete(c.contexts, target)
		widget = nil
	}
	if next == nil {
		return nil
	}
	if widget == nil {
		c.contexts[target] = new(tabContext)
		widget = next.Mount(c.contexts[target])
		target.AddChild(widget)
	}
	next.Update(c.contexts[target], widget)
	c.rows[target], c.widgets[target] = next, widget
	return []gui.Widget{widget}
}
func TestTreeViewControlledUncontrolledAndCallbacks(t *testing.T) {
	m := widgets.NewTreeStore[string]()
	m.Append("", "root", "Root")
	m.Append("root", "child", "Child")
	c := new(treeBindingContext)
	calls := 0
	v := TreeView(m, func(_ widgets.TreeRow, title string) baseui.View { return baseui.Label(title) }).Expanded([]string{"root"}).Selection([]string{"child"}).Current("child").OnSelection(func([]string) { calls++ })
	tree := v.Mount(c).(*widgets.TreeView)
	v.Update(c, tree)
	if calls != 0 || !tree.Expanded("root") || tree.Current() != "child" || !slices.Equal(tree.Selection(), []string{"child"}) {
		t.Fatal("controlled update feedback or wrong application order")
	}
	tree.LayoutVisible(geometry.Size{Width: 200, Height: 100}, geometry.Point{})
	if len(c.widgets) != 2 {
		t.Fatal("rows were not built")
	}
	var label *gui.Label
	for _, widget := range c.widgets {
		if widget.(*gui.Label).Text() == "Child" {
			label = widget.(*gui.Label)
		}
	}
	if label == nil {
		t.Fatal("typed row data missing")
	}
	v = TreeView(m, func(_ widgets.TreeRow, title string) baseui.View { return baseui.Label("new:" + title) }).OnSelection(func([]string) { calls += 10 })
	v.Update(c, tree)
	tree.LayoutVisible(geometry.Size{Width: 200, Height: 100}, geometry.Point{})
	if label.Text() != "new:Child" || tree.Current() != "child" || !tree.Expanded("root") {
		t.Fatal("uncontrolled rebuild reset state or retained old builder")
	}
	tree.SetSelection([]string{"root"})
	if calls != 10 {
		t.Fatal("latest callback not used")
	}
	v = TreeView(m, nil).Expanded(nil).Selection(nil).Current("").Indentation(0)
	v.Update(c, tree)
	if tree.Current() != "" || len(tree.Selection()) != 0 || len(tree.ExpandedIDs()) != 0 || tree.Indentation() != 0 {
		t.Fatal("explicit zero values not applied")
	}
	v = TreeView(m, nil)
	v.Update(c, tree)
	if tree.Indentation() != 16 {
		t.Fatal("omission did not restore indentation default")
	}
	v.Unmount(c, tree)
	tree.SetSelection([]string{"root"})
	if calls != 10 {
		t.Fatal("unmounted binding retained callback")
	}
}
func TestTreeViewRowUnbindAndContentReplacement(t *testing.T) {
	m := widgets.NewTreeStore[int]()
	m.Append("", "a", 1)
	c := new(treeBindingContext)
	v := TreeView(m, func(widgets.TreeRow, int) baseui.View { return baseui.Label("old") })
	tree := v.Mount(c).(*widgets.TreeView)
	v.Update(c, tree)
	tree.LayoutVisible(geometry.Size{Width: 200, Height: 50}, geometry.Point{})
	var old gui.Widget
	for _, w := range c.widgets {
		old = w
	}
	v = TreeView(m, func(widgets.TreeRow, int) baseui.View { return baseui.Button("replacement") })
	v.Update(c, tree)
	tree.LayoutVisible(geometry.Size{Width: 200, Height: 50}, geometry.Point{})
	if old.Parent() != nil || len(c.widgets) != 1 {
		t.Fatal("row replacement retained old child")
	}
	m.Remove("a")
	if len(c.widgets) != 0 {
		t.Fatal("removed node retained coordinated content")
	}
	v.Unmount(c, tree)
}
