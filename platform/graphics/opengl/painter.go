package opengl

import (
	"fmt"
	"image"

	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/platform/graphics/utils"
	"github.com/golang-gui/goui/platform/typography"

	"github.com/golang-gui/nanovgo"
	"github.com/golang-gui/nanovgo/gl"
)

type Painter struct {
	ctx  Context
	vg   *nanovgo.Context
	typo typography.Context
	imgs []int
}

func NewPainter(win NativeWindow, typoCtx typography.Context) (_ graphics.Painter, err error) {
	p := new(Painter)
	p.typo = typoCtx
	p.ctx, err = NewContext(win, nil, DefaultConfig)
	if err != nil {
		return nil, fmt.Errorf("create opengl context err: %v", err)
	}

	err = p.ctx.MakeCurrent()
	if err != nil {
		return nil, fmt.Errorf("make current err: %v", err)
	}

	p.vg, err = nanovgo.NewContext(p.ctx, nanovgo.AntiAlias)
	if err != nil {
		p.Destroy()
		return nil, fmt.Errorf("create nanovgo context err: %v", err)
	}

	p.imgs = make([]int, 0, 512)
	return p, nil
}

func (p *Painter) Name() string {
	return "OpenGL"
}

func (p *Painter) Destroy() {
	if p.vg != nil {
		p.vg.Delete()
		p.vg = nil
	}
	if p.ctx != nil {
		p.ctx.Destroy()
		p.ctx = nil
	}
}

func (p *Painter) Begin(width, height uint) {
	p.ctx.MakeCurrent()
	gl.Viewport(0, 0, int(width), int(height))
	gl.Clear(gl.COLOR_BUFFER_BIT | gl.DEPTH_BUFFER_BIT | gl.STENCIL_BUFFER_BIT)
	p.vg.BeginFrame(int(width), int(height), 1.0)
}

func (p *Painter) End() {
	p.vg.EndFrame()
	for _, img := range p.imgs {
		p.vg.DeleteImage(img)
	}
	p.imgs = p.imgs[:0]
	p.ctx.SwapBuffers()
	p.ctx.ClearCurrent()
}

func (p *Painter) Clear(color graphics.Color) {
	gl.ClearColor(color.R, color.G, color.B, color.A)
	gl.Clear(gl.COLOR_BUFFER_BIT)
}

func (p *Painter) FillRect(rect graphics.Rectangle, brush graphics.Brush) {
	if p.beginFill(brush) {
		defer p.end()
		p.vg.BeginPath()
		p.vg.Rect(rect.X, rect.Y, rect.Width, rect.Height)
		p.vg.Fill()
	}
}

func (p *Painter) FillRoundRect(rect graphics.Rectangle, radius float32, brush graphics.Brush) {
	if p.beginFill(brush) {
		defer p.end()
		p.vg.BeginPath()
		p.vg.RoundedRect(rect.X, rect.Y, rect.Width, rect.Height, radius)
		p.vg.Fill()
	}
}

func (p *Painter) FillEllipse(center graphics.Point, xRadius, yRadius float32, brush graphics.Brush) {
	if p.beginFill(brush) {
		defer p.end()
		p.vg.BeginPath()
		p.vg.Ellipse(center.X, center.Y, xRadius, yRadius)
		p.vg.Fill()
	}
}

func (p *Painter) FillPath(path graphics.Path, brush graphics.Brush) {
	if p.beginFill(brush) {
		defer p.end()
		p.vg.BeginPath()
		closed := p.doPath(path)
		if !closed {
			p.vg.ClosePath()
		}
		p.vg.Fill()
	}
}

func (p *Painter) DrawLine(p0, p1 graphics.Point, strokeWidth float32, brush graphics.Brush) {
	if p.beginDraw(strokeWidth, brush) {
		defer p.end()
		p.vg.BeginPath()
		p.vg.MoveTo(p0.X, p0.Y)
		p.vg.LineTo(p1.X, p1.Y)
		p.vg.Stroke()
	}
}

func (p *Painter) DrawRect(rect graphics.Rectangle, strokeWidth float32, brush graphics.Brush) {
	if p.beginDraw(strokeWidth, brush) {
		defer p.end()
		p.vg.BeginPath()
		p.vg.Rect(rect.X, rect.Y, rect.Width, rect.Height)
		p.vg.Stroke()
	}
}

func (p *Painter) DrawRoundRect(rect graphics.Rectangle, radius, strokeWidth float32, brush graphics.Brush) {
	if p.beginDraw(strokeWidth, brush) {
		defer p.end()
		p.vg.BeginPath()
		p.vg.RoundedRect(rect.X, rect.Y, rect.Width, rect.Height, radius)
		p.vg.Stroke()
	}
}

