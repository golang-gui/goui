package ui

import (
	"github.com/golang-gui/goui/core/optional"
	"github.com/golang-gui/goui/gui"
	baseui "github.com/golang-gui/goui/ui"
	"github.com/golang-gui/goui/widgets"
)

type ProgressShape = widgets.ProgressShape

const (
	ProgressLinear   = widgets.ProgressLinear
	ProgressCircular = widgets.ProgressCircular
)

type ProgressBarView struct {
	baseui.ViewBase[ProgressBarView]
	value         float32
	shape         ProgressShape
	indeterminate bool
	thickness     optional.Optional[float32]
}

// ProgressBar declares a completion fraction in [0,1]. Activity animation is
// owned by the GUI widget, not by View rebuilds or a declarative timer.
func ProgressBar(value float32) *ProgressBarView {
	v := &ProgressBarView{value: value}
	v.Self = v
	return v
}

func (v *ProgressBarView) Value(value float32) *ProgressBarView {
	v.value = value
	return v
}

func (v *ProgressBarView) Shape(shape ProgressShape) *ProgressBarView {
	v.shape = shape
	return v
}

func (v *ProgressBarView) Indeterminate(indeterminate bool) *ProgressBarView {
	v.indeterminate = indeterminate
	return v
}

// Thickness sets bar / ring thickness in DIP; removing it restores the GUI
// constructor default. An explicit zero suppresses drawing and animation.
func (v *ProgressBarView) Thickness(thickness float32) *ProgressBarView {
	v.thickness.SetValue(thickness)
	return v
}

func (v *ProgressBarView) Build() baseui.View { return v }

func (v *ProgressBarView) Mount(ctx baseui.BuildContext) gui.Widget {
	p := widgets.NewProgressBar()
	ctx.SetState(p.Thickness())
	return p
}

func (v *ProgressBarView) Update(ctx baseui.BuildContext, widget gui.Widget) {
	p := widget.(*widgets.ProgressBar)
	p.SetValue(v.value)
	p.SetShape(v.shape)
	p.SetThickness(v.thickness.ValueOr(ctx.State().(float32)))
	p.SetIndeterminate(v.indeterminate)
}

func (v *ProgressBarView) Unmount(baseui.BuildContext, gui.Widget) {}

var _ baseui.WidgetView = (*ProgressBarView)(nil)
