package modern

import (
	"fmt"
	"image/color"
	"testing"

	"github.com/golang-gui/goui/style"
)

func TestCheckButtonMatchesModernButtons(t *testing.T) {
	for _, dark := range []bool{false, true} {
		for _, accent := range []color.Color{nil, color.Black, color.White, rgb(0xD03030), rgb(0xFFFF00)} {
			t.Run(fmt.Sprintf("dark=%v/accent=%v", dark, accent), func(t *testing.T) {
				sheet := Sheet(Options{Dark: dark, AccentColor: accent})
				p := colorsFor(dark)
				for _, state := range []style.State{style.Normal, style.Hovered, style.Pressed, style.Disabled} {
					ordinary := sheet.Resolve(style.Sel{Name: "button", State: state})
					toggle := sheet.Resolve(style.Sel{Name: "check-button", Part: "button", State: state})
					bg, _ := ordinary.BackgroundColor()
					gotBG, _ := toggle.BackgroundColor()
					if rgba(gotBG) != rgba(bg) {
						t.Errorf("unchecked state=%v background=%v; ordinary button=%v", state, gotBG, bg)
					}
					for _, selection := range []string{"", "-checked", "-mixed"} {
						button := sheet.Resolve(style.Sel{Name: "check-button", Part: "button" + selection, State: state})
						edge, _ := button.BorderColor()
						width, _ := button.BorderWidth()
						if rgba(edge) != p.border || width != 1 {
							t.Errorf("state=%v selection=%q changed the neutral button outline: %v/%v", state, selection, edge, width)
						}
						if radius, _ := button.Radius(); radius != 6 {
							t.Errorf("button radius=%v; want 6", radius)
						}
						if selection != "" && state != style.Disabled {
							bg, _ := button.BackgroundColor()
							if contrast(rgba(bg), p.text) < 4.5 {
								t.Errorf("selected button text is not legible: %v", bg)
							}
							if rgba(bg) == p.button {
								t.Error("selection disappeared into the ordinary button surface")
							}
						}
					}
				}
				for _, selection := range []string{"", "-checked", "-mixed"} {
					normal := sheet.Resolve(style.Sel{Name: "check-button", Part: "indicator" + selection})
					normalEdge, _ := normal.BorderColor()
					for _, state := range []style.State{style.Normal, style.Hovered, style.Pressed} {
						indicator := sheet.Resolve(style.Sel{Name: "check-button", Part: "indicator" + selection, State: state})
						edge, _ := indicator.BorderColor()
						if radius, _ := indicator.Radius(); radius != 4 || selection == "" && rgba(edge) != rgba(normalEdge) {
							t.Errorf("indicator selection=%q state=%v: radius/outline changed", selection, state)
						}
						if selection != "" {
							bg, _ := indicator.BackgroundColor()
							fg, _ := indicator.ForegroundColor()
							if rgba(fg) != rgb(0xFFFFFF) || rgba(edge) != rgba(bg) {
								t.Errorf("selected checkbox needs white ink and a solid matching outline: bg=%v fg=%v edge=%v", bg, fg, edge)
							}
						}
					}
				}
			})
		}
	}
}

