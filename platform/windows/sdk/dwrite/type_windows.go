package dwrite

type FactoryType uint32

const (
	/// <summary>
	/// Shared factory allow for re-use of cached font data across multiple in process components.
	/// Such factories also take advantage of cross process font caching components for better performance.
	/// </summary>
	DWRITE_FACTORY_TYPE_SHARED FactoryType = iota

	/// <summary>
	/// Objects created from the isolated factory do not interact with internal DirectWrite state from other components.
	/// </summary>
	DWRITE_FACTORY_TYPE_ISOLATED
)

type FontWeight uint32

const (
	/// <summary>
	/// Predefined font weight : Thin (100).
	/// </summary>
	DWRITE_FONT_WEIGHT_THIN FontWeight = 100

	/// <summary>
	/// Predefined font weight : Extra-light (200).
	/// </summary>
	DWRITE_FONT_WEIGHT_EXTRA_LIGHT FontWeight = 200

	/// <summary>
	/// Predefined font weight : Ultra-light (200).
	/// </summary>
	DWRITE_FONT_WEIGHT_ULTRA_LIGHT FontWeight = 200

	/// <summary>
	/// Predefined font weight : Light (300).
	/// </summary>
	DWRITE_FONT_WEIGHT_LIGHT FontWeight = 300

	/// <summary>
	/// Predefined font weight : Semi-light (350).
	/// </summary>
	DWRITE_FONT_WEIGHT_SEMI_LIGHT FontWeight = 350

	/// <summary>
	/// Predefined font weight : Normal (400).
	/// </summary>
	DWRITE_FONT_WEIGHT_NORMAL FontWeight = 400

	/// <summary>
	/// Predefined font weight : Regular (400).
	/// </summary>
	DWRITE_FONT_WEIGHT_REGULAR FontWeight = 400

	/// <summary>
	/// Predefined font weight : Medium (500).
	/// </summary>
	DWRITE_FONT_WEIGHT_MEDIUM FontWeight = 500

	/// <summary>
	/// Predefined font weight : Demi-bold (600).
	/// </summary>
	DWRITE_FONT_WEIGHT_DEMI_BOLD FontWeight = 600

	/// <summary>
	/// Predefined font weight : Semi-bold (600).
	/// </summary>
	DWRITE_FONT_WEIGHT_SEMI_BOLD FontWeight = 600

	/// <summary>
	/// Predefined font weight : Bold (700).
	/// </summary>
	DWRITE_FONT_WEIGHT_BOLD FontWeight = 700

	/// <summary>
	/// Predefined font weight : Extra-bold (800).
	/// </summary>
	DWRITE_FONT_WEIGHT_EXTRA_BOLD FontWeight = 800

	/// <summary>
	/// Predefined font weight : Ultra-bold (800).
	/// </summary>
	DWRITE_FONT_WEIGHT_ULTRA_BOLD FontWeight = 800

	/// <summary>
	/// Predefined font weight : Black (900).
	/// </summary>
	DWRITE_FONT_WEIGHT_BLACK FontWeight = 900

	/// <summary>
	/// Predefined font weight : Heavy (900).
	/// </summary>
	DWRITE_FONT_WEIGHT_HEAVY FontWeight = 900

	/// <summary>
	/// Predefined font weight : Extra-black (950).
	/// </summary>
	DWRITE_FONT_WEIGHT_EXTRA_BLACK FontWeight = 950

	/// <summary>
	/// Predefined font weight : Ultra-black (950).
	/// </summary>
	DWRITE_FONT_WEIGHT_ULTRA_BLACK FontWeight = 950
)

type FontStyle uint32

const (
	/// <summary>
	/// Font slope style : Normal.
	/// </summary>
	DWRITE_FONT_STYLE_NORMAL FontStyle = iota

	/// <summary>
	/// Font slope style : Oblique.
	/// </summary>
	DWRITE_FONT_STYLE_OBLIQUE

	/// <summary>
	/// Font slope style : Italic.
	/// </summary>
	DWRITE_FONT_STYLE_ITALIC
)

