package modern

import (
	"image/color"

	"github.com/golang-gui/goui/style"
)

// Checkboxes use a solid accent with a white mark. Radios keep an open center,
// while the larger toggle button uses neutral surfaces and independently styled text.
func checkButtonRules(p palette, accent, focus color.RGBA) []style.Rule {
	ordinary := [3]color.RGBA{p.button, mix(p.button, p.text, .04), mix(p.button, p.text, .08)}
	selected := [3]color.RGBA{
		mix(p.button, p.text, .12),
		mix(p.button, p.text, .16),
		mix(p.button, p.text, .20),
	}
	white, black := rgb(0xFFFFFF), rgb(0)
	// Darken very light accents rather than swapping the check to a black mark.
	fill := contrastColor(accent, black, []color.RGBA{white}, 4.5)
	checkFill := [3]color.RGBA{fill, mix(fill, black, .06), mix(fill, black, .12)}
	radioMark := contrastColor(accent, p.text, ordinary[:], 4.5)
	rules := []style.Rule{style.Name("check-button").BackgroundColor(color.Transparent).BorderWidth(0)}
	for _, shape := range []string{"indicator", "radio", "button"} {
		for _, selection := range []string{"", "-checked", "-mixed"} {
			if shape == "radio" && selection == "-mixed" {
				continue // Groups do not admit Mixed.
			}
			backgrounds, fg := ordinary, p.text
			edge, width, radius := mix(p.border, p.text, .20), float32(1), float32(4)
			disabledBackground, disabledEdge := p.button, p.disabled
			switch shape {
			case "indicator":
				if selection != "" {
					backgrounds, fg, edge = checkFill, white, fill
					disabledBackground = mix(p.button, p.text, .20)
					disabledEdge = disabledBackground
				}
			case "radio":
				radius = 9
				if selection != "" {
					fg, edge, width = radioMark, radioMark, 2
				}
			case "button":
				edge, disabledEdge, radius = p.border, p.border, 6
				if selection != "" {
					backgrounds = selected
					disabledBackground = mix(p.button, p.text, .12)
				}
			}
			part := shape + selection
			edges := [3]color.RGBA{edge, edge, edge}
			if shape == "indicator" && selection != "" {
				edges = backgrounds
			}
			rules = append(rules,
				style.Name("check-button").Part(part).BackgroundColor(backgrounds[0]).ForegroundColor(fg).BorderColor(edges[0]).BorderWidth(width).Radius(radius),
				style.Name("check-button").Part(part).State(style.Hovered).BackgroundColor(backgrounds[1]).BorderColor(edges[1]),
				style.Name("check-button").Part(part).State(style.Pressed).BackgroundColor(backgrounds[2]).BorderColor(edges[2]),
				style.Name("check-button").Part(part).State(style.Disabled).BackgroundColor(disabledBackground).ForegroundColor(p.disabled).BorderColor(disabledEdge),
			)
		}
	}
	return append(rules,
		style.Name("check-button").Part("focus").BackgroundColor(color.Transparent).BorderWidth(0).Radius(6),
		style.Name("check-button").Part("focus").State(style.FocusVisible).BorderColor(focus).BorderWidth(2),
	)
}
