package gui

import (
	"sync"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/platform/typography"
	"github.com/golang-gui/goui/style"
	"github.com/golang-gui/goui/theme/basic"
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

// DefaultStyleSheet returns the complete Basic fallback used when the
// application has no custom stylesheet. It does not enable Modern.
func DefaultStyleSheet() style.StyleSheet { return defaultStyleSheet() }

// Cache immutable fallback values; Basic does not watch system settings.
var defaultStyleSheet = sync.OnceValue(basic.Sheet)

// DefaultStyleRules returns fresh Basic rules for built-in and registered controls.
// It is retained for applications that append overrides before building a sheet.
func DefaultStyleRules() []style.Rule { return basic.Rules() }

// ResolveStyle resolves (name, part, state) against the application's current
// style sheet, falling back to the built-in default when there is none.
func ResolveStyle(name, part string, state style.State) style.Style {
	sheet := DefaultStyleSheet()
	if App != nil {
		if s := App.StyleSheet(); s != nil {
			sheet = s
		}
	}
	return sheet.Resolve(style.Sel{Name: name, Part: part, State: state})
}

// textFormatFromStyle builds a text format from a resolved style plus the
// caller-owned wrap mode and alignment. Font family, size and color come
// entirely from the style; the widget carries no font defaults of its own. An
// unset color is left nil for the renderer to default.
func textFormatFromStyle(s style.Style, wrap typography.WrapMode, align typography.TextAlignment) typography.TextFormat {
	family, _ := s.FontFamily()
	size, _ := s.FontSize()
	foreground, _ := s.ForegroundColor()
	return typography.TextFormat{
		Font:      typography.FontInfo{Family: family, Size: size},
		WrapMode:  wrap,
		TextAlign: align,
		TextColor: foreground,
	}
}

// textLineHeight approximates one text line's height in logical pixels from a
// point size. Logical pixels are 96 DPI, so 1 pt (1/72") is 96/72 logical px.
// Used as the fallback line height when there is no text to measure.
func textLineHeight(sizePt float32) float32 {
	return sizePt * 96.0 / 72.0
}

// paintStyledBox paints a widget's background and border from its resolved
// style. Unset fields are skipped: no background color means no fill, and a
// missing border width or color means no border. Widgets trust the resolved
// style and carry no hard-coded fallbacks of their own.
func paintStyledBox(p Painter, rect geometry.Rectangle, s style.Style) {
	radius, _ := s.Radius()
	if bg, ok := s.BackgroundColor(); ok && bg != nil {
		fill := graphics.ColorOf(bg)
		if radius > 0 {
			p.FillRoundRect(rect, radius, fill)
		} else {
			p.FillRect(rect, fill)
		}
	}
	paintStyledBorder(p, rect, s)
}

func paintStyledBorder(p Painter, rect geometry.Rectangle, s style.Style) {
	radius, _ := s.Radius()
	width, ok := s.BorderWidth()
	if !ok || width <= 0 {
		return
	}
	bc, ok := s.BorderColor()
	if !ok || bc == nil {
		return
	}
	stroke := graphics.ColorOf(bc)
	// Stroke primitives are centered on their path. Keep the complete border
	// inside the Widget's clip by moving its centerline inward by half its width.
	halfWidth := width / 2
	borderRect := rect.Inset(halfWidth)
	borderRadius := max(radius-halfWidth, 0)
	if borderRadius > 0 {
		p.DrawRoundRect(borderRect, borderRadius, width, stroke)
	} else {
		p.DrawRect(borderRect, width, stroke)
	}
}

func buttonStyleName(button *WidgetBase) string {
	if name := button.StyleName(); name != "" {
		return name
	}
	return styleNameButton
}

// Focus is an independent pass over the button's hover/press background. Its
// optional tint and border do not alter the child widgets' styles.
func paintButtonFocus(p Painter, rect geometry.Rectangle, button *WidgetBase) {
	if IsEnabled(button) && button.Focused() {
		state := style.Focused
		if button.FocusVisible() {
			state = style.FocusVisible
		}
		focus := ResolveWidgetStyle(button, buttonStyleName(button), stylePartFocus, state)
		if bg, ok := focus.BackgroundColor(); ok && bg != nil && graphics.ColorOf(bg).A > 0 {
			paintStyledBox(p, rect, focus)
		} else {
			// The default pointer-focus background is transparent: skip the
			// no-op fill while retaining an application's optional border.
			paintStyledBorder(p, rect, focus)
		}
	}
}