type FontStretch uint32

const (
	/// <summary>
	/// Predefined font stretch : Not known (0).
	/// </summary>
	DWRITE_FONT_STRETCH_UNDEFINED FontStretch = 0

	/// <summary>
	/// Predefined font stretch : Ultra-condensed (1).
	/// </summary>
	DWRITE_FONT_STRETCH_ULTRA_CONDENSED FontStretch = 1

	/// <summary>
	/// Predefined font stretch : Extra-condensed (2).
	/// </summary>
	DWRITE_FONT_STRETCH_EXTRA_CONDENSED FontStretch = 2

	/// <summary>
	/// Predefined font stretch : Condensed (3).
	/// </summary>
	DWRITE_FONT_STRETCH_CONDENSED FontStretch = 3

	/// <summary>
	/// Predefined font stretch : Semi-condensed (4).
	/// </summary>
	DWRITE_FONT_STRETCH_SEMI_CONDENSED FontStretch = 4

	/// <summary>
	/// Predefined font stretch : Normal (5).
	/// </summary>
	DWRITE_FONT_STRETCH_NORMAL FontStretch = 5

	/// <summary>
	/// Predefined font stretch : Medium (5).
	/// </summary>
	DWRITE_FONT_STRETCH_MEDIUM FontStretch = 5

	/// <summary>
	/// Predefined font stretch : Semi-expanded (6).
	/// </summary>
	DWRITE_FONT_STRETCH_SEMI_EXPANDED FontStretch = 6

	/// <summary>
	/// Predefined font stretch : Expanded (7).
	/// </summary>
	DWRITE_FONT_STRETCH_EXPANDED FontStretch = 7

	/// <summary>
	/// Predefined font stretch : Extra-expanded (8).
	/// </summary>
	DWRITE_FONT_STRETCH_EXTRA_EXPANDED FontStretch = 8

	/// <summary>
	/// Predefined font stretch : Ultra-expanded (9).
	/// </summary>
	DWRITE_FONT_STRETCH_ULTRA_EXPANDED FontStretch = 9
)

type TextAlignment uint32

const (
	/// <summary>
	/// The leading edge of the paragraph text is aligned to the layout box's leading edge.
	/// </summary>
	DWRITE_TEXT_ALIGNMENT_LEADING TextAlignment = iota

	/// <summary>
	/// The trailing edge of the paragraph text is aligned to the layout box's trailing edge.
	/// </summary>
	DWRITE_TEXT_ALIGNMENT_TRAILING

	/// <summary>
	/// The center of the paragraph text is aligned to the center of the layout box.
	/// </summary>
	DWRITE_TEXT_ALIGNMENT_CENTER

	/// <summary>
	/// Align text to the leading side, and also justify text to fill the lines.
	/// </summary>
	DWRITE_TEXT_ALIGNMENT_JUSTIFIED
)

type ParagraphAlignment uint32

const (
	/// <summary>
	/// The first line of paragraph is aligned to the flow's beginning edge of the layout box.
	/// </summary>
	DWRITE_PARAGRAPH_ALIGNMENT_NEAR ParagraphAlignment = iota

	/// <summary>
	/// The last line of paragraph is aligned to the flow's ending edge of the layout box.
	/// </summary>
	DWRITE_PARAGRAPH_ALIGNMENT_FAR

	/// <summary>
	/// The center of the paragraph is aligned to the center of the flow of the layout box.
	/// </summary>
	DWRITE_PARAGRAPH_ALIGNMENT_CENTER
)

type WordWrapping uint32

