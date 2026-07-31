package software

import (
	"image"
	"image/color"
	"math"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/platform/graphics/utils"
	"github.com/golang-gui/goui/platform/typography"

	"github.com/goexlib/mathx"
	"github.com/srwiley/rasterx"
	"github.com/srwiley/scanFT"
	"golang.org/x/image/math/fixed"
)

type Painter struct {
	drawable  Drawable
	typo      typography.Context
	bgra      image.RGBA
	line      image.RGBA
	viewport  graphics.Rectangle
	scanner   rasterx.Scanner
	filler    *rasterx.Filler
	stroker   *rasterx.Stroker
	pixelBuf  []byte
	lineBuf   []byte
	scale     float32
	transform geometry.Transform
}

type Drawable interface {
	Draw(img image.Image) error
}

func NewPainter(drawable Drawable, typo typography.Context) (graphics.Painter, error) {
	p := new(Painter)
	p.drawable = drawable
	p.typo = typo
	return p, nil
}

func (p *Painter) Name() string {
	return "Software"
}

func (p *Painter) Destroy() {

}

func (p *Painter) Begin(width, height, scale float32) {
	// TODO: scale
	viewport := graphics.Rect(0, 0, width, height)
	if p.viewport.Width != viewport.Width || p.viewport.Height != viewport.Height {
		p.viewport = viewport
		initRGBA(&p.bgra, int(width), int(height), p.pixelBuf)
		p.pixelBuf = p.bgra.Pix

		initRGBA(&p.line, int(width), 1, p.lineBuf)
		p.lineBuf = p.line.Pix

		p.scanner = scanFT.NewScannerFT(int(width), int(height), scanFT.NewRGBAPainter(&p.bgra))
		p.filler = rasterx.NewFiller(int(width), int(height), p.scanner)
		p.stroker = rasterx.NewStroker(int(width), int(height), p.scanner)
		p.scale = scale
	}
	p.filler.Clear()
	p.stroker.Clear()
	p.transform = geometry.Identity()
}

func (p *Painter) End() {
	p.drawable.Draw(graphics.Bitmap{
		Width:  p.bgra.Rect.Max.X,
		Height: p.bgra.Rect.Max.Y,
		Stride: p.bgra.Stride,
		Format: graphics.PixelFormatBGRA, // reversed
		Pixels: p.bgra.Pix,
	})
}

func (p *Painter) Clear(color graphics.Color) {
	p.fillLine(color)
	for y := 0; y < p.bgra.Rect.Max.Y; y++ {
		offset := p.bgra.PixOffset(0, y)
		end := offset + p.bgra.Stride
		copy(p.bgra.Pix[offset:end], p.line.Pix)
	}
}

func (p *Painter) FillRect(rect graphics.Rectangle, brush graphics.Brush) {
	rect = rect.Scale(p.scale)
	rect = p.transformRect(rect)
	if color, ok := brush.(graphics.Color); ok {
		if rect.Contains(p.viewport) {
			p.Clear(color)
			return
		}

		defer p.filler.Clear()
		p.filler.SetClip(toClipRect(rect.X, rect.Y, rect.Width, rect.Height))
		defer p.filler.SetClip(image.Rectangle{})
		p.filler.SetColor(reverseColor(color))
		p1 := rect.RightBottom()
		rasterx.AddRect(float64(rect.X), float64(rect.Y), float64(p1.X), float64(p1.Y), 0, p.filler)
		p.filler.Draw()
	}
}

func (p *Painter) FillRoundRect(rect graphics.Rectangle, radius float32, brush graphics.Brush) {
	rect = rect.Scale(p.scale)
	radius *= p.scale
	rect = p.transformRect(rect)
	if color, ok := brush.(graphics.Color); ok {
		defer p.filler.Clear()
		p.filler.SetClip(toClipRect(rect.X, rect.Y, rect.Width, rect.Height))
		defer p.filler.SetClip(image.Rectangle{})
		p.filler.SetColor(reverseColor(color))
		p1 := rect.RightBottom()
		rasterx.AddRoundRect(float64(rect.X), float64(rect.Y), float64(p1.X), float64(p1.Y), float64(radius), float64(radius), 0, rasterx.RoundGap, p.filler)
		p.filler.Draw()
	}
}

