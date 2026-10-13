package modern

import (
	"github.com/golang-gui/goui/style"
	"testing"
)

func TestBorderBoxInvisibleDefault(t *testing.T) {
	for _, dark := range []bool{false, true} {
		for _, state := range []style.State{style.Normal, style.Hovered, style.Pressed, style.Disabled} {
			s := Sheet(Options{Dark: dark}).Resolve(style.Sel{Name: "border-box", State: state})
			bg, ok := s.BackgroundColor()
			if !ok || bg == nil {
				t.Fatal("missing explicit transparent background")
			}
			_, _, _, a := bg.RGBA()
			width, _ := s.BorderWidth()
			radius, _ := s.Radius()
			if a != 0 || width != 0 || radius != 0 {
				t.Fatal("modern theme decorated default frame")
			}
		}
	}
}
