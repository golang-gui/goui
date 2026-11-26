package main

import (
	"fmt"
	"image/color"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/style"
	"github.com/golang-gui/goui/theme/modern"
)

// palette is the chat's own fixed design, independent of system accents.
// One accent carries the brand: sent bubbles, the current conversation, the
// navigation and the send button.
type palette struct {
	rail, sidebar, window, surface, border, text, muted color.NRGBA
	sent, sentText, received, accent                    color.NRGBA
	hover, current, online, shade                       color.NRGBA
}

var (
	lightPalette = palette{
		rail: rgb(0xF0F1F5), sidebar: rgb(0xFFFFFF), window: rgb(0xF5F6F8), surface: rgb(0xFFFFFF),
		border: rgb(0xE5E6EB), text: rgb(0x18181B), muted: rgb(0x71717A),
		sent: rgb(0x2563EB), sentText: rgb(0xFFFFFF), received: rgb(0xFFFFFF), accent: rgb(0x2563EB),
		hover: rgb(0xF3F4F6), current: rgb(0xEAF1FE), online: rgb(0x22A55B),
		shade: color.NRGBA{A: 20},
	}
	darkPalette = palette{
		rail: rgb(0x0B0B0E), sidebar: rgb(0x141417), window: rgb(0x0F0F12), surface: rgb(0x18181B),
		border: rgb(0x27272A), text: rgb(0xE4E4E7), muted: rgb(0x8B8B94),
		sent: rgb(0x3B6FE0), sentText: rgb(0xFFFFFF), received: rgb(0x222227), accent: rgb(0x4F8CFF),
		hover: rgb(0x1C1C20), current: rgb(0x1A2336), online: rgb(0x3CCB7F),
		shade: color.NRGBA{A: 90},
	}
)

// avatarColors are the fixed avatar tones, indexed by contact.Color.
var avatarColors = []color.NRGBA{
	rgb(0xE07A5F), rgb(0x3D85C6), rgb(0x6FAF8C), rgb(0xD4A373), rgb(0x8E7DBE), rgb(0x4FA3A0),
}

func avatarIndex(i int) int { return (i%len(avatarColors) + len(avatarColors)) % len(avatarColors) }

func avatarColor(i int) color.NRGBA { return avatarColors[avatarIndex(i)] }

func rgb(hex uint32) color.NRGBA {
	return color.NRGBA{R: uint8(hex >> 16), G: uint8(hex >> 8), B: uint8(hex), A: 255}
}

// Style names owned by this example.
const (
	styleRail           = "chat-rail"
	styleLogo           = "chat-logo"
	styleLogoIcon       = "chat-logo-icon"
	styleNav            = "chat-nav"
	styleNavCurrent     = "chat-nav-current"
	styleNavIcon        = "chat-nav-icon"
	styleNavIconCurrent = "chat-nav-icon-current"
	styleIconButton     = "chat-icon-button"
	styleIcon           = "chat-icon"
	styleSidebar        = "chat-sidebar"
	styleConversation   = "chat-conversation"
	styleTitle          = "chat-title"
	styleMuted          = "chat-muted"
	styleSmall          = "chat-small"
	styleOnline         = "chat-online"
	styleRow            = "chat-row"
	styleRowCurrent     = "chat-row-current"
	styleIndicator      = "chat-indicator"
	styleBadge          = "chat-badge"
	styleBadgeText      = "chat-badge-text"
	styleSent           = "chat-sent"
	styleSentText       = "chat-sent-text"
	styleSentTime       = "chat-sent-time"
	styleReceived       = "chat-received"
	styleReceivedText   = "chat-received-text"
	styleReceipt        = "chat-receipt"
	styleDay            = "chat-day"
	styleDayText        = "chat-day-text"
	styleSearch         = "chat-search"
	styleComposer       = "chat-composer"
	styleComposerBar    = "chat-composer-bar"
	styleSend           = "chat-send"
	styleSendIcon       = "chat-send-icon"
	styleDivider        = "chat-divider"
	styleShade          = "chat-shade"
	styleDot            = "chat-online-dot"
	styleTyping         = "chat-typing"
)

// avatarSizes are the avatar diameters used by the views, in DIP.
var avatarSizes = []int{28, 36, 44}

