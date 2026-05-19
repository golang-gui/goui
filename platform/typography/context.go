package typography

import "image/color"

type Context interface {
	Name() string
	Destroy()
	AddFont(fontFile string) error
	NewTextLayout(text string, format TextFormat, width, height float32) (TextLayout, error)
	DrawText(text string, format TextFormat, width, height float32, foreground color.Color, buf []byte) (bitmap TextBitmap, err error)
	DrawTextLayout(layout TextLayout, foreground color.Color, buf []byte) (bitmap TextBitmap, err error)
}

type TextFormat struct {
	Font      FontInfo
	WrapMode  WrapMode
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
	WrapChar
	WrapWordChar
)
