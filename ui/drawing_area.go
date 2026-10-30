package ui

import (
	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/gui"
)

type DrawFunc = gui.DrawFunc

// DrawingAreaView coordinates an explicitly regenerated Software drawing area.
// New closure values do not invalidate cached pixels; use OnShouldDraw to
// decide when content changes require regeneration.
type DrawingAreaView struct {
	ViewBase[DrawingAreaView]
	draw       DrawFunc
	shouldDraw func(*bool)
}

// DrawingArea declares local DIP drawing with no natural size. Set layout
// constraints as needed. The draw function runs on the GUI thread, with a
// borrowed Software Painter; it must not retain the Painter or mutate layout.
func DrawingArea(draw DrawFunc) *DrawingAreaView {
	v := &DrawingAreaView{draw: draw}
	v.Self = v
	return v
}

// OnShouldDraw queries before each actual paint. Preserve the initial value
// with *redraw = *redraw || dirty, or override it to implement a custom policy.
// The signal does not request frames. Schedule updates through existing paint
// or frame APIs. Removing this modifier restores the GUI default policy.
func (v *DrawingAreaView) OnShouldDraw(fn func(redraw *bool)) *DrawingAreaView {
	v.shouldDraw = fn
	return v
}

type drawingAreaState struct {
	draw       DrawFunc
	shouldDraw func(*bool)
	handle     signal.Handle
}

func (v *DrawingAreaView) Mount(ctx BuildContext) gui.Widget {
	s := &drawingAreaState{draw: v.draw, shouldDraw: v.shouldDraw}
	a := gui.NewDrawingArea(func(p gui.Painter, size geometry.Size) {
		if s.draw != nil {
			s.draw(p, size)
		}
	})
	s.handle = a.ConnectShouldDraw(func(redraw *bool) {
		if s.shouldDraw != nil {
			s.shouldDraw(redraw)
		}
	})
	ctx.SetState(s)
	return a
}
func (v *DrawingAreaView) Update(ctx BuildContext, _ gui.Widget) {
	s := ctx.State().(*drawingAreaState)
	s.draw, s.shouldDraw = v.draw, v.shouldDraw
}
func (*DrawingAreaView) Unmount(ctx BuildContext, _ gui.Widget) {
	if s, ok := ctx.State().(*drawingAreaState); ok {
		s.handle.Disconnect()
		s.draw, s.shouldDraw = nil, nil
	}
}

var _ WidgetView = (*DrawingAreaView)(nil)
