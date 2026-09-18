// Package draw adapts offscreen Software drawing to gui.IconSource.
package draw

import (
	"image"
	"image/draw"

	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/platform/graphics/software"
)

// Source generates final pixels using an isolated Software Painter. Keep Draw
// stable while in use. Coordinates are physical pixels; bounds covers the image.
// Draw must not retain p or call its Begin, End or Destroy methods.
type Source struct {
	Draw func(p graphics.Painter, size graphics.Size, foreground graphics.Color)
}

var _ gui.IconSource = (*Source)(nil)

// Image draws into an isolated Software surface. A missing Draw function or
// a request outside 1..4096 pixels per edge returns nil without drawing.
func (s *Source) Image(width, height int, foreground gui.Color) image.Image {
	if s == nil || s.Draw == nil {
		return nil
	}
	if width <= 0 || height <= 0 || width > 4096 || height > 4096 {
		return nil
	}
	target := &surface{}
	p, err := software.NewPainter(target)
	if err != nil {
		return nil
	}
	defer p.Destroy()
	func() {
		p.Begin(float32(width), float32(height), 1)
		defer p.End()
		p.Clear(graphics.Color{})
		s.Draw(p, graphics.Size{Width: float32(width), Height: float32(height)}, foreground)
	}()
	return target.image
}

type surface struct{ image *image.RGBA }

func (*surface) Transparent() bool { return true }
func (s *surface) Draw(src image.Image) error {
	// Software's output buffer belongs to the Painter. Snapshot before Destroy.
	b := src.Bounds()
	s.image = image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(s.image, s.image.Bounds(), src, b.Min, draw.Src)
	return nil
}
