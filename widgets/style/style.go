// Package style supplies explicitly composed fallback rules for widgets.
// It does not register rules or modify the application's stylesheet. Modern
// already supplies its own complete tab appearance and does not need these rules.
package style

import (
	"image/color"

	"github.com/golang-gui/goui/gui"
	basestyle "github.com/golang-gui/goui/style"
)

// Rules returns fresh widget fallback rules, without GUI's basic-widget rules.
// Append these to gui.DefaultStyleRules(), then append application overrides
// and pass the result to style.Sheet. Font defaults are explicitly copied from
// GUI's default Label style, not inherited from a parent or the active theme.
func Rules() []basestyle.Rule {
	label := gui.DefaultStyleSheet().Resolve(basestyle.Sel{Name: "label"})
	family, _ := label.FontFamily()
	size, _ := label.FontSize()
	return []basestyle.Rule{
		basestyle.Name("tab-bar").BackgroundColor(color.Transparent),
		basestyle.Name("tab-bar").Part("separator").ForegroundColor(color.RGBA{R: 198, G: 198, B: 206, A: 255}),
		basestyle.Name("tab-item").BackgroundColor(color.Transparent).Radius(6),
		basestyle.Name("tab-item").State(basestyle.Hovered).BackgroundColor(color.RGBA{R: 230, G: 230, B: 230, A: 255}),
		basestyle.Name("tab-item").Part("selected").BackgroundColor(color.RGBA{R: 210, G: 210, B: 210, A: 255}),
		basestyle.Name("tab-item").Part("dragging").BackgroundColor(color.White).BorderColor(color.RGBA{R: 192, G: 192, B: 192, A: 255}).BorderWidth(1),
		basestyle.Name("tab-item-text").ForegroundColor(color.Black).FontFamily(family).FontSize(size),
		basestyle.Name("tab-item-text-selected").ForegroundColor(color.Black).FontFamily(family).FontSize(size),
		basestyle.Name("tab-item-icon").ForegroundColor(color.Black),
		basestyle.Name("tab-item-icon-selected").ForegroundColor(color.Black),
		basestyle.Name("tab-close-button").BackgroundColor(color.Transparent).Radius(4),
		basestyle.Name("tab-close-button-text").ForegroundColor(color.Black).FontFamily(family).FontSize(size),
		basestyle.Name("tab-scroll-button").BackgroundColor(color.Transparent).Radius(4).FontSize(size),
		basestyle.Name("tab-scroll-button-text").ForegroundColor(color.Black).FontFamily(family).FontSize(size),
	}
}
