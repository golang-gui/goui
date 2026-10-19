// Package basic provides the fallback appearance for GOUI widgets.
// It contains style values only and does not depend on GUI or widget types.
package basic

import (
	"image/color"
	"runtime"

	"github.com/golang-gui/goui/style"
)

const (
	styleNameWindow               = "window"
	styleNamePopover              = "popover"
	styleNameWidget               = "widget"
	styleNameLabel                = "label"
	styleNameIcon                 = "icon"
	styleNameButton               = "button"
	styleNameTextInput            = "text-input"
	styleNameTextView             = "text-view"
	styleNameScrollView           = "scroll-view"
	stylePartTrough               = "trough"
	stylePartThumb                = "thumb"
	stylePartFocus                = "focus"
	styleNameMenu                 = "menu"
	styleNameMenuItem             = "menu-item"
	styleNameMenuItemText         = "menu-item-text"
	styleNameMenuItemTextDisabled = "menu-item-text-disabled"
	styleNameMenuSeparator        = "menu-separator"
)

const defaultFontSize float32 = 14
const scrollbarRadius float32 = 4

var defaultAccentColor color.Color = color.RGBA{R: 70, G: 130, B: 220, A: 255}

func defaultLabelFontFamily() string {
	switch runtime.GOOS {
	case "windows":
		return "Segoe UI"
	case "linux":
		return "sans-serif"
	default:
		return ""
	}
}

// Rules returns fresh rules for built-in controls followed by registered controls.
// The first call freezes Basic registration.
func Rules() []style.Rule {
	return registeredRules(registered.snapshot())
}

func registeredRules(entries []basicRegistration) []style.Rule {
	rules := append(coreRules(), WidgetRules()...)
	base := style.Sheet(rules...)
	for _, entry := range entries {
		rules = append(rules, entry.rules(base)...)
	}
	return rules
}

