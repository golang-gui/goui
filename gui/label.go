package gui

import (
	"image/color"
	"runtime"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform/typography"
)

const labelMeasureExtent = 1 << 20

type Label struct {
	WidgetBase
	text   string
	format typography.TextFormat
}

func NewLabel(text string) *Label {
	label := &Label{
		text:   text,
		format: DefaultLabelTextFormat(),
	}
	return label
}

func DefaultLabelTextFormat() typography.TextFormat {
	return typography.TextFormat{
		Font: typography.FontInfo{
			Family: defaultLabelFontFamily(),
			Size:   14,
		},
		WrapMode:  typography.WrapNone,
		TextAlign: typography.TextAlignBegin,
		TextColor: typography.DefaultTextColor(),
	}
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

func (l *Label) TextFormat() typography.TextFormat {
	return l.format
}

func (l *Label) SetTextFormat(format typography.TextFormat) {
	format = normalizeLabelTextFormat(format)
	if sameTextFormat(l.format, format) {
		return
	}
	l.format = format
	l.RequestLayout()
}

func (l *Label) Measure(available geometry.Size) geometry.Size {
	if !l.Visible() {
		return geometry.Size{}
	}
	textLayout := l.newTextLayout(measureTextSize(available))
	if textLayout == nil {
		return geometry.Size{}
	}
	defer textLayout.Destroy()

	width, height := textLayout.MeasureSize()
	return geometry.Size{
		Width:  width,
		Height: height,
	}
}

func (l *Label) Paint(p Painter) {
	if !l.Visible() {
		return
	}
	textLayout := l.newTextLayout(l.Rect().Size)
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

func (l *Label) newTextLayout(size geometry.Size) typography.TextLayout {
	if App == nil {
		return nil
	}
	typo := App.Typography()
	if typo == nil {
		return nil
	}
	textLayout, err := typo.NewTextLayout(l.text, normalizeLabelTextFormat(l.format), size.Width, size.Height)
	if err != nil {
		return nil
	}
	return textLayout
}

func normalizeLabelTextFormat(format typography.TextFormat) typography.TextFormat {
	if format.Font.Family == "" {
		format.Font.Family = defaultLabelFontFamily()
	}
	if format.Font.Size <= 0 {
		format.Font.Size = 14
	}
	if format.TextColor == nil {
		format.TextColor = typography.DefaultTextColor()
	}
	return format
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

func sameTextFormat(a, b typography.TextFormat) bool {
	return a.Font == b.Font &&
		a.WrapMode == b.WrapMode &&
		a.TextAlign == b.TextAlign &&
		sameColor(a.TextColor, b.TextColor)
}

func sameColor(a, b color.Color) bool {
	if a == nil || b == nil {
		return a == b
	}
	ar, ag, ab, aa := a.RGBA()
	br, bg, bb, ba := b.RGBA()
	return ar == br && ag == bg && ab == bb && aa == ba
}
