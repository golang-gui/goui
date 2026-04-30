package d2d1

import "github.com/golang-gui/goui/platform/win32/sdk/dxgi"

type (
	Enum = uint32
	Tag  = uint64
)

type Point2F struct {
	X, Y float32
}

type SizeF struct {
	Width, Height float32
}

type SizeU struct {
	Width, Height uint32
}

type ColorF struct {
	R, G, B, A float32 // D3DCOLORVALUE
}

type RectF struct {
	Left, Top, Right, Bottom float32
}

type RoundRect struct {
	Rect    RectF
	RadiusX float32
	RadiusY float32
}

type Ellipse struct {
	Point   Point2F
	RadiusX float32
	RadiusY float32
}

type Matrix3x2F struct {
	M [2][3]float32
}

type FactoryType uint32

const (
	D2D1_FACTORY_TYPE_SINGLE_THREADED FactoryType = 0
	D2D1_FACTORY_TYPE_MULTI_THREADED  FactoryType = 1
)

type FactoryOptions struct {
	DebugLevel DebugLevel
}

type DebugLevel uint32

const (
	D2D1_DEBUG_LEVEL_NONE        DebugLevel = 0
	D2D1_DEBUG_LEVEL_ERROR       DebugLevel = 1
	D2D1_DEBUG_LEVEL_WARNING     DebugLevel = 2
	D2D1_DEBUG_LEVEL_INFORMATION DebugLevel = 3
)

type FeatureLevel uint32

const (
	D2D1_FEATURE_LEVEL_DEFAULT FeatureLevel = 0
	D2D1_FEATURE_LEVEL_9       FeatureLevel = 0x9100 // D3D_FEATURE_LEVEL_9_1,
	D2D1_FEATURE_LEVEL_10      FeatureLevel = 0xa000 // D3D_FEATURE_LEVEL_10_0,

)

type RenderTargetProperties struct {
	Type        RenderTargetType
	PixelFormat PixelFormat
	DpiX        float32
	DpiY        float32
	Usage       RenderTargetUsage
	MinLevel    FeatureLevel
}

type RenderTargetType uint32

const (
	D2D1_RENDER_TARGET_TYPE_DEFAULT  RenderTargetType = 0
	D2D1_RENDER_TARGET_TYPE_SOFTWARE RenderTargetType = 1
	D2D1_RENDER_TARGET_TYPE_HARDWARE RenderTargetType = 2
)

type RenderTargetUsage uint32

const (
	D2D1_RENDER_TARGET_USAGE_NONE                  RenderTargetUsage = 0x00000000
	D2D1_RENDER_TARGET_USAGE_FORCE_BITMAP_REMOTING RenderTargetUsage = 0x00000001
	D2D1_RENDER_TARGET_USAGE_GDI_COMPATIBLE        RenderTargetUsage = 0x00000002
)

type PixelFormat struct {
	Format    dxgi.Format
	AlphaMode AlphaMode
}

type AlphaMode uint32

const (
	D2D1_ALPHA_MODE_UNKNOWN       AlphaMode = 0
	D2D1_ALPHA_MODE_PREMULTIPLIED AlphaMode = 1
	D2D1_ALPHA_MODE_STRAIGHT      AlphaMode = 2
	D2D1_ALPHA_MODE_IGNORE        AlphaMode = 3
)

type HwndRenderTargetProperties struct {
	Hwnd           uintptr
	PixelSize      SizeU
	PresentOptions PresentOptions
}

type PresentOptions uint32

const (
	D2D1_PRESENT_OPTIONS_NONE            PresentOptions = 0x00000000
	D2D1_PRESENT_OPTIONS_RETAIN_CONTENTS PresentOptions = 0x00000001
	D2D1_PRESENT_OPTIONS_IMMEDIATELY     PresentOptions = 0x00000002
)

type BrushProperties struct {
	Opacity   float32
	Transform Matrix3x2F
}

type BezierSegment struct {
	Point1 Point2F
	Point2 Point2F
	Point3 Point2F
}

type ArcSegment struct {
	Point          Point2F
	Size           SizeF
	RotationAngle  float32
	SweepDirection SweepDirection
	ArcSize        ArcSize
}

type ArcSize uint32

const (
	D2D1_ARC_SIZE_SMALL ArcSize = 0
	D2D1_ARC_SIZE_LARGE ArcSize = 1
)

type SweepDirection uint32

const (
	D2D1_SWEEP_DIRECTION_COUNTER_CLOCKWISE SweepDirection = 0
	D2D1_SWEEP_DIRECTION_CLOCKWISE         SweepDirection = 1
)

type StrokeStyleProperties struct {
	StartCap   CapStyle
	EndCap     CapStyle
	DashCap    CapStyle
	LineJoin   LineJoin
	MiterLimit float32
	DashStyle  DashStyle
	DashOffset float32
}

type WindowState uint32

const (
	D2D1_WINDOW_STATE_NONE     WindowState = 0x0000000
	D2D1_WINDOW_STATE_OCCLUDED WindowState = 0x0000001
)

