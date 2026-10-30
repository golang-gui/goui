package ui

import (
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
)

// 查询均否决绘图；UI 测试只验证协调，不引入底层绘制后端依赖。
type drawingAreaQueryPainter struct{ gui.Painter }

func (drawingAreaQueryPainter) PixelScale() float32 { return 1 }

func TestDrawingAreaLatestCallbacksAndSingleQueryConnection(t *testing.T) {
	root := newRoot()
	defer root.unmountWindow()
	generation, calls, got, queries := 1, 0, 0, 0
	view := func() View {
		captured := generation
		return DrawingArea(func(Painter, geometry.Size) { calls++; got = captured }).ID("drawing").MinSize(40, 20).OnShouldDraw(func(v *bool) { queries += captured; *v = false })
	}
	a := root.update(view()).(*gui.DrawingArea)
	a.Arrange(geometry.Rect(0, 0, 40, 20))
	a.Paint(drawingAreaQueryPainter{})
	if calls != 0 || queries != 1 {
		t.Fatal("查询未否决或连接错误")
	}
	a.DrawFunc()(nil, geometry.Size{Width: 40, Height: 20})
	generation = 2
	if root.update(view()) != a {
		t.Fatal("重建替换了控件")
	}
	a.Paint(drawingAreaQueryPainter{})
	if calls != 1 || queries != 3 {
		t.Fatal("查询不是最新闭包或重复连接")
	}
	a.DrawFunc()(nil, geometry.Size{Width: 40, Height: 20})
	if calls != 2 || got != 2 {
		t.Fatal("稳定代理没有调用最新函数")
	}
}

func TestDrawingAreaUnmountClearsCallbacks(t *testing.T) {
	root := newRoot()
	called := 0
	a := root.update(DrawingArea(func(Painter, geometry.Size) { called++ }).OnShouldDraw(func(v *bool) { called += 100; *v = false })).(*gui.DrawingArea)
	a.Arrange(geometry.Rect(0, 0, 8, 5))
	root.unmountWindow()
	// 脱离 UI 后借用旧 GUI 对象，代理不得再访问已释放的声明闭包。
	a.DrawFunc()(nil, geometry.Size{Width: 8, Height: 5})
	a.ConnectShouldDraw(func(v *bool) { *v = false })
	a.Paint(drawingAreaQueryPainter{})
	if called != 0 {
		t.Fatal("卸载仍持有声明回调", called)
	}
}

func TestDrawingAreaDeclarationTransfersLatestState(t *testing.T) {
	a, b := transferRoots()
	defer a.unmountWindow()
	defer b.unmountWindow()
	source := a.update(VBox(DrawingArea(func(Painter, geometry.Size) {}).ID("moving"))).(*gui.LinearBox)
	target := b.update(VBox()).(*gui.LinearBox)
	moving := source.Children()[0].(*gui.DrawingArea)
	if err := (&Coordinator{owner: a.root}).TransferChild(moving, target, func() error { source.RemoveChild(moving); target.AddChild(moving); return nil }, nil); err != nil {
		t.Fatal(err)
	}
	called, queries := 0, 0
	b.update(VBox(DrawingArea(func(Painter, geometry.Size) { called++ }).ID("moving").OnShouldDraw(func(v *bool) { queries++; *v = false })))
	a.unmountWindow()
	moving.Arrange(geometry.Rect(0, 0, 8, 5))
	moving.Paint(drawingAreaQueryPainter{})
	moving.DrawFunc()(nil, geometry.Size{Width: 8, Height: 5})
	if called != 1 || queries != 1 || target.Children()[0] != moving {
		t.Fatal("移交后使用旧闭包、重复连接或更换控件")
	}
}
