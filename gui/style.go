package gui

import (
	"image/color"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/platform/typography"
	"github.com/golang-gui/goui/style"
)

const (
	styleNameWidget    = "widget"
	styleNameLabel     = "label"
	styleNameButton    = "button"
	styleNameTextInput = "text-input"
	styleNameBox       = "box"
	styleNameImage     = "image"
)

func DefaultStyleSheet() style.StyleSheet {
	return style.Sheet(defaultStyleRules(nil)...)
}

// defaultStyleRules builds the built-in style sheet from the given settings so
// the default appearance tracks the system accent color and UI font. A nil
// settings value falls back to gui defaults. The palette is light-only for now;
// dark-mode selection by settings.ColorScheme() is future work.
func defaultStyleRules(s *Settings) []style.Rule {
	accent := defaultAccentColor
	family := defaultLabelFontFamily()
	size := defaultFontSize
	if s != nil {
		accent = s.AccentColor()
		if f := s.FontFamily(); f != "" {
			family = f
		}
		size = s.FontSize()
	}

	return []style.Rule{
		style.Name(styleNameWidget).
			BackgroundColor(color.Transparent).
			ForegroundColor(color.Black).
			BorderColor(color.Transparent).
			BorderWidth(0).
			Radius(0).
			FontFamily(family).
			FontSize(size),

		style.Name(styleNameLabel).
			ForegroundColor(color.Black).
			FontFamily(family).
			FontSize(size),

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
	}
}

func resolveStyle(widget Widget, part string, state style.State) style.Style {
	if widget == nil {
		return style.Style{}
	}
	sheet := widgetStyleSheet(widget)
	if sheet == nil {
		return style.Style{}
	}
	return sheet.Resolve(style.Sel{
		Name:  widget.StyleName(),
		Part:  part,
		State: state,
	}, widget.StyleRules())
}

func widgetStyleSheet(widget Widget) style.StyleSheet {
	if win, ok := widget.Window().(*window); ok && win != nil && win.app != nil {
		if sheet := win.app.resolvedStyleSheet(); sheet != nil {
			return sheet
		}
	}
	if app, ok := App.(*application); ok && app != nil {
		if sheet := app.resolvedStyleSheet(); sheet != nil {
			return sheet
		}
	}
	return DefaultStyleSheet()
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

	width, ok := s.BorderWidth()
	if !ok || width <= 0 {
		return
	}
	bc, ok := s.BorderColor()
	if !ok || bc == nil {
		return
	}
	stroke := graphics.ColorOf(bc)
	if radius > 0 {
		p.DrawRoundRect(rect, radius, width, stroke)
	} else {
		p.DrawRect(rect, width, stroke)
	}
}
