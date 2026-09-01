package direct2d

import (
	"fmt"
	"image"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/platform/typography"
	"github.com/golang-gui/goui/platform/typography/directwrite"

	"github.com/golang-gui/goui/platform/windows/sdk/com"
	"github.com/golang-gui/goui/platform/windows/sdk/d2d1"
	"github.com/golang-gui/goui/platform/windows/sdk/dxgi"

	"github.com/goexlib/mathx"
)

type Painter struct {
	typoCtx     typography.Context
	dwTypo      *directwrite.Context
	factory     *d2d1.Factory
	render      *d2d1.HwndRenderTarget
	colorBrush  *d2d1.SolidColorBrush
	color       d2d1.ColorF
	linearStops *d2d1.GradientStopCollection
	linearBrush *d2d1.LinearGradientBrush
	linearStart graphics.Color
	linearEnd   graphics.Color
	hasLinear   bool
	sizeU       d2d1.SizeU
	rect        d2d1.RectF
	roundRect   d2d1.RoundRect
	ellipse     d2d1.Ellipse
	clip        d2d1.RectF
	imageBuf    []byte
	scale       float32
	transform   geometry.Transform
	matrix      d2d1.Matrix3x2F
}

type NativeWindow interface {
	NativeHandle() uintptr
}

func NewPainter(win NativeWindow, typoCtx typography.Context) (_ graphics.Painter, err error) {
	p := new(Painter)
	p.typoCtx = typoCtx
	p.dwTypo, _ = typoCtx.(*directwrite.Context)

	p.factory, err = d2d1.CreateFactory[d2d1.Factory](d2d1.D2D1_FACTORY_TYPE_SINGLE_THREADED, d2d1.IID_ID2D1Factory, nil)
	if err != nil {
		return nil, fmt.Errorf("create d2d factory err: %v", err)
	}

	props := d2d1.RenderTargetProperties{
		DpiX: 96,
		DpiY: 96,
	}
	hwndProps := d2d1.HwndRenderTargetProperties{
		Hwnd: win.NativeHandle(),
	}
	var hr com.HRESULT
	p.render, hr = p.factory.CreateHwndRenderTarget(&props, &hwndProps)
	if hr.Failed() {
		p.Destroy()
		return nil, fmt.Errorf("create d2d render target err: %v", hr)
	}

	p.colorBrush, hr = p.render.CreateSolidColorBrush(&p.color, nil)
	if hr.Failed() {
		p.Destroy()
		return nil, fmt.Errorf("create solid color brush err: %v", hr)
	}

	return p, nil
}

func (p *Painter) Name() string {
	return "Direct2D"
}

func (p *Painter) Destroy() {
	if p.linearBrush != nil {
		p.linearBrush.Release()
		p.linearBrush = nil
	}
	if p.linearStops != nil {
		p.linearStops.Release()
		p.linearStops = nil
	}
	if p.colorBrush != nil {
		p.colorBrush.Release()
		p.colorBrush = nil
	}
	if p.render != nil {
		p.render.Release()
		p.render = nil
	}
	if p.factory != nil {
		p.factory.Release()
		p.factory = nil
	}
}

func (p *Painter) Begin(width, height, scale float32) {
	p.sizeU.Width = uint32(width)
	p.sizeU.Height = uint32(height)
	p.render.Resize(&p.sizeU)
	dpi := 96 * scale
	p.render.SetDpi(dpi, dpi)
	p.render.BeginDraw()
	p.scale = scale
	// Reset transform to identity at the start of each frame.
	p.SetTransform(geometry.Identity())
}

func (p *Painter) End() {
	// Defensive: pop any clip the GUI layer forgot to restore. D2D requires
	// the clip stack to be balanced before EndDraw; an unbalanced stack fails
	// the draw and leaves the window blank.
	p.SetClipRect(graphics.Rectangle{})
	p.render.EndDraw(nil, nil)
}

func (p *Painter) Clear(color graphics.Color) {
	p.color.R, p.color.G, p.color.B, p.color.A = color.R, color.G, color.B, color.A
	p.render.Clear(&p.color)
}

func (p *Painter) FillRect(rect graphics.Rectangle, brush graphics.Brush) {
	if d2dBrush := p.setBrush(brush); d2dBrush != nil {
		p.setRect(p.snapRect(rect))
		p.render.FillRectangle(&p.rect, d2dBrush)
	}
}