func (p *Painter) FillEllipse(center graphics.Point, xRadius, yRadius float32, brush graphics.Brush) {
	center = center.Scale(p.scale)
	xRadius *= p.scale
	yRadius *= p.scale
	center.X, center.Y = p.transformPoint(center.X, center.Y)
	if color, ok := brush.(graphics.Color); ok {
		defer p.filler.Clear()
		p.filler.SetClip(toClipRect(center.X-xRadius, center.Y-yRadius, center.X+xRadius, center.Y+yRadius))
		defer p.filler.SetClip(image.Rectangle{})
		rasterx.AddEllipse(float64(center.X), float64(center.Y), float64(xRadius), float64(yRadius), 0, p.filler)
		p.filler.SetColor(reverseColor(color))
		p.filler.Draw()
	}
}

func (p *Painter) FillPath(path graphics.Path, brush graphics.Brush) {
	if color, ok := brush.(graphics.Color); ok {
		defer p.filler.Clear()
		closed, clip := p.doPath(p.filler, path)
		if !closed {
			p.filler.Stop(true)
		}
		p.filler.SetClip(clip)
		defer p.filler.SetClip(image.Rectangle{})
		p.filler.SetColor(color)
		p.filler.Draw()
	}
}

func (p *Painter) DrawLine(p0, p1 graphics.Point, strokeWidth float32, brush graphics.Brush) {
	p0 = p0.Scale(p.scale)
	p1 = p1.Scale(p.scale)
	strokeWidth *= p.scale
	p0.X, p0.Y = p.transformPoint(p0.X, p0.Y)
	p1.X, p1.Y = p.transformPoint(p1.X, p1.Y)
	if color, ok := brush.(graphics.Color); ok {
		defer p.stroker.Clear()
		p.stroker.SetClip(toClipRect(min(p0.X, p1.X), min(p0.Y, p1.Y), mathx.Abs(p1.X-p0.X), mathx.Abs(p1.Y-p0.Y)).Inset(-uptoPixel(strokeWidth)))
		defer p.stroker.SetClip(image.Rectangle{})
		p.stroker.SetStroke(toFixedI(strokeWidth), toFixedI(4), rasterx.ButtCap, nil, rasterx.FlatGap, rasterx.MiterClip)
		p.stroker.SetColor(reverseColor(color))

		p.stroker.Start(toFixedP(p0.X, p0.Y))
		p.stroker.Line(toFixedP(p1.X, p1.Y))
		p.stroker.Stop(false)

		p.stroker.Draw()
	}
}

func (p *Painter) DrawRect(rect graphics.Rectangle, strokeWidth float32, brush graphics.Brush) {
	rect = rect.Scale(p.scale)
	strokeWidth *= p.scale
	rect = p.transformRect(rect)
	if color, ok := brush.(graphics.Color); ok {
		defer p.stroker.Clear()
		p.stroker.SetClip(toClipRect(rect.X, rect.Y, rect.Width, rect.Height).Inset(-uptoPixel(strokeWidth)))
		defer p.stroker.SetClip(image.Rectangle{})
		p.stroker.SetStroke(toFixedI(strokeWidth), toFixedI(4), rasterx.ButtCap, nil, rasterx.FlatGap, rasterx.MiterClip)
		p.stroker.SetColor(reverseColor(color))
		p1 := rect.RightBottom()
		rasterx.AddRect(float64(rect.X), float64(rect.Y), float64(p1.X), float64(p1.Y), 0, p.stroker)
		p.stroker.Draw()
	}
}

func (p *Painter) DrawRoundRect(rect graphics.Rectangle, radius, strokeWidth float32, brush graphics.Brush) {
	rect = rect.Scale(p.scale)
	radius *= p.scale
	strokeWidth *= p.scale
	rect = p.transformRect(rect)
	if color, ok := brush.(graphics.Color); ok {
		defer p.stroker.Clear()
		p.stroker.SetClip(toClipRect(rect.X, rect.Y, rect.Width, rect.Height).Inset(-uptoPixel(strokeWidth)))
		defer p.stroker.SetClip(image.Rectangle{})
		p.stroker.SetStroke(toFixedI(strokeWidth), toFixedI(4), rasterx.ButtCap, nil, rasterx.FlatGap, rasterx.MiterClip)
		p.stroker.SetColor(reverseColor(color))
		p1 := rect.RightBottom()
		rasterx.AddRoundRect(float64(rect.X), float64(rect.Y), float64(p1.X), float64(p1.Y), float64(radius), float64(radius), 0, rasterx.RoundGap, p.stroker)
		p.stroker.Draw()
	}
}