func TestCheckButtonStyle(t *testing.T) {
	for _, dark := range []bool{false, true} {
		for _, accent := range []color.Color{nil, color.Black, color.White, rgb(0xD03030), rgb(0xFFFF00)} {
			sheet := Sheet(Options{Dark: dark, AccentColor: accent})
			for _, shape := range []string{"indicator", "radio", "button"} {
				for _, selection := range []string{"", "-checked", "-mixed"} {
					if shape == "radio" && selection == "-mixed" {
						continue
					}
					for _, state := range []style.State{style.Normal, style.Hovered, style.Pressed, style.Disabled} {
						s := sheet.Resolve(style.Sel{Name: "check-button", Part: shape + selection, State: state})
						bg, bgOK := s.BackgroundColor()
						fg, fgOK := s.ForegroundColor()
						width, _ := s.BorderWidth()
						wantWidth := float32(1)
						if shape == "radio" && selection != "" {
							wantWidth = 2
						}
						if !bgOK || !fgOK || rgba(bg).A != 255 || rgba(fg).A != 255 || width != wantWidth {
							t.Fatal("incomplete appearance part")
						}
						if state != style.Disabled && shape != "button" && selection != "" && contrast(rgba(bg), rgba(fg)) < 4.5 {
							t.Fatalf("low mark contrast dark=%v accent=%v bg=%v fg=%v", dark, accent, bg, fg)
						}
						if shape == "radio" {
							ordinaryBG, _ := sheet.Resolve(style.Sel{Name: "button", State: state}).BackgroundColor()
							if rgba(bg) != rgba(ordinaryBG) {
								t.Fatal("radio gap should retain the neutral surface")
							}
						}
					}
				}
			}
			focus := sheet.Resolve(style.Sel{Name: "check-button", Part: "focus", State: style.FocusVisible})
			if width, _ := focus.BorderWidth(); width != 2 {
				t.Fatal("keyboard focus not 2 DIP")
			}
			focus = sheet.Resolve(style.Sel{Name: "check-button", Part: "focus", State: style.Focused})
			if width, _ := focus.BorderWidth(); width != 0 {
				t.Fatal("pointer focus changed appearance")
			}
			label := sheet.Resolve(style.Sel{Name: "check-button-text"})
			fg, _ := label.ForegroundColor()
			for _, state := range []style.State{style.Normal, style.Hovered, style.Pressed, style.Disabled} {
				bg, _ := sheet.Resolve(style.Sel{Name: "check-button", Part: "button-checked", State: state}).BackgroundColor()
				if contrast(rgba(fg), rgba(bg)) < 4.5 {
					t.Fatal("independent child label illegible on selected button")
				}
			}
			if size, _ := label.FontSize(); size != 14 {
				t.Fatal("label relies on parent font")
			}
		}
	}
}

func TestCheckButtonSelectionSurfaces(t *testing.T) {
	// Independent sRGB expectations for neutral button selection in all accents.
	// Checkbox selection instead uses a solid blue fill with white ink.
	for _, tc := range []struct {
		dark  bool
		fills [3]color.RGBA
	}{
		{false, [3]color.RGBA{rgb(0xE4E4E5), rgb(0xDBDBDC), rgb(0xD2D2D3)}},
		{true, [3]color.RGBA{rgb(0x48484E), rgb(0x505055), rgb(0x57575C)}},
	} {
		sheet := Sheet(Options{Dark: tc.dark})
		s := sheet.Resolve(style.Sel{Name: "check-button", Part: "indicator-checked"})
		fill, _ := s.BackgroundColor()
		mark, _ := s.ForegroundColor()
		bg := rgba(fill)
		if rgba(mark) != rgb(0xFFFFFF) || bg.B <= bg.G || bg.G <= bg.R {
			t.Fatalf("dark=%v fill=%v mark=%v; want solid blue with white ink", tc.dark, fill, mark)
		}
		for _, accent := range []color.Color{nil, color.Black, color.White, rgb(0xD03030), rgb(0xFFFF00)} {
			sheet := Sheet(Options{Dark: tc.dark, AccentColor: accent})
			for _, part := range []string{"button-checked", "button-mixed"} {
				for i, state := range []style.State{style.Normal, style.Hovered, style.Pressed} {
					buttonBG, _ := sheet.Resolve(style.Sel{Name: "check-button", Part: part, State: state}).BackgroundColor()
					if rgba(buttonBG) != tc.fills[i] {
						t.Fatalf("dark=%v accent=%v part=%s state=%v fill=%v; want neutral %v", tc.dark, accent, part, state, buttonBG, tc.fills[i])
					}
				}
			}
		}
	}
}
