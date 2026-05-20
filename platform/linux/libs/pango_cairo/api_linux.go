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

func ShowLayout(cr cairo.Context, layout pango.Layout) {
	// void	pango_cairo_show_layout(cairo_t* cr, PangoLayout* layout)
	pangoCairoShowLayout.CallRaw(uintptr(cr), layout.GObject)
}
