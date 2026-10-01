package main

import (
	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/ui"
	"github.com/golang-gui/goui/widgets"
)

// 用例自己的 Bin 观察正常绘制，再投递窗口准备步骤。
// 不强制布局、不重入事件循环，也不在 Paint 内转移子树。
type frameWidget struct {
	gui.WidgetBase
	child gui.Widget
	scale float32
	after func(*frameWidget)
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
func (w *frameWidget) Paint(p gui.Painter) {
	w.scale = p.PixelScale()
	if next := w.after; next != nil {
		w.after = nil
		gui.App.Post(func() {
			if !w.Destroyed() {
				next(w)
			}
		})
	}
}

type frameView struct {
	ui.ViewBase[frameView]
	key    string
	frames map[string]*frameWidget
	child  ui.View
	after  func(*frameWidget)
}

func (v *frameView) Build() ui.View { return v }
func (v *frameView) Mount(ui.BuildContext) gui.Widget {
	w := new(frameWidget)
	w.SetLayoutManager(layout.NewFillLayout())
	v.frames[v.key] = w
	return w
}
func (v *frameView) Update(ctx ui.BuildContext, widget gui.Widget) {
	w := widget.(*frameWidget)
	w.after = v.after
	ctx.UpdateChild(w, v.child)
}
func (*frameView) Unmount(ui.BuildContext, gui.Widget) {}
func firstTab(info gui.WidgetInfo) (geometry.Point, bool) {
	if info.Role == widgets.RoleTab {
		return info.Bounds.Pos, info.Bounds.Width > 0 && info.Bounds.Height > 0
	}
	for _, child := range info.Children {
		if point, ok := firstTab(child); ok {
			return point, true
		}
	}
	return geometry.Point{}, false
}
