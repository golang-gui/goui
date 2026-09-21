// Package modern builds an explicitly enabled, platform-independent stylesheet.
// It does not query system settings or subscribe to changes. Applications pass
// settings into Options and replace their stylesheet when those values change.
package modern

import (
	"image/color"
	"math"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/style"
)

const (
	// Primary is a tinted button style. Its child Label keeps the normal text
	// color: GOUI styles do not implicitly inherit through the widget tree.
	Primary = "primary"
	// MutedText is a secondary text style for Label and other text widgets.
	MutedText = "muted-text"
	// AccentIcon uses a contrast-adjusted accent foreground for drawing icons.
	AccentIcon = "accent-icon"
)

// Options contains resolved appearance values, not system-following policies.
// The zero value produces a light sheet with a blue accent and 14 pt text.
type Options struct {
	Dark bool
	// AccentColor is copied and made opaque. Nil or fully transparent uses
	// #4682DC; a partially transparent color supplies its unpremultiplied RGB.
	AccentColor color.Color
	// FontFamily may be empty to use the text backend's default font.
	FontFamily string
	// FontSize is in points. Non-positive or non-finite values use 14 pt.
	FontSize float32
}

// Sheet returns a complete sheet; GUI fallback rules need not be appended.
func Sheet(options Options) style.StyleSheet { return style.Sheet(Rules(options)...) }

// Rules returns independently owned rules. Append application rules to override
// specific selectors, then pass the result to style.Sheet. No window decoration
// selectors are supplied: window styling controls only its background.
func Rules(options Options) []style.Rule {
	p := colorsFor(options.Dark)
	accent := accentColor(options.AccentColor)
	size := options.FontSize
	if size <= 0 || math.IsNaN(float64(size)) || math.IsInf(float64(size), 0) {
		size = 14
	}
	text := func(name string) style.Rule {
		return style.Name(name).ForegroundColor(p.text).FontFamily(options.FontFamily).FontSize(size)
	}
	buttonHover, buttonPressed := mix(p.button, p.text, .04), mix(p.button, p.text, .08)
	primary := [3]color.RGBA{
		tintedBackground(p.button, accent, p.text, .12),
		tintedBackground(p.button, accent, p.text, .18),
		tintedBackground(p.button, accent, p.text, .24),
	}
	focus := focusColor(accent, p.text, []color.RGBA{
		p.window, p.surface, p.button, buttonHover, buttonPressed,
		primary[0], primary[1], primary[2],
	})
	rules := []style.Rule{
		style.Name("window").BackgroundColor(p.window),
		text("widget").BackgroundColor(color.Transparent).BorderWidth(0).Radius(0),
		text("label"),
		style.Name("icon").ForegroundColor(p.text),
		style.Name(AccentIcon).ForegroundColor(focus),
		text("label").State(style.Disabled).ForegroundColor(p.disabled),
		text(MutedText).ForegroundColor(p.muted),
		text("header-bar").BackgroundColor(p.window),
		text("text-input").BackgroundColor(p.surface).BorderColor(p.border).BorderWidth(1).Radius(6),
		style.Name("text-input").State(style.Focused).BorderColor(focus).BorderWidth(2),
		style.Name("text-input").State(style.Disabled).ForegroundColor(p.disabled),
		text("text-view").BackgroundColor(p.surface),
		style.Name("text-input").Part("selection").BackgroundColor(mix(p.surface, accent, .25)),
		style.Name("text-view").Part("selection").BackgroundColor(mix(p.surface, accent, .25)),
		style.Name("text-input").Part("caret").ForegroundColor(p.text),
		style.Name("text-view").Part("caret").ForegroundColor(p.text),
		text("scroll-view").BackgroundColor(color.Transparent),
		style.Name("scroll-view").Part("trough").BackgroundColor(mix(p.window, p.text, .08)).Radius(4),
		style.Name("scroll-view").Part("thumb").BackgroundColor(mix(p.window, p.text, .42)).Radius(4),
		style.Name("scroll-view").Part("thumb").State(style.Hovered).BackgroundColor(mix(p.window, p.text, .55)),
		style.Name("scroll-view").Part("thumb").State(style.Pressed).BackgroundColor(mix(p.window, p.text, .65)),
		text("popover").BackgroundColor(p.surface).BorderColor(p.border).BorderWidth(1).Radius(8).Shadow(style.Shadow{}),
		text("menu").BackgroundColor(p.surface).BorderColor(p.border).BorderWidth(1).Radius(8).
			Shadow(style.Shadow{Color: color.NRGBA{A: 46}, Offset: geometry.Point{Y: 4}, BlurRadius: 16}),
		style.Name("menu-item").BackgroundColor(color.Transparent).Radius(4),
		text("menu-item-text"),
		text("menu-item-text-disabled").ForegroundColor(p.disabled),
		style.Name("menu-item").State(style.Hovered).BackgroundColor(p.menuHover),
		style.Name("menu-item").State(style.Pressed).BackgroundColor(p.menuPressed),
		style.Name("menu-separator").BackgroundColor(p.border),
	}
	for _, button := range []struct {
		name string
		bg   [3]color.RGBA
	}{
		{"button", [3]color.RGBA{p.button, buttonHover, buttonPressed}},
		{Primary, primary},
	} {
		rules = append(rules,
			text(button.name).BackgroundColor(button.bg[0]).BorderColor(p.border).BorderWidth(1).Radius(6),
			style.Name(button.name).State(style.Hovered).BackgroundColor(button.bg[1]),
			style.Name(button.name).State(style.Pressed).BackgroundColor(button.bg[2]),
			style.Name(button.name).State(style.Disabled).ForegroundColor(p.disabled),
			// Focus painting remains available, but is disabled by default.
			style.Name(button.name).Part("focus").State(style.Focused).BorderColor(focus).BorderWidth(0),
		)
	}
	return rules
}

