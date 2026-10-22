package typography

import (
	"image"
	"image/color"

	"github.com/golang-gui/goui/core/geometry"
)

// GlyphContext is an optional native capability for retaining a complete
// paragraph's shaping independently of its eventual line breaks. Callers own
// the returned layout. It has WrapNone and TextAlignBegin; all other format
// fields are retained. Forced paragraph separators are not supported.
type GlyphContext interface {
	NewGlyphLayout(text string, format TextFormat) (GlyphLayout, error)
}

// GlyphMetrics is a snapshot of one completely shaped paragraph. Clusters are
// in logical byte order. VisualOrder lists their indices in native visual order
// (including zero-advance clusters). Widths are advances, independent of X;
// callers must not infer ordering from potentially rounded X coordinates.
// Returned slices are immutable and remain valid after layout destruction.
type GlyphMetrics struct {
	Line          TextLine
	Clusters      []TextCluster
	VisualOrder   []int
	BaseDirection TextDirection
	// TabWidth is the effective repeating native tab interval in logical
	// units, including the default selected when TextFormat.TabWidth is zero.
	// It is positive when the paragraph contains a tab.
	TabWidth float32
	// InkPadding conservatively bounds glyph ink outside any range's advance
	// box, in logical units. It includes offsets and fallback fonts, and is
	// independent of the range's original whole-paragraph X coordinate.
	InkPadding geometry.Insets
}

// GlyphLayout retains native glyphs and their full original shaping context.
// Like TextLayout it is thread-affine. Mutations invalidate all borrowed native
// data; callers sharing a layout between projections must keep it immutable.
type GlyphLayout interface {
	TextLayout
	GlyphMetrics() (GlyphMetrics, error)
	// RasterizeGlyphs paints a logical range of complete shaping clusters, in
	// their original visual order, packed from local X=0 and translated by
	// offset (physical raster pixels). offset preserves fractional phase when
	// composing several ranges on one pixel grid. Baseline and Y otherwise match
	// GlyphMetrics.Line. It does not shape a substring or draw decorations.
	// foreground overrides glyph color; nil preserves source colors. Color
	// glyphs retain native rendering. Invalid cluster boundaries return errors.
	// scale, region, pixels and buf obey Rasterize's contract, in this range's
	// local raster plane. No whole-line X coordinate is needed, even for huge
	// source paragraphs.
	RasterizeGlyphs(start, length int, foreground color.Color, offset geometry.Point, scale float32, region image.Rectangle, buf []byte) (TextBitmap, error)
}
