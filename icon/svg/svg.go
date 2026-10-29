// Package svg supplies static SVG implementations of gui.IconSource.
// Files, embed resources and strings are supplied by the caller through io.Reader.
// SVG parsing and rasterization follow the supported subset of oksvg.
package svg

import (
	"bytes"
	"fmt"
	"image"
	"io"
	"sync"

	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/icon/internal/pixels"
	"github.com/golang-gui/oksvg"
)

// Parse reads and compiles an SVG icon. The result uses the Icon's foreground;
// WithRawColors preserves the SVG colors. Parsing never opens external
// resources. On error the returned Source is nil. Limit: 4 MiB of XML.
func Parse(r io.Reader) (*Source, error) {
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
	doc, err := oksvg.ReadIconStream(bytes.NewReader(data), oksvg.StrictErrorMode)
	if err != nil {
		return nil, fmt.Errorf("svg: parse: %w", err)
	}
	return &Source{doc: doc}, nil
}

// Source is immutable SVG content. Image serializes a bounded CPU cache;
// sharing never shares any painter-native resources. The zero value is invalid.
type Source struct {
	doc           *oksvg.SvgIcon
	rawColors     bool
	mu            sync.Mutex
	raw           image.Image
	width, height int
	colored       image.Image
	foreground    gui.Color
}

var _ gui.IconSource = (*Source)(nil)

// AspectRatio returns the parsed viewBox width divided by its height.
// It describes the drawing viewport, independently of requested image sizes.
// A nil or uninitialized Source returns zero.
func (s *Source) AspectRatio() float64 {
	if s == nil || s.doc == nil {
		return 0
	}
	return s.doc.ViewBox.W / s.doc.ViewBox.H
}

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
		s.raw = s.doc.Rasterize(width, height)
		s.width, s.height = width, height
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
