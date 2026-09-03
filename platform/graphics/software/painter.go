package software

import (
	"fmt"
	"image"
	"image/color"
	"math"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/platform/graphics/internal/boxshadow"
	"github.com/golang-gui/goui/platform/graphics/internal/textbitmap"
	"github.com/golang-gui/goui/platform/graphics/utils"
	"github.com/golang-gui/goui/platform/typography"

	"github.com/srwiley/rasterx"
	"github.com/srwiley/scanFT"
	"golang.org/x/image/math/fixed"
)

type Painter struct {
	drawable          Drawable
	bgra              image.RGBA
	line              image.RGBA
	viewport          graphics.Rectangle
	scanner           rasterx.Scanner
	filler            *rasterx.Filler
	stroker           *rasterx.Stroker
	pixelBuf          []byte
	lineBuf           []byte
	outputBuf         []byte
	images            map[*imageResource]struct{}
	textImages        *textbitmap.ImageCache[graphics.Image]
	textPixels        []byte
	pendingTextImages int
	scale             float32
	transform         geometry.Transform
	clip              image.Rectangle
	activeFrame       bool
}

type imageResource struct {
	owner          *Painter
	width          int
	height         int
	bitmap         graphics.Bitmap
	destroyed      bool
	pendingDestroy bool
}

func (i *imageResource) Size() (width, height int) {
	if i == nil {
		return 0, 0
	}
	return i.width, i.height
}

func (i *imageResource) Update(src image.Image) error {
	if i == nil || i.destroyed || i.owner == nil {
		return fmt.Errorf("software: update destroyed image")
	}
	return i.owner.updateImage(i, src)
}

func (i *imageResource) Destroy() {
	if i == nil || i.destroyed || i.owner == nil {
		return
	}
	i.owner.destroyImage(i)
}

type Drawable interface {
	Draw(img image.Image) error
}

func NewPainter(drawable Drawable) (graphics.Painter, error) {
	p := new(Painter)
	p.drawable = drawable
	p.images = make(map[*imageResource]struct{})
	p.textImages = textbitmap.NewImageCache(4, p.releaseTextImage)
	return p, nil
}

func (p *Painter) Name() string {
	return "Software"
}

func (p *Painter) Destroy() {
	if p.activeFrame {
		panic("software: destroy painter during active frame")
	}
	p.textImages.Destroy()
	for img := range p.images {
		img.owner = nil
		img.bitmap.Pixels = nil
		img.destroyed = true
	}
	clear(p.images)
	p.pendingTextImages = 0
	p.textPixels = nil
}

func (p *Painter) NewImage(src image.Image) (graphics.Image, error) {
	if src == nil {
		return nil, fmt.Errorf("software: create image from nil source")
	}
	bounds := src.Bounds()
	if bounds.Empty() {
		return nil, fmt.Errorf("software: create empty image")
	}
	img := &imageResource{
		owner:  p,
		width:  bounds.Dx(),
		height: bounds.Dy(),
		bitmap: graphics.CopyToBitmap(src, graphics.PixelFormatBGRA, nil),
	}
	p.images[img] = struct{}{}
	return img, nil
}

func (p *Painter) updateImage(img *imageResource, src image.Image) error {
	if p.activeFrame {
		panic("software: update image during active frame")
	}
	if img == nil || img.destroyed || img.owner != p {
		return fmt.Errorf("software: update invalid image")
	}
	if src == nil {
		return fmt.Errorf("software: update image from nil source")
	}
	bounds := src.Bounds()
	if bounds.Dx() != img.width || bounds.Dy() != img.height {
		return fmt.Errorf("software: update image size %dx%d, want %dx%d", bounds.Dx(), bounds.Dy(), img.width, img.height)
	}
	img.bitmap = graphics.CopyToBitmap(src, graphics.PixelFormatBGRA, img.bitmap.Pixels)
	return nil
}

func (p *Painter) destroyImage(img *imageResource) {
	if img == nil || img.destroyed || img.owner != p {
		return
	}
	if p.activeFrame {
		panic("software: destroy image during active frame")
	}
	p.finishImageDestroy(img)
}

