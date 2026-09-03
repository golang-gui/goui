package opengl

import (
	"fmt"
	"image"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/platform/graphics/internal/boxshadow"
	"github.com/golang-gui/goui/platform/graphics/internal/textbitmap"
	"github.com/golang-gui/goui/platform/graphics/utils"
	"github.com/golang-gui/goui/platform/typography"

	"github.com/golang-gui/nanovgo"
	"github.com/golang-gui/nanovgo/gl"
)

type Painter struct {
	ctx               Context
	vg                *nanovgo.Context
	win               NativeWindow
	images            map[*imageResource]struct{}
	textImages        *textbitmap.ImageCache[graphics.Image]
	textPixels        []byte
	pendingTextImages int
	scale             float32
	transform         geometry.Transform
	lastWidth         float32
	lastHeight        float32

	hasFrame     bool
	activeFrame  bool
	resizedFrame bool
	resizedPaint bool
}

type imageResource struct {
	owner          *Painter
	width          int
	height         int
	handle         int
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
		return fmt.Errorf("opengl: update destroyed image")
	}
	return i.owner.updateImage(i, src)
}

func (i *imageResource) Destroy() {
	if i == nil || i.destroyed || i.owner == nil {
		return
	}
	i.owner.destroyImage(i)
}

func NewPainter(win NativeWindow) (_ graphics.Painter, err error) {
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
	_, p.resizedPaint = p.ctx.(GLXContext)

	p.images = make(map[*imageResource]struct{})
	p.textImages = textbitmap.NewImageCache(4, p.releaseTextImage)
	return p, nil
}

func (p *Painter) Name() string {
	return "OpenGL"
}

func (p *Painter) Destroy() {
	if p.activeFrame {
		panic("opengl: destroy painter during active frame")
	}
	if p.vg != nil {
		p.textImages.Destroy()
		_ = p.ctx.MakeCurrent()
		for img := range p.images {
			img.owner = nil
			img.handle = 0
			img.destroyed = true
		}
		clear(p.images)
		p.pendingTextImages = 0
		p.textPixels = nil
		p.vg.Delete()
		p.vg = nil
		p.ctx.ClearCurrent()
	}
	if p.ctx != nil {
		p.ctx.Destroy()
		p.ctx = nil
	}
}

func (p *Painter) NewImage(src image.Image) (graphics.Image, error) {
	if p.vg == nil || p.ctx == nil {
		return nil, fmt.Errorf("opengl: create image on destroyed painter")
	}
	if src == nil {
		return nil, fmt.Errorf("opengl: create image from nil source")
	}
	bounds := src.Bounds()
	if bounds.Empty() {
		return nil, fmt.Errorf("opengl: create empty image")
	}

	bitmap := imageUploadBitmap(src)
	madeCurrent := !p.activeFrame
	if madeCurrent {
		if err := p.ctx.MakeCurrent(); err != nil {
			return nil, fmt.Errorf("opengl: make context current for image: %w", err)
		}
		defer p.ctx.ClearCurrent()
	}
	handle := p.vg.CreateImageRGBA(bitmap.Width, bitmap.Height, nanovgo.ImagePreMultiplied, bitmap.Pixels)
	if handle == 0 {
		return nil, fmt.Errorf("opengl: create native image")
	}
	img := &imageResource{
		owner:  p,
		width:  bounds.Dx(),
		height: bounds.Dy(),
		handle: handle,
	}
	p.images[img] = struct{}{}
	return img, nil
}

