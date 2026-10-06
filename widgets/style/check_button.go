package style

import (
	"image/color"

	"github.com/golang-gui/goui/gui"
	basestyle "github.com/golang-gui/goui/style"
)

// checkButtonRules supplies the complete indicator/button parts. Each part is
// a stable selection state; ordinary State still describes transient feedback.
// The explicit fallback does not extend the global State enum.
func checkButtonRules() []basestyle.Rule {
	// Copy the bare Button palette when constructing this explicit sheet; there
	// is no cross-widget style inheritance or lookup against the active theme.
	defaults := gui.DefaultStyleSheet()
	var buttonBackgrounds [3]color.Color
	for i, state := range []basestyle.State{basestyle.Normal, basestyle.Hovered, basestyle.Pressed} {
		buttonBackgrounds[i], _ = defaults.Resolve(basestyle.Sel{Name: "button", State: state}).BackgroundColor()
	}
	button := defaults.Resolve(basestyle.Sel{Name: "button"})
	buttonEdge, _ := button.BorderColor()
	buttonWidth, _ := button.BorderWidth()
	buttonRadius, _ := button.Radius()
	var text color.Color = color.Black
	var border color.Color = color.RGBA{R: 160, G: 160, B: 168, A: 255}
	var mark color.Color = color.RGBA{R: 45, G: 91, B: 160, A: 255}
	selected := [3]color.Color{
		color.RGBA{R: 185, G: 185, B: 185, A: 255},
		color.RGBA{R: 176, G: 176, B: 176, A: 255},
		color.RGBA{R: 168, G: 168, B: 168, A: 255},
	}
	checkFill := [3]color.Color{
		color.RGBA{R: 51, G: 112, B: 204, A: 255},
		color.RGBA{R: 48, G: 105, B: 192, A: 255},
		color.RGBA{R: 44, G: 97, B: 179, A: 255},
	}
	var disabled color.Color = color.RGBA{R: 150, G: 150, B: 150, A: 255}
	var focus color.Color = color.RGBA{R: 100, G: 145, B: 210, A: 255}
	rules := []basestyle.Rule{basestyle.Name("check-button").BackgroundColor(color.Transparent).BorderWidth(0)}
	for _, shape := range []string{"indicator", "radio", "button"} {
		ordinary := [3]color.Color{color.White, color.RGBA{R: 245, G: 245, B: 245, A: 255}, color.RGBA{R: 232, G: 232, B: 232, A: 255}}
		radius, width, outline := float32(4), float32(1), border
		if shape == "button" {
			ordinary, radius, width, outline = buttonBackgrounds, buttonRadius, buttonWidth, buttonEdge
		}
		for _, state := range []string{"", "-checked", "-mixed"} {
			if shape == "radio" && state == "-mixed" {
				continue
			}
			partWidth, partRadius := width, radius
			backgrounds, fg, edge := ordinary, text, outline
			if state != "" {
				backgrounds = selected
				if shape == "indicator" {
					backgrounds, fg, edge = checkFill, color.White, checkFill[0]
				}
			}
			if shape == "radio" {
				backgrounds, partRadius = ordinary, 9
				if state != "" {
					fg, edge, partWidth = mark, mark, 2
				}
			}
			disabledBackground, disabledEdge := ordinary[0], outline
			if state != "" && shape != "radio" {
				disabledBackground = color.RGBA{R: 220, G: 220, B: 220, A: 255}
			}
			if shape != "button" {
				disabledEdge = disabled
			}
			edges := [3]color.Color{edge, edge, edge}
			if shape == "indicator" && state != "" {
				edges = backgrounds
				disabledEdge = disabledBackground
			}
			part := shape + state
			rules = append(rules,
				basestyle.Name("check-button").Part(part).BackgroundColor(backgrounds[0]).ForegroundColor(fg).BorderColor(edges[0]).BorderWidth(partWidth).Radius(partRadius),
				basestyle.Name("check-button").Part(part).State(basestyle.Hovered).BackgroundColor(backgrounds[1]).BorderColor(edges[1]),
				basestyle.Name("check-button").Part(part).State(basestyle.Pressed).BackgroundColor(backgrounds[2]).BorderColor(edges[2]),
				basestyle.Name("check-button").Part(part).State(basestyle.Disabled).BackgroundColor(disabledBackground).ForegroundColor(disabled).BorderColor(disabledEdge),
			)
		}
	}
	rules = append(rules,
		basestyle.Name("check-button").Part("focus").BackgroundColor(color.Transparent).BorderWidth(0).Radius(buttonRadius),
		basestyle.Name("check-button").Part("focus").State(basestyle.FocusVisible).BorderColor(focus).BorderWidth(2),
	)
	return rules
}
