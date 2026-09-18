// Package svg supplies static SVG implementations of gui.IconSource.
// Files, embed resources and strings are supplied by the caller through io.Reader.
// It implements a checked subset, not a browser SVG engine; see DesignSVG.md.
package svg

import (
	"fmt"
	"image"
	"io"
	"math"
	"sync"

	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/icon/internal/pixels"
	"github.com/golang-gui/oksvg"
	"github.com/srwiley/rasterx"
	"github.com/srwiley/scanFT"
)

// Parse reads and compiles an SVG icon. The result uses the Icon's foreground;
// WithRawColors preserves the SVG colors. Parsing never opens external
// resources. On error the returned Source is nil. Limit: 4 MiB of XML.
func Parse(r io.Reader) (*Source, error) {
	doc, err := parse(r)
	if err != nil {
		return nil, err
	}
	return &Source{doc: doc}, nil
}

// Source is immutable SVG content. Image serializes a bounded CPU cache;
// sharing never shares any painter-native resources. The zero value is invalid.
type Source struct {
	doc           *document
	rawColors     bool
	mu            sync.Mutex
	raw           image.Image
	width, height int
	colored       image.Image
	foreground    gui.Color
}

var _ gui.IconSource = (*Source)(nil)

// WithRawColors creates an independent configuration sharing parsed paths.
// Create it outside UI build to preserve source identity across rebuilds.
func (s *Source) WithRawColors() *Source {
	if s == nil {
		return nil
	}
	return &Source{doc: s.doc, rawColors: true}
}

// Image supplies pixels from a parsed SVG. A nil/zero source or a request
// outside 1..4096 pixels per edge returns nil without allocating a raster.
func (s *Source) Image(width, height int, foreground gui.Color) image.Image {
	if s == nil || s.doc == nil || width <= 0 || height <= 0 || width > 4096 || height > 4096 {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.raw == nil || s.width != width || s.height != height {
		raw := s.doc.rasterize(width, height)
		s.raw, s.width, s.height = raw, width, height
		s.colored = nil
	}
	if s.rawColors {
		return s.raw
	}
	if s.colored == nil || s.foreground != foreground {
		s.colored = pixels.Tint(s.raw, foreground)
		s.foreground = foreground
	}
	return s.colored
}

type document struct{ icon *oksvg.SvgIcon }

func parse(r io.Reader) (_ *document, err error) {
	if r == nil {
		return nil, fmt.Errorf("svg: nil reader")
	}
	data, err := io.ReadAll(io.LimitReader(r, (4<<20)+1))
	if err != nil {
		return nil, fmt.Errorf("svg: read: %w", err)
	}
	if len(data) > 4<<20 {
		return nil, fmt.Errorf("svg: document exceeds 4 MiB")
	}
	data, err = normalize(data)
	if err != nil {
		return nil, err
	}
	// oksvg is a third-party parser. Malformed input must not bring down the
	// GUI, including parser panics; never return partially compiled content.
	defer func() {
		if v := recover(); v != nil {
			err = fmt.Errorf("svg: parser rejected malformed input: %v", v)
		}
	}()
	icon, err := compile(data)
	if err != nil {
		return nil, fmt.Errorf("svg: parse: %w", err)
	}
	v := icon.ViewBox
	if !finite(v.X) || !finite(v.Y) || !finite(v.W) || !finite(v.H) || v.W <= 0 || v.H <= 0 {
		return nil, fmt.Errorf("svg: finite positive viewBox or width/height required")
	}
	for id, gradient := range icon.Grads {
		if len(gradient.Stops) == 0 {
			return nil, fmt.Errorf("svg: gradient %q has no stops", id)
		}
	}
	return &document{icon: icon}, nil
}

// rasterize receives a validated size; parsing has already prepared all paths.
func (d *document) rasterize(width, height int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	scanner := scanFT.NewScannerFT(width, height, scanFT.NewRGBAPainter(img))
	raster := rasterx.NewDasher(width, height, scanner)
	v := d.icon.ViewBox
	scale := min(float64(width)/v.W, float64(height)/v.H)
	x, y := (float64(width)-v.W*scale)/2, (float64(height)-v.H*scale)/2
	// Do not call SetTarget on the shared icon. Its translation also assumes
	// a zero viewBox origin when scaling. Compose T(center) S(scale) T(-origin).
	target := rasterx.Identity.Translate(x, y).Scale(scale, scale).Translate(-v.X, -v.Y)
	for _, path := range d.icon.SVGPaths {
		// DrawTransformed modifies receiver-local drawing state. Copying the
		// path value preserves the immutable shared compilation across calls.
		// rasterx transforms path coordinates, but not stroke/dash dimensions.
		path.LineWidth *= scale
		path.DashOffset *= scale
		if len(path.Dash) != 0 {
			path.Dash = append([]float64(nil), path.Dash...)
			for j := range path.Dash {
				path.Dash[j] *= scale
			}
		}
		path.DrawTransformed(raster, 1, target)
	}
	return img
}

func finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }
