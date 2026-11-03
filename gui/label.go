package gui

import (
	"image/color"

	"github.com/golang-gui/goui/core/colors"
	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui/textformat"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/typography"
	"github.com/golang-gui/goui/style"
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

// LabelInfo describes text interpretation and selection without exposing source
// markup or native resources. Text itself is reported in WidgetInfo.Text.
type LabelInfo struct {
	Formatted   bool          `json:"formatted"`
	FormatError string        `json:"formatError,omitempty"`
	Selectable  bool          `json:"selectable"`
	Selection   TextSelection `json:"selection"`
}

// Label displays text. A leading \{format\} enables nested \{styleName:body\}
// ranges using the current stylesheet; other sources are displayed verbatim.
// Malformed syntax falls back to the entire source. Selection is opt-in.
type Label struct {
	WidgetBase
	text        string
	parsed      textformat.Text
	formatErr   error
	wrapMode    WrapMode
	textAlign   TextAlign
	selectState *labelSelection

	// Resolved text values belong to this label, independently of native range
	// setters, which do not change TextLayout.Format's base attributes.
	textStyle      labelTextStyle
	runStyles      []labelTextStyle
	textStyleValid bool

	// The native layout is reused until text/font changes or unmount.
	cachedLayout typography.TextLayout
	cachedWidth  float32
	cachedHeight float32
	layoutValid  bool

	// Nested layouts can alternate several constraints. Retain only detached
	// metrics, not a second native layout or any shaping/selection geometry.
	measurements [4]labelMeasurement
	measureNext  uint8
}

type labelMeasurement struct {
	size  geometry.Size
	value layout.Measurement
	valid bool
}

func NewLabel(text string) *Label {
	label := &Label{}
	label.SetText(text)
	label.ConnectUnmount(func() {
		label.releaseLayout()
		if s := label.selectState; s != nil {
			s.cancel()
		}
	})
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
	plainChanged := l.PlainText() != parsed.Plain
	l.text, l.parsed, l.formatErr = text, parsed, err
	l.invalidateLayout()
	l.RequestLayout()
	l.requestSemanticUpdate()
	if s := l.selectState; s != nil && plainChanged {
		previous := s.value
		s.value, s.upstream = TextSelection{}, false
		s.cancel()
		if !l.Destroyed() && l.text == text && s.value == (TextSelection{}) && previous != s.value {
			s.changed.Emit(s.value, s.revision)
		}
	}
}

func (l *Label) WrapMode() WrapMode {
	return l.wrapMode
}

func (l *Label) SetWrapMode(mode WrapMode) {
	if l.wrapMode == mode {
		return
	}
	l.wrapMode = mode
	l.textStyleValid = false
	l.clearMeasurements()
	l.invalidateSelectionGeometry()
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
	l.textStyleValid = false
	l.clearMeasurements()
	l.invalidateSelectionGeometry()
	l.RequestLayout()
}

func (l *Label) Measure(c layout.Constraint) layout.Measurement {
	if !l.Visible() {
		return layout.Measurement{}
	}
	l.ensureTextStyle()
	if l.Destroyed() {
		return layout.Measurement{}
	}
	size := measureTextSize(c.Max)
	for _, cached := range l.measurements {
		if cached.valid && cached.size == size {
			measured := cached.value
			measured.Size = l.constrain(c, measured.Size)
			return measured
		}
	}
	var textLayout typography.TextLayout
	cacheResult := true
	if !l.layoutValid || l.cachedLayout == nil {
		// Keep the first layout for display; later measurement queries never
		// change the native layout used by painting and selection.
		textLayout = l.ensureLayout(size)
	} else if l.cachedWidth == size.Width && l.cachedHeight == size.Height {
		textLayout = l.cachedLayout
		// Display metrics are already available without reshaping. Do not
		// evict an independent query result to retain the same information.
		cacheResult = false
	} else {
		textLayout = l.newTextLayout(l.textStyle, size)
		if textLayout != nil {
			defer textLayout.Destroy()
		}
	}
	if textLayout == nil {
		return layout.Measurement{}
	}

	width, height := textLayout.MeasureSize()
	if height <= 0 {
		height = textLineHeight(l.textStyle.format.Font.Size)
	}
	measured := layout.Measured(geometry.Size{Width: width, Height: height})
	if l.PlainText() != "" {
		lines, _ := textLayout.MeasureMetrics()
		if len(lines) > 0 {
			measured.Baseline = lines[0].Baseline
			measured.HasBaseline = true
		}
	}
	if cacheResult {
		l.measurements[l.measureNext] = labelMeasurement{size: size, value: measured, valid: true}
		l.measureNext = (l.measureNext + 1) % uint8(len(l.measurements))
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
		if l.Selectable() {
			l.selectState.paint(p)
		}
		p.DrawTextLayout(geometry.Point{}, textLayout)
	}
}

func (l *Label) Arrange(rect geometry.Rectangle) {
	if l.Rect().Size != rect.Size {
		l.invalidateSelectionGeometry()
	}
	l.WidgetBase.Arrange(rect)
}

func (l *Label) Snapshot() WidgetInfo {
	info := l.WidgetBase.Snapshot()
	info.Role = RoleLabel
	info.Text = l.PlainText()
	if l.parsed.Formatted || l.formatErr != nil || l.Selectable() {
		value := LabelInfo{Formatted: l.parsed.Formatted, Selectable: l.Selectable(), Selection: l.Selection()}
		if l.formatErr != nil {
			value.FormatError = l.formatErr.Error()
		}
		info.SetAttribute(LabelInfoKey, value)
	}
	return info
}

func (l *Label) invalidateLayout() {
	l.layoutValid = false
	l.textStyleValid = false
	l.clearMeasurements()
	l.invalidateSelectionGeometry()
}

// StyleChanged compares resolved text attributes. Notifications alone do not
// discard native layouts or cached text images; creation remains lazy.
func (l *Label) StyleChanged() {
	l.textStyleValid = false
	l.ensureTextStyle()
}

func (l *Label) ensureTextStyle() {
	if l.textStyleValid {
		return
	}
	next := l.resolvedTextStyle().snapshot()
	var runs []labelTextStyle
	if len(l.parsed.Runs) > 0 {
		runs = make([]labelTextStyle, len(l.parsed.Runs))
		for i, run := range l.parsed.Runs {
			value := next
			for _, name := range run.Styles {
				value = value.with(ResolveWidgetStyle(l, name, style.PartDefault, l.State()))
			}
			runs[i] = value.snapshot()
		}
	}
	previous, previousRuns := l.textStyle, l.runStyles
	if l.layoutValid {
		changed := !previous.sameFontAndDecoration(next) || len(previousRuns) != len(runs)
		for i := range runs {
			if changed || !previousRuns[i].sameFontAndDecoration(runs[i]) {
				changed = true
				break
			}
		}
		if changed {
			l.invalidateLayout()
		}
	}
	l.textStyle, l.runStyles, l.textStyleValid = next, runs, true
	native := l.cachedLayout
	if native == nil || !l.layoutValid {
		return
	}
	current := func() bool { return !l.Destroyed() && l.layoutValid && l.cachedLayout == native && l.textStyleValid }
	if previous.format.WrapMode != next.format.WrapMode {
		l.clearMeasurements()
		l.invalidateSelectionGeometry()
		native.SetWrapMode(next.format.WrapMode)
		if !current() {
			return
		}
	}
	if previous.format.TextAlign != next.format.TextAlign {
		l.clearMeasurements()
		l.invalidateSelectionGeometry()
		native.SetTextAlignment(next.format.TextAlign)
		if !current() {
			return
		}
	}
	length := len(l.PlainText())
	baseChanged := !colors.Equal(previous.format.TextColor, next.format.TextColor)
	if baseChanged && length > 0 {
		native.SetTextColor(0, length, next.format.TextColor)
		if !current() {
			return
		}
	}
	for i, value := range runs {
		// A full-range update also overwrites explicit inline colors.
		if baseChanged || !colors.Equal(previousRuns[i].format.TextColor, value.format.TextColor) {
			run := l.parsed.Runs[i]
			native.SetTextColor(run.Start, run.End-run.Start, value.format.TextColor)
			if !current() {
				return
			}
		}
	}
}

func (l *Label) ensureLayout(size geometry.Size) typography.TextLayout {
	l.ensureTextStyle()
	if l.Destroyed() {
		return nil
	}
	if !l.layoutValid || l.cachedLayout == nil {
		l.releaseLayout()
		if l.Destroyed() {
			return nil
		}
		l.ensureTextStyle()
		l.cachedLayout = l.newTextLayout(l.textStyle, size)
		if l.cachedLayout == nil {
			return nil
		}
		l.layoutValid = true
		l.cachedWidth, l.cachedHeight = size.Width, size.Height
	}

	if l.cachedWidth != size.Width || l.cachedHeight != size.Height {
		l.invalidateSelectionGeometry()
		l.cachedLayout.SetSize(size.Width, size.Height)
		l.cachedWidth = size.Width
		l.cachedHeight = size.Height
	}

	return l.cachedLayout
}

func (l *Label) releaseLayout() {
	l.clearMeasurements()
	l.invalidateSelectionGeometry()
	native := l.cachedLayout
	l.cachedLayout = nil
	l.layoutValid = false
	if native != nil {
		native.Destroy()
	}
}

func (l *Label) clearMeasurements() {
	l.measurements = [4]labelMeasurement{}
	l.measureNext = 0
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
		for i, run := range l.parsed.Runs {
			value := l.runStyles[i]
			textLayout.SetTextFont(run.Start, run.End-run.Start, value.format.Font)
			textLayout.SetTextColor(run.Start, run.End-run.Start, value.format.TextColor)
			textLayout.SetUnderline(run.Start, run.End-run.Start, value.underline)
			textLayout.SetStrikethrough(run.Start, run.End-run.Start, value.strike)
		}
	}
	return textLayout
}