func avatarTextStyle(size int) string { return fmt.Sprintf("chat-avatar-text-%d", size) }

// senderStyle colors a group member's name with their avatar tone.
func senderStyle(color int) string { return fmt.Sprintf("chat-sender-%d", avatarIndex(color)) }

const bubbleRadius, bubbleTail = 16, 4

func styleRules(p palette, dark bool, family string) []style.Rule {
	rules := modern.Rules(modern.Options{Dark: dark, AccentColor: p.accent, FontFamily: family, FontSize: 14})
	text := func(name string, c color.Color, size float32) style.Rule {
		return style.Name(name).ForegroundColor(c).FontFamily(family).FontSize(size)
	}
	for _, size := range avatarSizes {
		rules = append(rules, text(avatarTextStyle(size), color.White, float32(size*2/5)).FontWeight(600))
	}
	for i, c := range avatarColors {
		// Keep names readable on each background: deepen in light, lift in dark.
		ink := mix(c, rgb(0x000000), .18)
		if dark {
			ink = mix(c, rgb(0xFFFFFF), .2)
		}
		rules = append(rules, text(senderStyle(i), ink, 12).FontWeight(600))
	}
	// Icon-only buttons: no resting surface or border, a quiet hover fill.
	for _, part := range []string{"", "expanded"} {
		rules = append(rules,
			style.Name(styleIconButton).Part(part).BackgroundColor(color.Transparent).BorderWidth(0).Radius(10),
			style.Name(styleIconButton).Part(part).State(style.Hovered).BackgroundColor(p.hover).BorderWidth(0),
			style.Name(styleIconButton).Part(part).State(style.Pressed).BackgroundColor(mix(p.hover, p.text, .06)).BorderWidth(0),
		)
	}
	rules = append(rules,
		style.Name(styleIconButton).Part("expanded").BackgroundColor(mix(p.hover, p.text, .06)),
		style.Name(styleIconButton).Part("focus").BackgroundColor(color.Transparent).BorderWidth(0).Radius(10),
		style.Name(styleIconButton).Part("focus").State(style.FocusVisible).BorderColor(p.accent).BorderWidth(2),
	)
	return append(rules,
		style.Name("window").BackgroundColor(p.window),
		style.Name("header-bar").BackgroundColor(p.surface),
		text("label", p.text, 14),
		style.Name(styleRail).BackgroundColor(p.rail),
		style.Name(styleLogo).BackgroundColor(p.accent).Radius(11),
		style.Name(styleLogoIcon).ForegroundColor(color.White),
		style.Name(styleNav).BackgroundColor(color.Transparent).Radius(11),
		style.Name(styleNavCurrent).BackgroundColor(mix(p.rail, p.accent, .14)).Radius(11),
		style.Name(styleNavIcon).ForegroundColor(p.muted),
		style.Name(styleNavIconCurrent).ForegroundColor(p.accent),
		style.Name(styleIcon).ForegroundColor(p.muted),
		style.Name(styleSidebar).BackgroundColor(p.sidebar),
		style.Name(styleConversation).BackgroundColor(p.window),
		style.Name(styleDivider).BackgroundColor(p.border),
		style.Name(styleShade).ForegroundColor(p.shade),
		text(styleTitle, p.text, 15).FontWeight(600),
		text(styleMuted, p.muted, 13),
		text(styleSmall, p.muted, 11),
		text(styleOnline, p.online, 12),
		style.Name(styleDot).BackgroundColor(p.online).BorderColor(p.surface).BorderWidth(2).Radius(6),
		style.Name(styleTyping).ForegroundColor(p.muted),
		style.Name(styleComposerBar).BackgroundColor(p.surface),
		text("list-row-title", p.text, 14).FontWeight(600),
		text("list-row-subtitle", p.muted, 13),
		style.Name(styleRow).BackgroundColor(color.Transparent).BorderWidth(0).Radius(12),
		style.Name(styleRow).State(style.Hovered).BackgroundColor(p.hover).BorderWidth(0),
		style.Name(styleRow).State(style.Pressed).BackgroundColor(mix(p.hover, p.text, .04)).BorderWidth(0),
		style.Name(styleRowCurrent).BackgroundColor(p.current).BorderWidth(0).Radius(12),
		style.Name(styleRowCurrent).State(style.Hovered).BackgroundColor(p.current).BorderWidth(0),
		style.Name(styleRowCurrent).State(style.Pressed).BackgroundColor(p.current).BorderWidth(0),
		style.Name(styleRow).Part("focus").BackgroundColor(color.Transparent).BorderWidth(0).Radius(12),
		style.Name(styleRow).Part("focus").State(style.FocusVisible).BorderColor(p.accent).BorderWidth(2),
		style.Name(styleRowCurrent).Part("focus").BackgroundColor(color.Transparent).BorderWidth(0).Radius(12),
		style.Name(styleRowCurrent).Part("focus").State(style.FocusVisible).BorderColor(p.accent).BorderWidth(2),
		style.Name(styleIndicator).BackgroundColor(p.accent).Radius(1.5),
		style.Name(styleBadge).BackgroundColor(p.accent).Radius(9),
		text(styleBadgeText, color.White, 11).FontWeight(600),
		// Bubbles keep one tight corner towards their sender's side.
		style.Name(styleSent).BackgroundColor(p.sent).
			CornerRadius(geometry.CornerRadius{TopLeft: bubbleRadius, TopRight: bubbleRadius, BottomRight: bubbleTail, BottomLeft: bubbleRadius}),
		style.Name(styleReceived).BackgroundColor(p.received).BorderColor(p.border).BorderWidth(1).
			CornerRadius(geometry.CornerRadius{TopLeft: bubbleRadius, TopRight: bubbleRadius, BottomRight: bubbleRadius, BottomLeft: bubbleTail}),
		text(styleSentText, p.sentText, 14),
		text(styleSentTime, mix(p.sent, p.sentText, .7), 11),
		text(styleReceivedText, p.text, 14),
		text(styleReceipt, p.muted, 11),
		style.Name(styleDay).BackgroundColor(p.surface).BorderColor(p.border).BorderWidth(1).Radius(10),
		text(styleDayText, p.muted, 11),
		text(styleSearch, p.text, 13).BackgroundColor(p.window).BorderColor(p.window).BorderWidth(1).Radius(10),
		style.Name(styleSearch).State(style.Focused).BorderColor(p.accent).BorderWidth(1),
		style.Name(styleSearch).Part("placeholder").ForegroundColor(p.muted),
		style.Name(styleSearch).Part("caret").ForegroundColor(p.text),
		style.Name(styleSearch).Part("selection").BackgroundColor(mix(p.window, p.accent, .25)),
		text(styleComposer, p.text, 14).BackgroundColor(p.window).BorderColor(p.window).BorderWidth(1).Radius(20),
		style.Name(styleComposer).State(style.Focused).BorderColor(p.accent).BorderWidth(1),
		style.Name(styleComposer).Part("placeholder").ForegroundColor(p.muted),
		style.Name(styleComposer).Part("caret").ForegroundColor(p.text),
		style.Name(styleComposer).Part("selection").BackgroundColor(mix(p.window, p.accent, .25)),
		style.Name(styleSend).BackgroundColor(p.accent).BorderWidth(0).Radius(20),
		style.Name(styleSend).State(style.Hovered).BackgroundColor(mix(p.accent, p.text, .12)),
		style.Name(styleSend).State(style.Pressed).BackgroundColor(mix(p.accent, p.text, .22)),
		style.Name(styleSend).State(style.Disabled).BackgroundColor(p.border),
		style.Name(styleSend).Part("focus").BackgroundColor(color.Transparent).BorderWidth(0),
		style.Name(styleSend).Part("focus").State(style.FocusVisible).BorderColor(p.text).BorderWidth(2),
		style.Name(styleSendIcon).ForegroundColor(color.White),
		style.Name(styleSendIcon).State(style.Disabled).ForegroundColor(p.muted),
		style.Name("scroll-view").Part("trough").BackgroundColor(color.Transparent),
	)
}

// mix blends two opaque colors in sRGB.
func mix(a, b color.NRGBA, t float32) color.NRGBA {
	f := func(x, y uint8) uint8 { return uint8(float32(x)*(1-t) + float32(y)*t + .5) }
	return color.NRGBA{R: f(a.R, b.R), G: f(a.G, b.G), B: f(a.B, b.B), A: 255}
}
