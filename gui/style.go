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
	weight, _ := s.FontWeight()
	slant, _ := s.FontStyle()
	return typography.TextFormat{
		Font:      typography.FontInfo{Family: family, Size: size, Weight: typography.FontWeight(weight), Style: typography.FontStyle(slant)},
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

// WidgetStyle resolves a widget's own name, part and current state through the
// nearest StyleScope. An empty StyleName uses defaultName; styles do not inherit
// from parent widgets.
func WidgetStyle(widget Widget, defaultName, part string) style.Style {
	name := widget.StyleName()
	if name == "" {
		name = defaultName
	}
	return ResolveWidgetStyle(widget, name, part, widget.State())
}

// PaintWidgetStyle paints the default Part over the widget's local bounds.
// Call explicitly from Paint. It does not paint children or change layout/clip.
func PaintWidgetStyle(p Painter, widget Widget, defaultName string) {
	PaintStyledBox(p, geometry.Rectangle{Size: widget.Rect().Size}, WidgetStyle(widget, defaultName, style.PartDefault))
}

// PaintStyledBackground paints the specified style's background, including its
// radius. Missing or transparent colors and empty rectangles emit no commands.
// A rounded background does not clip children.
func PaintStyledBackground(p Painter, rect geometry.Rectangle, s style.Style) {
	if rect.Width <= 0 || rect.Height <= 0 {
		return
	}
	bg, ok := s.BackgroundColor()
	if !ok || bg == nil {
		return
	}
	fill := graphics.ColorOf(bg)
	if fill.A == 0 {
		return
	}
	radius, _ := s.Radius()
	if radius > 0 {
		p.FillRoundRect(rect, radius, fill)
	} else {
		p.FillRect(rect, fill)
	}
}

// PaintStyledBox paints the background then the border. Shadow and text are
// separate drawing operations; unset fields emit no commands.
func PaintStyledBox(p Painter, rect geometry.Rectangle, s style.Style) {
	PaintStyledBackground(p, rect, s)
	PaintStyledBorder(p, rect, s)
}

// PaintStyledBorder paints an inset border without repainting the background.
// It does not reserve layout space. Missing/transparent colors, nonpositive
// widths and empty rectangles emit no commands.
func PaintStyledBorder(p Painter, rect geometry.Rectangle, s style.Style) {
	if rect.Width <= 0 || rect.Height <= 0 {
		return
	}
	width, ok := s.BorderWidth()
	if !ok || width <= 0 {
		return
	}
	bc, ok := s.BorderColor()
	if !ok || bc == nil {
		return
	}
	stroke := graphics.ColorOf(bc)
	if stroke.A == 0 {
		return
	}
	radius, _ := s.Radius()
	// Once the inner contour collapses, the border covers the whole shape.
	if width >= min(rect.Width, rect.Height)/2 {
		if radius > 0 {
			p.FillRoundRect(rect, radius, stroke)
		} else {
			p.FillRect(rect, stroke)
		}
		return
	}
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
func paintButtonFocus(p Painter, widget Widget, button *WidgetBase) {
	if IsEnabled(button) && button.Focused() {
		focus := ResolveWidgetStyle(button, buttonStyleName(button), stylePartFocus, button.State())
		PaintButtonFocus(p, widget, focus)
	}
}
