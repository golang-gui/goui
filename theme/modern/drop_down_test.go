package modern

import (
	"image/color"
	"testing"

	"github.com/golang-gui/goui/style"
)

func TestDropDownModernRules(t *testing.T) {
	for _, dark := range []bool{false, true} {
		blue := Sheet(Options{Dark: dark, AccentColor: color.RGBA{B: 200, A: 255}, FontFamily: "Sans", FontSize: 14})
		red := Sheet(Options{Dark: dark, AccentColor: color.RGBA{R: 200, A: 255}, FontFamily: "Sans", FontSize: 14})
		for _, sel := range []style.Sel{{Name: "drop-down-item", State: style.Hovered}, {Name: "drop-down-item", Part: "selected"}, {Name: "drop-down-item", Part: "selected", State: style.Hovered}} {
			a, _ := blue.Resolve(sel).BackgroundColor()
			b, _ := red.Resolve(sel).BackgroundColor()
			if a == nil || a != b {
				t.Fatal("option fill depends on accent")
			}
		}
		focus := blue.Resolve(style.Sel{Name: "drop-down", Part: "focus", State: style.FocusVisible})
		if width, _ := focus.BorderWidth(); width != 2 {
			t.Fatal("keyboard focus width")
		}
		if width, _ := blue.Resolve(style.Sel{Name: "drop-down", Part: "focus", State: style.Normal}).BorderWidth(); width != 0 {
			t.Fatal("mouse focus got keyboard border")
		}
		popup := blue.Resolve(style.Sel{Name: "drop-down-popup"})
		if radius, _ := popup.Radius(); radius != 8 {
			t.Fatal("popup radius")
		}
		if shadow, _ := popup.Shadow(); shadow.BlurRadius != 16 || shadow.Color == nil {
			t.Fatal("popup shadow")
		}
		for _, name := range []string{"drop-down-text", "drop-down-placeholder", "drop-down-item-text", "drop-down-item-text-disabled"} {
			s := blue.Resolve(style.Sel{Name: name})
			if family, _ := s.FontFamily(); family != "Sans" {
				t.Fatal("child font is not independently defined")
			}
			if size, _ := s.FontSize(); size != 14 {
				t.Fatal("child font size")
			}
		}
	}
}
