package main

import (
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/style"
	"github.com/golang-gui/goui/theme/modern"
)

func TestPanelPosition(t *testing.T) {
	for _, test := range []struct {
		name            string
		available, size geometry.Size
		left, fill      bool
		want            geometry.Point
	}{
		{"right", geometry.Size{Width: 800, Height: 500}, geometry.Size{Width: 300, Height: 200}, false, false, geometry.Point{X: 488, Y: 12}},
		{"left", geometry.Size{Width: 800, Height: 500}, geometry.Size{Width: 300, Height: 200}, true, false, geometry.Point{X: 12, Y: 12}},
		{"fill", geometry.Size{Width: 800, Height: 500}, geometry.Size{Width: 800, Height: 500}, false, true, geometry.Point{}},
		{"fill-left", geometry.Size{Width: 800, Height: 500}, geometry.Size{Width: 800, Height: 500}, true, true, geometry.Point{}},
		{"small", geometry.Size{Width: 100, Height: 80}, geometry.Size{Width: 100, Height: 80}, true, false, geometry.Point{}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := panelPosition(test.available, test.size, test.left, test.fill); got != test.want {
				t.Fatalf("position=%+v, want %+v", got, test.want)
			}
		})
	}
}

func TestCentered(t *testing.T) {
	for _, available := range []geometry.Size{{Width: 900, Height: 600}, {Width: 400, Height: 200}} {
		size := geometry.Size{Width: 300, Height: 100}
		p := centered(available, size)
		if p.X*2+size.Width != available.Width || p.Y*2+size.Height != available.Height {
			t.Fatalf("panel is not centered: %+v", p)
		}
	}
}

func TestDemoState(t *testing.T) {
	d := newDemo()
	d.text.Set("edited")
	d.change(func(s *demoState) { s.showPanel = false; s.left = true; s.fill = true })
	d.change(func(s *demoState) { s.showPanel = true })
	if d.text.Get() != "edited" || !d.state.Get().left || !d.state.Get().fill {
		t.Fatal("hiding discarded panel state")
	}
	d.change(func(s *demoState) { s.showConfirm = true })
	d.change(func(s *demoState) { s.showConfirm = false }) // 取消确认。
	if d.state.Get().confirmations != 0 {
		t.Fatal("cancel applied the action")
	}
	d.change(func(s *demoState) { s.showConfirm = true })
	d.confirm()
	d.confirm() // 过期的重复确认不能再次执行动作。
	if d.state.Get().confirmations != 1 || d.state.Get().showConfirm || d.state.Get().background != 0 {
		t.Fatal("confirmation changed unrelated state or ran twice")
	}
}

func TestSurfaceChildAndStyles(t *testing.T) {
	w := &surfaceWidget{}
	first, second := gui.NewLabel("first"), gui.NewLabel("second")
	w.SetChild(first)
	w.SetChild(second)
	if first.Parent() != nil || second.Parent() != w || w.Snapshot().Role != gui.RoleBox {
		t.Fatal("surface did not replace its single child")
	}
	w.SetChild(nil)
	if second.Parent() != nil || len(w.Children()) != 0 {
		t.Fatal("surface did not detach content")
	}
	for _, dark := range []bool{false, true} {
		sheet := demoStyle(modern.Options{Dark: dark})
		mask := sheet.Resolve(style.Sel{Name: "ui-overlay-mask"})
		c, ok := mask.BackgroundColor()
		if !ok || c == nil {
			t.Fatal("missing mask color")
		}
		_, _, _, alpha := c.RGBA()
		if alpha == 0 || alpha == 0xffff {
			t.Fatal("mask must be translucent")
		}
		text := sheet.Resolve(style.Sel{Name: "label"})
		if size, ok := text.FontSize(); !ok || size <= 0 {
			t.Fatal("modern text style was lost")
		}
	}
}

func TestSurfaceWidthPreferenceDoesNotPreventFill(t *testing.T) {
	w := surface(nil).Mount(nil).(*surfaceWidget)
	w.SetChild(gui.NewTextInput()) // TextInput 可能占用全部可用宽度。
	w.SetMaxSize(geometry.Size{Width: 360})
	available := geometry.Size{Width: 800, Height: 400}
	if got := w.Measure(layout.Loose(available)); got.Width > 360 || got.Width <= 0 {
		t.Fatalf("floating panel lost its width preference: %+v", got)
	}
	if got := w.Measure(layout.Tight(available)); got.Size != available {
		t.Fatalf("Fill must override the panel's width preference: %+v", got)
	}
	w.Arrange(geometry.Rectangle{Size: available})
	if w.Child().Rect().Size != available {
		t.Fatal("panel did not arrange content in its allocated area")
	}
}
