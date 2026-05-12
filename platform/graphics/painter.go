package graphics

import (
	"github.com/golang-gui/goui/platform/typography"
	"image"
)

type Painter interface {
	Name() string
	Destroy()
	Begin(width, height, scale float32)
	End()
	Clear(color Color)
	FillRect(rect Rectangle, brush Brush)
	FillRoundRect(rect Rectangle, radius float32, brush Brush)
	FillEllipse(center Point, xRadius, yRadius float32, brush Brush)
	FillPath(path Path, brush Brush)
	DrawLine(p0, p1 Point, strokeWidth float32, brush Brush)
	DrawRect(rect Rectangle, strokeWidth float32, brush Brush)
	DrawRoundRect(rect Rectangle, radius, strokeWidth float32, brush Brush)
	DrawEllipse(center Point, xRadius, yRadius, strokeWidth float32, brush Brush)
	DrawPath(path Path, strokeWidth float32, brush Brush)
	DrawText(rect Rectangle, text string, format typography.TextFormat, brush Brush)
	DrawTextLayout(origin Point, layout typography.TextLayout, brush Brush)
	DrawImage(rect Rectangle, img image.Image)
}