func coreRules() []style.Rule {
	accent := defaultAccentColor
	family := defaultLabelFontFamily()
	size := defaultFontSize

	return []style.Rule{
		style.Name("border-box").BackgroundColor(color.Transparent).
			BorderColor(color.Transparent).BorderWidth(0).Radius(0),
		style.Name(styleNameWindow).BackgroundColor(color.White),
		style.Name(styleNamePopover).BackgroundColor(color.White),
		style.Name(styleNameIcon).ForegroundColor(color.Black),
		style.Name(styleNameWidget).
			BackgroundColor(color.Transparent).
			ForegroundColor(color.Black).
			BorderColor(color.Transparent).
			BorderWidth(0).
			Radius(0).
			FontFamily(family).
			FontSize(size),
		style.Name(styleNameWidget).State(style.Disabled).
			ForegroundColor(color.RGBA{R: 140, G: 140, B: 140, A: 255}),

		style.Name(styleNameLabel).
			ForegroundColor(color.Black).
			FontFamily(family).
			FontSize(size),
		style.Name(styleNameLabel).State(style.Disabled).
			ForegroundColor(color.RGBA{R: 140, G: 140, B: 140, A: 255}),
		style.Name(styleNameIcon).State(style.Disabled).
			ForegroundColor(color.RGBA{R: 140, G: 140, B: 140, A: 255}),

		style.Name(styleNameButton).
			BackgroundColor(color.RGBA{R: 210, G: 210, B: 210, A: 255}).
			ForegroundColor(color.Black).
			BorderColor(color.Transparent).
			BorderWidth(0).
			Radius(4).
			FontFamily(family).
			FontSize(size),
		style.Name(styleNameButton).
			State(style.Hovered).
			BackgroundColor(color.RGBA{R: 230, G: 230, B: 230, A: 255}),
		style.Name(styleNameButton).
			State(style.Pressed).
			BackgroundColor(color.RGBA{R: 180, G: 180, B: 180, A: 255}),
		style.Name(styleNameButton).State(style.Disabled).
			ForegroundColor(color.RGBA{R: 140, G: 140, B: 140, A: 255}),
		style.Name(styleNameButton).Part(stylePartFocus).State(style.Focused).
			BackgroundColor(color.Transparent).BorderWidth(0),
		style.Name(styleNameButton).Part(stylePartFocus).State(style.FocusVisible).
			BackgroundColor(color.Transparent).
			BorderColor(color.RGBA{R: 102, G: 147, B: 217, A: 255}).BorderWidth(2),

		style.Name(styleNameTextInput).
			BackgroundColor(color.White).
			ForegroundColor(color.Black).
			BorderColor(color.RGBA{R: 180, G: 180, B: 180, A: 255}).
			BorderWidth(1).
			Radius(0).
			FontFamily(family).
			FontSize(size),
		style.Name(styleNameTextInput).
			State(style.Focused).
			BorderColor(accent),
		style.Name(styleNameTextInput).State(style.Disabled).
			BackgroundColor(color.RGBA{R: 245, G: 245, B: 245, A: 255}).
			ForegroundColor(color.RGBA{R: 140, G: 140, B: 140, A: 255}),
		style.Name(styleNameTextInput).Part("selection").BackgroundColor(color.RGBA{R: 185, G: 210, B: 245, A: 255}),
		style.Name(styleNameTextInput).Part("placeholder").ForegroundColor(color.RGBA{R: 117, G: 117, B: 117, A: 255}),
		style.Name(styleNameTextInput).Part("placeholder").State(style.Disabled).ForegroundColor(color.RGBA{R: 140, G: 140, B: 140, A: 255}),
		style.Name(styleNameTextView).
			BackgroundColor(color.White).
			ForegroundColor(color.Black).
			FontFamily(family).FontSize(size),
		style.Name(styleNameTextView).Part("selection").BackgroundColor(color.RGBA{R: 185, G: 210, B: 245, A: 255}),
		style.Name(styleNameTextView).Part("caret").ForegroundColor(color.Black),
		style.Name(styleNameTextView).Part("placeholder").ForegroundColor(color.RGBA{R: 117, G: 117, B: 117, A: 255}),
		style.Name(styleNameTextView).Part("placeholder").State(style.Disabled).ForegroundColor(color.RGBA{R: 140, G: 140, B: 140, A: 255}),
		style.Name(styleNameTextView).State(style.Disabled).
			ForegroundColor(color.RGBA{R: 140, G: 140, B: 140, A: 255}),

		style.Name(styleNameScrollView).
			BackgroundColor(color.Transparent).
			ForegroundColor(color.RGBA{R: 120, G: 120, B: 120, A: 255}),

		// ScrollBar parts (resolved under the same "scroll-view" name; ScrollView
		// and ScrollBar are one unit in the style system). fallbackChain lets
		// these inherit the base scroll-view rule above.
		style.Name(styleNameScrollView).Part(stylePartTrough).
			BackgroundColor(color.RGBA{R: 0, G: 0, B: 0, A: 24}).
			Radius(scrollbarRadius),
		style.Name(styleNameScrollView).Part(stylePartThumb).
			BackgroundColor(color.RGBA{R: 140, G: 140, B: 140, A: 255}).
			Radius(scrollbarRadius),
		style.Name(styleNameScrollView).Part(stylePartThumb).State(style.Hovered).
			BackgroundColor(color.RGBA{R: 110, G: 110, B: 110, A: 255}),
		style.Name(styleNameScrollView).Part(stylePartThumb).State(style.Pressed).
			BackgroundColor(color.RGBA{R: 90, G: 90, B: 90, A: 255}),

		style.Name(styleNameMenu).
			BackgroundColor(color.RGBA{R: 250, G: 250, B: 250, A: 255}).
			ForegroundColor(color.Black).
			BorderColor(color.RGBA{R: 200, G: 200, B: 200, A: 255}).
			BorderWidth(1).
			Radius(4),
		style.Name(styleNameMenuItem).
			BackgroundColor(color.Transparent),
		style.Name(styleNameMenuItemText).
			FontFamily(family).FontSize(size).ForegroundColor(color.Black),
		style.Name(styleNameMenuItemTextDisabled).
			FontFamily(family).FontSize(size).
			ForegroundColor(color.RGBA{R: 160, G: 160, B: 160, A: 255}),
		style.Name(styleNameMenuItem).
			State(style.Hovered).
			BackgroundColor(color.RGBA{R: 225, G: 232, B: 240, A: 255}),
		style.Name(styleNameMenuItem).
			State(style.Pressed).
			BackgroundColor(color.RGBA{R: 200, G: 210, B: 225, A: 255}),
		style.Name(styleNameMenuSeparator).
			BackgroundColor(color.RGBA{R: 200, G: 200, B: 200, A: 255}),
	}
}

// Sheet returns an independently constructed, complete fallback stylesheet.
func Sheet() style.StyleSheet { return style.Sheet(Rules()...) }
