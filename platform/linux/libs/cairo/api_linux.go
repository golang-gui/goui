package cairo

import "github.com/goexlib/cgo"

var (
	libcairo = cgo.NewLazyLibrary("libcairo.so.2")

	cairoImageSurfaceCreate          = libcairo.NewSymbol("cairo_image_surface_create")
	cairoImageSurfaceCreateForData   = libcairo.NewSymbol("cairo_image_surface_create_for_data")
	cairoSurfaceStatus               = libcairo.NewSymbol("cairo_surface_status")
	cairoSurfaceDestroy              = libcairo.NewSymbol("cairo_surface_destroy")
	cairoFormatStrideForWidth        = libcairo.NewSymbol("cairo_format_stride_for_width")
	cairoCreate                      = libcairo.NewSymbol("cairo_create")
	cairoStatus                      = libcairo.NewSymbol("cairo_status")
	cairoDestroy                     = libcairo.NewSymbol("cairo_destroy")
	cairoScale                       = libcairo.NewSymbol("cairo_scale")
	cairoSetSourceRgb                = libcairo.NewSymbol("cairo_set_source_rgb")
	cairoSetSourceRgba               = libcairo.NewSymbol("cairo_set_source_rgba")
	cairoPaint                       = libcairo.NewSymbol("cairo_paint")
	cairoMoveTo                      = libcairo.NewSymbol("cairo_move_to")
	cairoLineTo                      = libcairo.NewSymbol("cairo_line_to")
	cairoStroke                      = libcairo.NewSymbol("cairo_stroke")
	cairoSetLineWidth                = libcairo.NewSymbol("cairo_set_line_width")
	cairoSetFontOptions              = libcairo.NewSymbol("cairo_set_font_options")
	cairoFontOptionsCreate           = libcairo.NewSymbol("cairo_font_options_create")
	cairoFontOptionsDestroy          = libcairo.NewSymbol("cairo_font_options_destroy")
	cairoFontOptionsSetAntialias     = libcairo.NewSymbol("cairo_font_options_set_antialias")
	cairoFontOptionsSetSubpixelOrder = libcairo.NewSymbol("cairo_font_options_set_subpixel_order")
	cairoFontOptionsSetHintStyle     = libcairo.NewSymbol("cairo_font_options_set_hint_style")
	cairoFontOptionsSetHintMetrics   = libcairo.NewSymbol("cairo_font_options_set_hint_metrics")
	cairoStatusToString              = libcairo.NewSymbol("cairo_status_to_string")
)

// FormatStrideForWidth returns the stride for an image surface with the given format and width.
func FormatStrideForWidth(format Format, width int) int {
	// cairo_format_stride_for_width(cairo_format_t format, int width) -> int
	ret, _, _ := cairoFormatStrideForWidth.CallRaw(uintptr(format), uintptr(width))
	return int(ret)
}

// ImageSurfaceCreate creates an image surface of the specified format and dimensions.
func ImageSurfaceCreate(format Format, width, height int) Surface {
	// cairo_surface_t* cairo_image_surface_create(cairo_format_t format, int width, int height)
	ret, _, _ := cairoImageSurfaceCreate.CallRaw(uintptr(format), uintptr(width), uintptr(height))
	return Surface(ret)
}

// ImageSurfaceCreateForData creates an image surface for the provided pixel data.
func ImageSurfaceCreateForData(data []byte, format Format, width, height, stride int) Surface {
	// cairo_surface_t* cairo_image_surface_create_for_data(unsigned char* data, cairo_format_t format, int width, int height, int stride)
	ret, _, _ := cairoImageSurfaceCreateForData.CallRaw(uintptr(cgo.CSlice(data)), uintptr(format), uintptr(width), uintptr(height), uintptr(stride))
	return Surface(ret)
}

func (s Surface) Status() Status {
	ret, _, _ := cairoSurfaceStatus.CallRaw(uintptr(s))
	return Status(ret)
}

// Destroy decrements the reference count on surface.
func (s Surface) Destroy() {
	// void cairo_surface_destroy(cairo_surface_t* surface)
	cairoSurfaceDestroy.CallRaw(uintptr(s))
}

// Create creates a new cairo drawing context.
func Create(surface Surface) Context {
	// cairo_t* cairo_create(cairo_surface_t* target)
	ret, _, _ := cairoCreate.CallRaw(uintptr(surface))
	return Context(ret)
}

func (cr Context) Status() Status {
	ret, _, _ := cairoStatus.CallRaw(uintptr(cr))
	return Status(ret)
}

