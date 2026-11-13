package ui

import (
	"slices"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	baseui "github.com/golang-gui/goui/ui"
	"github.com/golang-gui/goui/widgets"
)

type treeAppearanceViewAPI interface {
	RowStyle(func(widgets.TreeRow, string) string) *TreeViewView[string]
	RowContentDepth(func(widgets.TreeRow, string) (int, bool)) *TreeViewView[string]
}

// 本组绑定替身不执行通用ViewBase修饰器，独立标签在自身Update设置自己的样式。
type treeOwnStyleLabel struct{ *baseui.LabelView }

func (v *treeOwnStyleLabel) Update(ctx baseui.BuildContext, widget gui.Widget) {
	v.LabelView.Update(ctx, widget)
	widget.SetStyleName("own-label")
}

func styledTreeView(t *testing.T, v *TreeViewView[string], name func(widgets.TreeRow, string) string, depth func(widgets.TreeRow, string) (int, bool)) *TreeViewView[string] {
	t.Helper()
	api, ok := any(v).(treeAppearanceViewAPI)
	if !ok {
		t.Fatal("声明式TreeView缺少RowStyle／RowContentDepth")
	}
	api.RowStyle(name)
	api.RowContentDepth(depth)
	return v
}

func treeOuterRowAt(t *testing.T, tree *widgets.TreeView, y float32) gui.Widget {
	t.Helper()
	w := gui.Pick(tree, geometry.Point{X: 80, Y: y})
	for w != nil && w.Snapshot().Role != widgets.RoleTreeItem {
		w = w.Parent()
	}
	if w == nil {
		t.Fatalf("y=%g没有树行", y)
	}
	return w
}

func TestTreeRowAppearanceAdapterLatestCallbacksAndOmission(t *testing.T) {
	m := newTreeBindingModel()
	c := new(treeBindingContext)
	item := func(_ widgets.TreeRow, text string) baseui.View { return &treeOwnStyleLabel{baseui.Label(text)} }
	oldCalls, newCalls, depths := 0, 0, 0
	v := styledTreeView(t, TreeView(m, item).Expanded([]string{"root"}), func(row widgets.TreeRow, text string) string {
		oldCalls++
		return "old-" + text
	}, func(row widgets.TreeRow, text string) (int, bool) {
		depths++
		if row.ID == "child" && (row.Depth != 1 || text != "Child") {
			t.Error("深度回调没有获得当前绑定快照和数据")
		}
		return 0, true
	})
	tree := v.Mount(c).(*widgets.TreeView)
	defer tree.SetModel(nil)
	defer func() { v.Unmount(c, tree) }()
	v.Update(c, tree)
	layout := func(offset float32) {
		tree.Arrange(geometry.Rect(0, 0, 200, 56))
		tree.LayoutVisible(geometry.Size{Width: 200, Height: 56}, geometry.Point{Y: offset})
	}
	layout(0)
	row := treeOuterRowAt(t, tree, 42)
	delegate := tree.Delegate()
	if row.StyleName() != "old-Child" || oldCalls != 2 || depths != 2 {
		t.Fatal("首次绑定未应用外层样式或深度回调", row.StyleName(), oldCalls, depths)
	}
	for _, label := range c.widgets {
		if label.StyleName() != "own-label" {
			t.Fatal("外层行样式泄漏到声明式内容")
		}
	}
	tree.SetCurrent("child")
	tree.SetSelection([]string{"child"})
	layout(0)
	before := oldCalls
	v = styledTreeView(t, TreeView(m, item), func(row widgets.TreeRow, text string) string {
		newCalls++
		if row.ID == "child" && (!row.Selected || !row.Current) {
			t.Error("样式回调的Selected／Current快照过期")
		}
		return "new-" + text
	}, nil)
	v.Update(c, tree)
	layout(0)
	if treeOuterRowAt(t, tree, 42) != row || row.StyleName() != "new-Child" || oldCalls != before || newCalls != 2 || tree.Delegate() != delegate || m.connections != 1 {
		t.Fatal("重建未使用最新回调，或替换了行壳／委托／模型订阅")
	}
	for _, id := range []string{"x", "y", "z"} {
		m.Append("", id, id)
	}
	layout(84)
	for _, y := range []float32{14, 42} {
		w := treeOuterRowAt(t, tree, y)
		if w.StyleName() == "new-Child" || w.StyleName() == "old-Child" {
			t.Fatal("离屏回收保留了原节点的外层样式")
		}
	}
	v = TreeView(m, item)
	v.Update(c, tree)
	layout(0)
	if treeOuterRowAt(t, tree, 42).StyleName() != "" || tree.Current() != "child" || !slices.Equal(tree.Selection(), []string{"child"}) || !tree.Expanded("root") {
		t.Fatal("省略未恢复默认外观，或改变了用户状态")
	}
}

type treeStyleLookupModel struct {
	*treeBindingModel
	onItem func()
}

func (m *treeStyleLookupModel) Item(id string) string {
	if fn := m.onItem; fn != nil {
		m.onItem = nil
		fn()
	}
	return m.treeBindingModel.Item(id)
}

func TestTreeRowAppearanceAdapterStopsAfterLookupUnmounts(t *testing.T) {
	for _, depth := range []bool{false, true} {
		t.Run(map[bool]string{false: "样式", true: "深度"}[depth], func(t *testing.T) {
			m := &treeStyleLookupModel{treeBindingModel: newTreeBindingModel()}
			c := new(treeBindingContext)
			calls := 0
			var name func(widgets.TreeRow, string) string
			var origin func(widgets.TreeRow, string) (int, bool)
			if depth {
				origin = func(widgets.TreeRow, string) (int, bool) { calls++; return 9, true }
			} else {
				name = func(widgets.TreeRow, string) string { calls++; return "stale" }
			}
			v := styledTreeView(t, TreeView[string](m, nil), name, origin)
			tree := v.Mount(c).(*widgets.TreeView)
			defer tree.SetModel(nil)
			v.Update(c, tree)
			m.onItem = func() { v.Unmount(c, tree) }
			tree.Arrange(geometry.Rect(0, 0, 200, 56))
			tree.LayoutVisible(geometry.Size{Width: 200, Height: 56}, geometry.Point{})
			if calls != 0 {
				t.Fatal("读取模型卸载声明式宿主后仍调用旧外观回调")
			}
		})
	}
}
