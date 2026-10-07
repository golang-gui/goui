package main

import (
	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/style"
	"github.com/golang-gui/goui/ui"
)

// 示例自己的 Bin：只画自身背景，不向孩子传样式，不遍历绘制子树。
type surfaceWidget struct {
	gui.WidgetBase
	child gui.Widget
}

func (w *surfaceWidget) Child() gui.Widget { return w.child }
func (w *surfaceWidget) SetChild(child gui.Widget) {
	if child == w.child {
		return
	}
	if w.child != nil {
		w.RemoveChild(w.child)
	}
	w.child = child
	if child != nil {
		w.WidgetBase.AddChild(w, child)
	}
}
func (w *surfaceWidget) Snapshot() gui.WidgetInfo {
	info := w.WidgetBase.Snapshot()
	info.Role = gui.RoleBox
	return info
}
func (w *surfaceWidget) Paint(p gui.Painter) {
	s := gui.ResolveStyle(w.StyleName(), style.PartDefault, style.Normal)
	r := geometry.Rectangle{Size: w.Rect().Size}
	radius, _ := s.Radius()
	if c, ok := s.BackgroundColor(); ok && c != nil {
		p.FillRoundRect(r, radius, graphics.ColorOf(c))
	}
	width, _ := s.BorderWidth()
	if c, ok := s.BorderColor(); ok && c != nil && width > 0 {
		p.DrawRoundRect(r.Inset(width/2), max(0, radius-width/2), width, graphics.ColorOf(c))
	}
}

type surfaceView struct {
	ui.ViewBase[surfaceView]
	child ui.View
	after func()
}

func surface(child ui.View) *surfaceView { v := &surfaceView{child: child}; v.Self = v; return v }
func (v *surfaceView) Build() ui.View    { return v }
func (v *surfaceView) Mount(ui.BuildContext) gui.Widget {
	w := new(surfaceWidget)
	w.SetLayoutManager(layout.NewFillLayout())
	return w
}
func (v *surfaceView) Update(ctx ui.BuildContext, w gui.Widget) {
	ctx.UpdateChild(w.(*surfaceWidget), v.child)
	if v.after != nil {
		ctx.AfterUpdate(v.after)
	}
}
func (*surfaceView) Unmount(ui.BuildContext, gui.Widget) {}
func center(available, size geometry.Size, p *geometry.Point) {
	*p = geometry.Point{X: max(0, (available.Width-size.Width)/2), Y: max(0, (available.Height-size.Height)/2)}
}
