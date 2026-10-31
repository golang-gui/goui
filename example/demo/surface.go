package main

import (
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/ui"
)

// This demo Bin paints its own background. It neither passes styles to its
// child nor traverses the child subtree during painting.
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
	gui.PaintWidgetStyle(p, w, "widget")
}

// afterView adds a declarative completion hook without a layout Widget.
type afterView struct {
	ui.ComposeBase[afterView]
	child ui.View
	after func()
}

func afterUpdate(child ui.View, fn func()) *afterView {
	v := &afterView{child: child, after: fn}
	v.Self = v
	return v
}
func (v *afterView) Build(ctx ui.BuildContext) ui.View {
	if v.after != nil {
		ctx.AfterUpdate(v.after)
	}
	return v.child
}
