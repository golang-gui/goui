package gui

import (
	"sync"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/platform/graphics/utils"
	"github.com/golang-gui/goui/platform/typography"
	"github.com/golang-gui/goui/style"
	"github.com/golang-gui/goui/theme/basic"
)

const (
	styleNameWindow                   = "window"
	styleNamePopover                  = "popover"
	styleNameWidget                   = "widget"
	styleNameLabel                    = "label"
	styleNameIcon                     = "icon"
	styleNameButton                   = "button"
	styleNameTextInput                = "text-input"
	styleNameTextView                 = "text-view"
	styleNameScrollView               = "scroll-view"
	stylePartTrough                   = "trough"
	stylePartThumb                    = "thumb"
	stylePartFocus                    = "focus"
	styleNameMenu                     = "menu"
	styleNameMenuItem                 = "menu-item"
	styleNameMenuItemText             = "menu-item-text"
	styleNameMenuItemTextDisabled     = "menu-item-text-disabled"
	styleNameMenuItemShortcut         = "menu-item-shortcut"
	styleNameMenuItemShortcutDisabled = "menu-item-shortcut-disabled"
	styleNameMenuSeparator            = "menu-separator"
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
	radius, _ := s.CornerRadius()
	paintStyledBackground(p, rect, s, radius, geometry.Insets{})
}

func paintStyledBackground(p Painter, rect geometry.Rectangle, s style.Style, radius geometry.CornerRadius, insets geometry.Insets) {
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
	paintInsetBackground(p, rect, radius, insets, fill)
}

func paintInsetBackground(p Painter, rect geometry.Rectangle, radius geometry.CornerRadius, insets geometry.Insets, brush graphics.Brush) {
	if insets == geometry.UniformInsets(insets.Left) {
		if insets.Left > 0 {
			radius = radius.Normalize(rect.Size).Inset(insets.Left)
			rect = rect.Inset(insets.Left)
		}
		paintRoundedBackground(p, rect, radius, brush)
	} else {
		p.FillPath(utils.RoundRectInsetPath(rect, radius, insets), brush)
	}
}

func paintRoundedBackground(p Painter, rect geometry.Rectangle, radius geometry.CornerRadius, brush graphics.Brush) {
	if emptyRect(rect) {
		return
	}
	radius = radius.Normalize(rect.Size)
	if radius == (geometry.CornerRadius{}) {
		p.FillRect(rect, brush)
	} else if radius.Uniform() {
		p.FillRoundRect(rect, radius.TopLeft, brush)
	} else {
		p.FillPath(utils.RoundRectPath(rect, radius), brush)
	}
}

// PaintStyledBox paints the background then the border. An opaque border hides
// the background's AA edge beneath its centerline. Translucent borders retain
// the complete background beneath them. Shadow and text are separate operations.
func PaintStyledBox(p Painter, rect geometry.Rectangle, s style.Style) {
	radius, _ := s.CornerRadius()
	paintStyledBox(p, rect, s, radius, styledBorderSides(s))
}

func paintStyledBox(p Painter, rect geometry.Rectangle, s style.Style, radius geometry.CornerRadius, sides style.BorderSides) {
	paintStyledBackground(p, rect, s, radius, borderInsets(sides, opaqueBorderInset(s)))
	paintStyledBorder(p, rect, s, radius, sides)
}

func styledBorderSides(s style.Style) style.BorderSides {
	if sides, set := s.BorderSides(); set {
		return sides
	}
	return style.BorderAll
}

func borderInsets(sides style.BorderSides, width float32) (insets geometry.Insets) {
	if sides&style.BorderTop != 0 {
		insets.Top = width
	}
	if sides&style.BorderRight != 0 {
		insets.Right = width
	}
	if sides&style.BorderBottom != 0 {
		insets.Bottom = width
	}
	if sides&style.BorderLeft != 0 {
		insets.Left = width
	}
	return
}

func opaqueBorderInset(s style.Style) float32 {
	width, ok := s.BorderWidth()
	width = normalizeLayoutValue(width)
	border, hasColor := s.BorderColor()
	if ok && hasColor && width > 0 && border != nil {
		if _, _, _, alpha := border.RGBA(); alpha == 65535 {
			return width / 2
		}
	}
	return 0
}

// PaintStyledBorder paints an inset border without repainting the background.
// It does not reserve layout space. Missing/transparent colors, nonpositive
// widths, empty rectangles and an explicit empty BorderSides emit no commands.
// Unset BorderSides paints all edges; selected edges share one width and color.
func PaintStyledBorder(p Painter, rect geometry.Rectangle, s style.Style) {
	radius, _ := s.CornerRadius()
	paintStyledBorder(p, rect, s, radius, styledBorderSides(s))
}

func paintStyledBorder(p Painter, rect geometry.Rectangle, s style.Style, radius geometry.CornerRadius, sides style.BorderSides) {
	if sides == 0 || rect.Width <= 0 || rect.Height <= 0 {
		return
	}
	width, ok := s.BorderWidth()
	width = normalizeLayoutValue(width)
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
	radius = radius.Normalize(rect.Size)
	if sides != style.BorderAll {
		p.FillPath(utils.RoundRectBorderInsetsPath(rect, radius, borderInsets(sides, width)), stroke)
		return
	}
	// Once the inner contour collapses, the border covers the whole shape.
	if width >= min(rect.Width, rect.Height)/2 {
		paintRoundedBackground(p, rect, radius, stroke)
		return
	}
	// Small radii cannot retain their outer contour through a centerline
	// stroke. Fill the actual ring, including when inner corners collapse.
	if !radius.Uniform() || radius.TopLeft > 0 && radius.TopLeft < width/2 {
		p.FillPath(utils.RoundRectBorderPath(rect, radius, width), stroke)
		return
	}
	halfWidth := width / 2
	borderRect := rect.Inset(halfWidth)
	borderRadius := max(radius.TopLeft-halfWidth, 0)
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