func (p *Painter) finishImageDestroy(img *imageResource) {
	if img.pendingDestroy {
		img.pendingDestroy = false
		p.pendingTextImages--
	}
	delete(p.images, img)
	img.owner = nil
	img.bitmap.Pixels = nil
	img.destroyed = true
}

func (p *Painter) releaseTextImage(img graphics.Image) {
	native, ok := img.(*imageResource)
	if !ok || native == nil || native.owner != p || native.destroyed {
		return
	}
	if p.activeFrame {
		if !native.pendingDestroy {
			native.pendingDestroy = true
			p.pendingTextImages++
		}
		return
	}
	p.destroyImage(native)
}

func (p *Painter) flushPendingTextImages() {
	if p.pendingTextImages == 0 {
		return
	}
	for img := range p.images {
		if img.pendingDestroy {
			p.finishImageDestroy(img)
		}
	}
}

func (p *Painter) Begin(width, height, scale float32) {
	// TODO: scale
	viewport := graphics.Rect(0, 0, width, height)
	if p.viewport.Width != viewport.Width || p.viewport.Height != viewport.Height || p.scale != scale {
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
	p.clip = image.Rectangle{}
	p.activeFrame = true
}

func (p *Painter) End() {
	if cap(p.outputBuf) < len(p.bgra.Pix) {
		p.outputBuf = make([]byte, len(p.bgra.Pix))
	} else {
		p.outputBuf = p.outputBuf[:len(p.bgra.Pix)]
	}
	for i := 0; i < len(p.bgra.Pix); i += 4 {
		b, g, r, a := p.bgra.Pix[i], p.bgra.Pix[i+1], p.bgra.Pix[i+2], p.bgra.Pix[i+3]
		if a != 0 {
			b = uint8(min(255, int(b)*255/int(a)))
			g = uint8(min(255, int(g)*255/int(a)))
			r = uint8(min(255, int(r)*255/int(a)))
		}
		p.outputBuf[i], p.outputBuf[i+1], p.outputBuf[i+2], p.outputBuf[i+3] = b, g, r, a
	}
	p.drawable.Draw(graphics.Bitmap{
		Width:  p.bgra.Rect.Max.X,
		Height: p.bgra.Rect.Max.Y,
		Stride: p.bgra.Stride,
		Format: graphics.PixelFormatBGRA, // reversed
		Pixels: p.outputBuf,
	})
	p.activeFrame = false
	p.flushPendingTextImages()
}

func (p *Painter) Clear(color graphics.Color) {
	p.fillLine(color)
	for y := 0; y < p.bgra.Rect.Max.Y; y++ {
		offset := p.bgra.PixOffset(0, y)
		end := offset + p.bgra.Stride
		copy(p.bgra.Pix[offset:end], p.line.Pix)
	}
}

func (p *Painter) DrawBoxShadow(rect graphics.Rectangle, radius float32, shadow graphics.BoxShadow) {
	if shadow.Color.A <= 0 {
		return
	}
	shape, ok := boxshadow.Normalize(rect, radius, shadow.Offset, shadow.BlurRadius, shadow.SpreadRadius)
	if !ok {
		return
	}

	defer p.filler.Clear()
	bounds := shape.Bounds()
	clip := p.addRect(p.filler, bounds)
	p.setShapeClip(clip)
	defer p.restoreClip()
	inverse := p.deviceTransform().Inverse()
	p.filler.SetColor(rasterx.ColorFunc(func(x, y int) color.Color {
		point := inverse.TransformPoint(geometry.Point{X: float32(x) + 0.5, Y: float32(y) + 0.5})
		coverage := shape.Alpha(point)
		if coverage <= 0 {
			return color.RGBA{}
		}
		c := shadow.Color
		c.A *= coverage
		return reverseColor(c)
	}))
	p.filler.Draw()
}

func (p *Painter) FillRect(rect graphics.Rectangle, brush graphics.Brush) {
	if p.setBrush(brush) {
		defer p.filler.Clear()
		clip := p.addRect(p.filler, rect)
		p.setShapeClip(clip)
		defer p.restoreClip()
		p.filler.Draw()
	}
}

func (p *Painter) FillRoundRect(rect graphics.Rectangle, radius float32, brush graphics.Brush) {
	if p.setBrush(brush) {
		defer p.filler.Clear()
		clip := p.addRoundRect(p.filler, rect, radius)
		p.setShapeClip(clip)
		defer p.restoreClip()
		p.filler.Draw()
	}
}

func (p *Painter) FillEllipse(center graphics.Point, xRadius, yRadius float32, brush graphics.Brush) {
	if p.setBrush(brush) {
		defer p.filler.Clear()
		clip := p.addEllipse(p.filler, center, xRadius, yRadius)
		p.setShapeClip(clip)
		defer p.restoreClip()
		p.filler.Draw()
	}
}

func (p *Painter) FillPath(path graphics.Path, brush graphics.Brush) {
	if p.setBrush(brush) {
		defer p.filler.Clear()
		closed, clip := p.doPath(p.filler, path)
		if !closed {
			p.filler.Stop(true)
		}
		p.setShapeClip(clip)
		defer p.restoreClip()
		p.filler.Draw()
	}
}

func (p *Painter) DrawLine(p0, p1 graphics.Point, strokeWidth float32, brush graphics.Brush) {
	if p.setBrush(brush) {
		defer p.stroker.Clear()
		p0, p1 = p.devicePoint(p0), p.devicePoint(p1)
		strokeWidth = p.deviceStrokeWidth(strokeWidth)
		p.setShapeClip(p.clipForPoints([]graphics.Point{p0, p1}, strokeWidth/2))
		defer p.restoreClip()
		p.stroker.SetStroke(toFixedI(strokeWidth), toFixedI(4), rasterx.ButtCap, nil, rasterx.FlatGap, rasterx.MiterClip)

		p.stroker.Start(toFixedP(p0.X, p0.Y))
		p.stroker.Line(toFixedP(p1.X, p1.Y))
		p.stroker.Stop(false)

		p.stroker.Draw()
	}
}

func (p *Painter) DrawRect(rect graphics.Rectangle, strokeWidth float32, brush graphics.Brush) {
	if p.setBrush(brush) {
		defer p.stroker.Clear()
		strokeWidth = p.deviceStrokeWidth(strokeWidth)
		// rasterx expands the stroke while path segments are added, so the
		// requested width must be configured before building the path.
		p.stroker.SetStroke(toFixedI(strokeWidth), toFixedI(4), rasterx.ButtCap, nil, rasterx.FlatGap, rasterx.MiterClip)
		clip := p.addRect(p.stroker, rect)
		p.setShapeClip(clip.Inset(-uptoPixel(strokeWidth)))
		defer p.restoreClip()
		p.stroker.Draw()
	}
}

func (p *Painter) DrawRoundRect(rect graphics.Rectangle, radius, strokeWidth float32, brush graphics.Brush) {
	if p.setBrush(brush) {
		defer p.stroker.Clear()
		strokeWidth = p.deviceStrokeWidth(strokeWidth)
		p.stroker.SetStroke(toFixedI(strokeWidth), toFixedI(4), rasterx.ButtCap, nil, rasterx.FlatGap, rasterx.MiterClip)
		clip := p.addRoundRect(p.stroker, rect, radius)
		p.setShapeClip(clip.Inset(-uptoPixel(strokeWidth)))
		defer p.restoreClip()
		p.stroker.Draw()
	}
}

func (p *Painter) DrawEllipse(center graphics.Point, xRadius, yRadius, strokeWidth float32, brush graphics.Brush) {
	if p.setBrush(brush) {
		defer p.stroker.Clear()
		strokeWidth = p.deviceStrokeWidth(strokeWidth)
		p.stroker.SetStroke(toFixedI(strokeWidth), toFixedI(4), rasterx.ButtCap, nil, rasterx.FlatGap, rasterx.MiterClip)
		clip := p.addEllipse(p.stroker, center, xRadius, yRadius)
		p.setShapeClip(clip.Inset(-uptoPixel(strokeWidth)))
		defer p.restoreClip()
		p.stroker.Draw()
	}
}

func (p *Painter) DrawPath(path graphics.Path, strokeWidth float32, brush graphics.Brush) {
	if p.setBrush(brush) {
		defer p.stroker.Clear()
		strokeWidth = p.deviceStrokeWidth(strokeWidth)
		p.stroker.SetStroke(toFixedI(strokeWidth), toFixedI(4), rasterx.ButtCap, nil, rasterx.FlatGap, rasterx.MiterClip)

		closed, clip := p.doPath(p.stroker, path)
		if !closed {
			p.stroker.Stop(false)
		}

		p.setShapeClip(clip.Inset(-uptoPixel(strokeWidth)))
		defer p.restoreClip()

		p.stroker.Draw()
	}
}

func (p *Painter) DrawTextLayout(origin graphics.Point, layout typography.TextLayout) {
	if layout == nil {
		return
	}

	rasterScale := textbitmap.RasterScale(p.scale, p.transform)
	if rasterScale <= 0 {
		return
	}
	if img, ok := p.textImages.Lookup(layout, rasterScale); ok {
		p.drawTextImage(origin, rasterScale, img)
		return
	}

	textBitmap, err := layout.Rasterize(rasterScale, p.textPixels)
	if err != nil || textBitmap.Width <= 0 || textBitmap.Height <= 0 {
		return
	}
	p.textPixels = textBitmap.Pixels

	bitmap := graphics.Bitmap{
		Width:  textBitmap.Width,
		Height: textBitmap.Height,
		Stride: textBitmap.Stride,
		Format: graphics.PixelFormatRGBA,
		Pixels: textBitmap.Pixels,
	}
	img, err := p.NewImage(bitmap)
	if err != nil {
		return
	}
	p.textImages.Store(layout, rasterScale, img)
	p.drawTextImage(origin, rasterScale, img)
}

func (p *Painter) drawTextImage(origin graphics.Point, rasterScale float32, img graphics.Image) {
	width, height := img.Size()
	if width <= 0 || height <= 0 {
		return
	}
	origin = textbitmap.SnapOrigin(origin, p.transform, p.scale)
	drawRect := graphics.Rect(
		origin.X,
		origin.Y,
		float32(width)/rasterScale,
		float32(height)/rasterScale,
	)
	p.DrawImage(drawRect, img)
}

func (p *Painter) SetTransform(t geometry.Transform) {
	p.transform = t
}

func (p *Painter) setBrush(brush graphics.Brush) bool {
	switch brush := brush.(type) {
	case graphics.Color:
		p.scanner.SetColor(reverseColor(brush))
		return true
	case graphics.LinearGradient:
		start := brush.Start
		end := brush.End
		dx := end.X - start.X
		dy := end.Y - start.Y
		lengthSquared := dx*dx + dy*dy
		if lengthSquared <= 1e-12 || p.scale <= 0 {
			p.scanner.SetColor(reverseColor(brush.StartColor))
			return true
		}

		inverse := p.deviceTransform().Inverse()
		p.scanner.SetColor(rasterx.ColorFunc(func(x, y int) color.Color {
			point := inverse.TransformPoint(geometry.Point{X: float32(x) + 0.5, Y: float32(y) + 0.5})
			t := ((point.X-start.X)*dx + (point.Y-start.Y)*dy) / lengthSquared
			if t < 0 {
				t = 0
			} else if t > 1 {
				t = 1
			}
			return reverseColor(interpolateGradientColor(brush.StartColor, brush.EndColor, t))
		}))
		return true
	default:
		return false
	}
}

func interpolateGradientColor(start, end graphics.Color, t float32) graphics.Color {
	lerp := func(a, b float32) float32 { return a + (b-a)*t }
	a := lerp(start.A, end.A)
	r := lerp(start.R*start.A, end.R*end.A)
	g := lerp(start.G*start.A, end.G*end.A)
	b := lerp(start.B*start.A, end.B*end.A)
	if a > 0 {
		r /= a
		g /= a
		b /= a
	}
	return graphics.Color{R: r, G: g, B: b, A: a}
}

func (p *Painter) deviceTransform() geometry.Transform {
	return geometry.Scale(p.scale, p.scale).Multiply(p.transform)
}

func (p *Painter) devicePoint(point graphics.Point) graphics.Point {
	return p.deviceTransform().TransformPoint(point)
}

func (p *Painter) deviceStrokeWidth(width float32) float32 {
	t := p.deviceTransform()
	sx := float32(math.Hypot(float64(t.A11), float64(t.A21)))
	sy := float32(math.Hypot(float64(t.A12), float64(t.A22)))
	return width * max(sx, sy)
}

func (p *Painter) clipForPoints(points []graphics.Point, outset float32) image.Rectangle {
	if len(points) == 0 {
		return image.Rectangle{}
	}
	minX, minY := points[0].X, points[0].Y
	maxX, maxY := minX, minY
	for _, point := range points[1:] {
		minX, minY = min(minX, point.X), min(minY, point.Y)
		maxX, maxY = max(maxX, point.X), max(maxY, point.Y)
	}
	return toClipRect(minX-outset, minY-outset, maxX-minX+2*outset, maxY-minY+2*outset)
}

func (p *Painter) addRect(add rasterx.Adder, rect graphics.Rectangle) image.Rectangle {
	points := []graphics.Point{
		p.devicePoint(rect.LeftTop()),
		p.devicePoint(graphics.Point{X: rect.X + rect.Width, Y: rect.Y}),
		p.devicePoint(rect.RightBottom()),
		p.devicePoint(graphics.Point{X: rect.X, Y: rect.Y + rect.Height}),
	}
	add.Start(toFixedP(points[0].X, points[0].Y))
	for _, point := range points[1:] {
		add.Line(toFixedP(point.X, point.Y))
	}
	add.Stop(true)
	return p.clipForPoints(points, 0)
}

func (p *Painter) addRoundRect(add rasterx.Adder, rect graphics.Rectangle, radius float32) image.Rectangle {
	radius = min(radius, min(rect.Width, rect.Height)/2)
	if radius <= 0 {
		return p.addRect(add, rect)
	}
	const kappa = 0.55228475
	points := make([]graphics.Point, 0, 16)
	point := func(x, y float32) fixed.Point26_6 {
		device := p.devicePoint(graphics.Point{X: x, Y: y})
		points = append(points, device)
		return toFixedP(device.X, device.Y)
	}
	x0, y0 := rect.X, rect.Y
	x1, y1 := rect.X+rect.Width, rect.Y+rect.Height
	r := radius
	c := r * kappa
	add.Start(point(x0+r, y0))
	add.Line(point(x1-r, y0))
	add.CubeBezier(point(x1-r+c, y0), point(x1, y0+r-c), point(x1, y0+r))
	add.Line(point(x1, y1-r))
	add.CubeBezier(point(x1, y1-r+c), point(x1-r+c, y1), point(x1-r, y1))
	add.Line(point(x0+r, y1))
	add.CubeBezier(point(x0+r-c, y1), point(x0, y1-r+c), point(x0, y1-r))
	add.Line(point(x0, y0+r))
	add.CubeBezier(point(x0, y0+r-c), point(x0+r-c, y0), point(x0+r, y0))
	add.Stop(true)
	return p.clipForPoints(points, 0)
}

func (p *Painter) addEllipse(add rasterx.Adder, center graphics.Point, xRadius, yRadius float32) image.Rectangle {
	const kappa = 0.55228475
	points := make([]graphics.Point, 0, 13)
	point := func(x, y float32) fixed.Point26_6 {
		device := p.devicePoint(graphics.Point{X: x, Y: y})
		points = append(points, device)
		return toFixedP(device.X, device.Y)
	}
	cx, cy := center.X, center.Y
	cxk, cyk := xRadius*kappa, yRadius*kappa
	add.Start(point(cx+xRadius, cy))
	add.CubeBezier(point(cx+xRadius, cy+cyk), point(cx+cxk, cy+yRadius), point(cx, cy+yRadius))
	add.CubeBezier(point(cx-cxk, cy+yRadius), point(cx-xRadius, cy+cyk), point(cx-xRadius, cy))
	add.CubeBezier(point(cx-xRadius, cy-cyk), point(cx-cxk, cy-yRadius), point(cx, cy-yRadius))
	add.CubeBezier(point(cx+cxk, cy-yRadius), point(cx+xRadius, cy-cyk), point(cx+xRadius, cy))
	add.Stop(true)
	return p.clipForPoints(points, 0)
}

func (p *Painter) DrawImage(rect graphics.Rectangle, img graphics.Image) {
	if rect.Width <= 0 || rect.Height <= 0 {
		return
	}
	native, ok := img.(*imageResource)
	if !ok || native == nil || native.owner != p || native.destroyed {
		panic("software: image does not belong to painter or was destroyed")
	}
	if !p.drawAxisAlignedBitmap(rect, native.bitmap) {
		p.drawBitmap(rect, native.bitmap)
	}
}

func (p *Painter) SetClipRect(rect graphics.Rectangle) {
	rect = rect.Scale(p.scale)
	p.clip = image.Rectangle{}
	if rect.X != 0 || rect.Y != 0 || rect.Width != 0 || rect.Height != 0 {
		p.clip = image.Rect(
			int(math.Floor(float64(rect.X))),
			int(math.Floor(float64(rect.Y))),
			int(math.Ceil(float64(rect.X+rect.Width))),
			int(math.Ceil(float64(rect.Y+rect.Height))),
		)
	}
	p.restoreClip()
}

func (p *Painter) setShapeClip(clip image.Rectangle) {
	if p.clip != (image.Rectangle{}) {
		clip = clip.Intersect(p.clip)
	}
	p.scanner.SetClip(clip)
}

func (p *Painter) restoreClip() {
	p.scanner.SetClip(p.clip)
}

func (p *Painter) drawBitmap(rect graphics.Rectangle, bitmap graphics.Bitmap) {
	if rect.Width <= 0 || rect.Height <= 0 || bitmap.Width <= 0 || bitmap.Height <= 0 {
		return
	}
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
		point := p.devicePoint(graphics.Point{X: c.x, Y: c.y})
		dev[i].x, dev[i].y = point.X, point.Y
	}

	// Bounding box of the transformed quad for clipping.
	minX := min(dev[0].x, min(dev[1].x, min(dev[2].x, dev[3].x)))
	minY := min(dev[0].y, min(dev[1].y, min(dev[2].y, dev[3].y)))
	maxX := max(dev[0].x, max(dev[1].x, max(dev[2].x, dev[3].x)))
	maxY := max(dev[0].y, max(dev[1].y, max(dev[2].y, dev[3].y)))
	p.setShapeClip(toClipRect(minX, minY, maxX-minX, maxY-minY))
	defer p.restoreClip()

	// Precompute the inverse transform so ColorFunc can map device pixels
	// back to source bitmap coordinates.
	srcBounds := bitmap.Bounds()
	srcW := float32(srcBounds.Dx())
	srcH := float32(srcBounds.Dy())
	inv := p.deviceTransform().Inverse()
	// scanFT consumes the returned color synchronously. Reusing it avoids one
	// interface-boxing allocation for every destination pixel.
	var sample color.RGBA

	p.filler.SetColor(rasterx.ColorFunc(func(x, y int) color.Color {
		// Device pixel → local logical coordinate → source rect space.
		src := inv.TransformPoint(geometry.Point{X: float32(x) + 0.5, Y: float32(y) + 0.5})
		// Integer source coordinates denote pixel centers. Sampling between those
		// centers preserves the glyph bitmap's grayscale antialiasing after rotation.
		bx := float32(srcBounds.Min.X) + (src.X-rect.X)/rect.Width*srcW - 0.5
		by := float32(srcBounds.Min.Y) + (src.Y-rect.Y)/rect.Height*srcH - 0.5
		sample = sampleBitmapBilinear(bitmap, bx, by)
		return &sample
	}))

	// Build a quad path through the 4 transformed corners.
	p.filler.Start(toFixedP(dev[0].x, dev[0].y))
	p.filler.Line(toFixedP(dev[1].x, dev[1].y))
	p.filler.Line(toFixedP(dev[2].x, dev[2].y))
	p.filler.Line(toFixedP(dev[3].x, dev[3].y))
	p.filler.Stop(true)
	p.filler.Draw()
}

