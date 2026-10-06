package style

import (
	"image/color"

	basestyle "github.com/golang-gui/goui/style"
)

func switchRules() []basestyle.Rule {
	white := color.RGBA{R: 255, G: 255, B: 255, A: 255}
	rules := []basestyle.Rule{basestyle.Name("switch").BackgroundColor(color.Transparent).BorderWidth(0)}
	for _, checked := range []bool{false, true} {
		suffix := ""
		backgrounds := [3]color.RGBA{{R: 199, G: 199, B: 201, A: 255}, {R: 189, G: 189, B: 191, A: 255}, {R: 179, G: 179, B: 181, A: 255}}
		if checked {
			suffix = "-checked"
			backgrounds = [3]color.RGBA{{R: 51, G: 112, B: 204, A: 255}, {R: 48, G: 105, B: 192, A: 255}, {R: 44, G: 97, B: 179, A: 255}}
		}
		for _, part := range []string{"track", "thumb"} {
			fills, width, radius := backgrounds, float32(1), float32(10)
			edges := [3]color.RGBA{{R: 186, G: 186, B: 188, A: 255}, {R: 176, G: 176, B: 179, A: 255}, {R: 167, G: 167, B: 169, A: 255}}
			disabled := color.RGBA{R: 220, G: 220, B: 220, A: 255}
			if checked {
				edges = fills
				disabled = color.RGBA{R: 193, G: 203, B: 217, A: 255}
			}
			if part == "thumb" {
				fills = [3]color.RGBA{white, white, white}
				edges = [3]color.RGBA{}
				width, radius = 0, 8
				disabled = color.RGBA{R: 246, G: 246, B: 246, A: 255}
			}
			rules = append(rules,
				basestyle.Name("switch").Part(part+suffix).BackgroundColor(fills[0]).BorderColor(edges[0]).BorderWidth(width).Radius(radius),
				basestyle.Name("switch").Part(part+suffix).State(basestyle.Hovered).BackgroundColor(fills[1]).BorderColor(edges[1]),
				basestyle.Name("switch").Part(part+suffix).State(basestyle.Pressed).BackgroundColor(fills[2]).BorderColor(edges[2]),
				basestyle.Name("switch").Part(part+suffix).State(basestyle.Disabled).BackgroundColor(disabled).BorderColor(disabled),
			)
		}
	}
	return append(rules,
		basestyle.Name("switch").Part("focus").BackgroundColor(color.Transparent).BorderWidth(0).Radius(12),
		basestyle.Name("switch").Part("focus").State(basestyle.FocusVisible).BorderColor(color.RGBA{R: 100, G: 145, B: 210, A: 255}).BorderWidth(2),
	)
}
