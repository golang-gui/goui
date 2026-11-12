package basic

import (
	"image/color"

	"github.com/golang-gui/goui/style"
)

func listRowRules(base style.StyleSheet) []style.Rule {
	label := base.Resolve(style.Sel{Name: "label"})
	foreground, _ := label.ForegroundColor()
	family, _ := label.FontFamily()
	size, _ := label.FontSize()
	disabled, _ := base.Resolve(style.Sel{Name: "label", State: style.Disabled}).ForegroundColor()
	surface, _ := base.Resolve(style.Sel{Name: "text-input"}).BackgroundColor()
	hover, _ := base.Resolve(style.Sel{Name: "button", State: style.Hovered}).BackgroundColor()
	pressed, _ := base.Resolve(style.Sel{Name: "button", State: style.Pressed}).BackgroundColor()
	// Basic reuses its existing neutral colors. A plain outline provides a
	// visible fallback without introducing a separate settings palette.
	return []style.Rule{
		style.Name("list-row").BackgroundColor(surface).BorderColor(disabled).BorderWidth(1).Radius(2),
		style.Name("list-row").State(style.Hovered).BackgroundColor(hover),
		style.Name("list-row").State(style.Pressed).BackgroundColor(pressed),
		style.Name("list-row").State(style.Disabled).BackgroundColor(surface),
		style.Name("list-row").Part("focus").BackgroundColor(color.Transparent).BorderWidth(0).Radius(2),
		style.Name("list-row").Part("focus").State(style.FocusVisible).BorderColor(foreground).BorderWidth(2),
		style.Name("list-row-title").ForegroundColor(foreground).FontFamily(family).FontSize(size),
		style.Name("list-row-subtitle").ForegroundColor(foreground).FontFamily(family).FontSize(size * .9),
		style.Name("list-row-title").State(style.Disabled).ForegroundColor(disabled),
		style.Name("list-row-subtitle").State(style.Disabled).ForegroundColor(disabled),
	}
}