func (p *Painter) DrawEllipse(center graphics.Point, xRadius, yRadius, strokeWidth float32, brush graphics.Brush) {
	center = center.Scale(p.scale)
	xRadius *= p.scale
	yRadius *= p.scale
	if color, ok := brush.(graphics.Color); ok {
		defer p.stroker.Clear()
		p.stroker.SetClip(toClipRect(center.X-xRadius, center.Y-yRadius, center.X+xRadius, center.Y+yRadius).Inset(-uptoPixel(strokeWidth)))
		defer p.stroker.SetClip(image.Rectangle{})
		p.stroker.SetStroke(toFixedI(strokeWidth), toFixedI(4), rasterx.ButtCap, nil, rasterx.FlatGap, rasterx.MiterClip)
		p.stroker.SetColor(reverseColor(color))
		rasterx.AddEllipse(float64(center.X), float64(center.Y), float64(xRadius), float64(yRadius), 0, p.stroker)
		p.stroker.Draw()
	}
}

func (p *Painter) DrawPath(path graphics.Path, strokeWidth float32, brush graphics.Brush) {
	strokeWidth *= p.scale
	if color, ok := brush.(graphics.Color); ok {
		defer p.stroker.Clear()
		p.stroker.SetStroke(toFixedI(strokeWidth), toFixedI(4), rasterx.ButtCap, nil, rasterx.FlatGap, rasterx.MiterClip)
		p.stroker.SetColor(reverseColor(color))

		closed, clip := p.doPath(p.stroker, path)
		if !closed {
			p.stroker.Stop(false)
		}

		p.stroker.SetClip(clip.Inset(-uptoPixel(strokeWidth)))
		defer p.stroker.SetClip(image.Rectangle{})

		p.stroker.Draw()
	}
}

