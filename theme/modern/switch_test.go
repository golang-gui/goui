package modern

import (
	"image/color"
	"testing"

	"github.com/golang-gui/goui/style"
)

func TestSwitchTheme(t *testing.T) {
	for _, dark := range []bool{false, true} {
		for _, accent := range []color.Color{nil, color.Black, color.White, rgb(0xD03030), rgb(0xFFFF00)} {
			sheet := Sheet(Options{Dark: dark, AccentColor: accent})
			for _, state := range []style.State{style.Normal, style.Hovered, style.Pressed, style.Disabled, style.Hovered | style.Pressed, style.Disabled | style.Hovered | style.Pressed} {
				for _, suffix := range []string{"", "-checked"} {
					track := sheet.Resolve(style.Sel{Name: "switch", Part: "track" + suffix, State: state})
					thumb := sheet.Resolve(style.Sel{Name: "switch", Part: "thumb" + suffix, State: state})
					bg, bgOK := track.BackgroundColor()
					ink, inkOK := thumb.BackgroundColor()
					width, _ := track.BorderWidth()
					if !bgOK || !inkOK || rgba(bg).A != 255 || rgba(ink).A != 255 || width != 1 {
						t.Fatal("incomplete switch parts")
					}
					if width, _ := thumb.BorderWidth(); width != 0 {
						t.Fatal("thumb must be a full disc without an outline")
					}
					disabled := state&style.Disabled != 0
					if !disabled && rgba(ink) != rgb(0xFFFFFF) {
						t.Fatal("enabled thumb must stay white in all interaction states")
					}
					edge, _ := track.BorderColor()
					if suffix != "" && rgba(edge) != rgba(bg) {
						t.Fatal("checked track outline must follow its fill")
					}
					if suffix == "" && contrast(rgba(edge), rgba(bg)) > 1.35 {
						t.Fatal("off track outline is too strong")
					}
					// Disabled parts remain distinct, but no active-accent contrast is required.
					if disabled && contrast(rgba(bg), rgba(ink)) < 1.2 {
						t.Fatalf("disabled thumb disappears dark=%v accent=%v", dark, accent)
					}
					if suffix != "" && !disabled && contrast(rgba(bg), rgba(ink)) < 3 {
						t.Fatalf("thumb disappears dark=%v accent=%v state=%v", dark, accent, state)
					}
				}
			}
			focus := sheet.Resolve(style.Sel{Name: "switch", Part: "focus", State: style.FocusVisible})
			if width, _ := focus.BorderWidth(); width != 2 {
				t.Fatal("keyboard focus not 2 DIP")
			}
			focus = sheet.Resolve(style.Sel{Name: "switch", Part: "focus", State: style.Focused})
			if width, _ := focus.BorderWidth(); width != 0 {
				t.Fatal("mouse focus decoration")
			}
			if size, _ := sheet.Resolve(style.Sel{Name: "switch-text"}).FontSize(); size != 14 {
				t.Fatal("text lacks independent font")
			}
		}
	}
}