type palette struct {
	window, surface, button, text, muted, disabled, border color.RGBA
	menuHover, menuPressed                                 color.RGBA
}

func colorsFor(dark bool) palette {
	if dark {
		return palette{
			window: rgb(0x202024), surface: rgb(0x28282D), button: rgb(0x323238),
			text: rgb(0xECECEE), muted: rgb(0xB8B8C2), disabled: rgb(0x797983), border: rgb(0x55555F),
			menuHover: rgb(0x3E3E42), menuPressed: rgb(0x4F4F53),
		}
	}
	return palette{
		window: rgb(0xF6F6F8), surface: rgb(0xFFFFFF), button: rgb(0xFFFFFF),
		text: rgb(0x202024), muted: rgb(0x61616B), disabled: rgb(0x97979F), border: rgb(0xC6C6CE),
		menuHover: rgb(0xE6E6E6), menuPressed: rgb(0xD1D1D1),
	}
}

func rgb(hex uint32) color.RGBA {
	return color.RGBA{R: uint8(hex >> 16), G: uint8(hex >> 8), B: uint8(hex), A: 255}
}

func accentColor(c color.Color) color.RGBA {
	if c == nil {
		return rgb(0x4682DC)
	}
	r, g, b, a := c.RGBA()
	if a == 0 {
		return rgb(0x4682DC)
	}
	return color.RGBA{R: uint8((r*255 + a/2) / a), G: uint8((g*255 + a/2) / a), B: uint8((b*255 + a/2) / a), A: 255}
}

// mix blends in sRGB and stores a quantized, opaque value. Contrast checks use
// that same stored value, rather than an idealized unquantized intermediate.
func mix(base, tint color.RGBA, amount float64) color.RGBA {
	channel := func(a, b uint8) uint8 { return uint8(math.Round(float64(a)*(1-amount) + float64(b)*amount)) }
	return color.RGBA{channel(base.R, tint.R), channel(base.G, tint.G), channel(base.B, tint.B), 255}
}

func luminance(c color.RGBA) float64 {
	linear := func(v uint8) float64 {
		s := float64(v) / 255
		if s <= .04045 {
			return s / 12.92
		}
		return math.Pow((s+.055)/1.055, 2.4)
	}
	return .2126*linear(c.R) + .7152*linear(c.G) + .0722*linear(c.B)
}

func contrast(a, b color.RGBA) float64 {
	x, y := luminance(a), luminance(b)
	return (max(x, y) + .05) / (min(x, y) + .05)
}

func tintedBackground(base, accent, text color.RGBA, amount float64) color.RGBA {
	for step := 255; step >= 0; step-- {
		bg := mix(base, accent, amount*float64(step)/255)
		if contrast(bg, text) >= 4.5 {
			return bg
		}
	}
	return base
}

func focusColor(accent, text color.RGBA, backgrounds []color.RGBA) color.RGBA {
	for step := 0; step <= 255; step++ {
		candidate := mix(accent, text, float64(step)/255)
		valid := true
		for _, bg := range backgrounds {
			if contrast(candidate, bg) < 3 {
				valid = false
				break
			}
		}
		if valid {
			return candidate
		}
	}
	return text
}
