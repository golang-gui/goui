package gui

import (
	"runtime"

	"github.com/golang-gui/goui/core/geometry"
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

type Label struct {
	WidgetBase
	text      string
	wrapMode  WrapMode
	textAlign TextAlign
}

func NewLabel(text string) *Label {
	label := &Label{text: text}
	label.SetStyleName(styleNameLabel)
	return label
}

func (l *Label) Text() string {
	return l.text
}

func (l *Label) SetText(text string) {
	if l.text == text {
		return
	}
	l.text = text
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
	l.RequestLayout()
}

func (l *Label) Measure(c layout.Constraint) geometry.Size {
	if !l.Visible() {
		return geometry.Size{}
	}
	format := l.resolvedTextFormat()
	textLayout := l.newTextLayout(format, measureTextSize(c.Max))
	if textLayout == nil {
		return geometry.Size{}
	}
	defer textLayout.Destroy()

	width, height := textLayout.MeasureSize()
	if height <= 0 {
		height = textLineHeight(format.Font.Size)
	}
	return geometry.Size{Width: width, Height: height}
}

func (l *Label) Paint(p Painter) {
	if !l.Visible() {
		return
	}
	textLayout := l.newTextLayout(l.resolvedTextFormat(), l.Rect().Size)
	if textLayout != nil {
		p.DrawTextLayout(geometry.Point{}, textLayout)
		textLayout.Destroy()
	}
	l.PaintChildren(p)
}

func (l *Label) Snapshot() WidgetInfo {
	info := l.WidgetBase.Snapshot()
	info.Role = RoleLabel
	info.Text = l.text
	return info
}

func (l *Label) newTextLayout(format typography.TextFormat, size geometry.Size) typography.TextLayout {
	if App == nil {
		return nil
	}
	typo := App.Typography()
	if typo == nil {
		return nil
	}
	textLayout, err := typo.NewTextLayout(l.text, format, size.Width, size.Height)
	if err != nil {
		return nil
	}
	return textLayout
}

func (l *Label) resolvedTextFormat() typography.TextFormat {
	return textFormatFromStyle(resolveStyle(l, style.PartDefault, style.Normal), l.wrapMode, l.textAlign)
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