func (p *Painter) drawAxisAlignedBitmap(rect graphics.Rectangle, bitmap graphics.Bitmap) bool {
	transform := p.deviceTransform()
	if transform.A12 != 0 || transform.A21 != 0 || transform.A11 == 0 || transform.A22 == 0 {
		return false
	}

	x0 := transform.A11*rect.X + transform.TX
	x1 := transform.A11*(rect.X+rect.Width) + transform.TX
	y0 := transform.A22*rect.Y + transform.TY
	y1 := transform.A22*(rect.Y+rect.Height) + transform.TY
	if !pixelAligned(x0) || !pixelAligned(x1) || !pixelAligned(y0) || !pixelAligned(y1) {
		return false
	}

	left, right := int(math.Round(float64(min(x0, x1)))), int(math.Round(float64(max(x0, x1))))
	top, bottom := int(math.Round(float64(min(y0, y1)))), int(math.Round(float64(max(y0, y1))))
	clip := image.Rect(left, top, right, bottom).Intersect(p.bgra.Rect)
	if p.clip != (image.Rectangle{}) {
		clip = clip.Intersect(p.clip)
	}
	if clip.Empty() {
		return true
	}

	srcBounds := bitmap.Bounds()
	srcW, srcH := float32(srcBounds.Dx()), float32(srcBounds.Dy())
	deviceW, deviceH := x1-x0, y1-y0
	for y := clip.Min.Y; y < clip.Max.Y; y++ {
		v := (float32(y) + 0.5 - y0) / deviceH * srcH
		sy := int(v)
		if sy < 0 || sy >= srcBounds.Dy() {
			continue
		}
		for x := clip.Min.X; x < clip.Max.X; x++ {
			u := (float32(x) + 0.5 - x0) / deviceW * srcW
			sx := int(u)
			if sx < 0 || sx >= srcBounds.Dx() {
				continue
			}
			si := bitmap.PixOffset(srcBounds.Min.X+sx, srcBounds.Min.Y+sy)
			di := p.bgra.PixOffset(x, y)
			compositePremultipliedBGRA(p.bgra.Pix[di:di+4], bitmap.Pixels[si:si+4])
		}
	}
	return true
}

