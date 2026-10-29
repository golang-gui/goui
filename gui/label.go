package gui

import (
	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/typography"
	"github.com/golang-gui/goui/style"
	"github.com/golang-gui/goui/textformat"
)

const labelMeasureExtent = 1 << 20

// WrapMode and TextAlign re-export the typography enums so gui consumers use gui
// types only, without importing the platform typography package.
type (
	WrapMode  = typography.WrapMode
	TextAlign = typography.TextAlignment
)

const (
	WrapNone     = typography.WrapNone
	WrapChar     = typography.WrapChar
	WrapWordChar = typography.WrapWordChar
)

const (
	TextAlignBegin  = typography.TextAlignBegin
	TextAlignEnd    = typography.TextAlignEnd
	TextAlignCenter = typography.TextAlignCenter
	TextAlignFill   = typography.TextAlignFill
)

// LabelInfoKey identifies Label-specific data in WidgetInfo.Attributes.
const LabelInfoKey = "goui.label"

// LabelInfo describes text interpretation without exposing source markup or
// native layout resources. Text itself is reported in WidgetInfo.Text.
type LabelInfo struct {
	Formatted   bool   `json:"formatted"`
	FormatError string `json:"formatError,omitempty"`
}

// Label displays text. A leading \{format\} enables nested \{styleName:body\}
// ranges using the current stylesheet; other sources are displayed verbatim.
// Malformed syntax falls back to the entire source. Label is not selectable.
type Label struct {
	WidgetBase
	text      string
	parsed    textformat.Text
	formatErr error
	wrapMode  WrapMode
	textAlign TextAlign

	// TextLayout cache. Reused across Measure and Paint. Invalidated by
	// SetText / SetWrapMode / SetTextAlign / StyleChanged. Released on unmount.
	cachedLayout typography.TextLayout
	cachedWidth  float32
	cachedHeight float32
	layoutValid  bool
}

func NewLabel(text string) *Label {
	label := &Label{}
	label.SetText(text)
	label.ConnectUnmount(label.releaseLayout)
	return label
}

// Text returns the original source, including any format syntax.
func (l *Label) Text() string {
	return l.text
}

// PlainText returns the displayed text. Invalid format syntax is displayed
// verbatim, so an error returns the complete source rather than partial output.
func (l *Label) PlainText() string { return l.parsed.Plain }

// SetText changes the source and invalidates layout. Format syntax is parsed
// once per changed source; themes and native layout are resolved lazily.
func (l *Label) SetText(text string) {
	if l.text == text {
		return
	}
	parsed, err := textformat.Parse(text)
	if err != nil {
		parsed = textformat.Text{Plain: text}
	}
	l.text, l.parsed, l.formatErr = text, parsed, err
	l.invalidateLayout()
	l.RequestLayout()
	l.requestSemanticUpdate()
}

func (l *Label) WrapMode() WrapMode {
	return l.wrapMode
}

func (l *Label) SetWrapMode(mode WrapMode) {
	if l.wrapMode == mode {
		return
	}
	l.wrapMode = mode
	l.invalidateLayout()
	l.RequestLayout()
}

func (l *Label) TextAlign() TextAlign {
	return l.textAlign
}

func (l *Label) SetTextAlign(align TextAlign) {
	if l.textAlign == align {
		return
	}
	l.textAlign = align
	l.invalidateLayout()
	l.RequestLayout()
}

func (l *Label) Measure(c layout.Constraint) layout.Measurement {
	if !l.Visible() {
		return layout.Measurement{}
	}
	textLayout := l.ensureLayout(measureTextSize(c.Max))
	if textLayout == nil {
		return layout.Measurement{}
	}

	width, height := textLayout.MeasureSize()
	if height <= 0 {
		format := l.resolvedTextFormat()
		height = textLineHeight(format.Font.Size)
	}
	measured := layout.Measured(geometry.Size{Width: width, Height: height})
	if l.PlainText() != "" {
		lines, _ := textLayout.MeasureMetrics()
		if len(lines) > 0 {
			measured.Baseline = lines[0].Baseline
			measured.HasBaseline = true
		}
	}
	measured.Size = l.constrain(c, measured.Size)
	return measured
}

func (l *Label) Paint(p Painter) {
	if !l.Visible() {
		return
	}
	textLayout := l.ensureLayout(l.Rect().Size)
	if textLayout != nil {
		p.DrawTextLayout(geometry.Point{}, textLayout)
	}
}

func (l *Label) Snapshot() WidgetInfo {
	info := l.WidgetBase.Snapshot()
	info.Role = RoleLabel
	info.Text = l.PlainText()
	if l.parsed.Formatted || l.formatErr != nil {
		value := LabelInfo{Formatted: l.parsed.Formatted}
		if l.formatErr != nil {
			value.FormatError = l.formatErr.Error()
		}
		info.SetAttribute(LabelInfoKey, value)
	}
	return info
}

