package basic

import (
	"image/color"
	"testing"

	"github.com/golang-gui/goui/core/colors"
	"github.com/golang-gui/goui/style"
)

func TestPlaceholderColors(t *testing.T) {
	sheet := Sheet()
	for _, name := range []string{"text-input", "text-view"} {
		for _, state := range []style.State{style.Normal, style.Focused, style.Disabled} {
			got, ok := sheet.Resolve(style.Sel{Name: name, Part: "placeholder", State: state}).ForegroundColor()
			want := color.RGBA{R: 117, G: 117, B: 117, A: 255}
			if state == style.Disabled {
				want = color.RGBA{R: 140, G: 140, B: 140, A: 255}
			}
			if !ok || !colors.Equal(got, want) {
				t.Fatalf("%s/%v: hint color %v, want %v", name, state, got, want)
			}
		}
	}
}
