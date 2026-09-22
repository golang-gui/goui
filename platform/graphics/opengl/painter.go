package opengl

import (
	"fmt"
	"image"
	"sync"

	"github.com/goexlib/cgo"
	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/platform/graphics/internal/boxshadow"
	"github.com/golang-gui/goui/platform/graphics/internal/offscreen"
	"github.com/golang-gui/goui/platform/graphics/internal/pixelsnap"
	"github.com/golang-gui/goui/platform/graphics/internal/textbitmap"
	"github.com/golang-gui/goui/platform/graphics/utils"
	"github.com/golang-gui/goui/platform/typography"

	"github.com/golang-gui/nanovgo"
	"github.com/golang-gui/nanovgo/gl"
)

type Painter struct {
	ctx           Context
	vg            *nanovgo.Context
	images        map[*imageResource]struct{}
	textImages    *textbitmap.ImageCache[graphics.Image]
	textPixels    []byte
	pendingImages int
	scale         float32
	transform     geometry.Transform

	activeFrame bool
	transparent bool
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

// NewPainter creates an OpenGL painter for win. The caller must lock the owning
// OS thread before creating the window and keep all painter operations, including
// Destroy, on that thread. The painter must be destroyed before the window.
func NewPainter(win NativeWindow) (_ graphics.Painter, err error) {
	p := new(Painter)
	p.transparent = win.Transparent()
	alphaBits := 0
	if p.transparent {
		alphaBits = 8
	}
	p.ctx, err = NewContext(win, nil, Config{
		PixelFormat: PixelFormat{
			RedBits:      8,
			GreenBits:    8,
			BlueBits:     8,
			AlphaBits:    alphaBits,
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
		p.ctx.Destroy()
		return nil, fmt.Errorf("make current err: %v", err)
	}

	if p.vg, err = nanovgo.NewContext(p.ctx, nanovgo.AntiAlias); err != nil {
		p.Destroy()
		return nil, fmt.Errorf("create nanovgo context err: %v", err)
	}

	p.images = make(map[*imageResource]struct{})
	p.textImages = textbitmap.NewImageCache(4, func(img graphics.Image) { img.Destroy() })
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
			img.pendingDestroy = false
		}
		clear(p.images)
		p.pendingImages = 0
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
	img.destroyed = true
	img.pendingDestroy = true
	p.pendingImages++
	if p.activeFrame {
		return
	}
	if img.handle != 0 && p.vg != nil {
		if err := p.ctx.MakeCurrent(); err != nil {
			// Keep the invalid resource pending for a later End or Painter.Destroy.
			panic(fmt.Sprintf("opengl: make context current to destroy image: %v", err))
		}
		p.destroyImageCurrent(img)
		p.ctx.ClearCurrent()
		return
	}
	p.finishImageDestroy(img)
}

func (p *Painter) destroyImageCurrent(img *imageResource) {
	if img == nil || img.owner != p {
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
		p.pendingImages--
	}
	delete(p.images, img)
	img.owner = nil
	img.handle = 0
	img.destroyed = true
}

func (p *Painter) flushPendingImages() {
	if p.pendingImages == 0 {
		return
	}
	for img := range p.images {
		if img.pendingDestroy {
			p.destroyImageCurrent(img)
		}
	}
}

func (p *Painter) Begin(width, height, scale float32) {
	if err := p.ctx.MakeCurrent(); err != nil {
		panic(fmt.Sprintf("opengl: begin frame: %v", err))
	}
	gl.Viewport(0, 0, int(width), int(height))
	gl.Clear(gl.COLOR_BUFFER_BIT | gl.DEPTH_BUFFER_BIT | gl.STENCIL_BUFFER_BIT)
	// NanoVG's Go API accepts integer frame dimensions. Use physical pixels
	// here and put DPI scaling in the transform: truncating width/scale would
	// stretch the viewport at fractional DPI and undo device-pixel alignment.
	p.vg.BeginFrame(int(width), int(height), 1)
	p.activeFrame = true
	p.scale = scale
	p.SetTransform(geometry.Identity())
}

func (p *Painter) End() {
	defer p.ctx.ClearCurrent()
	defer func() {
		// Discard any commands left if EndFrame panicked before submission.
		p.vg.CancelFrame()
		p.activeFrame = false
		p.flushPendingImages()
	}()
	p.vg.EndFrame()
	p.ctx.SwapBuffers()
}

func (p *Painter) Clear(color graphics.Color) {
	// glClear writes framebuffer channels directly, unlike NanoVG paints.
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
	// The same native boundary conversion is used by all NanoVG paints.
	color := nanoVGColor(shadow.Color)
	p.vg.Save()
	defer p.vg.Restore()
	p.vg.BeginPath()
	if shape.BlurRadius <= 0 {
		rect := pixelsnap.Rect(shape.Rect, p.transform, p.scale)
		p.vg.RoundedRect(rect.X, rect.Y, rect.Width, rect.Height, shape.Radius)
		p.vg.SetFillColor(color)
	} else {
		bounds := shape.Bounds()
		p.vg.Rect(bounds.X, bounds.Y, bounds.Width, bounds.Height)
		p.vg.SetFillPaint(nanovgo.BoxShadow(
			shape.Rect.X, shape.Rect.Y, shape.Rect.Width, shape.Rect.Height,
			shape.Radius, shape.BlurRadius, color,
		))
	}
	p.vg.Fill()
}

func (p *Painter) FillRect(rect graphics.Rectangle, brush graphics.Brush) {
	if p.beginFill(brush) {
		defer p.end()
		rect = pixelsnap.Rect(rect, p.transform, p.scale)
		p.vg.BeginPath()
		p.vg.Rect(rect.X, rect.Y, rect.Width, rect.Height)
		p.vg.Fill()
	}
}

func (p *Painter) FillRoundRect(rect graphics.Rectangle, radius float32, brush graphics.Brush) {
	if p.beginFill(brush) {
		defer p.end()
		rect = pixelsnap.Rect(rect, p.transform, p.scale)
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
		p0, p1 = pixelsnap.Line(p0, p1, strokeWidth, p.transform, p.scale)
		p.vg.BeginPath()
		p.vg.MoveTo(p0.X, p0.Y)
		p.vg.LineTo(p1.X, p1.Y)
		p.vg.Stroke()
	}
}

func (p *Painter) DrawRect(rect graphics.Rectangle, strokeWidth float32, brush graphics.Brush) {
	if p.beginDraw(strokeWidth, brush) {
		defer p.end()
		rect = pixelsnap.StrokeRect(rect, strokeWidth, p.transform, p.scale)
		p.vg.BeginPath()
		p.vg.Rect(rect.X, rect.Y, rect.Width, rect.Height)
		p.vg.Stroke()
	}
}

func (p *Painter) DrawRoundRect(rect graphics.Rectangle, radius, strokeWidth float32, brush graphics.Brush) {
	if p.beginDraw(strokeWidth, brush) {
		defer p.end()
		rect = pixelsnap.StrokeRect(rect, strokeWidth, p.transform, p.scale)
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
	// Keep p.transform in logical units for text rasterization and snapping;
	// only NanoVG sees the DIP-to-device transform (scale * t).
	s := p.scale
	p.vg.SetTransformByValue(a*s, b*s, c*s, d*s, e*s, f*s)
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
	// device space, applying only DPI scaling. Save and restore the transform
	// manually (NOT via vg.Save/Restore, which would revert the new scissor).
	xform := p.vg.CurrentTransform()
	p.vg.ResetTransform()
	p.vg.ResetScissor()
	if rect.X != 0 || rect.Y != 0 || rect.Width != 0 || rect.Height != 0 {
		// NanoVG's scissor has a one-device-pixel AA fringe. Put its edges on
		// the same pixel boundaries as the fill/stroke so it cannot attenuate
		// the widget border a second time. Keep rounded-corner AA enabled.
		rect = pixelsnap.Rect(rect, geometry.Identity(), p.scale)
		p.vg.Scissor(rect.X*p.scale, rect.Y*p.scale, rect.Width*p.scale, rect.Height*p.scale)
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
	// NanoVG accepts straight colors and premultiplies before interpolation.
	if color.A <= 0 {
		return nanovgo.Color{}
	}
	return nanovgo.Color{R: color.R / color.A, G: color.G / color.A, B: color.B / color.A, A: color.A}
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

// RenderImage offscreen GL entry points. Resolved once through the current
// context on the first RenderImage call (all three platforms resolve core GL
// procs through the current context, like NanoVG's own GL bindings) and then
// reused globally to avoid per-frame GetProcAddress lookups.
var (
	renderImageProcMu             sync.Mutex
	renderImageProcsReady         bool
	procGLGetIntegerv             uintptr
	procGLGenFramebuffers         uintptr
	procGLBindFramebuffer         uintptr
	procGLDeleteFramebuffers      uintptr
	procGLGenRenderbuffers        uintptr
	procGLBindRenderbuffer        uintptr
	procGLDeleteRenderbuffers     uintptr
	procGLRenderbufferStorage     uintptr
	procGLFramebufferRenderbuffer uintptr
	procGLCheckFramebufferStatus  uintptr
	procGLReadPixels              uintptr
)

// ensureRenderImageProcs resolves the offscreen GL entry points on first use.
// The caller must have made the context current. A failed load is not cached
// so a later call can retry; success is cached globally.
func ensureRenderImageProcs(ctx Context) error {
	renderImageProcMu.Lock()
	defer renderImageProcMu.Unlock()
	if renderImageProcsReady {
		return nil
	}
	targets := []struct {
		name string
		out  *uintptr
	}{
		{"glGetIntegerv", &procGLGetIntegerv},
		{"glGenFramebuffers", &procGLGenFramebuffers},
		{"glBindFramebuffer", &procGLBindFramebuffer},
		{"glDeleteFramebuffers", &procGLDeleteFramebuffers},
		{"glGenRenderbuffers", &procGLGenRenderbuffers},
		{"glBindRenderbuffer", &procGLBindRenderbuffer},
		{"glDeleteRenderbuffers", &procGLDeleteRenderbuffers},
		{"glRenderbufferStorage", &procGLRenderbufferStorage},
		{"glFramebufferRenderbuffer", &procGLFramebufferRenderbuffer},
		{"glCheckFramebufferStatus", &procGLCheckFramebufferStatus},
		{"glReadPixels", &procGLReadPixels},
	}
	for _, t := range targets {
		proc, err := ctx.GetProcAddress(t.name)
		if err != nil || proc == 0 {
			return fmt.Errorf("opengl: missing %s: %v", t.name, err)
		}
		*t.out = proc
	}
	renderImageProcsReady = true
	return nil
}

func (p *Painter) RenderImage(width, height int, scale float32, draw func()) (image.Image, error) {
	if p.activeFrame {
		return nil, fmt.Errorf("opengl: render image during active frame")
	}
	if err := offscreen.Validate(width, height, scale, draw); err != nil {
		return nil, err
	}
	if err := p.ctx.MakeCurrent(); err != nil {
		return nil, fmt.Errorf("opengl: render image: %w", err)
	}
	defer p.ctx.ClearCurrent()
	if err := ensureRenderImageProcs(p.ctx); err != nil {
		return nil, err
	}
	get := func(name uint32) int32 {
		var result int32
		cgo.Call(procGLGetIntegerv, name, &result)
		return result
	}
	const drawFramebuffer, readFramebuffer = uint32(0x8CA9), uint32(0x8CA8)
	oldDraw, oldRead, oldBuffer := get(0x8CA6), get(0x8CAA), get(0x8CA7)
	var viewport [4]int32
	cgo.Call(procGLGetIntegerv, uint32(gl.VIEWPORT), &viewport[0])
	var framebuffer uint32
	var buffers [2]uint32
	cgo.Call(procGLGenFramebuffers, int32(1), &framebuffer)
	cgo.Call(procGLGenRenderbuffers, int32(2), &buffers[0])
	defer func() {
		cgo.Call(procGLBindFramebuffer, drawFramebuffer, uint32(oldDraw))
		cgo.Call(procGLBindFramebuffer, readFramebuffer, uint32(oldRead))
		cgo.Call(procGLBindRenderbuffer, uint32(gl.RENDERBUFFER), uint32(oldBuffer))
		gl.Viewport(int(viewport[0]), int(viewport[1]), int(viewport[2]), int(viewport[3]))
		cgo.Call(procGLDeleteFramebuffers, int32(1), &framebuffer)
		cgo.Call(procGLDeleteRenderbuffers, int32(2), &buffers[0])
	}()
	cgo.Call(procGLBindFramebuffer, uint32(gl.FRAMEBUFFER), framebuffer)
	for i, format := range []uint32{gl.RGBA8, gl.DEPTH24_STENCIL8} {
		cgo.Call(procGLBindRenderbuffer, uint32(gl.RENDERBUFFER), buffers[i])
		cgo.Call(procGLRenderbufferStorage, uint32(gl.RENDERBUFFER), format, int32(width), int32(height))
		attachment := uint32(gl.COLOR_ATTACHMENT0)
		if i == 1 {
			attachment = gl.DEPTH_STENCIL_ATTACHMENT
		}
		cgo.Call(procGLFramebufferRenderbuffer, uint32(gl.FRAMEBUFFER), attachment, uint32(gl.RENDERBUFFER), buffers[i])
	}
	if status := cgo.CallRet[uint32](procGLCheckFramebufferStatus, uint32(gl.FRAMEBUFFER)); status != gl.FRAMEBUFFER_COMPLETE {
		return nil, fmt.Errorf("opengl: incomplete offscreen framebuffer %#x", status)
	}
	oldScale, oldTransform := p.scale, p.transform
	defer func() {
		p.vg.CancelFrame()
		p.activeFrame = false
		p.flushPendingImages()
		p.scale, p.transform = oldScale, oldTransform
	}()
	gl.Viewport(0, 0, width, height)
	gl.Disable(gl.SCISSOR_TEST)
	gl.ColorMask(true, true, true, true)
	gl.StencilMask(0xffffffff)
	gl.ClearColor(0, 0, 0, 0)
	gl.Clear(gl.COLOR_BUFFER_BIT | gl.DEPTH_BUFFER_BIT | gl.STENCIL_BUFFER_BIT)
	p.vg.BeginFrame(width, height, 1)
	p.activeFrame, p.scale = true, scale
	p.SetTransform(geometry.Identity())
	p.SetClipRect(graphics.Rectangle{})
	draw()
	p.vg.EndFrame()
	// Read tightly packed rows into independent CPU storage. Preserve pack
	// state, including a possible pixel-pack buffer, rather than assuming zero.
	packBuffer := get(0x88ED)
	gl.BindBuffer(gl.PIXEL_PACK_BUFFER, gl.Buffer{})
	defer gl.BindBuffer(gl.PIXEL_PACK_BUFFER, gl.Buffer{Value: uint32(packBuffer)})
	for _, item := range [][2]uint32{{0x0D05, 1}, {0x0D02, 0}, {0x0D03, 0}, {0x0D04, 0}} {
		old := get(item[0])
		gl.PixelStorei(gl.Enum(item[0]), gl.Int(item[1]))
		defer gl.PixelStorei(gl.Enum(item[0]), gl.Int(old))
	}
	result := image.NewRGBA(image.Rect(0, 0, width, height))
	cgo.Call(procGLReadPixels, int32(0), int32(0), int32(width), int32(height), uint32(gl.RGBA), uint32(gl.UNSIGNED_BYTE), &result.Pix[0])
	if err := gl.GetError(); err != gl.NO_ERROR {
		return nil, fmt.Errorf("opengl: read offscreen pixels: %#x", err)
	}
	row := make([]byte, result.Stride)
	for y := 0; y < height/2; y++ {
		top, bottom := y*result.Stride, (height-1-y)*result.Stride
		copy(row, result.Pix[top:top+result.Stride])
		copy(result.Pix[top:top+result.Stride], result.Pix[bottom:bottom+result.Stride])
		copy(result.Pix[bottom:bottom+result.Stride], row)
	}
	return result, nil
}
