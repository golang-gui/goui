package typography

import (
	"image/color"

	"github.com/golang-gui/goui/core/signal"
)

// TextLayout represents immutable UTF-8 text shaped and arranged within a
// mutable layout box. Layout dimensions and all metric coordinates are in
// logical units.
//
// A TextLayout is thread-affine and is not safe for concurrent use. The caller
// must destroy it before the Context that created it. Except for repeated calls
// to Destroy, no method may be called after destruction.
type TextLayout interface {
	// Destroy releases the layout's native resources. It is idempotent.
	Destroy()

	// Text returns the original text supplied at layout creation.
	Text() string

	// Format returns the base format supplied at creation with the current
	// paragraph alignment and wrapping mode. It does not include range formatting
	// applied through SetTextFont, SetTextColor, SetUnderline, or SetStrikethrough.
	Format() TextFormat

	// Size returns the maximum layout width and height in logical units. These
	// constraints are distinct from the occupied size returned by MeasureSize.
	Size() (maxWidth, maxHeight float32)

	// SetSize changes the maximum layout width and height in logical units.
	SetSize(maxWidth, maxHeight float32)

	// SetTextAlignment changes the paragraph alignment within the layout width.
	SetTextAlignment(align TextAlignment)

	// SetWrapMode changes automatic line wrapping within the layout width.
	SetWrapMode(wrap WrapMode)

	// SetTextFont applies font to a non-empty range of Text. start and length are
	// UTF-8 byte offsets and must describe valid rune boundaries.
	SetTextFont(start, length int, font FontInfo)

	// SetTextColor applies c to a non-empty range of Text. start and length are
	// UTF-8 byte offsets and must describe valid rune boundaries.
	SetTextColor(start, length int, c color.Color)

	// SetUnderline enables or disables underlining for a non-empty range of Text.
	// start and length are UTF-8 byte offsets and must describe valid rune
	// boundaries.
	SetUnderline(start, length int, underline bool)

	// SetStrikethrough enables or disables strikethrough for a non-empty range of
	// Text. start and length are UTF-8 byte offsets and must describe valid rune
	// boundaries.
	SetStrikethrough(start, length int, strike bool)

	// MeasureSize returns the occupied width and height under the current layout
	// constraints and formatting, in logical units.
	MeasureSize() (width, height float32)

	// MeasureMetrics returns line and shaping-cluster metrics for the current
	// layout. Coordinates are relative to the layout origin and use logical
	// units; Start and Length values are UTF-8 byte offsets into Text.
	MeasureMetrics() (lines []TextLine, clusters []TextCluster)

	// Rasterize renders the layout at scale physical pixels per logical unit.
	// scale must be finite and greater than zero. The returned bitmap dimensions
	// and stride are in physical pixels, and its pixels are premultiplied RGBA.
	// An empty layout may produce an empty bitmap.
	//
	// When buf has sufficient capacity its storage may be reused, in which case
	// bitmap.Pixels aliases buf. Neither the layout nor its Context retains the
	// returned pixel slice.
	Rasterize(scale float32, buf []byte) (bitmap TextBitmap, err error)

	// ConnectChanged registers a listener for layout changes that affect
	// measurement or rendering. Notifications are synchronous, run on the
	// layout's thread, and occur before the mutating method returns. A no-op
	// update does not need to emit a notification.
	//
	// The caller owns the returned handle and should disconnect it when the
	// listener is no longer valid.
	ConnectChanged(func()) signal.Handle

	// ConnectDestroy registers a listener notified synchronously during the first
	// call to Destroy, before native layout resources are released. The callback
	// is intended for releasing data associated with the layout and should not
	// call back into it.
	//
	// The caller owns the returned handle and should disconnect it if the listener
	// becomes invalid before the layout is destroyed.
	ConnectDestroy(func()) signal.Handle
}

// TextLine describes one laid-out line in layout-local logical coordinates.
type TextLine struct {
	// Start is the UTF-8 byte offset of the line in TextLayout.Text.
	Start int
	// Length is the line's length in UTF-8 bytes.
	Length int
	// X is the horizontal coordinate of the line bounds' top-left corner.
	X float32
	// Y is the vertical coordinate of the line bounds' top-left corner.
	Y float32
	// Width is the width of the line bounds.
	Width float32
	// Height is the height of the line bounds.
	Height float32
	// Baseline is the vertical baseline coordinate relative to the layout origin.
	Baseline float32
	// Clusters contains the shaping clusters belonging to this line.
	Clusters []TextCluster
}

// TextCluster describes a backend shaping result associated with a text range,
// in layout-local logical coordinates. A cluster may contain multiple Unicode
// code points, UTF-8 bytes, or positioned glyphs.
type TextCluster struct {
	// Start is the UTF-8 byte offset of the cluster in TextLayout.Text.
	Start int
	// Length is the cluster's length in UTF-8 bytes.
	Length int
	// X is the horizontal coordinate of the cluster bounds' top-left corner.
	X float32
	// Y is the vertical coordinate of the cluster bounds' top-left corner.
	Y float32
	// Width is the width of the cluster bounds.
	Width float32
	// Height is the height of the cluster bounds.
	Height float32
	// LineIndex identifies the containing entry in MeasureMetrics' lines result.
	LineIndex int
	// Direction is the cluster's resolved text direction.
	Direction TextDirection
}

// TextDirection is the resolved writing direction of a shaping cluster.
type TextDirection int

const (
	// TextLeftToRight indicates left-to-right text.
	TextLeftToRight TextDirection = iota
	// TextRightToLeft indicates right-to-left text.
	TextRightToLeft
)

// TextAlignment controls horizontal paragraph alignment in the layout box.
type TextAlignment int

const (
	// TextAlignBegin aligns text to the beginning edge of the paragraph.
	TextAlignBegin TextAlignment = iota
	// TextAlignEnd aligns text to the ending edge of the paragraph.
	TextAlignEnd
	// TextAlignCenter centers text within the layout width.
	TextAlignCenter
	// TextAlignFill expands eligible lines toward both layout edges.
	TextAlignFill
)
