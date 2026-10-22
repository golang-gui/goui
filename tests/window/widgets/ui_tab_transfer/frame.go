package main

import (
	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/ui"
	"github.com/golang-gui/goui/widgets"
)

// 用例自己的 Bin 记录正常布局下文档窗口的固定开销。
// AfterUpdate 只完成定位和移交，不测量隐藏目标、不依赖 Paint。
type frameWidget struct {
	gui.WidgetBase
	child                 gui.Widget
	initialSize, overhead geometry.Size
	sized                 bool
}

func (w *frameWidget) SetChild(child gui.Widget) {
	if w.child == child {
		return
	}
	if w.child != nil {
		w.WidgetBase.RemoveChild(w.child)
	}
	w.child = child
	if child != nil {
		w.WidgetBase.AddChild(w, child)
	}
}
func (w *frameWidget) Arrange(rect geometry.Rectangle) {
	w.WidgetBase.Arrange(rect)
	if w.sized || w.initialSize.Width <= 0 {
		return
	}
	view, ok := gui.FindWidget(w.Root(), "documents").(*widgets.TabView)
	if !ok || view.Rect().Width <= 0 || view.Rect().Height <= 0 {
		return
	}
	viewport := view.Rect().Size
	bounds := w.Window().Snapshot().Bounds.Size
	w.overhead = geometry.Size{Width: max(bounds.Width, w.initialSize.Width) - viewport.Width, Height: max(bounds.Height, w.initialSize.Height) - viewport.Height}
	w.sized = true
}

type frameView struct {
	ui.ViewBase[frameView]
	key    string
	frames map[string]*frameWidget
	child  ui.View
	after  func(*frameWidget)
	size   geometry.Size
}

func (v *frameView) Build() ui.View { return v }
func (v *frameView) Mount(ui.BuildContext) gui.Widget {
	w := &frameWidget{initialSize: v.size}
	w.SetLayoutManager(layout.NewFillLayout())
	v.frames[v.key] = w
	return w
}
func (v *frameView) Update(ctx ui.BuildContext, widget gui.Widget) {
	w := widget.(*frameWidget)
	ctx.UpdateChild(w, v.child)
	if next := v.after; next != nil {
		ctx.AfterUpdate(func() {
			if !w.Destroyed() {
				next(w)
			}
		})
	}
}
func (*frameView) Unmount(ui.BuildContext, gui.Widget) {}
