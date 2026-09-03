package typography

import "image/color"

// Context owns the platform facilities used to create and rasterize text
// layouts. It is thread-affine and is not safe for concurrent use.
//
// The caller must destroy every TextLayout created by a Context before
// destroying the Context itself. Destroying a Context does not implicitly
// destroy those layouts.
type Context interface {
	// Name returns the diagnostic name of the platform typography backend.
	Name() string

	// Destroy releases the Context's native resources. It must be called exactly
	// once, after all layouts created by the Context have been destroyed. No
	// method may be called after Destroy.
	Destroy()

	// AddFont registers the font file for font-family resolution by layouts
	// created afterward. Registration scope and lifetime are platform-defined;
	// in particular, AddFont does not guarantee that existing layouts change.
	// It returns an error when the platform cannot register the file.
	AddFont(fontFile string) error

	// NewTextLayout creates a layout for text using format and maximum layout
	// constraints width and height. The constraints are in logical units. The
	// text is fixed for the lifetime of the returned layout, while its constraints
	// and formatting may be changed through TextLayout methods.
	//
	// The caller owns the returned layout and must destroy it before the Context.
	NewTextLayout(text string, format TextFormat, width, height float32) (TextLayout, error)
}

// TextFormat describes the base formatting and paragraph behavior of a
// TextLayout. Range formatting applied later is stored by the layout and is not
// reflected back into this value.
type TextFormat struct {
	// Font is the base font applied to the entire text.
	Font FontInfo
	// WrapMode controls automatic line wrapping within the layout width.
	WrapMode WrapMode
	// TextAlign controls horizontal alignment within the layout width.
	TextAlign TextAlignment
	// TextColor is the base foreground color. A nil value uses
	// DefaultTextColor.
	TextColor color.Color
}

// FontInfo describes a requested font face. Font matching is performed by the
// platform typography backend, so the selected face may be a fallback.
type FontInfo struct {
	// Family is the platform font-family name.
	Family string
	// Size is the font size in typographic points (1/72 inch).
	Size float32
	// Weight requests a platform-specific font weight. Zero uses the backend's
	// default, and backends that do not support the value may ignore it.
	Weight float32
	// Width requests a platform-specific font width. Zero uses the backend's
	// default, and backends that do not support the value may ignore it.
	Width float32
}

// WrapMode controls where a TextLayout may insert automatic line breaks.
type WrapMode int

const (
	// WrapNone disables automatic line wrapping.
	WrapNone WrapMode = iota
	// WrapChar permits wrapping between characters.
	WrapChar
	// WrapWordChar prefers word boundaries and falls back to character
	// boundaries when a word does not fit.
	WrapWordChar
)

// DefaultTextColor returns the color used when TextFormat.TextColor is nil.
func DefaultTextColor() color.Color {
	return color.Black
}
