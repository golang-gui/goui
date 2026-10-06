package modern

import (
	"image/color"

	"github.com/golang-gui/goui/style"
)

func switchRules(p palette, accent, focus color.RGBA) []style.Rule {
	white, black := rgb(0xFFFFFF), rgb(0)
	// The small thumb stays light in both themes. A very light accent is
	// darkened at the theme boundary, rather than replacing it with dark ink.
	on := contrastColor(accent, black, []color.RGBA{white}, 3)
	off := mix(p.window, p.text, .22)
	rules := []style.Rule{style.Name("switch").BackgroundColor(color.Transparent).BorderWidth(0)}
	for _, checked := range []bool{false, true} {
		suffix, track := "", off
		if checked {
			suffix, track = "-checked", on
		}
		hover, press := mix(track, p.text, .06), mix(track, p.text, .12)
		disabled := mix(p.window, p.text, .12)
		if checked {
			hover, press = mix(track, black, .06), mix(track, black, .12)
			disabled = mix(disabled, on, .16)
		}
		for _, part := range []string{"track", "thumb"} {
			fills := [4]color.RGBA{track, hover, press, disabled}
			edges := fills
			width, radius := float32(1), float32(10)
			if part == "thumb" {
				// Keep the full disc visible; feedback belongs to the track.
				fills = [4]color.RGBA{white, white, white, mix(white, disabled, .25)}
				edges = [4]color.RGBA{}
				width, radius = 0, 8
			} else if !checked {
				for i := 0; i < 3; i++ {
					edges[i] = mix(fills[i], p.text, .08)
				}
			}
			rules = append(rules,
				style.Name("switch").Part(part+suffix).BackgroundColor(fills[0]).BorderColor(edges[0]).BorderWidth(width).Radius(radius),
				style.Name("switch").Part(part+suffix).State(style.Hovered).BackgroundColor(fills[1]).BorderColor(edges[1]),
				style.Name("switch").Part(part+suffix).State(style.Pressed).BackgroundColor(fills[2]).BorderColor(edges[2]),
				style.Name("switch").Part(part+suffix).State(style.Disabled).BackgroundColor(fills[3]).BorderColor(edges[3]),
			)
		}
	}
	return append(rules,
		style.Name("switch").Part("focus").BackgroundColor(color.Transparent).BorderWidth(0).Radius(12),
		style.Name("switch").Part("focus").State(style.FocusVisible).BorderColor(focus).BorderWidth(2),
	)
}
