package graphics

import (
	"image"

	"github.com/golang-gui/goui/platform/typography"
)

type Painter interface {
	Name() string
	Destroy()
	Begin(width, height, scale float32)
	End()
	SetClipRect(rect Rectangle)
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
	DrawTextLayout(origin Point, layout typography.TextLayout)
	DrawImage(rect Rectangle, img image.Image)
}