func (p *Painter) updateImage(img *imageResource, src image.Image) error {
	if p.activeFrame {
		panic("opengl: update image during active frame")
	}
	if img == nil || img.destroyed || img.owner != p || img.handle == 0 || p.vg == nil || p.ctx == nil {
		return fmt.Errorf("opengl: update invalid image")
	}
	if src == nil {
		return fmt.Errorf("opengl: update image from nil source")
	}
	bounds := src.Bounds()
	if bounds.Dx() != img.width || bounds.Dy() != img.height {
		return fmt.Errorf("opengl: update image size %dx%d, want %dx%d", bounds.Dx(), bounds.Dy(), img.width, img.height)
	}
	bitmap := imageUploadBitmap(src)
	if err := p.ctx.MakeCurrent(); err != nil {
		return fmt.Errorf("opengl: make context current to update image: %w", err)
	}
	err := p.vg.UpdateImage(img.handle, bitmap.Pixels)
	p.ctx.ClearCurrent()
	if err != nil {
		return fmt.Errorf("opengl: update native image: %w", err)
	}
	return nil
}

func imageUploadBitmap(src image.Image) graphics.Bitmap {
	bitmap, ok := graphics.ToBitmap(src, graphics.PixelFormatRGBA)
	if ok && bitmap.Stride == bitmap.Width*graphics.PixelFormatRGBA.BytesPerPixel() {
		return bitmap
	}
	return graphics.CopyToBitmap(src, graphics.PixelFormatRGBA, nil)
}

func (p *Painter) destroyImage(img *imageResource) {
	if img == nil || img.destroyed || img.owner != p {
		return
	}
	if p.activeFrame {
		panic("opengl: destroy image during active frame")
	}
	if img.handle != 0 && p.vg != nil {
		if err := p.ctx.MakeCurrent(); err != nil {
			panic(fmt.Sprintf("opengl: make context current to destroy image: %v", err))
		}
		p.destroyImageCurrent(img)
		p.ctx.ClearCurrent()
		return
	}
	p.finishImageDestroy(img)
}

func (p *Painter) destroyImageCurrent(img *imageResource) {
	if img == nil || img.destroyed || img.owner != p {
		return
	}
	if img.handle != 0 && p.vg != nil {
		p.vg.DeleteImage(img.handle)
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
	img.handle = 0
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
			p.destroyImageCurrent(img)
		}
	}
}

func (p *Painter) Begin(width, height, scale float32) {
	p.ctx.MakeCurrent()
	p.trackFrameSize(width, height)
	gl.Viewport(0, 0, int(width), int(height))
	gl.Clear(gl.COLOR_BUFFER_BIT | gl.DEPTH_BUFFER_BIT | gl.STENCIL_BUFFER_BIT)
	p.vg.BeginFrame(int(width/scale), int(height/scale), scale)
	p.activeFrame = true
	p.scale = scale
	p.transform = geometry.Identity()
}

func (p *Painter) End() {
	p.vg.EndFrame()
	p.activeFrame = false
	p.flushPendingTextImages()
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
	if layout == nil {
		return
	}

	// Text layouts are rasterized before NanoVG applies its transform. Match the
	// bitmap resolution to the largest final device-space stretch so transformed
	// text is never enlarged from a lower-resolution texture.
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
	// Keep the layout's logical size unchanged. The current NanoVG transform
	// scales this rectangle to the physical size for which the bitmap was drawn.
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

func (p *Painter) DrawImage(rect graphics.Rectangle, img graphics.Image) {
	if rect.Width <= 0 || rect.Height <= 0 {
		return
	}
	native, ok := img.(*imageResource)
	if !ok || native == nil || native.owner != p || native.destroyed || native.handle == 0 {
		panic("opengl: image does not belong to painter or was destroyed")
	}
	p.drawImageHandle(rect, native.handle)
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

func (p *Painter) drawImageHandle(rect graphics.Rectangle, img int) {
	p.vg.Save()
	p.vg.BeginPath()
	p.vg.SetFillPaint(nanovgo.ImagePattern(rect.X, rect.Y, rect.Width, rect.Height, 0, img, 1.0))
	p.vg.Rect(rect.X, rect.Y, rect.Width, rect.Height)
	p.vg.Fill()
	p.vg.Restore()
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
