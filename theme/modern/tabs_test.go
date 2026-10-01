package modern

import (
	"image/color"
	"testing"

	"github.com/golang-gui/goui/style"
)

func TestTabSurfacesAreNeutralAndReadable(t *testing.T) {
	for _, dark := range []bool{false, true} {
		p := colorsFor(dark)
		for _, accent := range []color.Color{nil, color.Black, color.White, rgb(0xff0000), rgb(0x0080ff)} {
			sheet := Sheet(Options{Dark: dark, AccentColor: accent})
			separator, _ := sheet.Resolve(style.Sel{Name: "tab-bar", Part: "separator"}).ForegroundColor()
			if rgba(separator) != mix(p.window, p.text, .18) {
				t.Fatal("separator must follow neutral theme palette, not accent")
			}
			for _, part := range []string{"", "selected", "dragging"} {
				for _, state := range []style.State{style.Normal, style.Hovered, style.Pressed} {
					sel := style.Sel{Name: "tab-item", Part: part, State: state}
					s := sheet.Resolve(sel)
					bg, _ := s.BackgroundColor()
					border, _ := s.BorderColor()
					// Changing only the accent must not change any tab surface.
					base := Sheet(Options{Dark: dark}).Resolve(sel)
					baseBG, _ := base.BackgroundColor()
					baseBorder, _ := base.BorderColor()
					if rgba(bg) != rgba(baseBG) || (part != "" && rgba(border) != rgba(baseBorder)) {
						t.Fatalf("accent leaked into tab: dark=%v part=%s state=%v", dark, part, state)
					}
					background := rgba(bg)
					if part == "" && state == style.Normal {
						if background.A != 0 {
							t.Fatal("idle tab must blend into its host")
						}
						background = p.window
					} else if background.A != 255 {
						t.Fatal("active tab surface must be opaque")
					}
					for _, foreground := range []color.RGBA{p.text, p.muted} {
						if contrast(background, foreground) < 4.5 {
							t.Fatalf("unreadable tab foreground: dark=%v part=%s state=%v", dark, part, state)
						}
					}
					width, _ := s.BorderWidth()
					radius, _ := s.Radius()
					if radius != 6 || (part == "" && width != 0) || (part != "" && width != 1) {
						t.Fatalf("unexpected tab outline: radius=%g width=%g", radius, width)
					}
				}
			}
			for _, name := range []string{"tab-close-button-text", "tab-scroll-button-text"} {
				fg, _ := sheet.Resolve(style.Sel{Name: name}).ForegroundColor()
				if rgba(fg) != p.muted {
					t.Fatalf("%s must use secondary foreground", name)
				}
			}
			for _, name := range []string{"tab-item-text", "tab-item-icon", "tab-item-text-selected", "tab-item-icon-selected"} {
				fg, _ := sheet.Resolve(style.Sel{Name: name}).ForegroundColor()
				if rgba(fg) != p.text {
					t.Fatalf("%s must use normal foreground, not accent", name)
				}
			}
			for _, name := range []string{"tab-close-button", "tab-scroll-button"} {
				for state, want := range map[style.State]color.RGBA{style.Normal: {}, style.Hovered: p.menuHover, style.Pressed: p.menuPressed} {
					bg, _ := sheet.Resolve(style.Sel{Name: name, State: state}).BackgroundColor()
					if rgba(bg) != want {
						t.Fatalf("%s state=%v lost neutral feedback", name, state)
					}
				}
			}
		}
	}
}
