package modern

import (
	"image/color"

	"github.com/golang-gui/goui/style"
)

func listRowRules(p palette, accent color.RGBA, family string, size float32) []style.Rule {
	// Reuse the existing neutral button surfaces. In dark mode this keeps a
	// row distinct from the page without another independent color palette.
	surface := p.button
	hover, pressed := mix(surface, p.text, .04), mix(surface, p.text, .08)
	border := mix(surface, p.text, .12)
	focus := focusColor(accent, p.text, []color.RGBA{p.window, surface, hover, pressed})
	return []style.Rule{
		style.Name("list-row").BackgroundColor(surface).BorderColor(border).BorderWidth(1).Radius(8),
		style.Name("list-row").State(style.Hovered).BackgroundColor(hover),
		style.Name("list-row").State(style.Pressed).BackgroundColor(pressed),
		style.Name("list-row").State(style.Disabled).BackgroundColor(surface),
		style.Name("list-row").Part("focus").BackgroundColor(color.Transparent).BorderWidth(0).Radius(8),
		style.Name("list-row").Part("focus").State(style.FocusVisible).BorderColor(focus).BorderWidth(2),
		style.Name("list-row-title").ForegroundColor(p.text).FontFamily(family).FontSize(size),
		style.Name("list-row-subtitle").ForegroundColor(p.muted).FontFamily(family).FontSize(size * .9),
		style.Name("list-row-title").State(style.Disabled).ForegroundColor(p.disabled),
		style.Name("list-row-subtitle").State(style.Disabled).ForegroundColor(p.disabled),
	}
}
