package style

import (
	"image/color"
	"strings"
	"testing"

	"github.com/golang-gui/goui/gui"
	basestyle "github.com/golang-gui/goui/style"
)

func TestSwitchFallback(t *testing.T) {
	sheet := basestyle.Sheet(append(gui.DefaultStyleRules(), Rules()...)...)
	for _, part := range []string{"track", "track-checked", "thumb", "thumb-checked"} {
		for _, state := range []basestyle.State{basestyle.Normal, basestyle.Hovered, basestyle.Pressed, basestyle.Disabled, basestyle.Hovered | basestyle.Pressed, basestyle.Disabled | basestyle.Hovered | basestyle.Pressed} {
			s := sheet.Resolve(basestyle.Sel{Name: "switch", Part: part, State: state})
			if bg, ok := s.BackgroundColor(); !ok || bg == nil {
				t.Fatal("missing background")
			}
			wantWidth := float32(1)
			if part == "track-checked" {
				wantWidth = 0
			}
			if strings.HasPrefix(part, "thumb") {
				wantWidth = 0
				if bg, _ := s.BackgroundColor(); state&basestyle.Disabled == 0 && color.RGBAModel.Convert(bg) != (color.RGBA{R: 255, G: 255, B: 255, A: 255}) {
					t.Fatal("enabled thumb must stay white")
				}
			}
			if width, _ := s.BorderWidth(); width != wantWidth {
				t.Fatal("unstable border")
			}
			if part == "track-checked" {
				bg, _ := s.BackgroundColor()
				edge, _ := s.BorderColor()
				if color.RGBAModel.Convert(bg) != color.RGBAModel.Convert(edge) {
					t.Fatal("checked track outline must follow its fill")
				}
			}
		}
	}
	if width, _ := sheet.Resolve(basestyle.Sel{Name: "switch", Part: "focus", State: basestyle.FocusVisible}).BorderWidth(); width != 2 {
		t.Fatal("missing focus")
	}
	if size, _ := sheet.Resolve(basestyle.Sel{Name: "switch-text"}).FontSize(); size <= 0 {
		t.Fatal("missing independent font")
	}
	if bg, ok := gui.DefaultStyleSheet().Resolve(basestyle.Sel{Name: "switch", Part: "track-checked"}).BackgroundColor(); !ok || color.RGBAModel.Convert(bg) != (color.RGBA{R: 70, G: 130, B: 220, A: 255}) {
		t.Fatal("default sheet lacks the selected switch track")
	}
}