func (p *Painter) FillRoundRect(rect graphics.Rectangle, radius float32, brush graphics.Brush) {
	if d2dBrush := p.setBrush(brush); d2dBrush != nil {
		rect = p.snapRect(rect)
		p.setRoundRect(rect, radius)
		p.render.FillRoundedRectangle(&p.roundRect, d2dBrush)
	}
}

func (p *Painter) FillEllipse(center graphics.Point, xRadius, yRadius float32, brush graphics.Brush) {
	if d2dBrush := p.setBrush(brush); d2dBrush != nil {
		p.setEllipse(p.snapPoint(center), xRadius, yRadius)
		p.render.FillEllipse(&p.ellipse, d2dBrush)
	}
}

func (p *Painter) FillPath(path graphics.Path, brush graphics.Brush) {
	if d2dBrush := p.setBrush(brush); d2dBrush != nil {
		geometry, err := p.createPathGeometry(p.snapPath(path), true)
		if err == nil {
			defer geometry.Release()
			p.render.FillGeometry(geometry, d2dBrush, nil)
		}
	}
}

func (p *Painter) DrawLine(p0, p1 graphics.Point, strokeWidth float32, brush graphics.Brush) {
	if d2dBrush := p.setBrush(brush); d2dBrush != nil {
		point0 := d2d1.Point2F{X: p.snap(p0.X), Y: p.snap(p0.Y)}
		point1 := d2d1.Point2F{X: p.snap(p1.X), Y: p.snap(p1.Y)}
		p.render.DrawLine(point0, point1, d2dBrush, strokeWidth, nil) // TODO: strokeStyle
	}
}

func (p *Painter) DrawRect(rect graphics.Rectangle, strokeWidth float32, brush graphics.Brush) {
	if d2dBrush := p.setBrush(brush); d2dBrush != nil {
		p.setRect(p.snapRect(rect))
		p.render.DrawRectangle(&p.rect, d2dBrush, strokeWidth, nil)
	}
}

func (p *Painter) DrawRoundRect(rect graphics.Rectangle, radius, strokeWidth float32, brush graphics.Brush) {
	if d2dBrush := p.setBrush(brush); d2dBrush != nil {
		rect = p.snapRect(rect)
		p.setRoundRect(rect, radius)
		p.render.DrawRoundedRectangle(&p.roundRect, d2dBrush, strokeWidth, nil)
	}
}

func (p *Painter) DrawEllipse(center graphics.Point, xRadius, yRadius, strokeWidth float32, brush graphics.Brush) {
	if d2dBrush := p.setBrush(brush); d2dBrush != nil {
		p.setEllipse(p.snapPoint(center), xRadius, yRadius)
		p.render.DrawEllipse(&p.ellipse, d2dBrush, strokeWidth, nil)
	}
}

func (p *Painter) DrawPath(path graphics.Path, strokeWidth float32, brush graphics.Brush) {
	if d2dBrush := p.setBrush(brush); d2dBrush != nil {
		geometry, err := p.createPathGeometry(p.snapPath(path), false)
		if err == nil {
			defer geometry.Release()
			p.render.DrawGeometry(geometry, d2dBrush, strokeWidth, nil)
		}
	}
}

func (p *Painter) DrawTextLayout(origin graphics.Point, layout typography.TextLayout) {
	if p.typoCtx != nil {
		if textLayout, ok := layout.(*directwrite.TextLayout); ok {
			point := d2d1.Point2F{X: p.snap(origin.X), Y: p.snap(origin.Y)}
			textLayout.Draw(&p.render.RenderTarget, point, d2d1.D2D1_DRAW_TEXT_OPTIONS_ENABLE_COLOR_FONT|d2d1.D2D1_DRAW_TEXT_OPTIONS_CLIP)
		}
		// TODO: draw text layout rendered bitmap
	}
}

func (p *Painter) SetTransform(t geometry.Transform) {
	p.transform = t
	p.matrix = d2dMatrix(t)
	p.render.SetTransform(&p.matrix)
}