// Destroy decrements the reference count on the drawing context.
func (cr Context) Destroy() {
	// void cairo_destroy(cairo_t* cr)
	cairoDestroy.CallRaw(uintptr(cr))
}

func (cr Context) Scale(sx, sy float64) {
	cgo.Call(cairoScale.Addr(), uintptr(cr), sx, sy)
}

// SetSourceRGB sets the source pattern to an opaque color.
func (cr Context) SetSourceRGB(r, g, b float64) {
	// void cairo_set_source_rgb(cairo_t* cr, double red, double green, double blue)
	cgo.Call(cairoSetSourceRgb.Addr(), uintptr(cr), r, g, b)
}

// SetSourceRGBA sets the source pattern to a translucent color.
func (cr Context) SetSourceRGBA(r, g, b, a float64) {
	// void cairo_set_source_rgba(cairo_t* cr, double red, double green, double blue, double alpha)
	cgo.Call(cairoSetSourceRgba.Addr(), uintptr(cr), r, g, b, a)
}

// Paint paints the current source everywhere within the current clip region.
func (cr Context) Paint() {
	// void cairo_paint(cairo_t* cr)
	cairoPaint.CallRaw(uintptr(cr))
}

// MoveTo begins a new sub-path.
func (cr Context) MoveTo(x, y float64) {
	// void cairo_move_to(cairo_t* cr, double x, double y)
	cgo.Call(cairoMoveTo.Addr(), uintptr(cr), x, y)
}

// LineTo adds a line to the path from the current point to (x, y).
func (cr Context) LineTo(x, y float64) {
	// void cairo_line_to(cairo_t* cr, double x, double y)
	cgo.Call(cairoLineTo.Addr(), uintptr(cr), x, y)
}

// Stroke strokes the current path according to the current line width, line join, line cap, and dash settings.
func (cr Context) Stroke() {
	// void cairo_stroke(cairo_t* cr)
	cairoStroke.CallRaw(uintptr(cr))
}

// SetLineWidth sets the current line width within the cairo context.
func (cr Context) SetLineWidth(width float64) {
	// void cairo_set_line_width(cairo_t* cr, double width)
	cgo.Call(cairoSetLineWidth.Addr(), uintptr(cr), width)
}

// SetFontOptions sets a set of custom font rendering options for the cairo context.
func (cr Context) SetFontOptions(options FontOptions) {
	// void cairo_set_font_options(cairo_t* cr, const cairo_font_options_t* options)
	cairoSetFontOptions.CallRaw(uintptr(cr), uintptr(options))
}

// FontOptionsCreate allocates a new font options object with all options initialized to default values.
func FontOptionsCreate() FontOptions {
	// cairo_font_options_t* cairo_font_options_create()
	ret, _, _ := cairoFontOptionsCreate.CallRaw()
	return FontOptions(ret)
}

// Destroy destroys a FontOptions object.
func (fo FontOptions) Destroy() {
	// void cairo_font_options_destroy(cairo_font_options_t* options)
	cairoFontOptionsDestroy.CallRaw(uintptr(fo))
}

// SetAntialias sets the antialiasing mode of the font options object.
func (fo FontOptions) SetAntialias(antialias Antialias) {
	// void cairo_font_options_set_antialias(cairo_font_options_t* options, cairo_antialias_t antialias)
	cairoFontOptionsSetAntialias.CallRaw(uintptr(fo), uintptr(antialias))
}

// SetSubpixelOrder sets the subpixel order of the font options object.
func (fo FontOptions) SetSubpixelOrder(order SubpixelOrder) {
	// void cairo_font_options_set_subpixel_order(cairo_font_options_t* options, cairo_subpixel_order_t subpixel_order)
	cairoFontOptionsSetSubpixelOrder.CallRaw(uintptr(fo), uintptr(order))
}

// SetHintStyle sets the hint style of the font options object.
func (fo FontOptions) SetHintStyle(style HintStyle) {
	// void cairo_font_options_set_hint_style(cairo_font_options_t* options, cairo_hint_style_t hint_style)
	cairoFontOptionsSetHintStyle.CallRaw(uintptr(fo), uintptr(style))
}

// SetHintMetrics sets the metrics hinting mode of the font options object.
func (fo FontOptions) SetHintMetrics(metrics HintMetrics) {
	// void cairo_font_options_set_hint_metrics(cairo_font_options_t* options, cairo_hint_metrics_t hint_metrics)
	cairoFontOptionsSetHintMetrics.CallRaw(uintptr(fo), uintptr(metrics))
}

func (s Status) String() string {
	ret, _, _ := cairoStatusToString.CallRaw(uintptr(s))
	return cgo.GoStringTemp(cgo.Pointer(ret))
}
