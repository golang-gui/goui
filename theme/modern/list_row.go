package modern

import (
	"image/color"

	"github.com/golang-gui/goui/style"
)

func listRowRules(p palette, focus color.RGBA, family string, size float32) []style.Rule {
	return []style.Rule{
		style.Name("list-row").BackgroundColor(color.Transparent).BorderWidth(0).Radius(8),
		style.Name("list-row").State(style.Hovered).BackgroundColor(mix(p.surface, p.text, .05)),
		style.Name("list-row").State(style.Pressed).BackgroundColor(mix(p.surface, p.text, .1)),
		style.Name("list-row").State(style.Disabled).BackgroundColor(color.Transparent),
		style.Name("list-row").Part("focus").BackgroundColor(color.Transparent).BorderWidth(0).Radius(8),
		style.Name("list-row").Part("focus").State(style.FocusVisible).BorderColor(focus).BorderWidth(2),
		style.Name("list-row-title").ForegroundColor(p.text).FontFamily(family).FontSize(size),
		style.Name("list-row-subtitle").ForegroundColor(p.muted).FontFamily(family).FontSize(size),
		style.Name("list-row-title").State(style.Disabled).ForegroundColor(p.disabled),
		style.Name("list-row-subtitle").State(style.Disabled).ForegroundColor(p.disabled),
	}
}
