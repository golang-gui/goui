// Package style supplies explicitly composed fallback rules for widgets.
// It does not register rules or modify the application's stylesheet. Modern
// already supplies its own complete widget appearance and does not need these rules.
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
	rules := []basestyle.Rule{
		basestyle.Name("tab-bar").BackgroundColor(color.Transparent),
		basestyle.Name("check-button").FontFamily(family).FontSize(size),
		basestyle.Name("check-button-text").ForegroundColor(color.Black).FontFamily(family).FontSize(size),
		basestyle.Name("switch-text").ForegroundColor(color.Black).FontFamily(family).FontSize(size),
		basestyle.Name("progress-bar").BackgroundColor(color.RGBA{R: 225, G: 225, B: 225, A: 255}).
			ForegroundColor(color.RGBA{R: 70, G: 130, B: 220, A: 255}).Radius(2),
		basestyle.Name("table-view").BackgroundColor(color.White).BorderColor(color.RGBA{R: 198, G: 198, B: 206, A: 255}).BorderWidth(1),
		basestyle.Name("table-grid").BorderColor(color.RGBA{R: 225, G: 225, B: 230, A: 255}).BorderWidth(0),
		basestyle.Name("table-grid").Part("horizontal").BorderWidth(1),
		basestyle.Name("table-header").BackgroundColor(color.RGBA{R: 245, G: 245, B: 245, A: 255}),
		basestyle.Name("table-header").State(basestyle.Hovered).BackgroundColor(color.RGBA{R: 235, G: 235, B: 235, A: 255}),
		basestyle.Name("table-header").State(basestyle.Pressed).BackgroundColor(color.RGBA{R: 225, G: 225, B: 225, A: 255}),
		basestyle.Name("table-header").Part("separator").ForegroundColor(color.RGBA{R: 198, G: 198, B: 206, A: 255}).BorderWidth(1),
		basestyle.Name("table-header").Part("sort").ForegroundColor(color.RGBA{R: 100, G: 110, B: 130, A: 255}),
		basestyle.Name("table-header-text").ForegroundColor(color.Black).FontFamily(family).FontSize(size),
		basestyle.Name("table-cell-text").ForegroundColor(color.Black).FontFamily(family).FontSize(size),
		basestyle.Name("table-cell-icon").ForegroundColor(color.Black),
		basestyle.Name("table-row").BackgroundColor(color.Transparent).Radius(4),
		basestyle.Name("table-row").State(basestyle.Hovered).BackgroundColor(color.RGBA{R: 235, G: 235, B: 235, A: 255}),
		basestyle.Name("table-row").State(basestyle.Pressed).BackgroundColor(color.RGBA{R: 220, G: 220, B: 220, A: 255}),
		basestyle.Name("table-row").Part("selected").BackgroundColor(color.RGBA{R: 220, G: 220, B: 220, A: 255}),
		basestyle.Name("table-row").Part("selected").State(basestyle.Hovered).BackgroundColor(color.RGBA{R: 210, G: 210, B: 210, A: 255}),
		basestyle.Name("table-row").Part("selected").State(basestyle.Pressed).BackgroundColor(color.RGBA{R: 200, G: 200, B: 200, A: 255}),
		basestyle.Name("table-row").Part("current").BackgroundColor(color.Transparent).BorderColor(color.RGBA{R: 100, G: 110, B: 130, A: 255}).BorderWidth(1),
		basestyle.Name("tree-view").BackgroundColor(color.Transparent),
		basestyle.Name("tree-item").BackgroundColor(color.Transparent).Radius(4),
		basestyle.Name("tree-item").State(basestyle.Hovered).BackgroundColor(color.RGBA{R: 235, G: 235, B: 235, A: 255}),
		basestyle.Name("tree-item").State(basestyle.Pressed).BackgroundColor(color.RGBA{R: 220, G: 220, B: 220, A: 255}),
		basestyle.Name("tree-item").Part("selected").BackgroundColor(color.RGBA{R: 210, G: 218, B: 230, A: 255}),
		basestyle.Name("tree-item").Part("selected").State(basestyle.Hovered).BackgroundColor(color.RGBA{R: 199, G: 209, B: 223, A: 255}),
		basestyle.Name("tree-item").Part("selected").State(basestyle.Pressed).BackgroundColor(color.RGBA{R: 188, G: 200, B: 216, A: 255}),
		basestyle.Name("tree-item").Part("current").BackgroundColor(color.Transparent).BorderColor(color.RGBA{R: 100, G: 110, B: 130, A: 255}).BorderWidth(1),
		basestyle.Name("tree-expander").ForegroundColor(color.Black).BackgroundColor(color.Transparent).Radius(4),
		basestyle.Name("tree-expander").State(basestyle.Hovered).BackgroundColor(color.RGBA{R: 235, G: 235, B: 235, A: 255}),
		basestyle.Name("tree-expander").State(basestyle.Pressed).BackgroundColor(color.RGBA{R: 220, G: 220, B: 220, A: 255}),
		basestyle.Name("tree-item-text").ForegroundColor(color.Black).FontFamily(family).FontSize(size),
		basestyle.Name("tree-item-icon").ForegroundColor(color.Black),
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
		basestyle.Name("split-handle").ForegroundColor(color.RGBA{R: 198, G: 198, B: 206, A: 255}),
	}
	rules = append(rules, checkButtonRules()...)
	return append(rules, switchRules()...)
}
