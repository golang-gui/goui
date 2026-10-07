package main

import (
	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/ui"
)

// ScrollBar 没有公共 UI 声明，此处示范如何用标准 WidgetView 作本地薄绑定。
type barView struct {
	ui.ViewBase[barView]
	value   float32
	changed func(float32)
}
type barBinding struct {
	handle  signal.Handle
	changed func(float32)
}

func scrollBar(value float32, changed func(float32)) *barView {
	v := &barView{value: value, changed: changed}
	v.Self = v
	return v
}
func (v *barView) Build() ui.View { return v }
func (*barView) Mount(ctx ui.BuildContext) gui.Widget {
	w := gui.NewScrollBar(layout.DirectionHorizontal)
	w.SetMax(100)
	s := new(barBinding)
	s.handle = w.ConnectChange(func(value float32) {
		if s.changed != nil {
			s.changed(value)
		}
	})
	ctx.SetState(s)
	return w
}
func (v *barView) Update(ctx ui.BuildContext, w gui.Widget) {
	ctx.State().(*barBinding).changed = v.changed
	w.(*gui.ScrollBar).SetValue(v.value)
}
func (*barView) Unmount(ctx ui.BuildContext, _ gui.Widget) {
	ctx.State().(*barBinding).handle.Disconnect()
}
