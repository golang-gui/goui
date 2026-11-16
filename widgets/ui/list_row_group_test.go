package ui

import (
	"testing"

	"github.com/golang-gui/goui/gui"
	baseui "github.com/golang-gui/goui/ui"
	"github.com/golang-gui/goui/widgets"
)

// groupRow 给声明加一个测试可读的键，等价于真实协调器使用的 ID 身份。
type groupRow struct {
	*ListRowView
	key string
}

func newGroupRow(key, title string) *groupRow {
	return &groupRow{ListRowView: ListRow(title), key: key}
}

// listRowGroupContext 只按键／位置协调 ListRow 子声明，用于验证分组视图的
// 挂载与子列表传递；通用协调算法由 ui 包使用真实 root 的用例覆盖。
type listRowGroupContext struct {
	tabContext
	rows   map[baseui.Container][]*listRowGroupNode
	mounts int
}

type listRowGroupNode struct {
	key     string
	view    *groupRow
	widget  gui.Widget
	context *tabContext
}

func (c *listRowGroupContext) UpdateChildren(target baseui.Container, views []baseui.View, apply ...func(int, gui.Widget)) []gui.Widget {
	if c.rows == nil {
		c.rows = make(map[baseui.Container][]*listRowGroupNode)
	}
	old := c.rows[target]
	result := make([]gui.Widget, len(views))
	var next []*listRowGroupNode
	for i, declaration := range views {
		view, ok := declaration.(*groupRow)
		if !ok {
			continue
		}
		var node *listRowGroupNode
		if view.key != "" {
			for _, candidate := range old {
				if candidate.key == view.key {
					node = candidate
					break
				}
			}
		} else if i < len(old) && old[i].key == "" {
			node = old[i]
		}
		if node == nil {
			node = &listRowGroupNode{key: view.key, context: new(tabContext)}
			node.widget = view.Mount(node.context)
			c.mounts++
			target.AddChild(node.widget)
		}
		node.view = view
		view.Update(node.context, node.widget)
		next = append(next, node)
		result[i] = node.widget
	}
	for _, node := range old {
		kept := false
		for _, live := range next {
			if live == node {
				kept = true
			}
		}
		if !kept {
			node.view.Unmount(node.context, node.widget)
			target.RemoveChild(node.widget)
		}
	}
	c.rows[target] = next
	var sibling gui.Widget
	for i := len(next) - 1; i >= 0; i-- {
		target.MoveChildBefore(next[i].widget, sibling)
		sibling = next[i].widget
	}
	if len(apply) != 0 && apply[0] != nil {
		for i, widget := range result {
			apply[0](i, widget)
		}
	}
	return result
}

func TestListRowGroupViewMountsWidgetsAndRetainsIdentity(t *testing.T) {
	c := new(listRowGroupContext)
	v := ListRowGroup(newGroupRow("updates", "Updates"), newGroupRow("version", "Version"))
	group := v.Mount(c).(*widgets.ListRowGroup)
	if group.StyleName() != "" || group.Focusable() {
		t.Fatal("分组不应自带样式名或焦点")
	}
	v.Update(c, group)
	rows := group.Children()
	if len(rows) != 2 || c.mounts != 2 {
		t.Fatalf("首次声明未挂载两行：%d", len(rows))
	}
	first, second := rows[0].(*widgets.ListRow), rows[1].(*widgets.ListRow)
	if first.Title() != "Updates" || second.Title() != "Version" || first.Parent() != group {
		t.Fatal("行内容或归属错误")
	}
	// 按声明键重排：实例保持不变，只调整顺序，不重新挂载。
	reordered := ListRowGroup(newGroupRow("version", "Version"), newGroupRow("updates", "Updates"))
	reordered.Update(c, group)
	if c.mounts != 2 {
		t.Fatalf("重排重新挂载了行：%d", c.mounts)
	}
	rows = group.Children()
	if len(rows) != 2 || rows[0] != second || rows[1] != first {
		t.Fatal("重排未保持行实例")
	}
	// 省略声明时移除对应行并解绑，不销毁调用方内容。
	removed := ListRowGroup(newGroupRow("version", "Version"))
	removed.Update(c, group)
	if rows = group.Children(); len(rows) != 1 || rows[0] != second || first.Parent() != nil || first.Destroyed() {
		t.Fatal("移除的声明没有被解绑")
	}
	// nil 声明是省略项，不产生空行。
	ListRowGroup(newGroupRow("version", "Version"), nil).Update(c, group)
	if len(group.Children()) != 1 || c.mounts != 2 {
		t.Fatal("nil 声明被当作空行挂载")
	}
	ListRowGroup().Update(c, group)
	if len(group.Children()) != 0 {
		t.Fatal("空声明没有清空行")
	}
	v.Unmount(c, group)
	if len(group.Children()) != 0 {
		t.Fatal("Unmount 改变了已挂载的行")
	}
}

func TestListRowGroupViewChildrenReplaceDeclaration(t *testing.T) {
	c := new(listRowGroupContext)
	v := ListRowGroup()
	group := v.Mount(c).(*widgets.ListRowGroup)
	v.Update(c, group)
	if len(group.Children()) != 0 {
		t.Fatal("空分组应无子行")
	}
	v.Children(newGroupRow("a", "A"), newGroupRow("b", "B"), newGroupRow("c", "C"))
	v.Update(c, group)
	if len(group.Children()) != 3 {
		t.Fatalf("Children 未替换声明：%d", len(group.Children()))
	}
	v.Children(newGroupRow("a", "A"))
	v.Update(c, group)
	if rows := group.Children(); len(rows) != 1 || rows[0].(*widgets.ListRow).Title() != "A" {
		t.Fatal("再次声明未收缩行列表")
	}
}