// d2dMatrix converts GOUI's column-vector transform into Direct2D's
// row-vector matrix layout. The linear off-diagonal terms must be
// transposed: Direct2D maps x' = x*M11 + y*M21 + M31, while
// geometry.Transform maps x' = A11*x + A12*y + TX.
func d2dMatrix(t geometry.Transform) d2d1.Matrix3x2F {
	return d2d1.Matrix3x2F{
		M11: t.A11, M12: t.A21,
		M21: t.A12, M22: t.A22,
		M31: t.TX, M32: t.TY,
	}
}

func (p *Painter) DrawImage(rect graphics.Rectangle, img image.Image) {
	bitmap, ok := graphics.ToBitmap(img, graphics.PixelFormatBGRA)
	if !ok {
		bitmap = graphics.CopyToBitmap(img, graphics.PixelFormatBGRA, p.imageBuf)
		p.imageBuf = bitmap.Pixels
	}
	p.drawBitmap(p.snapRect(rect), bitmap)
}

func (p *Painter) SetClipRect(rect graphics.Rectangle) {
	// D2D's PushAxisAlignedClip applies the current render target transform
	// to the clip rect. Since the clip rect is already in window-local
	// coordinates (the transform's offset has been applied by the GUI layer),
	// we must set the clip in identity transform space. Save and restore the
	// transform manually.
	prev := p.render.GetTransform()
	identity := d2d1.Matrix3x2F{M11: 1, M22: 1}
	p.render.SetTransform(&identity)

	var zero d2d1.RectF
	if p.clip != zero {
		p.render.PopAxisAlignedClip()
		p.clip = zero
	}
	if rect.X != 0 || rect.Y != 0 || rect.Width != 0 || rect.Height != 0 {
		p.clip.Left = rect.X
		p.clip.Top = rect.Y
		p.clip.Right = rect.X + rect.Width
		p.clip.Bottom = rect.Y + rect.Height
		p.render.PushAxisAlignedClip(&p.clip, d2d1.D2D1_ANTIALIAS_MODE_ALIASED)
	}

	p.render.SetTransform(&prev)
}

func (p *Painter) createPathGeometry(path graphics.Path, fill bool) (geometry *d2d1.Geometry, err error) {
	pathGeometry, hr := p.factory.CreatePathGeometry()
	if hr.Failed() {
		return nil, fmt.Errorf("create d2d path geometry err: %v", err)
	}

	sink, hr := pathGeometry.Open()
	if hr.Failed() {
		return nil, fmt.Errorf("open d2d path geometry sink err: %v", err)
	}
	defer sink.Release()

	begFigure := d2d1.D2D1_FIGURE_BEGIN_HOLLOW
	if fill {
		begFigure = d2d1.D2D1_FIGURE_BEGIN_FILLED
	}
	closed := false

	var (
		arc    d2d1.ArcSegment
		bezier d2d1.BezierSegment
	)

	path.Range(func(op graphics.PathOperation, args []float32) (stop bool) {
		switch op {
		case graphics.PathMoveTo:
			sink.BeginFigure(d2d1.Point2F{X: args[0], Y: args[1]}, begFigure)

		case graphics.PathLineTo:
			sink.AddLine(d2d1.Point2F{X: args[0], Y: args[1]})

		case graphics.PathArcTo:
			arc = makeArcSegment(args[0], args[1], args[2], args[3], args[4], args[5], args[6])
			sink.AddArc(&arc)

		case graphics.PathBezierTo:
			bezier = makeBezierSegment(args[0], args[1], args[2], args[3], args[4], args[5])
			sink.AddBezier(&bezier)

		case graphics.PathClose:
			closed = true
			sink.EndFigure(d2d1.D2D1_FIGURE_END_CLOSED)
		}
		return closed
	})
	if !closed {
		sink.EndFigure(d2d1.D2D1_FIGURE_END_OPEN)
	}
	sink.Close()

	return &pathGeometry.Geometry, nil
}

func (p *Painter) setBrush(brush graphics.Brush) *d2d1.Brush {
	switch brush := brush.(type) {
	case graphics.Color:
		return p.setColorBrush(brush)
	case graphics.LinearGradient:
		if brush.Start == brush.End {
			return p.setColorBrush(brush.StartColor)
		}
		if !p.setLinearGradient(brush) {
			return p.setColorBrush(brush.StartColor)
		}
		return &p.linearBrush.Brush
	default:
		return nil
	}
}

