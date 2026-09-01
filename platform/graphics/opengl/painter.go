package opengl

import (
	"fmt"
	"image"
	"math"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/platform/graphics/internal/boxshadow"
	"github.com/golang-gui/goui/platform/graphics/utils"
	"github.com/golang-gui/goui/platform/typography"

	"github.com/golang-gui/nanovgo"
	"github.com/golang-gui/nanovgo/gl"
)

type Painter struct {
	ctx        Context
	vg         *nanovgo.Context
	win        NativeWindow
	typo       typography.Context
	imgs       []int
	scale      float32
	transform  geometry.Transform
	lastWidth  float32
	lastHeight float32

	hasFrame     bool
	resizedFrame bool
	resizedPaint bool
}

func NewPainter(win NativeWindow, typoCtx typography.Context) (_ graphics.Painter, err error) {
	p := new(Painter)
	p.ctx, err = NewContext(win, nil, Config{
		PixelFormat: PixelFormat{
			RedBits:      8,
			GreenBits:    8,
			BlueBits:     8,
			AlphaBits:    0,
			DepthBits:    24,
			StencilBits:  8,
			Samples:      0,
			DoubleBuffer: true,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("create opengl context err: %v", err)
	}

	if err = p.ctx.MakeCurrent(); err != nil {
		return nil, fmt.Errorf("make current err: %v", err)
	}

	if p.vg, err = nanovgo.NewContext(p.ctx, nanovgo.AntiAlias); err != nil {
		p.Destroy()
		return nil, fmt.Errorf("create nanovgo context err: %v", err)
	}

	p.win = win
	p.typo = typoCtx
	_, p.resizedPaint = p.ctx.(GLXContext)

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

func (p *Painter) Begin(width, height, scale float32) {
	p.ctx.MakeCurrent()
	p.trackFrameSize(width, height)
	gl.Viewport(0, 0, int(width), int(height))
	gl.Clear(gl.COLOR_BUFFER_BIT | gl.DEPTH_BUFFER_BIT | gl.STENCIL_BUFFER_BIT)
	p.vg.BeginFrame(int(width/scale), int(height/scale), scale)
	p.scale = scale
	p.transform = geometry.Identity()
}

func (p *Painter) End() {
	p.vg.EndFrame()
	for _, img := range p.imgs {
		p.vg.DeleteImage(img)
	}
	p.imgs = p.imgs[:0]
	p.present()
}

func (p *Painter) trackFrameSize(width, height float32) {
	p.resizedFrame = p.hasFrame && (width != p.lastWidth || height != p.lastHeight)
	p.lastWidth, p.lastHeight = width, height
	p.hasFrame = true
}

func (p *Painter) present() {
	p.ctx.SwapBuffers()
	p.ctx.ClearCurrent()

	if p.resizedFrame && p.resizedPaint {
		p.resizedFrame = false
		_ = p.win.RequestPaint()
	}
}

func (p *Painter) Clear(color graphics.Color) {
	gl.ClearColor(color.R, color.G, color.B, color.A)
	gl.Clear(gl.COLOR_BUFFER_BIT)
}

func (p *Painter) DrawBoxShadow(rect graphics.Rectangle, radius float32, shadow graphics.BoxShadow) {
	if shadow.Color.A <= 0 {
		return
	}
	shape, ok := boxshadow.Normalize(rect, radius, shadow.Offset, shadow.BlurRadius, shadow.SpreadRadius)
	if !ok {
		return
	}
	if shape.BlurRadius <= 0 {
		p.FillRoundRect(shape.Rect, shape.Radius, shadow.Color)
		return
	}

	bounds := shape.Bounds()
	p.vg.Save()
	defer p.vg.Restore()
	p.vg.BeginPath()
	p.vg.Rect(bounds.X, bounds.Y, bounds.Width, bounds.Height)
	p.vg.SetFillPaint(nanovgo.BoxShadow(
		shape.Rect.X, shape.Rect.Y, shape.Rect.Width, shape.Rect.Height,
		shape.Radius, shape.BlurRadius, nanoVGColor(shadow.Color),
	))
	p.vg.Fill()
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
		textBitmap, err := p.typo.DrawTextLayout(layout, p.scale, nil)
		if err == nil {
			// Snap the text bitmap origin to the device pixel grid so the pre-rasterized
			// glyphs land 1:1 on physical pixels instead of being resampled (blurred).
			origin = snapTextOrigin(origin, p.transform, p.scale)
			drawRect := graphics.Rect(origin.X, origin.Y, float32(textBitmap.Width)/p.scale, float32(textBitmap.Height)/p.scale)
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

func (p *Painter) SetTransform(t geometry.Transform) {
	p.transform = t
	// NanoVG's SetTransform pre-multiplies the current transform. To set an
	// absolute transform, reset first, then apply.
	p.vg.ResetTransform()
	a, b, c, d, e, f := nanoVGTransformValues(t)
	p.vg.SetTransformByValue(a, b, c, d, e, f)
}

// nanoVGTransformValues converts GOUI's row-major transform to NanoVG's
// [a c e; b d f] parameter order.
func nanoVGTransformValues(t geometry.Transform) (a, b, c, d, e, f float32) {
	return t.A11, t.A21, t.A12, t.A22, t.TX, t.TY
}

// snapTextOrigin snaps the final position of a pre-rasterized text bitmap.
// A non-translation transform changes the bitmap itself, so it must remain
// free to be sampled by NanoVG rather than receiving a translation-only snap.
func snapTextOrigin(origin geometry.Point, transform geometry.Transform, scale float32) geometry.Point {
	if scale <= 0 || transform.A11 != 1 || transform.A12 != 0 ||
		transform.A21 != 0 || transform.A22 != 1 {
		return origin
	}
	return geometry.Point{
		X: float32(math.Round(float64((origin.X+transform.TX)*scale)))/scale - transform.TX,
		Y: float32(math.Round(float64((origin.Y+transform.TY)*scale)))/scale - transform.TY,
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
	// NanoVG's Scissor is transformed by the current transform. Since the
	// clip rect is already in window-local coordinates (the transform's
	// offset has been applied by the GUI layer), we must set the scissor in
	// identity transform space. Save and restore the transform manually
	// (NOT via vg.Save/Restore, which would also revert the scissor we just
	// set).
	xform := p.vg.CurrentTransform()
	p.vg.ResetTransform()
	p.vg.ResetScissor()
	if rect.X != 0 || rect.Y != 0 || rect.Width != 0 || rect.Height != 0 {
		p.vg.Scissor(rect.X, rect.Y, rect.Width, rect.Height)
	}
	p.vg.SetTransformByValue(xform[0], xform[1], xform[2], xform[3], xform[4], xform[5])
}

func (p *Painter) drawBitmap(rect graphics.Rectangle, bitmap graphics.Bitmap) {
	img := p.vg.CreateImageRGBA(bitmap.Width, bitmap.Height, nanovgo.ImagePreMultiplied, bitmap.Pixels)
	if img != 0 {
		p.imgs = append(p.imgs, img)
		p.vg.Save()
		p.vg.BeginPath()
		p.vg.SetFillPaint(nanovgo.ImagePattern(rect.X, rect.Y, rect.Width, rect.Height, 0, img, 1.0))
		p.vg.Rect(rect.X, rect.Y, rect.Width, rect.Height)
		p.vg.Fill()
		p.vg.Restore()
	}
	// TODO: add error log
}

func (p *Painter) beginFill(brush graphics.Brush) bool {
	p.vg.Save()
	switch brush := brush.(type) {
	case graphics.Color:
		p.vg.SetFillColor(nanoVGColor(brush))
	case graphics.LinearGradient:
		if brush.Start == brush.End {
			p.vg.SetFillColor(nanoVGColor(brush.StartColor))
		} else {
			p.vg.SetFillPaint(nanovgo.LinearGradient(
				brush.Start.X, brush.Start.Y, brush.End.X, brush.End.Y,
				nanoVGColor(brush.StartColor), nanoVGColor(brush.EndColor),
			))
		}
	default:
		p.vg.Restore()
		return false
	}
	return true
}

func (p *Painter) beginDraw(strokeWidth float32, brush graphics.Brush) bool {
	p.vg.Save()
	p.vg.SetStrokeWidth(strokeWidth)
	switch brush := brush.(type) {
	case graphics.Color:
		p.vg.SetStrokeColor(nanoVGColor(brush))
	case graphics.LinearGradient:
		if brush.Start == brush.End {
			p.vg.SetStrokeColor(nanoVGColor(brush.StartColor))
		} else {
			p.vg.SetStrokePaint(nanovgo.LinearGradient(
				brush.Start.X, brush.Start.Y, brush.End.X, brush.End.Y,
				nanoVGColor(brush.StartColor), nanoVGColor(brush.EndColor),
			))
		}
	default:
		p.vg.Restore()
		return false
	}
	return true
}

func nanoVGColor(color graphics.Color) nanovgo.Color {
	return nanovgo.Color{R: color.R, G: color.G, B: color.B, A: color.A}
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
