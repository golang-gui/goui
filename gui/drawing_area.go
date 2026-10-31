package gui

import (
	"fmt"
	"image"
	"log"
	"math"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/platform/graphics/software"
)

// DrawFunc generates DrawingArea content synchronously on the GUI thread.
// Coordinates and size are local DIP. The Painter uses an isolated Software
// target; it and images created through it must not escape this callback.
// Do not mutate the widget tree or layout from draw.
type DrawFunc func(p Painter, size geometry.Size)

// DrawingArea is a leaf widget with explicitly regenerated Software content.
// Ordinary paints reuse the cached native image. It has no natural size,
// automatic decoration, focus or input action; configure layout constraints
// and event controllers as for any other widget.
type DrawingArea struct {
	WidgetBase
	draw        DrawFunc
	shouldDraw  signal.Signal2[*bool, func() bool]
	revision    uint64
	painting    bool
	dirty       bool
	size        geometry.Size
	scale       float32
	raster      *drawingRaster
	image       graphics.Image
	imageSize   geometry.Size
	needsUpload bool
	lastError   string
}

// NewDrawingArea creates an initially empty area. The first paint requests
// generation; a nil draw function generates transparent content.
func NewDrawingArea(draw DrawFunc) *DrawingArea {
	a := &DrawingArea{draw: draw, dirty: true}
	a.ConnectUnmount(a.releaseDrawing)
	return a
}

func (a *DrawingArea) DrawFunc() DrawFunc { return a.draw }

// SetDrawFunc explicitly replaces the function and requests regeneration, even
// for the same function value. ShouldDraw listeners may still decline it.
func (a *DrawingArea) SetDrawFunc(draw DrawFunc) {
	if a.Destroyed() {
		return
	}
	a.draw = draw
	a.revision++
	a.dirty = true
	a.RequestPaint()
}

// ConnectShouldDraw queries whether this paint should regenerate content.
// The initial value is true on first use, explicit SetDrawFunc, size or raster
// scale changes. Listeners run in connection order and may override the value.
// True does not request another frame. Use RequestPaint and the existing frame
// mechanism to schedule updates. Declining keeps the old image at its original
// logical size, clipped to the current area; an empty cache stays transparent.
func (a *DrawingArea) ConnectShouldDraw(fn func(redraw *bool)) signal.Handle {
	return a.shouldDraw.Connect(func(redraw *bool, valid func() bool) {
		if valid() {
			fn(redraw)
		}
	})
}

func (a *DrawingArea) Measure(c layout.Constraint) layout.Measurement {
	if !a.Visible() {
		return layout.Measurement{}
	}
	return layout.Measured(a.constrain(c, geometry.Size{}))
}

func (a *DrawingArea) Paint(p Painter) {
	if a.Destroyed() || !a.Visible() || a.painting {
		return
	}
	size, scale := a.Rect().Size, p.PixelScale()
	if size.Width <= 0 || size.Height <= 0 || scale <= 0 || math.IsNaN(float64(scale)) || math.IsInf(float64(scale), 0) {
		return
	}
	a.painting = true
	defer func() { a.painting = false }()
	revision, root, parent := a.revision, a.Root(), a.Parent()
	valid := func() bool {
		return !a.Destroyed() && a.Visible() && a.revision == revision && a.Root() == root && a.Parent() == parent && a.Rect().Size == size
	}
	redraw := a.dirty || a.size != size || a.scale != scale
	a.shouldDraw.Emit(&redraw, valid)
	if !valid() {
		return
	}
	if redraw {
		if a.draw == nil {
			a.releaseImage()
			if a.raster != nil {
				a.raster.painter.Destroy()
				a.raster = nil
			}
			a.size, a.scale, a.dirty, a.needsUpload = size, scale, false, false
		} else if !a.rasterize(size, scale, valid) && !valid() {
			return
		}
	}
	if a.needsUpload && a.raster != nil {
		pixels := a.raster.surface.pixels
		bounds := pixels.Bounds()
		reuse := false
		if a.image != nil {
			w, h := a.image.Size()
			reuse = w == bounds.Dx() && h == bounds.Dy()
		}
		var native graphics.Image
		var err error
		if reuse {
			err = a.image.Update(pixels)
		} else {
			native, err = p.NewImage(pixels)
		}
		if !valid() {
			if native != nil {
				native.Destroy()
			}
			return
		}
		if err != nil {
			if native != nil {
				native.Destroy()
			}
			a.reportDrawError(err)
			if reuse {
				// Update is not transactional. Keep the CPU result for retry,
				// but do not draw potentially partially updated native pixels.
				return
			}
		} else {
			if !reuse {
				a.releaseImage()
				a.image = native
			}
			a.imageSize = geometry.Size{Width: float32(bounds.Dx()) / a.scale, Height: float32(bounds.Dy()) / a.scale}
			a.needsUpload = false
			a.lastError = ""
		}
	}
	if a.image != nil {
		p.Save()
		defer p.Restore()
		p.SetClipRect(geometry.Rectangle{Size: size})
		p.DrawImage(geometry.Rectangle{Size: a.imageSize}, a.image)
	}
}

// The active raster is borrowed locally. Unmount/reentrant callbacks can clear
// the widget without destroying a Software Painter between Begin and End.
func (a *DrawingArea) rasterize(size geometry.Size, scale float32, valid func() bool) bool {
	w, h := math.Ceil(float64(size.Width)*float64(scale)), math.Ceil(float64(size.Height)*float64(scale))
	if w < 1 || h < 1 || w > 16384 || h > 16384 || w*h > 64_000_000 || math.IsNaN(w) || math.IsNaN(h) {
		a.reportDrawError(fmt.Errorf("raster size out of range: %g x %g", w, h))
		return false
	}
	r := a.raster
	a.raster = nil
	a.needsUpload = false
	a.dirty = true
	if r == nil {
		r = new(drawingRaster)
		var err error
		r.painter, err = software.NewPainter(&r.surface)
		if err != nil {
			a.reportDrawError(err)
			return false
		}
	}
	retained := false
	defer func() {
		if !retained {
			r.painter.Destroy()
		}
	}()
	draw := a.draw
	func() {
		r.painter.Begin(float32(w), float32(h), scale)
		defer r.painter.End()
		r.painter.Clear(graphics.Color{})
		bounds := geometry.Rectangle{Size: size}
		local := newPainter(r.painter, bounds, scale)
		local.paintInBounds("DrawingArea.Draw", func(p Painter) { draw(p, size) }, bounds)
	}()
	if !valid() {
		return false
	}
	a.raster = r
	a.size, a.scale, a.dirty, a.needsUpload = size, scale, false, true
	retained = true
	return true
}

func (a *DrawingArea) reportDrawError(err error) {
	if message := err.Error(); message != a.lastError {
		a.lastError = message
		log.Printf("goui: DrawingArea: %s", message)
	}
}
func (a *DrawingArea) releaseImage() {
	if a.image != nil {
		a.image.Destroy()
		a.image = nil
	}
	a.imageSize = geometry.Size{}
}
func (a *DrawingArea) releaseDrawing() {
	a.revision++
	a.releaseImage()
	if a.raster != nil {
		a.raster.painter.Destroy()
		a.raster = nil
	}
	a.needsUpload = false
	a.dirty = true
	a.lastError = ""
}

type drawingRaster struct {
	painter graphics.Painter
	surface drawingSurface
}
type drawingSurface struct{ pixels image.Image }

func (*drawingSurface) Transparent() bool               { return true }
func (s *drawingSurface) Draw(pixels image.Image) error { s.pixels = pixels; return nil }
