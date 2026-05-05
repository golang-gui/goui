package typography

type TextLayout interface {
	Destroy()
	Name() string
	Text() string
	Format() TextFormat
	Size() (maxWidth, maxHeight float32)
	SetSize(maxWidth, maxHeight float32)
	SetTextAlignment(align TextAlignment)
	SetLineAlignment(align LineAlignment)
	SetWordWrap(wrap WrapMode)
	Attributes() []TextAttribute
	SetAttribute(attr TextAttribute)
	MeasureRect() (x, y, width, height float32)
	MeasureLines() (lines []TextLine, runs []TextRun)
}

type TextLine struct {
	Start    int
	Length   int
	X        float32
	Y        float32
	Width    float32
	Height   float32
	Baseline float32
	Runs     []TextRun
}

type TextRun struct {
	Start     int
	Length    int
	X         float32
	Y         float32
	Width     float32
	Height    float32
	Direction TextDirection
}

type TextAttribute struct {
	Start  int
	Length int
	Type   TextAttributeType
	Value  any
}

type TextAttributeType int

const (
	TextFont TextAttributeType = iota
	TextFgColor
	TextBgColor
	TextUnderline
	TextStrike
)

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