const (
	/// <summary>
	/// Words are broken across lines to avoid text overflowing the layout box.
	/// </summary>
	DWRITE_WORD_WRAPPING_WRAP WordWrapping = 0

	/// <summary>
	/// Words are kept within the same line even when it overflows the layout box.
	/// This option is often used with scrolling to reveal overflow text.
	/// </summary>
	DWRITE_WORD_WRAPPING_NO_WRAP WordWrapping = 1

	/// <summary>
	/// Words are broken across lines to avoid text overflowing the layout box.
	/// Emergency wrapping occurs if the word is larger than the maximum width.
	/// </summary>
	DWRITE_WORD_WRAPPING_EMERGENCY_BREAK WordWrapping = 2

	/// <summary>
	/// Only wrap whole words, never breaking words (emergency wrapping) when the
	/// layout width is too small for even a single word.
	/// </summary>
	DWRITE_WORD_WRAPPING_WHOLE_WORD WordWrapping = 3

	/// <summary>
	/// Wrap between any valid characters clusters.
	/// </summary>
	DWRITE_WORD_WRAPPING_CHARACTER WordWrapping = 4
)

type LineSpacingMethod uint32

const (
	/// <summary>
	/// Line spacing depends solely on the content, growing to accommodate the size of fonts and inline objects.
	/// </summary>
	DWRITE_LINE_SPACING_METHOD_DEFAULT LineSpacingMethod = iota

	/// <summary>
	/// Lines are explicitly set to uniform spacing, regardless of contained font sizes.
	/// This can be useful to avoid the uneven appearance that can occur from font fallback.
	/// </summary>
	DWRITE_LINE_SPACING_METHOD_UNIFORM

	/// <summary>
	/// Line spacing and baseline distances are proportional to the computed values based on the content, the size of the fonts and inline objects.
	/// </summary>
	DWRITE_LINE_SPACING_METHOD_PROPORTIONAL
)

type FontAxisTag uint32

const (
	DWRITE_FONT_AXIS_TAG_WEIGHT       FontAxisTag = 0x74686777
	DWRITE_FONT_AXIS_TAG_WIDTH        FontAxisTag = 0x68746477
	DWRITE_FONT_AXIS_TAG_SLANT        FontAxisTag = 0x746e6c73
	DWRITE_FONT_AXIS_TAG_OPTICAL_SIZE FontAxisTag = 0x7a73706f
	DWRITE_FONT_AXIS_TAG_ITALIC       FontAxisTag = 0x6c617469
)

type FontFaceType uint32

const (
	DWRITE_FONT_FACE_TYPE_CFF                 FontFaceType = 0
	DWRITE_FONT_FACE_TYPE_TRUETYPE            FontFaceType = 1
	DWRITE_FONT_FACE_TYPE_OPENTYPE_COLLECTION FontFaceType = 2
	DWRITE_FONT_FACE_TYPE_TYPE1               FontFaceType = 3
	DWRITE_FONT_FACE_TYPE_VECTOR              FontFaceType = 4
	DWRITE_FONT_FACE_TYPE_BITMAP              FontFaceType = 5
	DWRITE_FONT_FACE_TYPE_UNKNOWN             FontFaceType = 6
	DWRITE_FONT_FACE_TYPE_RAW_CFF             FontFaceType = 7
	DWRITE_FONT_FACE_TYPE_TRUETYPE_COLLECTION              = DWRITE_FONT_FACE_TYPE_OPENTYPE_COLLECTION
)

type MeasuringMode uint32

const (
	/// <summary>
	/// Text is measured using glyph ideal metrics whose values are independent to the current display resolution.
	/// </summary>
	DWRITE_MEASURING_MODE_NATURAL MeasuringMode = iota

	/// <summary>
	/// Text is measured using glyph display compatible metrics whose values tuned for the current display resolution.
	/// </summary>
	DWRITE_MEASURING_MODE_GDI_CLASSIC

	/// <summary>
	// Text is measured using the same glyph display metrics as text measured by GDI using a font
	// created with CLEARTYPE_NATURAL_QUALITY.
	/// </summary>
	DWRITE_MEASURING_MODE_GDI_NATURAL
)
