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

	// AddFont makes the file's font families available to layouts created afterward.
	// Duplicate registration is idempotent. Failed registration leaves existing
	// fonts available. Scope is platform-defined; existing layouts need not rematch.
	// The caller keeps the file accessible while its fonts are in use.
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
	// TabWidth is the repeating tab interval in logical units. Zero preserves
	// the backend default; positive values must be finite and at most 1048575.
	TabWidth float32
}

// FontInfo describes a requested font face. Font matching is performed by the
// platform typography backend, so the selected face may be a fallback.
type FontInfo struct {
	// Family is the font-family name; empty requests the platform default.
	Family string
	// Size is the font size in typographic points (1/72 inch). Zero requests 12 pt;
	// otherwise it must be finite and positive.
	Size float32
	// Weight requests a weight from 100 through 900; zero means Regular.
	Weight FontWeight
	// Width requests a standard width class; zero means Normal.
	Width FontWidth
	// Style requests an upright, italic or oblique face, not a painter transform.
	Style FontStyle
}

// FontWeight is an OpenType-style weight request. Zero defaults to Regular;
// otherwise values from 100 to 900 are valid, including intermediate weights.
type FontWeight uint16

const (
	FontWeightThin       FontWeight = 100
	FontWeightExtraLight FontWeight = 200
	FontWeightLight      FontWeight = 300
	FontWeightRegular    FontWeight = 400
	FontWeightMedium     FontWeight = 500
	FontWeightSemibold   FontWeight = 600
	FontWeightBold       FontWeight = 700
	FontWeightExtraBold  FontWeight = 800
	FontWeightBlack      FontWeight = 900
)

// FontWidth is a standard width class; zero defaults to Normal.
type FontWidth uint8

const (
	FontWidthUltraCondensed FontWidth = iota + 1
	FontWidthExtraCondensed
	FontWidthCondensed
	FontWidthSemiCondensed
	FontWidthNormal
	FontWidthSemiExpanded
	FontWidthExpanded
	FontWidthExtraExpanded
	FontWidthUltraExpanded
)

// FontStyle requests a native face's slant. Missing faces may use the closest
// native match; no painter transform is used to simulate a slant.
type FontStyle uint8

const (
	FontStyleNormal FontStyle = iota
	FontStyleItalic
	FontStyleOblique
)

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