type FigureBegin uint32

const (
	D2D1_FIGURE_BEGIN_FILLED FigureBegin = 0
	D2D1_FIGURE_BEGIN_HOLLOW FigureBegin = 1
)

type FigureEnd uint32

const (
	D2D1_FIGURE_END_OPEN   FigureEnd = 0
	D2D1_FIGURE_END_CLOSED FigureEnd = 1
)

type CapStyle uint32

const (
	/// <summary>
	/// Flat line cap.
	/// </summary>
	D2D1_CAP_STYLE_FLAT CapStyle = 0

	/// <summary>
	/// Square line cap.
	/// </summary>
	D2D1_CAP_STYLE_SQUARE CapStyle = 1

	/// <summary>
	/// Round line cap.
	/// </summary>
	D2D1_CAP_STYLE_ROUND CapStyle = 2

	/// <summary>
	/// Triangle line cap.
	/// </summary>
	D2D1_CAP_STYLE_TRIANGLE CapStyle = 3
)

type LineJoin uint32

const (
	/// <summary>
	/// Miter join.
	/// </summary>
	D2D1_LINE_JOIN_MITER LineJoin = 0

	/// <summary>
	/// Bevel join.
	/// </summary>
	D2D1_LINE_JOIN_BEVEL LineJoin = 1

	/// <summary>
	/// Round join.
	/// </summary>
	D2D1_LINE_JOIN_ROUND LineJoin = 2

	/// <summary>
	/// Miter/Bevel join.
	/// </summary>
	D2D1_LINE_JOIN_MITER_OR_BEVEL LineJoin = 3
)

type DashStyle uint32

const (
	D2D1_DASH_STYLE_SOLID        DashStyle = 0
	D2D1_DASH_STYLE_DASH         DashStyle = 1
	D2D1_DASH_STYLE_DOT          DashStyle = 2
	D2D1_DASH_STYLE_DASH_DOT     DashStyle = 3
	D2D1_DASH_STYLE_DASH_DOT_DOT DashStyle = 4
	D2D1_DASH_STYLE_CUSTOM       DashStyle = 5
)

type DrawTextOptions uint32

const (
	/// <summary>
	/// Do not snap the baseline of the text vertically.
	/// </summary>
	D2D1_DRAW_TEXT_OPTIONS_NO_SNAP DrawTextOptions = 0x00000001

	/// <summary>
	/// Clip the text to the content bounds.
	/// </summary>
	D2D1_DRAW_TEXT_OPTIONS_CLIP DrawTextOptions = 0x00000002

	/// <summary>
	/// Render color versions of glyphs if defined by the font.
	/// </summary>
	D2D1_DRAW_TEXT_OPTIONS_ENABLE_COLOR_FONT DrawTextOptions = 0x00000004

	/// <summary>
	/// Bitmap origins of color glyph bitmaps are not snapped.
	/// </summary>
	D2D1_DRAW_TEXT_OPTIONS_DISABLE_COLOR_BITMAP_SNAPPING DrawTextOptions = 0x00000008
	D2D1_DRAW_TEXT_OPTIONS_NONE                          DrawTextOptions = 0x00000000
	D2D1_DRAW_TEXT_OPTIONS_FORCE_DWORD                   DrawTextOptions = 0xffffffff
)

type BitmapProperties struct {
	PixelFormat PixelFormat
	DpiX        float32
	DpiY        float32
}

type BitmapInterpolationMode uint32

const (
	/// <summary>
	/// Nearest Neighbor filtering. Also known as nearest pixel or nearest point
	/// sampling.
	/// </summary>
	D2D1_BITMAP_INTERPOLATION_MODE_NEAREST_NEIGHBOR = BitmapInterpolationMode(D2D1_INTERPOLATION_MODE_DEFINITION_NEAREST_NEIGHBOR)

	/// <summary>
	/// Linear filtering.
	/// </summary>
	D2D1_BITMAP_INTERPOLATION_MODE_LINEAR = BitmapInterpolationMode(D2D1_INTERPOLATION_MODE_DEFINITION_LINEAR)
)

// / <summary>
// / This defines the superset of interpolation mode supported by D2D APIs
// / and built-in effects
// / </summary>
const (
	D2D1_INTERPOLATION_MODE_DEFINITION_NEAREST_NEIGHBOR    = 0
	D2D1_INTERPOLATION_MODE_DEFINITION_LINEAR              = 1
	D2D1_INTERPOLATION_MODE_DEFINITION_CUBIC               = 2
	D2D1_INTERPOLATION_MODE_DEFINITION_MULTI_SAMPLE_LINEAR = 3
	D2D1_INTERPOLATION_MODE_DEFINITION_ANISOTROPIC         = 4
	D2D1_INTERPOLATION_MODE_DEFINITION_HIGH_QUALITY_CUBIC  = 5
	D2D1_INTERPOLATION_MODE_DEFINITION_FANT                = 6
	D2D1_INTERPOLATION_MODE_DEFINITION_MIPMAP_LINEAR       = 7
)
