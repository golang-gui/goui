package basic

import (
	"image/color"

	"github.com/golang-gui/goui/style"
)

func listRowRules(family string, size float32) []style.Rule {
	return []style.Rule{
		style.Name("list-row").BackgroundColor(color.Transparent).BorderWidth(0).Radius(6),
		style.Name("list-row").State(style.Hovered).BackgroundColor(color.RGBA{R: 235, G: 235, B: 237, A: 255}),
		style.Name("list-row").State(style.Pressed).BackgroundColor(color.RGBA{R: 220, G: 220, B: 223, A: 255}),
		style.Name("list-row").State(style.Disabled).BackgroundColor(color.Transparent),
		style.Name("list-row").Part("focus").BackgroundColor(color.Transparent).BorderWidth(0).Radius(6),
		style.Name("list-row").Part("focus").State(style.FocusVisible).BorderColor(color.RGBA{R: 100, G: 145, B: 210, A: 255}).BorderWidth(2),
		style.Name("list-row-title").ForegroundColor(color.Black).FontFamily(family).FontSize(size),
		style.Name("list-row-subtitle").ForegroundColor(color.RGBA{R: 100, G: 100, B: 108, A: 255}).FontFamily(family).FontSize(size),
		style.Name("list-row-title").State(style.Disabled).ForegroundColor(color.RGBA{R: 140, G: 140, B: 140, A: 255}),
		style.Name("list-row-subtitle").State(style.Disabled).ForegroundColor(color.RGBA{R: 140, G: 140, B: 140, A: 255}),
	}
}
