package main

import (
	"image/color"

	"github.com/golang-gui/goui/style"
	"github.com/golang-gui/goui/theme/modern"
)

// palette is the monitor's own fixed design, independent of system accents.
type palette struct {
	window, surface, border, text, muted, accent, danger color.NRGBA
}

var (
	darkPalette = palette{
		window: rgb(0x111318), surface: rgb(0x1A1D23), border: rgb(0x2A2D35),
		text: rgb(0xD4D8E0), muted: rgb(0x7A8094), accent: rgb(0x3DD68C), danger: rgb(0xE05C5C),
	}
	lightPalette = palette{
		window: rgb(0xF4F5F7), surface: rgb(0xFFFFFF), border: rgb(0xE3E5EA),
		text: rgb(0x1C1F26), muted: rgb(0x6B7180), accent: rgb(0x1FA971), danger: rgb(0xD64545),
	}
)

func rgb(hex uint32) color.NRGBA {
	return color.NRGBA{R: uint8(hex >> 16), G: uint8(hex >> 8), B: uint8(hex), A: 255}
}

func withAlpha(c color.NRGBA, a uint8) color.NRGBA { c.A = a; return c }

// mix blends two opaque colors in sRGB.
func mix(a, b color.NRGBA, t float32) color.NRGBA {
	f := func(x, y uint8) uint8 { return uint8(float32(x)*(1-t) + float32(y)*t + .5) }
	return color.NRGBA{R: f(a.R, b.R), G: f(a.G, b.G), B: f(a.B, b.B), A: 255}
}

// Style names owned by this example.
const (
	styleCard       = "mon-card"
	styleTitle      = "mon-title"
	styleCaption    = "mon-caption"
	styleMuted      = "mon-muted"
	styleValue      = "mon-value"
	styleIcon       = "mon-icon"
	styleIconButton = "mon-icon-button"
	styleCell       = "mon-cell"
	styleCellMuted  = "mon-cell-muted"
	styleGauge      = "mon-gauge"
	styleGaugeHigh  = "mon-gauge-high"
	styleMeter      = "mon-meter"
	styleBar        = "mon-bar"
	styleBarHigh    = "mon-bar-high"
)

func styleRules(p palette, dark bool, family string) []style.Rule {
	rules := modern.Rules(modern.Options{Dark: dark, AccentColor: p.accent, FontFamily: family, FontSize: 13})
	text := func(name string, c color.Color, size float32) style.Rule {
		return style.Name(name).ForegroundColor(c).FontFamily(family).FontSize(size)
	}
	hover, selected := mix(p.surface, p.text, .04), mix(p.surface, p.text, .08)
	return append(rules,
		style.Name("window").BackgroundColor(p.window),
		style.Name("header-bar").BackgroundColor(p.window),
		text("label", p.text, 13),
		text(modern.Flat, p.text, 13),
		style.Name(styleCard).BackgroundColor(p.surface).BorderColor(p.border).BorderWidth(1).Radius(10),
		text(styleTitle, p.text, 14).FontWeight(600),
		text(styleCaption, p.muted, 12).FontWeight(500),
		text(styleMuted, p.muted, 12),
		text(styleValue, p.text, 26).FontWeight(600),
		// Icon-only button: no resting surface or border, a quiet hover fill.
		style.Name(styleIcon).ForegroundColor(p.muted),
		style.Name(styleIcon).State(style.Hovered).ForegroundColor(p.text),
		style.Name(styleIconButton).BackgroundColor(color.Transparent).BorderWidth(0).Radius(10),
		style.Name(styleIconButton).State(style.Hovered).BackgroundColor(selected).BorderWidth(0),
		style.Name(styleIconButton).State(style.Pressed).BackgroundColor(mix(selected, p.text, .06)).BorderWidth(0),
		style.Name(styleIconButton).Part("focus").BackgroundColor(color.Transparent).BorderWidth(0).Radius(10),
		style.Name(styleIconButton).Part("focus").State(style.FocusVisible).BorderColor(p.accent).BorderWidth(2),
		text(styleCell, p.text, 13),
		text(styleCellMuted, p.muted, 13),
		style.Name(styleGauge).BackgroundColor(p.border).ForegroundColor(p.accent),
		style.Name(styleGaugeHigh).BackgroundColor(p.border).ForegroundColor(p.danger),
		style.Name(styleMeter).BackgroundColor(p.border).ForegroundColor(p.accent).Radius(3),
		style.Name(styleBar).BackgroundColor(p.border).ForegroundColor(withAlpha(p.accent, 160)).Radius(2),
		style.Name(styleBarHigh).BackgroundColor(p.border).ForegroundColor(p.danger).Radius(2),
		// The single process table sits inside a card: no frame, quiet rules.
		style.Name("table-view").BackgroundColor(p.surface).BorderWidth(0),
		style.Name("table-view").Part("trough").BackgroundColor(p.surface),
		style.Name("table-view").Part("corner").BackgroundColor(p.surface),
		style.Name("table-view").Part("header").BackgroundColor(p.surface),
		style.Name("table-view").Part("header").State(style.Hovered).BackgroundColor(hover),
		style.Name("table-view").Part("header").State(style.Pressed).BackgroundColor(selected),
		style.Name("table-view").Part("header-text").ForegroundColor(p.muted).FontFamily(family).FontSize(12),
		style.Name("table-view").Part("header-separator").ForegroundColor(p.border),
		style.Name("table-view").Part("header-sort").ForegroundColor(p.muted),
		style.Name("table-view").Part("grid-horizontal").BorderColor(p.border),
		style.Name("table-view").Part("grid-bottom").BorderColor(p.border),
		style.Name("table-view").Part("row").State(style.Hovered).BackgroundColor(hover),
		style.Name("table-view").Part("row").State(style.Pressed).BackgroundColor(selected),
		style.Name("table-view").Part("row-selected").BackgroundColor(selected),
		style.Name("table-view").Part("row-selected").State(style.Hovered).BackgroundColor(selected),
	)
}
