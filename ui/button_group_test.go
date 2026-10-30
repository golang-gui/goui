package ui

import (
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"testing"
)

func TestButtonGroupCoordinatesChildrenAndPreservesIdentity(t *testing.T) {
	root := newRoot()
	t.Cleanup(root.unmountWindow)
	group := root.update(ButtonGroup(Button("First").ID("first"), MenuButton("Menu").Menu(MenuItem("Open", nil)))).(*gui.ButtonGroup)
	first, menu := group.Children()[0], group.Children()[1]
	if first.Parent() != group || menu.Parent() != group || group.Direction() != layout.DirectionHorizontal {
		t.Fatal("没有挂载实际容器")
	}
	root.update(ButtonGroup(Button("Updated").ID("first"), MenuButton("Menu").Menu(MenuItem("Open", nil))).Direction(layout.DirectionVertical))
	if group.Children()[0] != first || group.Children()[1] != menu || group.Direction() != layout.DirectionVertical {
		t.Fatal("更新替换了按钮或丢失方向")
	}
	if menu.(*gui.MenuButton).Group() != nil {
		t.Fatal("连排容器意外启用了菜单联动")
	}
	root.update(ButtonGroup())
	if len(group.Children()) != 0 || first.Parent() != nil || menu.Parent() != nil {
		t.Fatal("声明移除没有卸载子项")
	}
}
