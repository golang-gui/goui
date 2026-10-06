package ui

import (
	"testing"

	"github.com/golang-gui/goui/widgets"
)

func TestProgressBarReconcile(t *testing.T) {
	ctx := new(tabContext)
	v := ProgressBar(.25).Shape(ProgressCircular).Indeterminate(true).Thickness(3)
	p := v.Mount(ctx).(*widgets.ProgressBar)
	v.Update(ctx, p)
	if p.Value() != .25 || p.Shape() != widgets.ProgressCircular || !p.Indeterminate() || p.Thickness() != 3 {
		t.Fatal("initial declaration lost")
	}
	v = ProgressBar(.5).Shape(ProgressCircular).Indeterminate(true).Thickness(3)
	v.Update(ctx, p)
	if p.Value() != .5 || p.Shape() != widgets.ProgressCircular || !p.Indeterminate() || p.Thickness() != 3 {
		t.Fatal("updated declaration was not applied to the retained widget")
	}
	ProgressBar(1).Thickness(0).Update(ctx, p)
	if p.Shape() != widgets.ProgressLinear || p.Indeterminate() || p.Thickness() != 0 || p.Value() != 1 {
		t.Fatal("explicit zero thickness or removed mode was lost")
	}
	ProgressBar(0).Update(ctx, p)
	if p.Thickness() != 4 {
		t.Fatal("removed thickness did not restore constructor default")
	}
	p.SetValue(.8)
	p.SetShape(widgets.ProgressCircular)
	p.SetThickness(9)
	p.SetIndeterminate(true)
	ProgressBar(.1).Value(.2).Update(ctx, p)
	if p.Value() != .2 || p.Shape() != widgets.ProgressLinear || p.Thickness() != 4 || p.Indeterminate() {
		t.Fatal("declaration failed to restore externally changed GUI state")
	}
}
