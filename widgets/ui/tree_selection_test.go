package ui

import (
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	baseui "github.com/golang-gui/goui/ui"
	"github.com/golang-gui/goui/widgets"
)

func TestTreeSelectionAdjacencyReachesDeclarativeCallbacks(t *testing.T) {
	m := newTreeBindingModel()
	ctx := new(treeBindingContext)
	built, styled, decorated := map[string]widgets.TreeRow{}, map[string]widgets.TreeRow{}, map[string]widgets.TreeRow{}
	v := TreeView(m, func(row widgets.TreeRow, title string) baseui.View {
		built[row.ID] = row
		return baseui.Label(title)
	}).Expanded([]string{"root"}).SelectionMode(widgets.SelectionMultiple).Selection([]string{"root", "child"}).
		RowStyle(func(row widgets.TreeRow, _ string) string {
			styled[row.ID] = row
			if row.NextSelected {
				return "joined"
			}
			return "single"
		}).RowContentDepth(func(row widgets.TreeRow, _ string) (int, bool) { decorated[row.ID] = row; return 0, false })
	tree := v.Mount(ctx).(*widgets.TreeView)
	defer tree.SetModel(nil)
	defer v.Unmount(ctx, tree)
	v.Update(ctx, tree)
	layout := func() {
		tree.Arrange(geometry.Rect(0, 0, 200, 56))
		tree.LayoutVisible(geometry.Size{Width: 200, Height: 56}, geometry.Point{})
	}
	layout()
	original := built["root"]
	root := treeOuterRowAt(t, tree, 14)
	delegate := tree.Delegate()
	if original.PreviousSelected || !original.NextSelected || !built["child"].PreviousSelected || built["child"].NextSelected {
		t.Fatal("首次声明没有获得相邻选中信息", built)
	}
	tree.SetSelection([]string{"root"})
	layout()
	for _, snapshots := range []map[string]widgets.TreeRow{built, styled, decorated} {
		if snapshots["root"].NextSelected || !snapshots["root"].Selected || snapshots["child"].Selected || !snapshots["child"].PreviousSelected {
			t.Fatal("自身Selected未变时邻接快照没有更新", snapshots)
		}
	}
	if !original.NextSelected || treeOuterRowAt(t, tree, 14) != root || root.StyleName() != "single" || tree.Delegate() != delegate || tree.Model() != m {
		t.Fatal("快照并非独立值，或邻接变化替换了声明式身份")
	}
	if info := root.Snapshot(); info.Role != widgets.RoleTreeItem || !info.Selected || info.Focused {
		t.Fatal("样式回调改变了真实行语义", info)
	}
}
