package basic

import (
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/style"
)

func TestBorderBoxInvisibleDefault(t *testing.T) {
	for _, state := range []style.State{style.Normal, style.Hovered, style.Pressed, style.Disabled} {
		s := Sheet().Resolve(style.Sel{Name: "border-box", State: state})
		bg, ok := s.BackgroundColor()
		if !ok || bg == nil {
			t.Fatal("missing explicit transparent background")
		}
		_, _, _, a := bg.RGBA()
		width, _ := s.BorderWidth()
		radius, _ := s.CornerRadius()
		if a != 0 || width != 0 || radius != geometry.UniformCornerRadius(0) {
			t.Fatal("default frame gained decoration")
		}
	}
}