func pixelAligned(v float32) bool {
	return math.Abs(float64(v)-math.Round(float64(v))) < 1e-5
}

func compositePremultipliedBGRA(dst, src []byte) {
	sa := src[3]
	if sa == 0 {
		return
	}
	if sa == 255 {
		copy(dst, src)
		return
	}
	inv := uint16(255 - sa)
	dst[0] = byte(uint16(src[0]) + (uint16(dst[0])*inv+127)/255)
	dst[1] = byte(uint16(src[1]) + (uint16(dst[1])*inv+127)/255)
	dst[2] = byte(uint16(src[2]) + (uint16(dst[2])*inv+127)/255)
	dst[3] = byte(uint16(sa) + (uint16(dst[3])*inv+127)/255)
}

// sampleBitmapBilinear samples premultiplied bitmap channels and returns them
// in the software painter's internal BGRA byte order. Coordinates are expressed
// with integer values at pixel centers.
func sampleBitmapBilinear(bitmap graphics.Bitmap, x, y float32) color.RGBA {
	bounds := bitmap.Bounds()
	if bounds.Empty() {
		return color.RGBA{}
	}

	ix0 := int(math.Floor(float64(x)))
	iy0 := int(math.Floor(float64(y)))
	tx := x - float32(ix0)
	ty := y - float32(iy0)
	ix1, iy1 := ix0+1, iy0+1
	ix0 = min(max(ix0, bounds.Min.X), bounds.Max.X-1)
	ix1 = min(max(ix1, bounds.Min.X), bounds.Max.X-1)
	iy0 = min(max(iy0, bounds.Min.Y), bounds.Max.Y-1)
	iy1 = min(max(iy1, bounds.Min.Y), bounds.Max.Y-1)

	r00, g00, b00, a00 := bitmap.GetPixel(ix0, iy0)
	r10, g10, b10, a10 := bitmap.GetPixel(ix1, iy0)
	r01, g01, b01, a01 := bitmap.GetPixel(ix0, iy1)
	r11, g11, b11, a11 := bitmap.GetPixel(ix1, iy1)

	r := bilinearByte(r00, r10, r01, r11, tx, ty)
	g := bilinearByte(g00, g10, g01, g11, tx, ty)
	b := bilinearByte(b00, b10, b01, b11, tx, ty)
	a := bilinearByte(a00, a10, a01, a11, tx, ty)
	return color.RGBA{R: b, G: g, B: r, A: a}
}