func (p *Painter) DrawEllipse(center graphics.Point, xRadius, yRadius, strokeWidth float32, brush graphics.Brush) {
	if p.beginDraw(strokeWidth, brush) {
		defer p.end()
		p.vg.BeginPath()
		p.vg.Ellipse(center.X, center.Y, xRadius, yRadius)
		p.vg.Stroke()
	}
}

func (p *Painter) DrawPath(path graphics.Path, strokeWidth float32, brush graphics.Brush) {
	if p.beginDraw(strokeWidth, brush) {
		defer p.end()
		p.vg.BeginPath()
		p.doPath(path)
		p.vg.Stroke()
	}
}

func (p *Painter) DrawTextLayout(origin graphics.Point, layout typography.TextLayout) {
	if p.typo != nil {
		textBitmap, err := p.typo.DrawTextLayout(layout, nil)
		if err == nil {
			drawRect := graphics.Rect(origin.X, origin.Y, float32(textBitmap.Width), float32(textBitmap.Height))
			bitmap := graphics.Bitmap{
				Width:  textBitmap.Width,
				Height: textBitmap.Height,
				Stride: textBitmap.Stride,
				Format: graphics.PixelFormatRGBA,
				Pixels: textBitmap.Pixels,
			}
			p.drawBitmap(drawRect, bitmap)
		}
	}
}

func (p *Painter) DrawImage(rect graphics.Rectangle, img image.Image) {
	bitmap, ok := graphics.ToBitmap(img, graphics.PixelFormatRGBA)
	if !ok {
		bitmap = graphics.CopyToBitmap(img, graphics.PixelFormatRGBA, nil)
	}
	p.drawBitmap(rect, bitmap)
}

func (p *Painter) SetClipRect(rect graphics.Rectangle) {
	p.vg.ResetScissor()
	if rect.X != 0 || rect.Y != 0 || rect.Width != 0 || rect.Height != 0 {
		p.vg.Scissor(rect.X, rect.Y, rect.Width, rect.Height)
	}
}

func (p *Painter) drawBitmap(rect graphics.Rectangle, bitmap graphics.Bitmap) {
	img := p.vg.CreateImageRGBA(bitmap.Width, bitmap.Height, nanovgo.ImagePreMultiplied, bitmap.Pixels)
	if img != 0 {
		p.imgs = append(p.imgs, img)
		p.vg.BeginPath()
		p.vg.SetFillPaint(nanovgo.ImagePattern(rect.X, rect.Y, rect.Width, rect.Height, 0, img, 1.0))
		p.vg.Rect(rect.X, rect.Y, rect.Width, rect.Height)
		p.vg.Fill()
	}
	// TODO: add error log
}

func (p *Painter) beginFill(brush graphics.Brush) bool {
	if color, ok := brush.(graphics.Color); ok {
		p.vg.Save()
		p.vg.SetFillColor(nanovgo.Color{R: color.R, G: color.G, B: color.B, A: color.A})
		return true
	}
	return false
}

func (p *Painter) beginDraw(strokeWidth float32, brush graphics.Brush) bool {
	if color, ok := brush.(graphics.Color); ok {
		p.vg.Save()
		p.vg.SetStrokeWidth(strokeWidth)
		p.vg.SetStrokeColor(nanovgo.Color{R: color.R, G: color.G, B: color.B, A: color.A})
		return true
	}
	return false
}

func (p *Painter) end() {
	p.vg.Restore()
}

func (p *Painter) doPath(path graphics.Path) (closed bool) {
	var x, y float32
	path.Range(func(op graphics.PathOperation, args []float32) (stop bool) {
		switch op {
		case graphics.PathMoveTo:
			p.vg.MoveTo(args[0], args[1])
			x, y = args[0], args[1]

		case graphics.PathLineTo:
			p.vg.LineTo(args[0], args[1])
			x, y = args[0], args[1]

		case graphics.PathArcTo:
			p.arcTo(x, y, args[0], args[1], args[2], args[3], args[4], args[5], args[6])
			x, y = args[5], args[6]

		case graphics.PathBezierTo:
			p.vg.BezierTo(args[0], args[1], args[2], args[3], args[4], args[5])
			x, y = args[4], args[5]

		case graphics.PathClose:
			closed = true
			p.vg.ClosePath()
		}
		return closed
	})
	return
}

func (p *Painter) arcTo(sx, sy, rx, ry, angle, large, sweep, ex, ey float32) {
	lineTo := utils.LineTo(p.vg.LineTo)
	bezierTo := utils.BezierTo(p.vg.BezierTo)
	utils.ArcTo(lineTo, bezierTo, sx, sy, rx, ry, angle, large, sweep, ex, ey)
}
