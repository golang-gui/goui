package style

import (
	"image/color"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	basestyle "github.com/golang-gui/goui/style"
)

func dropDownRules() []basestyle.Rule {
	label := gui.DefaultStyleSheet().Resolve(basestyle.Sel{Name: "label"})
	family, _ := label.FontFamily()
	size, _ := label.FontSize()
	text, muted, edge := color.RGBA{A: 255}, color.RGBA{R: 130, G: 130, B: 130, A: 255}, color.RGBA{R: 192, G: 192, B: 192, A: 255}
	rules := []basestyle.Rule{
		basestyle.Name("drop-down").BackgroundColor(color.RGBA{R: 245, G: 245, B: 245, A: 255}).BorderColor(edge).BorderWidth(1).Radius(6),
		basestyle.Name("drop-down").State(basestyle.Hovered).BackgroundColor(color.RGBA{R: 235, G: 235, B: 235, A: 255}),
		basestyle.Name("drop-down").State(basestyle.Pressed).BackgroundColor(color.RGBA{R: 225, G: 225, B: 225, A: 255}),
		basestyle.Name("drop-down").Part("arrow").ForegroundColor(text),
		basestyle.Name("drop-down").Part("arrow").State(basestyle.Disabled).ForegroundColor(muted),
		basestyle.Name("drop-down").Part("focus").BackgroundColor(color.Transparent).BorderWidth(0).Radius(6),
		basestyle.Name("drop-down").Part("focus").State(basestyle.FocusVisible).BorderWidth(2).BorderColor(color.RGBA{R: 100, G: 145, B: 210, A: 255}),
		basestyle.Name("drop-down-popup").BackgroundColor(color.White).BorderColor(edge).BorderWidth(1).Radius(8).
			Shadow(basestyle.Shadow{Color: color.NRGBA{A: 46}, Offset: geometry.Point{Y: 4}, BlurRadius: 16}),
		basestyle.Name("drop-down-item").BackgroundColor(color.Transparent).BorderWidth(0).Radius(4),
		basestyle.Name("drop-down-item").State(basestyle.Hovered).BackgroundColor(color.RGBA{R: 235, G: 235, B: 235, A: 255}),
		basestyle.Name("drop-down-item").State(basestyle.Pressed).BackgroundColor(color.RGBA{R: 220, G: 220, B: 220, A: 255}),
		basestyle.Name("drop-down-item").Part("selected").BackgroundColor(color.RGBA{R: 225, G: 225, B: 225, A: 255}),
		basestyle.Name("drop-down-item").Part("selected").State(basestyle.Hovered).BackgroundColor(color.RGBA{R: 215, G: 215, B: 215, A: 255}),
		basestyle.Name("drop-down-item").Part("check").ForegroundColor(text),
		basestyle.Name("drop-down-item").Part("check").State(basestyle.Disabled).ForegroundColor(muted),
	}
	for _, name := range []string{"drop-down-text", "drop-down-item-text", "drop-down-text-disabled", "drop-down-item-text-disabled", "drop-down-placeholder"} {
		fg := text
		if name == "drop-down-text-disabled" || name == "drop-down-item-text-disabled" || name == "drop-down-placeholder" {
			fg = muted
		}
		rules = append(rules, basestyle.Name(name).FontFamily(family).FontSize(size).ForegroundColor(fg))
	}
	return rules
}
