// Package modern builds an explicitly enabled, platform-independent stylesheet.
// It does not query native settings or subscribe to changes. Applications pass
// values through Options or supply a settings snapshot through FromSettings.
package modern

import (
	"image/color"
	"math"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/style"
	"github.com/golang-gui/goui/theme/basic"
)

const (
	// Primary is a solid, borderless accent Button/MenuButton style.
	// Give its child labels and monochrome icons PrimaryContent explicitly;
	// styles do not inherit.
	Primary = "primary"
	// PrimaryContent is the contrasting foreground and typography for content
	// on Primary. It supplies no background, border or layout policy.
	PrimaryContent = "primary-content"
	// MutedText is a secondary text style for Label and other text widgets.
	MutedText = "muted-text"
	// AccentIcon uses a contrast-adjusted accent foreground for drawing icons.
	AccentIcon = "accent-icon"
	// Card is a rounded, subtly bordered surface without a shadow. The widget
	// must paint its own background; layout-only boxes do not become cards.
	Card = "card"
	// Sidebar is a neutral, square surface without a border. It does not style
	// navigation items or supply padding to its children.
	Sidebar = "sidebar"
	// Flat is a complete Button/MenuButton style: transparent and borderless
	// at rest or disabled, with the ordinary button surface and 1 DIP border
	// when hovered or pressed. Keyboard focus uses the independent focus part.
	Flat = "flat"
	// Warning, Error and Success are foreground presets for text and icons,
	// not action-button or input-validation styles. They follow Dark, not AccentColor.
	Warning = "warning"
	Error   = "error"
	Success = "success"
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

// Settings is the appearance subset consumed by FromSettings. Both gui.Settings
// and ui.Settings satisfy it; no language, signal or application is required.
// Calls retain the supplied settings object's threading contract.
type Settings interface {
	ColorScheme() gui.ColorScheme
	AccentColor() color.Color
	FontFamily() string
	FontSize() float32
}

// FromSettings creates a complete Modern stylesheet from the current settings.
// It does not retain settings, install the sheet or subscribe to changes. In a
// ui.Run build, use Root().StyleSheet(modern.FromSettings(app.Settings())); UI
// already rebuilds after settings changes. Imperative GUI applications can pass
// the result to SetStyleSheet and repeat that on Settings.ConnectChanged.
func FromSettings(settings Settings) style.StyleSheet {
	return Sheet(Options{
		Dark:        settings.ColorScheme() == gui.ColorSchemeDark,
		AccentColor: settings.AccentColor(),
		FontFamily:  settings.FontFamily(),
		FontSize:    settings.FontSize(),
	})
}

// Rules returns complete Basic, built-in Modern and registered Modern rules.
// The first call freezes both themes. Each result is independently owned. Append application rules to override
// specific selectors, then pass the result to style.Sheet. No window decoration
// selectors are supplied: window styling controls only its background.
func Rules(options Options) []style.Rule {
	entries := registered.snapshot()
	options = normalizedOptions(options)
	own := builtInRules(options)
	rules := append(basic.Rules(), own...)
	return registeredRules(options, style.Sheet(own...), rules, entries)
}

func registeredRules(options Options, base style.StyleSheet, rules []style.Rule, entries []modernRegistration) []style.Rule {
	for _, entry := range entries {
		rules = append(rules, entry.rules(options, base)...)
	}
	return rules
}

func normalizedOptions(options Options) Options {
	options.AccentColor = accentColor(options.AccentColor)
	if options.FontSize <= 0 || math.IsNaN(float64(options.FontSize)) || math.IsInf(float64(options.FontSize), 0) {
		options.FontSize = 14
	}
	return options
}

func builtInRules(options Options) []style.Rule {
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
	// Tabs use neutral surfaces rather than the accent/focus color. Keep the
	// selected and lifted surfaces opaque when they overlap neighboring tabs.
	tabSelected := p.surface
	if options.Dark {
		tabSelected = mix(p.window, p.text, .08)
	}
	tabBorder := mix(tabSelected, p.text, .10)
	// Tree/table rows share the window's neutral selection surface rather than
	// a pressed menu fill. Borders distinguish structure without competing with it.
	rowSelected := mix(p.window, p.text, .10)
	tableEdge, tableGrid := mix(p.window, p.text, .14), mix(p.window, p.text, .06)
	primary, primaryText := primaryColors(accent)
	focus := focusColor(accent, p.text, []color.RGBA{
		p.window, p.surface, p.button, buttonHover, buttonPressed,
	})
	// Soften the accent towards the palette's neutral border, not white in both
	// themes. The opaque color keeps hover/pressed backgrounds unchanged.
	tabFocus := mix(focus, p.border, .25)
	progressTrack := mix(p.window, p.text, .10)
	rules := []style.Rule{
		style.Name("border-box").BackgroundColor(color.Transparent).
			BorderColor(color.Transparent).BorderWidth(0).Radius(0),
		text("check-button"),
		text("check-button-text"),
		text("switch-text"),
		style.Name("table-view").BackgroundColor(p.window).BorderColor(tableEdge).BorderWidth(1),
		style.Name("table-grid").BorderColor(tableGrid).BorderWidth(0),
		style.Name("table-grid").Part("horizontal").BorderWidth(1),
		style.Name("table-header").BackgroundColor(mix(p.window, p.text, .02)),
		style.Name("table-header").State(style.Hovered).BackgroundColor(mix(p.window, p.text, .05)),
		style.Name("table-header").State(style.Pressed).BackgroundColor(mix(p.window, p.text, .08)),
		style.Name("table-header").Part("separator").ForegroundColor(tableEdge).BorderWidth(1),
		style.Name("table-header").Part("sort").ForegroundColor(p.muted),
		text("table-header-text"),
		text("table-cell-text"),
		style.Name("table-cell-icon").ForegroundColor(p.text),
		style.Name("table-row").BackgroundColor(color.Transparent).BorderWidth(0).Radius(4),
		style.Name("table-row").State(style.Hovered).BackgroundColor(mix(p.window, p.text, .04)),
		style.Name("table-row").State(style.Pressed).BackgroundColor(mix(p.window, p.text, .08)),
		style.Name("table-row").Part("selected").BackgroundColor(rowSelected),
		style.Name("table-row").Part("selected").State(style.Hovered).BackgroundColor(mix(rowSelected, p.text, .04)),
		style.Name("table-row").Part("selected").State(style.Pressed).BackgroundColor(mix(rowSelected, p.text, .08)),
		style.Name("table-row").Part("current").BackgroundColor(color.Transparent).BorderColor(p.muted).BorderWidth(0),
		style.Name("table-row").Part("current").State(style.FocusVisible).BorderWidth(1),
		style.Name("tree-view").BackgroundColor(color.Transparent),
		style.Name("tree-item").BackgroundColor(color.Transparent).BorderWidth(0).Radius(4),
		style.Name("tree-item").State(style.Hovered).BackgroundColor(p.menuHover),
		style.Name("tree-item").State(style.Pressed).BackgroundColor(p.menuPressed),
		style.Name("tree-item").Part("selected").BackgroundColor(rowSelected),
		style.Name("tree-item").Part("selected").State(style.Hovered).BackgroundColor(mix(rowSelected, p.text, .04)),
		style.Name("tree-item").Part("selected").State(style.Pressed).BackgroundColor(mix(rowSelected, p.text, .08)),
		style.Name("tree-item").Part("current").BackgroundColor(color.Transparent).BorderColor(p.muted).BorderWidth(0),
		style.Name("tree-item").Part("current").State(style.FocusVisible).BorderWidth(1),
		style.Name("tree-expander").ForegroundColor(p.muted).BackgroundColor(color.Transparent).Radius(4),
		style.Name("tree-expander").State(style.Hovered).ForegroundColor(p.text).BackgroundColor(p.menuHover),
		style.Name("tree-expander").State(style.Pressed).ForegroundColor(p.text).BackgroundColor(p.menuPressed),
		text("tree-item-text"),
		style.Name("tree-item-icon").ForegroundColor(p.text),
		style.Name("split-handle").ForegroundColor(mix(p.window, p.text, .18)),
		style.Name("window").BackgroundColor(p.window),
		text("widget").BackgroundColor(color.Transparent).BorderWidth(0).Radius(0),
		style.Name("widget").State(style.Disabled).ForegroundColor(p.disabled),
		text("label"),
		text(PrimaryContent).ForegroundColor(primaryText),
		style.Name("icon").ForegroundColor(p.text),
		style.Name("progress-bar").BackgroundColor(progressTrack).
			ForegroundColor(focusColor(accent, p.text, []color.RGBA{progressTrack, p.window})).Radius(2),
		style.Name(AccentIcon).ForegroundColor(focus),
		text("label").State(style.Disabled).ForegroundColor(p.disabled),
		text(MutedText).ForegroundColor(p.muted),
		text(Card).BackgroundColor(p.surface).BorderColor(mix(p.surface, p.text, .12)).
			BorderWidth(1).Radius(10).Shadow(style.Shadow{}),
		text(Sidebar).BackgroundColor(p.sidebar).
			BorderWidth(0).Radius(0).Shadow(style.Shadow{}),
		text(Warning).ForegroundColor(p.warning),
		text(Error).ForegroundColor(p.error),
		text(Success).ForegroundColor(p.success),
		text("header-bar").BackgroundColor(p.window),
		text("text-input").BackgroundColor(p.surface).BorderColor(p.border).BorderWidth(1).Radius(6),
		style.Name("text-input").State(style.Focused).BorderColor(focus).BorderWidth(2),
		style.Name("text-input").State(style.Disabled).ForegroundColor(p.disabled),
		style.Name("text-input").State(style.Disabled).BackgroundColor(p.window),
		text("text-view").BackgroundColor(p.surface),
		style.Name("text-view").State(style.Disabled).ForegroundColor(p.disabled),
		style.Name("text-input").Part("selection").BackgroundColor(mix(p.surface, accent, .25)),
		style.Name("text-view").Part("selection").BackgroundColor(mix(p.surface, accent, .25)),
		style.Name("text-input").Part("caret").ForegroundColor(p.text),
		style.Name("text-view").Part("caret").ForegroundColor(p.text),
		style.Name("text-input").Part("placeholder").ForegroundColor(p.muted),
		style.Name("text-input").Part("placeholder").State(style.Disabled).ForegroundColor(p.disabled),
		style.Name("text-view").Part("placeholder").ForegroundColor(p.muted),
		style.Name("text-view").Part("placeholder").State(style.Disabled).ForegroundColor(p.disabled),
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
		text("tab-bar").BackgroundColor(color.Transparent),
		style.Name("tab-bar").Part("separator").ForegroundColor(mix(p.window, p.text, .18)),
		style.Name("tab-item").BackgroundColor(color.Transparent).BorderWidth(0).Radius(6),
		style.Name("tab-item").State(style.Hovered).BackgroundColor(mix(p.window, p.text, .04)),
		style.Name("tab-item").State(style.Pressed).BackgroundColor(mix(p.window, p.text, .07)),
		style.Name("tab-item").Part("selected").BackgroundColor(tabSelected).BorderColor(tabBorder).BorderWidth(1),
		style.Name("tab-item").Part("selected").State(style.Hovered).BackgroundColor(mix(tabSelected, p.text, .02)),
		style.Name("tab-item").Part("selected").State(style.Pressed).BackgroundColor(mix(tabSelected, p.text, .04)),
		style.Name("tab-item").Part("dragging").BackgroundColor(tabSelected).BorderColor(mix(tabSelected, p.text, .18)).BorderWidth(1),
		text("tab-item-text"),
		text("tab-item-text-selected"),
		style.Name("tab-item-icon").ForegroundColor(p.text),
		style.Name("tab-item-icon-selected").ForegroundColor(p.text),
		style.Name("tab-close-button").BackgroundColor(color.Transparent).Radius(4),
		style.Name("tab-close-button").State(style.Hovered).BackgroundColor(p.menuHover),
		style.Name("tab-close-button").State(style.Pressed).BackgroundColor(p.menuPressed),
		text("tab-close-button-text").ForegroundColor(p.muted),
		style.Name("tab-scroll-button").BackgroundColor(color.Transparent).Radius(4),
		style.Name("tab-scroll-button").State(style.Hovered).BackgroundColor(p.menuHover),
		style.Name("tab-scroll-button").State(style.Pressed).BackgroundColor(p.menuPressed),
		text("tab-scroll-button-text").ForegroundColor(p.muted),
	}
	for _, button := range []struct {
		name string
		bg   [3]color.Color
	}{
		{"button", [3]color.Color{p.button, buttonHover, buttonPressed}},
		{Primary, [3]color.Color{primary[0], primary[1], primary[2]}},
		{Flat, [3]color.Color{color.Transparent, buttonHover, buttonPressed}},
	} {
		borderWidth := float32(1)
		foreground, keyboardBorder := p.text, tabFocus
		var disabledBackground color.Color = p.button
		hoverRule := style.Name(button.name).State(style.Hovered).BackgroundColor(button.bg[1])
		pressedRule := style.Name(button.name).State(style.Pressed).BackgroundColor(button.bg[2])
		if button.name == Flat {
			borderWidth = 0
			disabledBackground = color.Transparent
			// Flat hides only its resting surface, not the button's interaction
			// boundary. Other button styles still inherit their normal border.
			hoverRule = hoverRule.BorderWidth(1)
			pressedRule = pressedRule.BorderWidth(1)
		}
		if button.name == Primary {
			borderWidth = 0 // The solid fill defines the resting boundary.
			foreground = primaryText
			// Only keyboard focus uses a neutral border selected by the mode.
			keyboardBorder = rgb(0x414147)
			if options.Dark {
				keyboardBorder = rgb(0xD8D8DE)
			}
		}
		rules = append(rules,
			text(button.name).ForegroundColor(foreground).BackgroundColor(button.bg[0]).BorderColor(p.border).BorderWidth(borderWidth).Radius(6),
			hoverRule,
			pressedRule,
			style.Name(button.name).State(style.Disabled).ForegroundColor(p.disabled).BackgroundColor(disabledBackground),
			// Pointer focus has no decoration; keyboard focus uses a thin border.
			// Keep the optional border color for application Focused overrides.
			style.Name(button.name).Part("focus").State(style.Focused).
				BackgroundColor(color.Transparent).BorderColor(focus).BorderWidth(0),
			style.Name(button.name).Part("focus").State(style.FocusVisible).
				BackgroundColor(color.Transparent).BorderColor(keyboardBorder).BorderWidth(2),
		)
	}
	rules = append(rules, checkButtonRules(p, accent, tabFocus)...)
	rules = append(rules, switchRules(p, accent, tabFocus)...)
	rules = append(rules, dropDownRules(p, tabFocus, options.FontFamily, size)...)
	// Each content widget resolves its own name; state is shared through the
	// tree, but foreground rules are explicit rather than inherited colors.
	for _, name := range []string{"icon", AccentIcon, PrimaryContent, MutedText, Card, Sidebar, Warning, Error, Success, "check-button-text", "switch-text",
		"drop-down-text", "drop-down-placeholder", "table-header-text", "table-cell-text", "table-cell-icon",
		"tree-item-text", "tree-item-icon", "tab-item-text", "tab-item-text-selected", "tab-item-icon",
		"tab-item-icon-selected", "tab-close-button-text", "tab-scroll-button-text"} {
		rules = append(rules, style.Name(name).State(style.Disabled).ForegroundColor(p.disabled))
	}
	return rules
}

type palette struct {
	window, surface, button, text, muted, disabled, border color.RGBA
	sidebar                                                color.RGBA
	menuHover, menuPressed                                 color.RGBA
	warning, error, success                                color.RGBA
}

func colorsFor(dark bool) palette {
	if dark {
		return palette{
			window: rgb(0x202024), surface: rgb(0x28282D), button: rgb(0x323238),
			sidebar: rgb(0x18181C),
			text:    rgb(0xECECEE), muted: rgb(0xB8B8C2), disabled: rgb(0x797983), border: rgb(0x55555F),
			menuHover: rgb(0x3E3E42), menuPressed: rgb(0x4F4F53),
			warning: rgb(0xEAC16D), error: rgb(0xF3959A), success: rgb(0x83CAA2),
		}
	}
	return palette{
		window: rgb(0xF6F6F8), surface: rgb(0xFFFFFF), button: rgb(0xFFFFFF),
		sidebar: rgb(0xE2E2E8),
		text:    rgb(0x202024), muted: rgb(0x61616B), disabled: rgb(0x97979F), border: rgb(0xC6C6CE),
		menuHover: rgb(0xE6E6E6), menuPressed: rgb(0xD1D1D1),
		warning: rgb(0x8A5900), error: rgb(0xBA3038), success: rgb(0x276B43),
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

func primaryColors(accent color.RGBA) ([3]color.RGBA, color.RGBA) {
	black, white := rgb(0x000000), rgb(0xFFFFFF)
	foreground, contrastTarget := white, black
	// Keep white lettering for saturated accents. Very light accents use black
	// instead of turning yellow/pastel/white into a dark, unrelated surface.
	if luminance(accent) > .5 {
		foreground, contrastTarget = black, white
	}
	base := contrastColor(accent, contrastTarget, []color.RGBA{foreground}, 4.5)
	interaction := black
	if luminance(base) < .02 {
		interaction = white // Black still needs visible hover/pressed feedback.
	}
	return [3]color.RGBA{base, mix(base, interaction, .06), mix(base, interaction, .12)}, foreground
}

func focusColor(accent, text color.RGBA, backgrounds []color.RGBA) color.RGBA {
	return contrastColor(accent, text, backgrounds, 3)
}

// contrastColor preserves the accent hue where possible, moving towards the
// palette's text color only as far as needed for all supplied backgrounds.
func contrastColor(accent, text color.RGBA, backgrounds []color.RGBA, minimum float64) color.RGBA {
	for step := 0; step <= 255; step++ {
		candidate := mix(accent, text, float64(step)/255)
		valid := true
		for _, bg := range backgrounds {
			if contrast(candidate, bg) < minimum {
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
