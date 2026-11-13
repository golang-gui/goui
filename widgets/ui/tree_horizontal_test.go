package ui

import (
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	baseui "github.com/golang-gui/goui/ui"
	"github.com/golang-gui/goui/widgets"
)

func TestTreeRowInsetsAdapterUpdatesAndOmissionResets(t *testing.T) {
	m := newTreeBindingModel()
	ctx := new(treeBindingContext)
	item := func(_ widgets.TreeRow, title string) baseui.View { return baseui.Label(title) }
	v := TreeView(m, item).Expanded([]string{"root"}).RowInsets(12, 18)
	tree := v.Mount(ctx).(*widgets.TreeView)
	defer tree.SetModel(nil)
	defer func() { v.Unmount(ctx, tree) }()
	v.Update(ctx, tree)
	layout := func() {
		tree.Arrange(geometry.Rect(0, 0, 200, 56))
		tree.LayoutVisible(geometry.Size{Width: 200, Height: 56}, geometry.Point{X: 40})
	}
	layout()
	row := treeOuterRowAt(t, tree, 42)
	if row.Rect() != (geometry.Rect(12, 28, 170, 28)) {
		t.Fatal("声明内缩没有应用到实际行", row.Rect())
	}
	tree.SetCurrent("child")
	tree.SetSelection([]string{"child"})
	for _, insets := range [][2]float32{{20, 5}, {0, 0}} {
		v = TreeView(m, item)
		if insets != [2]float32{} {
			v.RowInsets(insets[0], insets[1])
		}
		v.Update(ctx, tree)
		layout()
		left, right := tree.RowInsets()
		if left != insets[0] || right != insets[1] || treeOuterRowAt(t, tree, 42) != row ||
			row.Rect() != (geometry.Rect(left, 28, 200-left-right, 28)) || tree.Current() != "child" || !tree.Expanded("root") {
			t.Fatal("更新／省略没有恢复几何，或重置了行身份与树状态", left, right, row.Rect())
		}
	}
}