func (l *Label) invalidateLayout() {
	l.layoutValid = false
}

// StyleChanged releases the old font/color resources; they are rebuilt lazily.
func (l *Label) StyleChanged() { l.releaseLayout() }

func (l *Label) ensureLayout(size geometry.Size) typography.TextLayout {
	if !l.layoutValid || l.cachedLayout == nil {
		l.releaseLayout()
		l.cachedLayout = l.newTextLayout(l.resolvedTextStyle(), size)
		if l.cachedLayout == nil {
			return nil
		}
		l.layoutValid = true
		l.cachedWidth, l.cachedHeight = size.Width, size.Height
	}

	if l.cachedWidth != size.Width || l.cachedHeight != size.Height {
		l.cachedLayout.SetSize(size.Width, size.Height)
		l.cachedWidth = size.Width
		l.cachedHeight = size.Height
	}

	return l.cachedLayout
}

func (l *Label) releaseLayout() {
	if l.cachedLayout != nil {
		l.cachedLayout.Destroy()
		l.cachedLayout = nil
	}
	l.layoutValid = false
}

func (l *Label) newTextLayout(base labelTextStyle, size geometry.Size) typography.TextLayout {
	if App == nil {
		return nil
	}
	typo := App.Typography()
	if typo == nil {
		return nil
	}
	textLayout, err := typo.NewTextLayout(l.PlainText(), base.format, size.Width, size.Height)
	if err != nil {
		return nil
	}
	if length := len(l.PlainText()); length > 0 {
		if base.underline {
			textLayout.SetUnderline(0, length, true)
		}
		if base.strike {
			textLayout.SetStrikethrough(0, length, true)
		}
		for _, run := range l.parsed.Runs {
			value := base
			for _, name := range run.Styles {
				value = value.with(ResolveWidgetStyle(l, name, style.PartDefault, l.textState()))
			}
			textLayout.SetTextFont(run.Start, run.End-run.Start, value.format.Font)
			color := value.format.TextColor
			if color == nil {
				color = typography.DefaultTextColor()
			}
			textLayout.SetTextColor(run.Start, run.End-run.Start, color)
			textLayout.SetUnderline(run.Start, run.End-run.Start, value.underline)
			textLayout.SetStrikethrough(run.Start, run.End-run.Start, value.strike)
		}
	}
	return textLayout
}

func (l *Label) resolvedTextFormat() typography.TextFormat { return l.resolvedTextStyle().format }

func (l *Label) textState() style.State {
	if !IsEnabled(l) {
		return style.Disabled
	}
	return style.Normal
}

func (l *Label) resolvedTextStyle() labelTextStyle {
	value := labelTextStyle{format: typography.TextFormat{WrapMode: l.wrapMode, TextAlign: l.textAlign}}
	value = value.with(ResolveWidgetStyle(l, styleNameLabel, style.PartDefault, l.textState()))
	if name := l.StyleName(); name != "" && name != styleNameLabel {
		value = value.with(ResolveWidgetStyle(l, name, style.PartDefault, l.textState()))
	}
	return value
}

// labelTextStyle is a complete text request assembled from optional style
// fields. Range setters replace native font requests, so unset fields must be
// merged here rather than passed as zero-valued partial FontInfo objects.
type labelTextStyle struct {
	format            typography.TextFormat
	underline, strike bool
}

func (v labelTextStyle) with(s style.Style) labelTextStyle {
	if value, ok := s.FontFamily(); ok {
		v.format.Font.Family = value
	}
	if value, ok := s.FontSize(); ok {
		v.format.Font.Size = value
	}
	if value, ok := s.FontWeight(); ok {
		v.format.Font.Weight = typography.FontWeight(value)
	}
	if value, ok := s.FontStyle(); ok {
		v.format.Font.Style = typography.FontStyle(value)
	}
	if value, ok := s.ForegroundColor(); ok {
		v.format.TextColor = value
	}
	if value, ok := s.Underline(); ok {
		v.underline = value
	}
	if value, ok := s.Strikethrough(); ok {
		v.strike = value
	}
	return v
}

// labelPlainText is for semantic fields which display Label source without
// retaining a Label. It never creates typography or other native resources.
func labelPlainText(source string) string {
	parsed, err := textformat.Parse(source)
	if err != nil {
		return source
	}
	return parsed.Plain
}

func measureTextSize(available geometry.Size) geometry.Size {
	if available.Width <= 0 {
		available.Width = labelMeasureExtent
	}
	if available.Height <= 0 {
		available.Height = labelMeasureExtent
	}
	return available
}
