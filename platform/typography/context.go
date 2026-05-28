package typography

import "image/color"

type Context interface {
	Name() string
	Destroy()
	AddFont(fontFile string) error
	NewTextLayout(text string, format TextFormat, width, height float32) (TextLayout, error)
	DrawTextLayout(layout TextLayout, buf []byte) (bitmap TextBitmap, err error)
}

type TextFormat struct {
	Font      FontInfo
	WrapMode  WrapMode
	TextAlign TextAlignment
	TextColor color.Color
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
	WrapChar
	WrapWordChar
)

func DefaultTextColor() color.Color {
	return color.Black
}
