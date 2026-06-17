package direct2d

import (
	"fmt"
	"image"

	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/platform/typography"
	"github.com/golang-gui/goui/platform/typography/directwrite"

	"github.com/golang-gui/goui/platform/windows/sdk/com"
	"github.com/golang-gui/goui/platform/windows/sdk/d2d1"
	"github.com/golang-gui/goui/platform/windows/sdk/dxgi"
)

type Painter struct {
	typoCtx    typography.Context
	dwTypo     *directwrite.Context
	factory    *d2d1.Factory
	render     *d2d1.HwndRenderTarget
	colorBrush *d2d1.SolidColorBrush
	color      d2d1.ColorF
	sizeU      d2d1.SizeU
	rect       d2d1.RectF
	roundRect  d2d1.RoundRect
	ellipse    d2d1.Ellipse
	clip       d2d1.RectF
	imageBuf   []byte
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
}

func (p *Painter) End() {
	p.render.EndDraw(nil, nil)
}

func (p *Painter) Clear(color graphics.Color) {
	p.color.R, p.color.G, p.color.B, p.color.A = color.R, color.G, color.B, color.A
	p.render.Clear(&p.color)
}

func (p *Painter) FillRect(rect graphics.Rectangle, brush graphics.Brush) {
	if d2dBrush := p.setBrush(brush); d2dBrush != nil {
		p.setRect(rect)
		p.render.FillRectangle(&p.rect, d2dBrush)
	}
	// TODO: error?
}

func (p *Painter) FillRoundRect(rect graphics.Rectangle, radius float32, brush graphics.Brush) {
	if d2dBrush := p.setBrush(brush); d2dBrush != nil {
		p.setRoundRect(rect, radius)
		p.render.FillRoundedRectangle(&p.roundRect, d2dBrush)
	}
}

func (p *Painter) FillEllipse(center graphics.Point, xRadius, yRadius float32, brush graphics.Brush) {
	if d2dBrush := p.setBrush(brush); d2dBrush != nil {
		p.setEllipse(center, xRadius, yRadius)
		p.render.FillEllipse(&p.ellipse, d2dBrush)
	}
}

func (p *Painter) FillPath(path graphics.Path, brush graphics.Brush) {
	if d2dBrush := p.setBrush(brush); d2dBrush != nil {
		geometry, err := p.createPathGeometry(path, true)
		if err == nil {
			defer geometry.Release()
			p.render.FillGeometry(geometry, d2dBrush, nil)
		}
	}
}

func (p *Painter) DrawLine(p0, p1 graphics.Point, strokeWidth float32, brush graphics.Brush) {
	if d2dBrush := p.setBrush(brush); d2dBrush != nil {
		point0 := d2d1.Point2F{X: p0.X, Y: p0.Y}
		point1 := d2d1.Point2F{X: p1.X, Y: p1.Y}
		p.render.DrawLine(point0, point1, d2dBrush, strokeWidth, nil) // TODO: strokeStyle
	}
}

func (p *Painter) DrawRect(rect graphics.Rectangle, strokeWidth float32, brush graphics.Brush) {
	if d2dBrush := p.setBrush(brush); d2dBrush != nil {
		p.setRect(rect)
		p.render.DrawRectangle(&p.rect, d2dBrush, strokeWidth, nil)
	}
}

func (p *Painter) DrawRoundRect(rect graphics.Rectangle, radius, strokeWidth float32, brush graphics.Brush) {
	if d2dBrush := p.setBrush(brush); d2dBrush != nil {
		p.setRoundRect(rect, radius)
		p.render.DrawRoundedRectangle(&p.roundRect, d2dBrush, strokeWidth, nil)
	}
}

func (p *Painter) DrawEllipse(center graphics.Point, xRadius, yRadius, strokeWidth float32, brush graphics.Brush) {
	if d2dBrush := p.setBrush(brush); d2dBrush != nil {
		p.setEllipse(center, xRadius, yRadius)
		p.render.DrawEllipse(&p.ellipse, d2dBrush, strokeWidth, nil)
	}
}

func (p *Painter) DrawPath(path graphics.Path, strokeWidth float32, brush graphics.Brush) {
	if d2dBrush := p.setBrush(brush); d2dBrush != nil {
		geometry, err := p.createPathGeometry(path, false)
		if err == nil {
			defer geometry.Release()
			p.render.DrawGeometry(geometry, d2dBrush, strokeWidth, nil)
		}
	}
}

func (p *Painter) DrawTextLayout(origin graphics.Point, layout typography.TextLayout) {
	if p.typoCtx != nil {
		if textLayout, ok := layout.(*directwrite.TextLayout); ok {
			point := d2d1.Point2F{X: origin.X, Y: origin.Y}
			textLayout.Draw(&p.render.RenderTarget, point, d2d1.D2D1_DRAW_TEXT_OPTIONS_ENABLE_COLOR_FONT|d2d1.D2D1_DRAW_TEXT_OPTIONS_CLIP)
		}
		// TODO: draw text layout rendered bitmap
	}
}

func (p *Painter) DrawImage(rect graphics.Rectangle, img image.Image) {
	bitmap, ok := graphics.ToBitmap(img, graphics.PixelFormatBGRA)
	if !ok {
		bitmap = graphics.CopyToBitmap(img, graphics.PixelFormatBGRA, p.imageBuf)
		p.imageBuf = bitmap.Pixels
	}
	p.drawBitmap(rect, bitmap)
}

func (p *Painter) SetClipRect(rect graphics.Rectangle) {
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
	if color, ok := brush.(graphics.Color); ok {
		p.color.R, p.color.G, p.color.B, p.color.A = color.R, color.G, color.B, color.A
		p.colorBrush.SetColor(&p.color)
		return &p.colorBrush.Brush
	}
	return nil
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