func bilinearByte(v00, v10, v01, v11 byte, tx, ty float32) byte {
	top := float32(v00) + (float32(v10)-float32(v00))*tx
	bottom := float32(v01) + (float32(v11)-float32(v01))*tx
	value := top + (bottom-top)*ty
	if value <= 0 {
		return 0
	}
	if value >= 255 {
		return 255
	}
	return byte(value + 0.5)
}

func (p *Painter) fillLine(c graphics.Color) {
	r, g, b, a := c.RGBA8()
	r = uint8(uint16(r) * uint16(a) / 255)
	g = uint8(uint16(g) * uint16(a) / 255)
	b = uint8(uint16(b) * uint16(a) / 255)
	for x := 0; x < p.line.Rect.Max.X; x++ {
		offset := p.line.PixOffset(x, 0)
		p.line.Pix[offset] = b
		p.line.Pix[offset+1] = g
		p.line.Pix[offset+2] = r
		p.line.Pix[offset+3] = a
	}
}

func (p *Painter) doPath(add rasterx.Adder, path graphics.Path) (closed bool, clip image.Rectangle) {
	points := make([]graphics.Point, 0, 16)
	devicePoint := func(x, y float32) fixed.Point26_6 {
		point := p.devicePoint(graphics.Point{X: x, Y: y})
		points = append(points, point)
		return toFixedP(point.X, point.Y)
	}
	emitLine := func(x, y float32) { add.Line(devicePoint(x, y)) }
	emitBezier := func(x0, y0, x1, y1, x2, y2 float32) {
		add.CubeBezier(devicePoint(x0, y0), devicePoint(x1, y1), devicePoint(x2, y2))
	}

	px := float32(0)
	py := float32(0)

	path.Range(func(op graphics.PathOperation, args []float32) (stop bool) {
		switch op {
		case graphics.PathMoveTo:
			add.Start(devicePoint(args[0], args[1]))
			px, py = args[0], args[1]

		case graphics.PathLineTo:
			emitLine(args[0], args[1])
			px, py = args[0], args[1]

		case graphics.PathArcTo:
			p.arcTo(emitLine, emitBezier, px, py, args[0], args[1], args[2], args[3], args[4], args[5], args[6])
			px, py = args[5], args[6]

		case graphics.PathBezierTo:
			emitBezier(args[0], args[1], args[2], args[3], args[4], args[5])
			px, py = args[4], args[5]

		case graphics.PathClose:
			closed = true
			add.Stop(true)
		}

		return closed
	})

	return closed, p.clipForPoints(points, 0)
}

func (p *Painter) arcTo(lineTo utils.LineTo, bezierTo utils.BezierTo, sx, sy, rx, ry, angle, large, sweep, ex, ey float32) {
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
	r = r * a / 65535
	g = g * a / 65535
	b = b * a / 65535
	return color.RGBA{
		R: uint8(b >> 8),
		G: uint8(g >> 8),
		B: uint8(r >> 8),
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
