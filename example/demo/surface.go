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

// 此例中的表单页仅纵向滚动，宽度跟随 viewport。普通 ScrollView 保留孩子
// 自然尺寸；本地适配器通过现有 Scrollable 协议向孩子提供有限宽度，让 Label
// 和 HWrap 真正换行。不修改框架的滚动或布局策略。
type flowWidget struct {
	surfaceWidget
	content geometry.Size
}

func (w *flowWidget) Measure(c layout.Constraint) layout.Measurement {
	width := c.Max.Width
	if width >= layout.Inf {
		width = 640
	}
	if w.child != nil {
		w.content = w.child.Measure(layout.Constraint{Min: geometry.Size{Width: width}, Max: geometry.Size{Width: width, Height: layout.Inf}}).Size
	}
	return layout.Measured(w.content).Constrain(c)
}
func (w *flowWidget) ContentSize() geometry.Size   { return w.content }
func (w *flowWidget) Arrange(r geometry.Rectangle) { w.WidgetBase.Arrange(r) }
func (w *flowWidget) LayoutVisible(viewport geometry.Size, offset geometry.Point) {
	if w.child == nil {
		return
	}
	w.content = w.child.Measure(layout.Constraint{Min: geometry.Size{Width: viewport.Width}, Max: geometry.Size{Width: viewport.Width, Height: layout.Inf}}).Size
	w.child.Arrange(geometry.Rect(-offset.X, -offset.Y, viewport.Width, max(viewport.Height, w.content.Height)))
}

// AddChild 必须使用最外层的 Widget 作为 parent，而不是嵌入的 helper。
func (w *flowWidget) SetChild(child gui.Widget) {
	if w.child == child {
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

type flowView struct {
	ui.ViewBase[flowView]
	child ui.View
}

func flow(child ui.View) *flowView                 { v := &flowView{child: child}; v.Self = v; return v }
func (v *flowView) Build() ui.View                 { return v }
func (*flowView) Mount(ui.BuildContext) gui.Widget { return new(flowWidget) }
func (v *flowView) Update(ctx ui.BuildContext, w gui.Widget) {
	ctx.UpdateChild(w.(*flowWidget), v.child)
}
func (*flowView) Unmount(ui.BuildContext, gui.Widget) {}
