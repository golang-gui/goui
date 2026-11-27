package main

import (
	"image/color"

	"github.com/golang-gui/goui/style"
	"github.com/golang-gui/goui/theme/modern"
)

// The player has one light design. Album colors tint the drawn surfaces; the
// controls stay neutral so they read the same over every album.
var (
	inkColor    = rgb(0x1D1D1F)
	mutedColor  = rgb(0x6E6E73)
	likedColor  = rgb(0xFF375F)
	vinylColor  = rgb(0x141416)
	spindleTone = rgb(0xF2F2F5)
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

func white(a uint8) color.NRGBA { return color.NRGBA{R: 255, G: 255, B: 255, A: a} }

// Style names owned by this example.
const (
	styleTitle      = "music-title"
	styleArtist     = "music-artist"
	styleTime       = "music-time"
	stylePanel      = "music-panel"
	stylePanelTitle = "music-panel-title"
	styleCount      = "music-count"
	styleRow        = "music-row"
	styleRowCurrent = "music-row-current"
	styleLength     = "music-length"
	styleIcon       = "music-icon"
	styleLikedIcon  = "music-liked-icon"
	styleIconButton = "music-icon-button"
	stylePlay       = "music-play"
	stylePlayIcon   = "music-play-icon"
	styleClear      = "music-clear"
)

func styleRules(family string) []style.Rule {
	rules := modern.Rules(modern.Options{AccentColor: inkColor, FontFamily: family, FontSize: 14})
	text := func(name string, c color.Color, size float32) style.Rule {
		return style.Name(name).ForegroundColor(c).FontFamily(family).FontSize(size)
	}
	// Quiet buttons: no resting surface, a soft hover fill and a focus ring.
	focus := func(name string, radius float32) []style.Rule {
		return []style.Rule{
			style.Name(name).Part("focus").BackgroundColor(color.Transparent).BorderWidth(0).Radius(radius),
			style.Name(name).Part("focus").State(style.FocusVisible).BorderColor(inkColor).BorderWidth(2),
		}
	}
	rules = append(rules, focus(styleIconButton, 22)...)
	rules = append(rules, focus(stylePlay, 32)...)
	rules = append(rules, focus(styleRow, 12)...)
	rules = append(rules, focus(styleRowCurrent, 12)...)
	return append(rules,
		style.Name("window").BackgroundColor(rgb(0xF5F5F7)),
		style.Name("header-bar").BackgroundColor(color.Transparent).BorderWidth(0),
		style.Name(styleClear).BackgroundColor(color.Transparent),
		text("label", inkColor, 14),
		text(styleTitle, inkColor, 26).FontWeight(700),
		text(styleArtist, mutedColor, 15),
		text(styleTime, mutedColor, 12),
		text(stylePanelTitle, inkColor, 17).FontWeight(700),
		text(styleCount, mutedColor, 13),
		text(styleLength, mutedColor, 13),
		text("list-row-title", inkColor, 14).FontWeight(600),
		text("list-row-subtitle", mutedColor, 13),
		// The queue is a frosted sheet over the ambient light.
		style.Name(stylePanel).BackgroundColor(white(150)).BorderColor(white(210)).BorderWidth(1).Radius(22),
		style.Name(styleRow).BackgroundColor(color.Transparent).BorderWidth(0).Radius(12),
		style.Name(styleRow).State(style.Hovered).BackgroundColor(white(140)).BorderWidth(0),
		style.Name(styleRow).State(style.Pressed).BackgroundColor(white(200)).BorderWidth(0),
		style.Name(styleRowCurrent).BackgroundColor(white(235)).BorderWidth(0).Radius(12),
		style.Name(styleRowCurrent).State(style.Hovered).BackgroundColor(white(235)).BorderWidth(0),
		style.Name(styleRowCurrent).State(style.Pressed).BackgroundColor(white(235)).BorderWidth(0),
		style.Name(styleIcon).ForegroundColor(inkColor),
		style.Name(styleLikedIcon).ForegroundColor(likedColor),
		style.Name(styleIconButton).BackgroundColor(color.Transparent).BorderWidth(0).Radius(22),
		style.Name(styleIconButton).State(style.Hovered).BackgroundColor(withAlpha(inkColor, 14)).BorderWidth(0),
		style.Name(styleIconButton).State(style.Pressed).BackgroundColor(withAlpha(inkColor, 28)).BorderWidth(0),
		style.Name(stylePlay).BackgroundColor(inkColor).BorderWidth(0).Radius(32),
		style.Name(stylePlay).State(style.Hovered).BackgroundColor(mix(inkColor, rgb(0xFFFFFF), .14)).BorderWidth(0),
		style.Name(stylePlay).State(style.Pressed).BackgroundColor(mix(inkColor, rgb(0xFFFFFF), .24)).BorderWidth(0),
		style.Name(stylePlayIcon).ForegroundColor(color.White),
		style.Name("scroll-view").Part("trough").BackgroundColor(color.Transparent),
	)
}
