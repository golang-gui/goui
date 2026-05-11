package typography

import "image/color"

type TextLayout interface {
	Destroy()
	Text() string
	Format() TextFormat
	Size() (maxWidth, maxHeight float32)
	SetSize(maxWidth, maxHeight float32)
	SetTextAlignment(align TextAlignment)
	SetLineAlignment(align LineAlignment)
	SetWordWrap(wrap WrapMode)
	SetTextFont(start, length int, font FontInfo)
	SetTextColor(start, length int, c color.Color)
	SetUnderline(start, length int, underline bool)
	SetStrikethrough(start, length int, strike bool)
	MeasureRect() (x, y, width, height float32)
	MeasureMetrics() (lines []TextLine, clusters []TextCluster)
}

type TextLine struct {
	Start    int
	Length   int
	X        float32
	Y        float32
	Width    float32
	Height   float32
	Baseline float32
	Clusters []TextCluster
}

type TextCluster struct {
	Start     int
	Length    int
	X         float32
	Y         float32
	Width     float32
	Height    float32
	LineIndex int
	Direction TextDirection
}

type TextDirection int

const (
	TextLeftToRight TextDirection = iota
	TextRightToLeft
)

type TextAlignment int

const (
	TextAlignBegin TextAlignment = iota
	TextAlignEnd
	TextAlignCenter
	TextAlignFill
)

type LineAlignment int

const (
	LineAlignBegin LineAlignment = iota
	LineAlignEnd
	LineAlignCenter
)
