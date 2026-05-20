package pango

// Opaque types
type (
	FontDescription uintptr
	Language        uintptr
)

// Rectangle represents a rectangle
type Rectangle struct {
	X      int32
	Y      int32
	Width  int32
	Height int32
}

// WrapMode describes how to wrap the lines of a PangoLayout
type WrapMode int32

const (
	WrapWord     WrapMode = 0
	WrapChar     WrapMode = 1
	WrapWordChar WrapMode = 2
)

// Alignment describes how to align the lines of a PangoLayout
type Alignment int32

const (
	AlignLeft   Alignment = 0
	AlignCenter Alignment = 1
	AlignRight  Alignment = 2
)

// Direction represents a direction in the Unicode bidirectional algorithm
type Direction int32

const (
	DirectionLTR     Direction = 0
	DirectionRTL     Direction = 1
	DirectionTTBLTR  Direction = 2
	DirectionTTBRTL  Direction = 3
	DirectionWeakLTR Direction = 4
	DirectionWeakRTL Direction = 5
	DirectionNeutral Direction = 6
)

// Scale is the scale between dimensions used for Pango distances and device units
const Scale = 1024

// AttrType distinguishes between different types of attributes
type AttrType int32

const (
	AttrInvalid            AttrType = 0
	AttrLanguage           AttrType = 1
	AttrFamily             AttrType = 2
	AttrStyle              AttrType = 3
	AttrWeight             AttrType = 4
	AttrVariant            AttrType = 5
	AttrStretch            AttrType = 6
	AttrSize               AttrType = 7
	AttrFontDesc           AttrType = 8
	AttrForeground         AttrType = 9
	AttrBackground         AttrType = 10
	AttrUnderline          AttrType = 11
	AttrStrikethrough      AttrType = 12
	AttrRise               AttrType = 13
	AttrShape              AttrType = 14
	AttrScale              AttrType = 15
	AttrFallback           AttrType = 16
	AttrLetterSpacing      AttrType = 17
	AttrUnderlineColor     AttrType = 18
	AttrStrikethroughColor AttrType = 19
	AttrAbsoluteSize       AttrType = 20
	AttrGravity            AttrType = 21
	AttrGravityHint        AttrType = 22
	AttrFontFeatures       AttrType = 23
	AttrForegroundAlpha    AttrType = 24
	AttrBackgroundAlpha    AttrType = 25
)

type Underline uint32

const (
	UnderlineNone Underline = iota
	UnderlineSingle
	UnderlineDouble
	UnderlineLow
	UnderlineError
	UnderlineSingleLine
	UnderlineDoubleLine
	UnderlineErrorLine
)
