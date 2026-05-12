package pango_cairo

// Re-export opaque types as aliases for convenience
type (
	CairoContext uintptr // cairo_t*
	FontOptions  uintptr // cairo_font_options_t*
	FontType     int32   // cairo_font_type_t
)

const (
	FontTypeToy    FontType = 0
	FontTypeFT     FontType = 1
	FontTypeWin32  FontType = 2
	FontTypeQuartz FontType = 3
	FontTypeUser   FontType = 4
)
