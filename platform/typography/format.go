package typography

type TextFormat struct {
	Font      FontInfo
	WordWrap  WrapMode
	TextAlign TextAlignment
	LineAlign LineAlignment
}

type FontInfo struct {
	Family string
	Size   float32
	Weight float32
	Width  float32
}

type WrapMode int

const (
	WrapNone WrapMode = iota
	WrapWord
	WrapChar
	WrapWordChar
)