func (l *Label) resolvedTextFormat() typography.TextFormat { return l.resolvedTextStyle().format }

func (l *Label) resolvedTextStyle() labelTextStyle {
	value := labelTextStyle{format: typography.TextFormat{WrapMode: l.wrapMode, TextAlign: l.textAlign}}
	value = value.with(ResolveWidgetStyle(l, styleNameLabel, style.PartDefault, l.State()))
	if name := l.StyleName(); name != "" && name != styleNameLabel {
		value = value.with(ResolveWidgetStyle(l, name, style.PartDefault, l.State()))
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

func (v labelTextStyle) sameFontAndDecoration(other labelTextStyle) bool {
	return v.format.Font == other.format.Font && v.underline == other.underline && v.strike == other.strike
}

// Freeze color values: color.Color may hold mutable or non-comparable values.
func (v labelTextStyle) snapshot() labelTextStyle {
	ink := v.format.TextColor
	if ink == nil {
		ink = typography.DefaultTextColor()
	}
	r, g, b, a := ink.RGBA()
	v.format.TextColor = color.RGBA64{R: uint16(r), G: uint16(g), B: uint16(b), A: uint16(a)}
	return v
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

func measureTextSize(available geometry.Size) geometry.Size {
	if available.Width <= 0 {
		available.Width = labelMeasureExtent
	}
	if available.Height <= 0 {
		available.Height = labelMeasureExtent
	}
	return available
}
