package modern

import (
	"image/color"
	"testing"

	"github.com/golang-gui/goui/style"
)

func TestProgressBarStyle(t *testing.T) {
	for _, dark := range []bool{false, true} {
		var track color.RGBA
		for i, accent := range []color.Color{nil, color.Black, color.White, rgb(0xD03030)} {
			s := resolved(Sheet(Options{Dark: dark, AccentColor: accent}), "progress-bar", style.Normal)
			bg, bgSet := s.BackgroundColor()
			fg, fgSet := s.ForegroundColor()
			if !bgSet || !fgSet || rgba(bg).A != 255 || rgba(fg).A != 255 || contrast(rgba(fg), rgba(bg)) < 3 {
				t.Fatalf("insufficient progress contrast: dark=%v accent=%v bg=%v fg=%v", dark, accent, bg, fg)
			}
			if i == 0 {
				track = rgba(bg)
			} else if rgba(bg) != track {
				t.Fatal("neutral track changes with accent")
			}
			if r, _ := s.Radius(); r != 2 {
				t.Fatal("missing capsule radius")
			}
		}
	}
}
