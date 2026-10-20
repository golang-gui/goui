package pango_cairo

import (
	"github.com/goexlib/cgo"
	"github.com/golang-gui/goui/platform/linux/libs/cairo"
	"github.com/golang-gui/goui/platform/linux/libs/pango"
)

var (
	libpangocairo = cgo.NewLazyLibrary("libpangocairo-1.0.so.0")

	pangoCairoCreateContext         = libpangocairo.NewSymbol("pango_cairo_create_context")
	pangoCairoFontMapGetDefault     = libpangocairo.NewSymbol("pango_cairo_font_map_get_default")
	pangoCairoFontMapNew            = libpangocairo.NewSymbol("pango_cairo_font_map_new")
	pangoCairoFontMapNewForFontType = libpangocairo.NewSymbol("pango_cairo_font_map_new_for_font_type")
	pangoCairoContextSetFontOptions = libpangocairo.NewSymbol("pango_cairo_context_set_font_options")
	pangoCairoUpdateContext         = libpangocairo.NewSymbol("pango_cairo_update_context")
	pangoCairoUpdateLayout          = libpangocairo.NewSymbol("pango_cairo_update_layout")
	pangoCairoShowLayout            = libpangocairo.NewSymbol("pango_cairo_show_layout")
	pangoCairoFontGetScaledFont     = libpangocairo.NewSymbol("pango_cairo_font_get_scaled_font")
	pangoCairoGlyphStringPath       = libpangocairo.NewSymbol("pango_cairo_glyph_string_path")
)

// CreateContext creates a PangoContext for the given Cairo context.
func CreateContext(cr cairo.Context) (c pango.Context) {
	// PangoContext* pango_cairo_create_context(cairo_t* cr)
	c.GObject, _, _ = pangoCairoCreateContext.CallRaw(uintptr(cr))
	return
}

// FontMapGetDefault gets a default PangoCairoFontMap to use with Cairo.
func FontMapGetDefault() (fm pango.FontMap) {
	// PangoFontMap* pango_cairo_font_map_get_default()
	fm.GObject, _, _ = pangoCairoFontMapGetDefault.CallRaw()
	return
}

func FontMapNew() (fm pango.FontMap) {
	fm.GObject, _, _ = pangoCairoFontMapNew.CallRaw()
	return
}

// FontMapNewForFontType creates a new PangoCairoFontMap object of the type suitable to be used with the given Cairo font backend.
// fontType must be cairo.FontTypeFT for FreeType-based font rendering.
func FontMapNewForFontType(fontType cairo.FontType) (fm pango.FontMap) {
	// PangoFontMap* pango_cairo_font_map_new_for_font_type(cairo_font_type_t fonttype)
	fm.GObject, _, _ = pangoCairoFontMapNewForFontType.CallRaw(uintptr(fontType))
	return
}

func UpdateLayout(cr cairo.Context, layout pango.Layout) {
	// void	pango_cairo_update_layout(cairo_t* cr, PangoLayout* layout)
	pangoCairoUpdateLayout.CallRaw(uintptr(cr), layout.GObject)
}

// ContextSetFontOptions copies options into the Pango context. A zero options
// value restores the defaults inherited from the Cairo target surface.
func ContextSetFontOptions(context pango.Context, options cairo.FontOptions) {
	// void pango_cairo_context_set_font_options(PangoContext*, const cairo_font_options_t*)
	pangoCairoContextSetFontOptions.CallRaw(context.GObject, uintptr(options))
}

func ShowLayout(cr cairo.Context, layout pango.Layout) {
	// void	pango_cairo_show_layout(cairo_t* cr, PangoLayout* layout)
	pangoCairoShowLayout.CallRaw(uintptr(cr), layout.GObject)
}

// FontScaledFont borrows the Cairo font owned by the supplied PangoCairo font.
func FontScaledFont(font pango.Font) cairo.ScaledFont {
	return cgo.CallRet[cairo.ScaledFont](pangoCairoFontGetScaledFont.Addr(), font)
}

// GlyphStringPath appends native glyph outlines (including missing-glyph boxes)
// at the current baseline. It does not change the current source color/alpha.
func GlyphStringPath(cr cairo.Context, font pango.Font, glyphs *pango.GlyphString) {
	cgo.Call(pangoCairoGlyphStringPath.Addr(), cr, font, glyphs)
}
