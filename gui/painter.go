package gui

import (
	"image"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/platform/typography"
)

type Painter interface {
	SetClipRect(rect geometry.Rectangle)
	Clear(color graphics.Color)
	FillRect(rect geometry.Rectangle, brush graphics.Brush)
	FillRoundRect(rect geometry.Rectangle, radius float32, brush graphics.Brush)
	FillEllipse(center geometry.Point, xRadius, yRadius float32, brush graphics.Brush)
	FillPath(path graphics.Path, brush graphics.Brush)
	DrawLine(p0, p1 geometry.Point, strokeWidth float32, brush graphics.Brush)
	DrawRect(rect geometry.Rectangle, strokeWidth float32, brush graphics.Brush)
	DrawRoundRect(rect geometry.Rectangle, radius, strokeWidth float32, brush graphics.Brush)
	DrawEllipse(center geometry.Point, xRadius, yRadius, strokeWidth float32, brush graphics.Brush)
	DrawPath(path graphics.Path, strokeWidth float32, brush graphics.Brush)
	DrawTextLayout(origin geometry.Point, layout typography.TextLayout)
	DrawImage(rect geometry.Rectangle, img image.Image)
}

func SubPainter(base Painter, rect geometry.Rectangle) Painter {
	return subPainter{
		base: base,
		rect: rect,
	}
}

type subPainter struct {
	base Painter
	rect geometry.Rectangle
}

func (p subPainter) SetClipRect(rect geometry.Rectangle) {
	if rect.Width == 0 && rect.Height == 0 {
		p.base.SetClipRect(p.rect)
		return
	}
	p.base.SetClipRect(p.translateRect(rect).Intersect(p.rect))
}

func (p subPainter) Clear(color graphics.Color) {
	p.base.FillRect(p.rect, color)
}

func (p subPainter) FillRect(rect geometry.Rectangle, brush graphics.Brush) {
	p.base.FillRect(p.translateRect(rect), brush)
}

func (p subPainter) FillRoundRect(rect geometry.Rectangle, radius float32, brush graphics.Brush) {
	p.base.FillRoundRect(p.translateRect(rect), radius, brush)
}

func (p subPainter) FillEllipse(center geometry.Point, xRadius, yRadius float32, brush graphics.Brush) {
	p.base.FillEllipse(center.Add(p.rect.Pos), xRadius, yRadius, brush)
}

func (p subPainter) FillPath(path graphics.Path, brush graphics.Brush) {
	p.base.FillPath(p.translatePath(path), brush)
}

func (p subPainter) DrawLine(p0, p1 geometry.Point, strokeWidth float32, brush graphics.Brush) {
	p.base.DrawLine(p0.Add(p.rect.Pos), p1.Add(p.rect.Pos), strokeWidth, brush)
}

func (p subPainter) DrawRect(rect geometry.Rectangle, strokeWidth float32, brush graphics.Brush) {
	p.base.DrawRect(p.translateRect(rect), strokeWidth, brush)
}

func (p subPainter) DrawRoundRect(rect geometry.Rectangle, radius, strokeWidth float32, brush graphics.Brush) {
	p.base.DrawRoundRect(p.translateRect(rect), radius, strokeWidth, brush)
}

func (p subPainter) DrawEllipse(center geometry.Point, xRadius, yRadius, strokeWidth float32, brush graphics.Brush) {
	p.base.DrawEllipse(center.Add(p.rect.Pos), xRadius, yRadius, strokeWidth, brush)
}

func (p subPainter) DrawPath(path graphics.Path, strokeWidth float32, brush graphics.Brush) {
	p.base.DrawPath(p.translatePath(path), strokeWidth, brush)
}

func (p subPainter) DrawTextLayout(origin geometry.Point, layout typography.TextLayout) {
	p.base.DrawTextLayout(origin.Add(p.rect.Pos), layout)
}

func (p subPainter) DrawImage(rect geometry.Rectangle, img image.Image) {
	p.base.DrawImage(p.translateRect(rect), img)
}

func (p subPainter) translateRect(rect geometry.Rectangle) geometry.Rectangle {
	rect.Pos = rect.Pos.Add(p.rect.Pos)
	return rect
}

func (p subPainter) translatePath(path graphics.Path) (translated graphics.Path) {
	empty := true
	path.Range(func(op graphics.PathOperation, args []float32) (stop bool) {
		switch op {
		case graphics.PathMoveTo:
			translated = graphics.MoveTo(args[0]+p.rect.X, args[1]+p.rect.Y)
			empty = false
		case graphics.PathLineTo:
			if empty {
				translated = graphics.MoveTo(p.rect.X, p.rect.Y)
				empty = false
			}
			translated = translated.LineTo(args[0]+p.rect.X, args[1]+p.rect.Y)
		case graphics.PathArcTo:
			if empty {
				translated = graphics.MoveTo(p.rect.X, p.rect.Y)
				empty = false
			}
			translated = translated.ArcTo(args[0], args[1], args[2], args[3], args[4], args[5]+p.rect.X, args[6]+p.rect.Y)
		case graphics.PathBezierTo:
			if empty {
				translated = graphics.MoveTo(p.rect.X, p.rect.Y)
				empty = false
			}
			translated = translated.BezierTo(
				args[0]+p.rect.X, args[1]+p.rect.Y,
				args[2]+p.rect.X, args[3]+p.rect.Y,
				args[4]+p.rect.X, args[5]+p.rect.Y,
			)
		case graphics.PathClose:
			if !empty {
				translated = translated.Close()
			}
		}
		return false
	})
	return translated
}
