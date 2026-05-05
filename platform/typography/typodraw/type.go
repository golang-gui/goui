package typodraw

import (
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/platform/typography"
)

type Context interface {
	DrawText(text string, format typography.TextFormat, brush graphics.Brush, size graphics.Size, pixelFormat graphics.PixelFormat, buf []byte) (bitmap TextBitmap, err error)
	DrawTextLayout(layout typography.TextLayout, brush graphics.Brush, pixelFormat graphics.PixelFormat, buf []byte) (bitmap TextBitmap, err error)
}

type TextBitmap struct {
	Offset graphics.Pos
	Bitmap graphics.Bitmap
}