func (p *Painter) setColorBrush(color graphics.Color) *d2d1.Brush {
	p.color.R, p.color.G, p.color.B, p.color.A = color.R, color.G, color.B, color.A
	p.colorBrush.SetColor(&p.color)
	return &p.colorBrush.Brush
}

func (p *Painter) setLinearGradient(gradient graphics.LinearGradient) bool {
	if !p.hasLinear || p.linearStart != gradient.StartColor || p.linearEnd != gradient.EndColor {
		if p.linearBrush != nil {
			p.linearBrush.Release()
			p.linearBrush = nil
		}
		if p.linearStops != nil {
			p.linearStops.Release()
			p.linearStops = nil
		}

		stops := [2]d2d1.GradientStop{
			{Position: 0, Color: d2d1.ColorF{R: gradient.StartColor.R, G: gradient.StartColor.G, B: gradient.StartColor.B, A: gradient.StartColor.A}},
			{Position: 1, Color: d2d1.ColorF{R: gradient.EndColor.R, G: gradient.EndColor.G, B: gradient.EndColor.B, A: gradient.EndColor.A}},
		}
		var hr com.HRESULT
		p.linearStops, hr = p.render.CreateGradientStopCollection(stops[:], d2d1.D2D1_GAMMA_2_2, d2d1.D2D1_EXTEND_MODE_CLAMP)
		if hr.Failed() {
			return false
		}
		props := d2d1.LinearGradientBrushProperties{
			StartPoint: d2d1.Point2F{X: gradient.Start.X, Y: gradient.Start.Y},
			EndPoint:   d2d1.Point2F{X: gradient.End.X, Y: gradient.End.Y},
		}
		p.linearBrush, hr = p.render.CreateLinearGradientBrush(&props, nil, p.linearStops)
		if hr.Failed() {
			p.linearStops.Release()
			p.linearStops = nil
			return false
		}
		p.linearStart, p.linearEnd = gradient.StartColor, gradient.EndColor
		p.hasLinear = true
	} else {
		p.linearBrush.SetStartPoint(d2d1.Point2F{X: gradient.Start.X, Y: gradient.Start.Y})
		p.linearBrush.SetEndPoint(d2d1.Point2F{X: gradient.End.X, Y: gradient.End.Y})
	}
	return true
}

func (p *Painter) setRect(rect graphics.Rectangle) {
	leftTop := rect.LeftTop()
	rightBottom := rect.RightBottom()
	p.rect.Left, p.rect.Top = leftTop.X, leftTop.Y
	p.rect.Right, p.rect.Bottom = rightBottom.X, rightBottom.Y
}

func (p *Painter) setRoundRect(rect graphics.Rectangle, radius float32) {
	leftTop := rect.LeftTop()
	rightBottom := rect.RightBottom()
	p.roundRect.Rect.Left, p.roundRect.Rect.Top = leftTop.X, leftTop.Y
	p.roundRect.Rect.Right, p.roundRect.Rect.Bottom = rightBottom.X, rightBottom.Y
	p.roundRect.RadiusX, p.roundRect.RadiusY = radius, radius
}

func (p *Painter) setEllipse(center graphics.Point, radiusX, radiusY float32) {
	p.ellipse.Point.X = center.X
	p.ellipse.Point.Y = center.Y
	p.ellipse.RadiusX = radiusX
	p.ellipse.RadiusY = radiusY
}

func makeArcSegment(rx, ry, xRotation, large, sweep, x, y float32) (arc d2d1.ArcSegment) {
	return d2d1.ArcSegment{
		Point:          d2d1.Point2F{X: x, Y: y},
		Size:           d2d1.SizeF{Width: rx, Height: ry},
		RotationAngle:  xRotation,
		SweepDirection: d2d1.SweepDirection(sweep),
		ArcSize:        d2d1.ArcSize(large),
	}
}

func makeBezierSegment(c1x, c1y, c2x, c2y, x, y float32) d2d1.BezierSegment {
	return d2d1.BezierSegment{
		Point1: d2d1.Point2F{X: c1x, Y: c1y},
		Point2: d2d1.Point2F{X: c2x, Y: c2y},
		Point3: d2d1.Point2F{X: x, Y: y},
	}
}

