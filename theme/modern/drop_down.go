package modern

import (
	"image/color"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/style"
)

func dropDownRules(p palette, focus color.RGBA, family string, size float32) []style.Rule {
	rules := []style.Rule{
		style.Name("drop-down").BackgroundColor(p.button).BorderColor(p.border).BorderWidth(1).Radius(6),
		style.Name("drop-down").State(style.Hovered).BackgroundColor(mix(p.button, p.text, .04)),
		style.Name("drop-down").State(style.Pressed).BackgroundColor(mix(p.button, p.text, .08)),
		style.Name("drop-down").State(style.Disabled).BackgroundColor(mix(p.window, p.button, .5)),
		style.Name("drop-down").Part("arrow").ForegroundColor(p.muted),
		style.Name("drop-down").Part("arrow").State(style.Disabled).ForegroundColor(p.disabled),
		style.Name("drop-down").Part("focus").BackgroundColor(color.Transparent).BorderWidth(0).Radius(6),
		style.Name("drop-down").Part("focus").State(style.FocusVisible).BorderColor(focus).BorderWidth(2),
		style.Name("drop-down-popup").BackgroundColor(p.surface).BorderColor(p.border).BorderWidth(1).Radius(8).
			Shadow(style.Shadow{Color: color.NRGBA{A: 46}, Offset: geometry.Point{Y: 4}, BlurRadius: 16}),
		style.Name("drop-down-item").BackgroundColor(color.Transparent).BorderWidth(0).Radius(4),
		style.Name("drop-down-item").State(style.Hovered).BackgroundColor(p.menuHover),
		style.Name("drop-down-item").State(style.Pressed).BackgroundColor(p.menuPressed),
		style.Name("drop-down-item").Part("selected").BackgroundColor(p.menuPressed),
		style.Name("drop-down-item").Part("selected").State(style.Hovered).BackgroundColor(mix(p.menuPressed, p.text, .04)),
		style.Name("drop-down-item").Part("check").ForegroundColor(p.text),
		style.Name("drop-down-item").Part("check").State(style.Disabled).ForegroundColor(p.disabled),
	}
	for _, name := range []string{"drop-down-text", "drop-down-item-text", "drop-down-text-disabled", "drop-down-item-text-disabled", "drop-down-placeholder"} {
		fg := p.text
		if name == "drop-down-placeholder" {
			fg = p.muted
		}
		if name == "drop-down-text-disabled" || name == "drop-down-item-text-disabled" {
			fg = p.disabled
		}
		rules = append(rules, style.Name(name).FontFamily(family).FontSize(size).ForegroundColor(fg))
	}
	return rules
}
