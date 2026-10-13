package basic

import (
	"image/color"

	"github.com/golang-gui/goui/style"
)

// WidgetRules returns the fallback rules for controls in package widgets.
// Rules already includes these; this subset supports the legacy widgets/style API.
func WidgetRules() []style.Rule {
	label := style.Sheet(coreRules()...).Resolve(style.Sel{Name: "label"})
	family, _ := label.FontFamily()
	size, _ := label.FontSize()
	rules := []style.Rule{
		style.Name("tab-bar").BackgroundColor(color.Transparent),
		style.Name("check-button").FontFamily(family).FontSize(size),
		style.Name("check-button-text").ForegroundColor(color.Black).FontFamily(family).FontSize(size),
		style.Name("switch-text").ForegroundColor(color.Black).FontFamily(family).FontSize(size),
		style.Name("progress-bar").BackgroundColor(color.RGBA{R: 225, G: 225, B: 225, A: 255}).
			ForegroundColor(color.RGBA{R: 70, G: 130, B: 220, A: 255}).Radius(2),
		style.Name("table-view").BackgroundColor(color.White).BorderColor(color.RGBA{R: 198, G: 198, B: 206, A: 255}).BorderWidth(1),
		style.Name("table-grid").BorderColor(color.RGBA{R: 225, G: 225, B: 230, A: 255}).BorderWidth(0),
		style.Name("table-grid").Part("horizontal").BorderWidth(1),
		style.Name("table-header").BackgroundColor(color.RGBA{R: 245, G: 245, B: 245, A: 255}),
		style.Name("table-header").State(style.Hovered).BackgroundColor(color.RGBA{R: 235, G: 235, B: 235, A: 255}),
		style.Name("table-header").State(style.Pressed).BackgroundColor(color.RGBA{R: 225, G: 225, B: 225, A: 255}),
		style.Name("table-header").Part("separator").ForegroundColor(color.RGBA{R: 198, G: 198, B: 206, A: 255}).BorderWidth(1),
		style.Name("table-header").Part("sort").ForegroundColor(color.RGBA{R: 100, G: 110, B: 130, A: 255}),
		style.Name("table-header-text").ForegroundColor(color.Black).FontFamily(family).FontSize(size),
		style.Name("table-cell-text").ForegroundColor(color.Black).FontFamily(family).FontSize(size),
		style.Name("table-cell-icon").ForegroundColor(color.Black),
		style.Name("table-row").BackgroundColor(color.Transparent).Radius(4),
		style.Name("table-row").State(style.Hovered).BackgroundColor(color.RGBA{R: 235, G: 235, B: 235, A: 255}),
		style.Name("table-row").State(style.Pressed).BackgroundColor(color.RGBA{R: 220, G: 220, B: 220, A: 255}),
		style.Name("table-row").Part("selected").BackgroundColor(color.RGBA{R: 220, G: 220, B: 220, A: 255}),
		style.Name("table-row").Part("selected").State(style.Hovered).BackgroundColor(color.RGBA{R: 210, G: 210, B: 210, A: 255}),
		style.Name("table-row").Part("selected").State(style.Pressed).BackgroundColor(color.RGBA{R: 200, G: 200, B: 200, A: 255}),
		style.Name("table-row").Part("current").BackgroundColor(color.Transparent).BorderColor(color.RGBA{R: 100, G: 110, B: 130, A: 255}).BorderWidth(1),
		style.Name("tree-view").BackgroundColor(color.Transparent),
		style.Name("tree-item").BackgroundColor(color.Transparent).Radius(4),
		style.Name("tree-item").State(style.Hovered).BackgroundColor(color.RGBA{R: 235, G: 235, B: 235, A: 255}),
		style.Name("tree-item").State(style.Pressed).BackgroundColor(color.RGBA{R: 220, G: 220, B: 220, A: 255}),
		style.Name("tree-item").Part("selected").BackgroundColor(color.RGBA{R: 210, G: 218, B: 230, A: 255}),
		style.Name("tree-item").Part("selected").State(style.Hovered).BackgroundColor(color.RGBA{R: 199, G: 209, B: 223, A: 255}),
		style.Name("tree-item").Part("selected").State(style.Pressed).BackgroundColor(color.RGBA{R: 188, G: 200, B: 216, A: 255}),
		style.Name("tree-item").Part("current").BackgroundColor(color.Transparent).BorderColor(color.RGBA{R: 100, G: 110, B: 130, A: 255}).BorderWidth(1),
		style.Name("tree-expander").ForegroundColor(color.Black).BackgroundColor(color.Transparent).Radius(4),
		style.Name("tree-expander").State(style.Hovered).BackgroundColor(color.RGBA{R: 235, G: 235, B: 235, A: 255}),
		style.Name("tree-expander").State(style.Pressed).BackgroundColor(color.RGBA{R: 220, G: 220, B: 220, A: 255}),
		style.Name("tree-item-text").ForegroundColor(color.Black).FontFamily(family).FontSize(size),
		style.Name("tree-item-icon").ForegroundColor(color.Black),
		style.Name("tab-bar").Part("separator").ForegroundColor(color.RGBA{R: 198, G: 198, B: 206, A: 255}),
		style.Name("tab-item").BackgroundColor(color.Transparent).Radius(6),
		style.Name("tab-item").State(style.Hovered).BackgroundColor(color.RGBA{R: 230, G: 230, B: 230, A: 255}),
		style.Name("tab-item").Part("selected").BackgroundColor(color.RGBA{R: 210, G: 210, B: 210, A: 255}),
		style.Name("tab-item").Part("dragging").BackgroundColor(color.White).BorderColor(color.RGBA{R: 192, G: 192, B: 192, A: 255}).BorderWidth(1),
		style.Name("tab-item-text").ForegroundColor(color.Black).FontFamily(family).FontSize(size),
		style.Name("tab-item-text-selected").ForegroundColor(color.Black).FontFamily(family).FontSize(size),
		style.Name("tab-item-icon").ForegroundColor(color.Black),
		style.Name("tab-item-icon-selected").ForegroundColor(color.Black),
		style.Name("tab-close-button").BackgroundColor(color.Transparent).Radius(4),
		style.Name("tab-close-button-text").ForegroundColor(color.Black).FontFamily(family).FontSize(size),
		style.Name("tab-scroll-button").BackgroundColor(color.Transparent).Radius(4).FontSize(size),
		style.Name("tab-scroll-button-text").ForegroundColor(color.Black).FontFamily(family).FontSize(size),
		style.Name("split-handle").ForegroundColor(color.RGBA{R: 198, G: 198, B: 206, A: 255}),
	}
	rules = append(rules, checkButtonRules()...)
	rules = append(rules, dropDownRules()...)
	rules = append(rules, switchRules()...)
	for _, name := range []string{"check-button-text", "switch-text", "drop-down-text", "drop-down-placeholder",
		"table-header-text", "table-cell-text", "table-cell-icon", "tree-item-text", "tree-item-icon",
		"tab-item-text", "tab-item-text-selected", "tab-item-icon", "tab-item-icon-selected",
		"tab-close-button-text", "tab-scroll-button-text"} {
		rules = append(rules, style.Name(name).State(style.Disabled).
			ForegroundColor(color.RGBA{R: 140, G: 140, B: 140, A: 255}))
	}
	return rules
}
