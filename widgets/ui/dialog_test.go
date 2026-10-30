package ui

import (
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/platform/events"
	baseui "github.com/golang-gui/goui/ui"
	"github.com/golang-gui/goui/widgets"
)

func TestDialogDeclarationReconcile(t *testing.T) {
	ctx := new(tabContext)
	first, latest := 0, 0
	v := Dialog(baseui.Label("body")).Padding(10).BackdropCaption(true).CloseOnOutside(true).OnCloseRequest(func() { first++ })
	d := v.Mount(ctx).(*widgets.Dialog)
	v.Update(ctx, d)
	child := d.Child()
	if child == nil || d.Padding() != geometry.UniformInsets(10) || !d.BackdropCaption() || !d.CloseOnOutside() {
		t.Fatal("声明属性未应用")
	}
	v = Dialog(baseui.Label("updated")).OnCloseRequest(func() { latest++ })
	v.Update(ctx, d)
	if d.Child() != child || d.Padding() != geometry.UniformInsets(24) || d.BackdropCaption() || d.CloseOnOutside() {
		t.Fatal("重建未复用内容或恢复默认值")
	}
	d.Arrange(geometry.Rect(0, 0, 500, 300))
	h := &checkInputHost{widget: d}
	dispatcher := new(gui.EventDispatcher)
	_ = dispatcher.DispatchEvent(h, events.KeyEvent{EventType: events.KeyDown, Key: events.KeyEscape})
	if first != 0 || latest != 1 || !d.Visible() {
		t.Fatal("关闭请求重复、使用旧回调或隐式关闭")
	}
	v.Unmount(ctx, d)
	_ = dispatcher.DispatchEvent(h, events.KeyEvent{EventType: events.KeyDown, Key: events.KeyEscape})
	if latest != 1 {
		t.Fatal("卸载后仍保留关闭连接")
	}
}
