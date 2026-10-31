package main

import (
	"image/color"

	"github.com/golang-gui/goui/style"
	"github.com/golang-gui/goui/theme/modern"
	"github.com/golang-gui/goui/ui"
)

func (d *demo) styleRules(settings ui.Settings) []style.Rule {
	dark := d.mode == 2 || d.mode == 0 && settings.ColorScheme() == ui.ColorSchemeDark
	accents := []color.Color{settings.AccentColor(), color.NRGBA{R: 118, G: 74, B: 213, A: 255}, color.NRGBA{R: 30, G: 151, B: 99, A: 255}, color.NRGBA{R: 217, G: 115, B: 36, A: 255}}
	rules := modern.Rules(modern.Options{Dark: dark, AccentColor: accents[d.accent], FontFamily: settings.FontFamily(), FontSize: settings.FontSize()})
	// Copy the ordinary button rules and set a neutral selected background for
	// the active category. The rules are independent values; they introduce no
	// style inheritance and leave the child Label's foreground unchanged.
	for _, rule := range rules {
		if rule.Sel.Name == "button" {
			rule.Sel.Name = "demo-nav-current"
			rules = append(rules, rule)
		}
	}
	selected := [3]color.NRGBA{{R: 208, G: 208, B: 214, A: 255}, {R: 200, G: 200, B: 207, A: 255}, {R: 189, G: 189, B: 197, A: 255}}
	if dark {
		selected = [3]color.NRGBA{{R: 52, G: 52, B: 60, A: 255}, {R: 61, G: 61, B: 69, A: 255}, {R: 69, G: 69, B: 78, A: 255}}
	}
	rules = append(rules,
		style.Name("demo-nav-current").BackgroundColor(selected[0]),
		style.Name("demo-nav-current").State(style.Hovered).BackgroundColor(selected[1]),
		style.Name("demo-nav-current").State(style.Pressed).BackgroundColor(selected[2]),
	)
	fontSize := settings.FontSize()
	if fontSize <= 0 {
		fontSize = 14
	}
	bg, fg := color.NRGBA{R: 245, G: 246, B: 248, A: 255}, color.NRGBA{R: 40, G: 45, B: 52, A: 255}
	if dark {
		bg = color.NRGBA{R: 30, G: 32, B: 36, A: 255}
		fg = color.NRGBA{R: 234, G: 236, B: 239, A: 255}
	}
	rules = append(rules,
		style.Name("demo-background").BackgroundColor(bg),
		style.Name("demo-transparent-window").BackgroundColor(color.Transparent),
		style.Name("demo-title").ForegroundColor(fg).FontSize(fontSize).FontFamily(settings.FontFamily()),
		style.Name("demo-heading").ForegroundColor(fg).FontSize(22).FontFamily(settings.FontFamily()),
		style.Name("demo-subheading").ForegroundColor(fg).FontSize(16).FontFamily(settings.FontFamily()),
		style.Name("demo-mask").BackgroundColor(color.NRGBA{A: 140}),
		style.Name("demo-swatch").ForegroundColor(accents[d.accent]),
		style.Name("demo-cell-edit").ForegroundColor(fg).FontFamily(settings.FontFamily()).FontSize(fontSize).BackgroundColor(color.Transparent).Radius(0).BorderWidth(0),
		style.Name("demo-cell-edit").State(style.Focused).BorderColor(accents[d.accent]).BorderWidth(1),
	)

	return rules
}