func (p *Painter) DrawTextLayout(origin graphics.Point, layout typography.TextLayout) {
	// Snap the text bitmap origin to the device pixel grid so the pre-rasterized
	// glyphs land 1:1 on physical pixels instead of being resampled (blurred).
	origin.X = float32(math.Round(float64(origin.X*p.scale))) / p.scale
	origin.Y = float32(math.Round(float64(origin.Y*p.scale))) / p.scale
	if p.typo != nil {
		textBitmap, err := p.typo.DrawTextLayout(layout, p.scale, nil)
		if err == nil {
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

func (p *Painter) Transform(t geometry.Transform) {
	p.transform = t
}

// transformPoint applies the current transform to a point.
func (p *Painter) transformPoint(x, y float32) (float32, float32) {
	t := p.transform
	return t.A11*x + t.A12*y + t.TX, t.A21*x + t.A22*y + t.TY
}

// transformRect applies the current transform to a rectangle's corners and
// returns the bounding box of the result.
func (p *Painter) transformRect(rect graphics.Rectangle) graphics.Rectangle {
	x0, y0 := p.transformPoint(rect.X, rect.Y)
	x1, y1 := p.transformPoint(rect.X+rect.Width, rect.Y)
	x2, y2 := p.transformPoint(rect.X+rect.Width, rect.Y+rect.Height)
	x3, y3 := p.transformPoint(rect.X, rect.Y+rect.Height)
	minX := min(x0, min(x1, min(x2, x3)))
	minY := min(y0, min(y1, min(y2, y3)))
	maxX := max(x0, max(x1, max(x2, x3)))
	maxY := max(y0, max(y1, max(y2, y3)))
	return graphics.Rect(minX, minY, maxX-minX, maxY-minY)
}

func (p *Painter) DrawImage(rect graphics.Rectangle, img image.Image) {
	bitmap, ok := graphics.ToBitmap(img, graphics.PixelFormatRGBA)
	if !ok {
		bitmap = graphics.CopyToBitmap(img, graphics.PixelFormatRGBA, nil)
	}
	p.drawBitmap(rect, bitmap)
}

func (p *Painter) SetClipRect(rect graphics.Rectangle) {
	rect = rect.Scale(p.scale)
	p.scanner.SetClip(image.Rectangle{})
	if rect.X != 0 || rect.Y != 0 || rect.Width != 0 || rect.Height != 0 {
		p.scanner.SetClip(toClipRect(rect.X, rect.Y, rect.Width, rect.Height))
	}
}

func (p *Painter) drawBitmap(rect graphics.Rectangle, bitmap graphics.Bitmap) {
	defer p.filler.Clear()

	// Transform the 4 corners of the destination rect to device space.
	// This supports arbitrary affine transforms (translate/rotate/scale).
	corners := [4]struct{ x, y float32 }{
		{rect.X, rect.Y},
		{rect.X + rect.Width, rect.Y},
		{rect.X + rect.Width, rect.Y + rect.Height},
		{rect.X, rect.Y + rect.Height},
	}
	var dev [4]struct{ x, y float32 }
	for i, c := range corners {
		dev[i].x, dev[i].y = p.transformPoint(c.x*p.scale, c.y*p.scale)
	}

	// Bounding box of the transformed quad for clipping.
	minX := min(dev[0].x, min(dev[1].x, min(dev[2].x, dev[3].x)))
	minY := min(dev[0].y, min(dev[1].y, min(dev[2].y, dev[3].y)))
	maxX := max(dev[0].x, max(dev[1].x, max(dev[2].x, dev[3].x)))
	maxY := max(dev[0].y, max(dev[1].y, max(dev[2].y, dev[3].y)))
	p.filler.SetClip(toClipRect(minX, minY, maxX-minX, maxY-minY))
	defer p.filler.SetClip(image.Rectangle{})

	// Precompute the inverse transform so ColorFunc can map device pixels
	// back to source bitmap coordinates.
	srcBounds := bitmap.Bounds()
	srcW := float32(srcBounds.Dx())
	srcH := float32(srcBounds.Dy())
	invScale := 1.0 / p.scale
	inv := p.transform.Inverse()

	p.filler.SetColor(rasterx.ColorFunc(func(x, y int) color.Color {
		// Device pixel → logical coord (undo scale) → inverse transform → source rect space
		lx := float32(x) * invScale
		ly := float32(y) * invScale
		src := inv.TransformPoint(geometry.Point{X: lx, Y: ly})
		// Map from rect space to bitmap pixel space.
		bx := srcBounds.Min.X + int((src.X-rect.X)/rect.Width*srcW)
		by := srcBounds.Min.Y + int((src.Y-rect.Y)/rect.Height*srcH)
		if bx < srcBounds.Min.X || bx >= srcBounds.Max.X || by < srcBounds.Min.Y || by >= srcBounds.Max.Y {
			return color.RGBA{0, 0, 0, 0}
		}
		return reverseColor(bitmap.At(bx, by))
	}))

	// Build a quad path through the 4 transformed corners.
	p.filler.Start(toFixedP(dev[0].x, dev[0].y))
	p.filler.Line(toFixedP(dev[1].x, dev[1].y))
	p.filler.Line(toFixedP(dev[2].x, dev[2].y))
	p.filler.Line(toFixedP(dev[3].x, dev[3].y))
	p.filler.Stop(true)
	p.filler.Draw()
}

func (p *Painter) fillLine(c graphics.Color) {
	r, g, b, a := c.RGBA8()
	for x := 0; x < p.line.Rect.Max.X; x++ {
		offset := p.line.PixOffset(x, 0)
		p.line.Pix[offset] = b
		p.line.Pix[offset+1] = g
		p.line.Pix[offset+2] = r
		p.line.Pix[offset+3] = a
	}
}

func (p *Painter) doPath(add rasterx.Adder, path graphics.Path) (closed bool, clip image.Rectangle) {
	minX := float32(math.MaxFloat32)
	minY := float32(math.MaxFloat32)
	maxX := float32(-math.MaxFloat32)
	maxY := float32(-math.MaxFloat32)

	updateBounds := func(x, y float32) {
		if x < minX {
			minX = x
		}
		if y < minY {
			minY = y
		}
		if x > maxX {
			maxX = x
		}
		if y > maxY {
			maxY = y
		}
	}

	px := float32(0)
	py := float32(0)
	sx := float32(0)
	sy := float32(0)

	path.Range(func(op graphics.PathOperation, args []float32) (stop bool) {
		switch op {
		case graphics.PathMoveTo:
			x, y := p.transformPoint(args[0]*p.scale, args[1]*p.scale)
			add.Start(toFixedP(x, y))
			px, py = x, y
			sx, sy = x, y
			updateBounds(x, y)

		case graphics.PathLineTo:
			x, y := p.transformPoint(args[0]*p.scale, args[1]*p.scale)
			add.Line(toFixedP(x, y))
			updateBounds(x, y)
			px, py = x, y

		case graphics.PathArcTo:
			rx, ry := args[0]*p.scale, args[1]*p.scale
			angle, large, sweep := args[2], args[3], args[4]
			x, y := p.transformPoint(args[5]*p.scale, args[6]*p.scale)
			p.arcTo(add, px, py, rx, ry, angle, large, sweep, x, y)

			updateBounds(px, py)
			updateBounds(x, y)

			cosA := mathx.Cos(angle * math.Pi / 180)
			sinA := mathx.Sin(angle * math.Pi / 180)
			rxAbs := mathx.Abs(rx)
			ryAbs := mathx.Abs(ry)
			cxApprox := (px + x) / 2
			cyApprox := (py + y) / 2
			dx := rxAbs*mathx.Abs(cosA) + ryAbs*mathx.Abs(sinA)
			dy := rxAbs*mathx.Abs(sinA) + ryAbs*mathx.Abs(cosA)
			updateBounds(cxApprox-dx, cyApprox-dy)
			updateBounds(cxApprox+dx, cyApprox+dy)
			px, py = x, y

		case graphics.PathBezierTo:
			x0, y0 := p.transformPoint(args[0]*p.scale, args[1]*p.scale)
			x1, y1 := p.transformPoint(args[2]*p.scale, args[3]*p.scale)
			x, y := p.transformPoint(args[4]*p.scale, args[5]*p.scale)
			add.CubeBezier(toFixedP(x0, y0), toFixedP(x1, y1), toFixedP(x, y))
			// 控制点 + 终点
			updateBounds(x0, y0)
			updateBounds(x1, y1)
			updateBounds(x, y)
			px, py = x, y

		case graphics.PathClose:
			closed = true
			add.Stop(true)
			updateBounds(sx, sy)
		}

		return closed
	})

	if minX > maxX {
		return
	}

	clip = image.Rect(int(math.Floor(float64(minX))),
		int(math.Floor(float64(minY))),
		int(math.Ceil(float64(maxX)))+1,
		int(math.Ceil(float64(maxY)))+1)

	return
}

func (p *Painter) arcTo(add rasterx.Adder, sx, sy, rx, ry, angle, large, sweep, ex, ey float32) {
	lineTo := utils.LineTo(func(x, y float32) {
		add.Line(toFixedP(x, y))
	})
	bezierTo := utils.BezierTo(func(x0, y0, x1, y1, x2, y2 float32) {
		add.CubeBezier(toFixedP(x0, y0), toFixedP(x1, y1), toFixedP(x2, y2))
	})
	utils.ArcTo(lineTo, bezierTo, sx, sy, rx, ry, angle, large, sweep, ex, ey)
}

func initRGBA(rgba *image.RGBA, width, height int, buf []byte) {
	rgba.Stride = width * 4
	rgba.Rect = image.Rect(0, 0, width, height)
	byteSize := rgba.Stride * height
	if cap(buf) >= byteSize {
		rgba.Pix = buf[:byteSize]
	} else {
		rgba.Pix = make([]uint8, byteSize)
	}
}

func reverseColor(c color.Color) color.RGBA {
	r, g, b, a := c.RGBA()
	return color.RGBA{
		R: byte(b),
		G: byte(g),
		B: byte(r),
		A: byte(a),
	}
}

func toFixedP(x, y float32) (p fixed.Point26_6) {
	p.X = fixed.Int26_6(x * 64)
	p.Y = fixed.Int26_6(y * 64)
	return
}

func toFixedI(v float32) fixed.Int26_6 {
	return fixed.Int26_6(v * 64)
}

func toClipRect(x, y, w, h float32) image.Rectangle {
	x0 := int(x) - 1
	y0 := int(y) - 1
	return image.Rect(x0, y0, uptoPixel(x+w)+1, uptoPixel(y+h)+1)
}

func roundPixel(v float32) int {
	return int(v + 0.5)
}

func uptoPixel(v float32) int {
	return int(v + 0.99)
}