func (p *Painter) drawBitmap(rect graphics.Rectangle, bitmap graphics.Bitmap) {
	if bitmap.Width <= 0 || bitmap.Height <= 0 {
		return
	}

	size := d2d1.SizeU{Width: uint32(bitmap.Width), Height: uint32(bitmap.Height)}
	props := d2d1.BitmapProperties{
		PixelFormat: d2d1.PixelFormat{
			Format:    dxgi.DXGI_FORMAT_B8G8R8A8_UNORM,
			AlphaMode: d2d1.D2D1_ALPHA_MODE_PREMULTIPLIED,
		},
		DpiX: 96,
		DpiY: 96,
	}

	d2dBitmap, hr := p.render.CreateBitmap(size, bitmap.Pixels, bitmap.Stride, &props)
	if hr.Failed() {
		return
	}
	defer d2dBitmap.Release()

	dstRect := d2d1.RectF{
		Left:   rect.X,
		Top:    rect.Y,
		Right:  rect.X + rect.Width,
		Bottom: rect.Y + rect.Height,
	}
	p.render.DrawBitmap(d2dBitmap, &dstRect, 1, d2d1.D2D1_BITMAP_INTERPOLATION_MODE_LINEAR, nil)
}

func (p *Painter) snap(x float32) float32 {
	if p.scale <= 0 || p.transform != geometry.Identity() {
		// Coordinates are local to the active transform. Snapping them before
		// Direct2D applies that transform shifts translated widgets and distorts
		// rotated or scaled geometry. Pixel snapping is only valid when local
		// and target coordinates are the same.
		return x
	}
	// D2D strokes the center of a path. For a stroke to fall entirely within
	// a single physical pixel, its center must align to a pixel center, which
	// in D2D is at half-integer coordinates (0.5, 1.5, 2.5...). Snap to pixel
	// center rather than pixel edge; otherwise a 1px border at an integer
	// coordinate straddles two pixels and appears blurred.
	return (mathx.Floor(x*p.scale) + 0.5) / p.scale
}

func (p *Painter) snapRect(rect graphics.Rectangle) geometry.Rectangle {
	// Snap the top-left corner to pixel center, then compute width/height so
	// the bottom-right corner also lands on a pixel center. Snapping width and
	// height independently would misalign the far edge (snap(X)+snap(W) !=
	// snap(X+W)).
	right := rect.X + rect.Width
	bottom := rect.Y + rect.Height
	rect.X = p.snap(rect.X)
	rect.Y = p.snap(rect.Y)
	rect.Width = p.snap(right) - rect.X
	rect.Height = p.snap(bottom) - rect.Y
	return rect
}

func (p *Painter) snapPoint(pt graphics.Point) graphics.Point {
	pt.X = p.snap(pt.X)
	pt.Y = p.snap(pt.Y)
	return pt
}

func (p *Painter) snapPath(path graphics.Path) graphics.Path {
	var snapped graphics.Path
	empty := true
	path.Range(func(op graphics.PathOperation, args []float32) (stop bool) {
		switch op {
		case graphics.PathMoveTo:
			snapped = graphics.MoveTo(p.snap(args[0]), p.snap(args[1]))
			empty = false
		case graphics.PathLineTo:
			if empty {
				snapped = graphics.MoveTo(p.snap(args[0]), p.snap(args[1]))
				empty = false
			}
			snapped = snapped.LineTo(p.snap(args[0]), p.snap(args[1]))
		case graphics.PathArcTo:
			if empty {
				snapped = graphics.MoveTo(p.snap(args[5]), p.snap(args[6]))
				empty = false
			}
			snapped = snapped.ArcTo(p.snap(args[0]), p.snap(args[1]), p.snap(args[2]), p.snap(args[3]), p.snap(args[4]), p.snap(args[5]), p.snap(args[6]))
		case graphics.PathBezierTo:
			if empty {
				snapped = graphics.MoveTo(p.snap(args[4]), p.snap(args[5]))
				empty = false
			}
			snapped = snapped.BezierTo(
				p.snap(args[0]), p.snap(args[1]),
				p.snap(args[2]), p.snap(args[3]),
				p.snap(args[4]), p.snap(args[5]),
			)
		case graphics.PathClose:
			snapped = snapped.Close()
		}
		return false
	})
	return snapped
}
