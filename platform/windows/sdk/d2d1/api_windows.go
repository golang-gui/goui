package d2d1

import (
	"runtime"
	"syscall"

	"github.com/goexlib/cgo"
	"github.com/golang-gui/goui/platform/windows/sdk/com"
	"github.com/golang-gui/goui/platform/windows/sdk/dwrite"
	"github.com/golang-gui/goui/platform/windows/sdk/dxgi"
	"github.com/golang-gui/goui/platform/windows/sdk/wic"
)

var (
	d2d1              = cgo.NewLazyLibrary("d2d1.dll")
	d2d1CreateFactory = d2d1.NewSymbol("D2D1CreateFactory")
)

var IID_ID2D1Factory = com.DefineGuid(0x06152247, 0x6f50, 0x465a, 0x92, 0x45, 0x11, 0x8b, 0xfd, 0x3b, 0x60, 0x07)

func CreateFactory[F isFactory](factoryType FactoryType, iid com.IID, factoryOptions *FactoryOptions) (factory *F, err error) {
	if err = d2d1CreateFactory.Find(); err == nil {
		ret, _, _ := d2d1CreateFactory.CallRaw(uintptr(factoryType), uintptr(cgo.Pointer(&iid)), uintptr(cgo.Pointer(factoryOptions)), uintptr(cgo.Pointer(&factory)))
		if com.HRESULT(ret).Failed() {
			err = com.HRESULT(ret)
		}
	}
	return
}

type isFactory interface {
	isFactory()
}

type FactoryClass struct {
	com.UnknownClass

	ReloadSystemMetrics            cgo.Symbol //HRESULT(ID2D1Factory *This) PURE;
	GetDesktopDpi                  cgo.Symbol //void(ID2D1Factory *This, FLOAT *dpiX, FLOAT *dpiY) PURE;
	CreateRectangleGeometry        cgo.Symbol //HRESULT(ID2D1Factory *This, const D2D1_RECT_F *rectangle, ID2D1RectangleGeometry **rectangleGeometry) PURE;
	CreateRoundedRectangleGeometry cgo.Symbol //HRESULT(ID2D1Factory *This, const D2D1_ROUNDED_RECT *roundedRectangle, ID2D1RoundedRectangleGeometry **roundedRectangleGeometry) PURE;
	CreateEllipseGeometry          cgo.Symbol //HRESULT(ID2D1Factory *This, const D2D1_ELLIPSE *ellipse, ID2D1EllipseGeometry **ellipseGeometry) PURE;
	CreateGeometryGroup            cgo.Symbol //HRESULT(ID2D1Factory *This, D2D1_FILL_MODE fillMode, ID2D1Geometry **geometries, UINT geometriesCount, ID2D1GeometryGroup **geometryGroup) PURE;
	CreateTransformedGeometry      cgo.Symbol //HRESULT(ID2D1Factory *This, ID2D1Geometry *sourceGeometry, const D2D1_MATRIX_3X2_F *transform, ID2D1TransformedGeometry **transformedGeometry) PURE;
	CreatePathGeometry             cgo.Symbol //HRESULT(ID2D1Factory *This, ID2D1PathGeometry **pathGeometry) PURE;
	CreateStrokeStyle              cgo.Symbol //HRESULT(ID2D1Factory *This, const D2D1_STROKE_STYLE_PROPERTIES *strokeStyleProperties, const FLOAT *dashes, UINT dashesCount, ID2D1StrokeStyle **strokeStyle) PURE;
	CreateDrawingStateBlock        cgo.Symbol //HRESULT(ID2D1Factory *This, const D2D1_DRAWING_STATE_DESCRIPTION *drawingStateDescription, IDWriteRenderingParams *textRenderingParams, ID2D1DrawingStateBlock **drawingStateBlock) PURE;
	CreateWicBitmapRenderTarget    cgo.Symbol //HRESULT(ID2D1Factory *This, IWICBitmap *target, const D2D1_RENDER_TARGET_PROPERTIES *renderTargetProperties, ID2D1RenderTarget **renderTarget) PURE;
	CreateHwndRenderTarget         cgo.Symbol //HRESULT(ID2D1Factory *This, const D2D1_RENDER_TARGET_PROPERTIES *renderTargetProperties, const D2D1_HWND_RENDER_TARGET_PROPERTIES *hwndRenderTargetProperties, ID2D1HwndRenderTarget **hwndRenderTarget) PURE;
	CreateDxgiSurfaceRenderTarget  cgo.Symbol //HRESULT(ID2D1Factory *This, IDXGISurface *dxgiSurface, const D2D1_RENDER_TARGET_PROPERTIES *renderTargetProperties, ID2D1RenderTarget **renderTarget) PURE;
	CreateDCRenderTarget           cgo.Symbol //HRESULT(ID2D1Factory *This, const D2D1_RENDER_TARGET_PROPERTIES *renderTargetProperties, ID2D1DCRenderTarget **dcRenderTarget) PURE;
}

type Factory struct {
	com.Unknown
}

func (this Factory) isFactory() {}

func (this *Factory) CreateHwndRenderTarget(renderTargetProperties *RenderTargetProperties, hwndRenderTargetProperties *HwndRenderTargetProperties) (hwndRenderTarget *HwndRenderTarget, hr com.HRESULT) {
	ret, _, _ := this.class().CreateHwndRenderTarget.CallRaw(uintptr(cgo.Pointer(this)), uintptr(cgo.Pointer(renderTargetProperties)), uintptr(cgo.Pointer(hwndRenderTargetProperties)), uintptr(cgo.Pointer(&hwndRenderTarget)))
	hr = com.HRESULT(ret)
	return
}

func (this *Factory) CreateWicBitmapRenderTarget(target *wic.Bitmap, props *RenderTargetProperties) (renderTarget *RenderTarget, hr com.HRESULT) {
	ret, _, _ := this.class().CreateWicBitmapRenderTarget.CallRaw(uintptr(cgo.Pointer(this)), uintptr(cgo.Pointer(target)), uintptr(cgo.Pointer(props)), uintptr(cgo.Pointer(&renderTarget)))
	hr = com.HRESULT(ret)
	return
}

func (this *Factory) CreatePathGeometry() (pathGeometry *PathGeometry, hr com.HRESULT) {
	ret, _, _ := this.class().CreatePathGeometry.CallRaw(uintptr(cgo.Pointer(this)), uintptr(cgo.Pointer(&pathGeometry)))
	hr = com.HRESULT(ret)
	return
}

func (this *Factory) CreateStrokeStyle(props *StrokeStyleProperties, dashs []float32) (style *StrokeStyle, hr com.HRESULT) {
	ret, _, _ := this.class().CreateStrokeStyle.CallRaw(uintptr(cgo.Pointer(this)), uintptr(cgo.Pointer(props)), uintptr(cgo.CSlice(dashs)), uintptr(len(dashs)), uintptr(cgo.Pointer(&style)))
	hr = com.HRESULT(ret)
	return
}

func (this *Factory) class() *FactoryClass {
	return (*FactoryClass)(this.Class)
}

var IID_ID2D1Factory1 = com.DefineGuid(0xbb12d362, 0xdaee, 0x4b9a, 0xaa, 0x1d, 0x14, 0xba, 0x40, 0x1c, 0xfa, 0x1f)
var CLSID_D2D1Shadow = com.DefineGuid(0xc67ea361, 0x1863, 0x4e69, 0x89, 0xdb, 0x69, 0x5d, 0x3e, 0x9a, 0x5b, 0x6b)

type Factory1Class struct {
	FactoryClass

	CreateDevice             cgo.Symbol //HRESULT(ID2D1Factory1 *This, IDXGIDevice *dxgiDevice,	ID2D1Device **d2dDevice) PURE
	CreateStrokeStyle        cgo.Symbol //HRESULT(ID2D1Factory1 *This,	CONST D2D1_STROKE_STYLE_PROPERTIES1 *strokeStyleProperties,	CONST FLOAT *dashes, UINT32 dashesCount,	ID2D1StrokeStyle1 **strokeStyle) PURE
	CreatePathGeometry       cgo.Symbol //HRESULT(ID2D1Factory1 *This,	ID2D1PathGeometry1 **pathGeometry) PURE
	CreateDrawingStateBlock  cgo.Symbol //HRESULT(ID2D1Factory1 *This,	CONST D2D1_DRAWING_STATE_DESCRIPTION1 *drawingStateDescription,	IDWriteRenderingParams *textRenderingParams,	ID2D1DrawingStateBlock1 **drawingStateBlock) PURE
	CreateGdiMetafile        cgo.Symbol //HRESULT(ID2D1Factory1 *This, IStream *metafileStream,	ID2D1GdiMetafile **metafile) PURE
	RegisterEffectFromStream cgo.Symbol //HRESULT(ID2D1Factory1 *This, REFCLSID classId,	IStream *propertyXml, CONST D2D1_PROPERTY_BINDING *bindings,	UINT32 bindingsCount,	CONST PD2D1_EFFECT_FACTORY effectFactory) PURE
	RegisterEffectFromString cgo.Symbol //HRESULT(ID2D1Factory1 *This,	REFCLSID classId, PCWSTR propertyXml,	CONST D2D1_PROPERTY_BINDING *bindings, UINT32 bindingsCount,	CONST PD2D1_EFFECT_FACTORY effectFactory) PURE
	UnregisterEffect         cgo.Symbol //HRESULT(ID2D1Factory1 *This, REFCLSID classId) PURE
	GetRegisteredEffects     cgo.Symbol //HRESULT(ID2D1Factory1 *This, CLSID *effects,	UINT32 effectsCount, UINT32 *effectsReturned,	UINT32 *effectsRegistered) PURE
	GetEffectProperties      cgo.Symbol //HRESULT(ID2D1Factory1 *This, REFCLSID effectId,	ID2D1Properties **properties) PURE
}

type Factory1 struct {
	Factory
}

func (this *Factory1) CreateDevice(dxgiDevice *dxgi.Device) (device *Device, hr com.HRESULT) {
	ret, _, _ := this.class().CreateDevice.CallRaw(uintptr(cgo.Pointer(this)), uintptr(cgo.Pointer(dxgiDevice)), uintptr(cgo.Pointer(&device)))
	return device, com.HRESULT(ret)
}

func (this *Factory1) class() *Factory1Class {
	return (*Factory1Class)(this.Class)
}

type ResourceClass struct {
	com.UnknownClass
	GetFactory cgo.Symbol // void(ID2D1Resource *This, ID2D1Factory **factory)
}

type Resource struct {
	com.Unknown
}

func (this *Resource) GetFactory() (factory *Factory) {
	(*ResourceClass)(this.Class).GetFactory.CallRaw(uintptr(cgo.Pointer(this)), uintptr(cgo.Pointer(&factory)))
	return
}

type RenderTargetClass struct {
	ResourceClass

	CreateBitmap                 cgo.Symbol // HRESULT(ID2D1RenderTarget *This, D2D1_SIZE_U size, const void *srcData, UINT32 pitch, const D2D1_BITMAP_PROPERTIES *bitmapProperties, ID2D1Bitmap **bitmap) PURE;
	CreateBitmapFromWicBitmap    cgo.Symbol // HRESULT(ID2D1RenderTarget *This, IWICBitmapSource *wicBitmapSource, const D2D1_BITMAP_PROPERTIES *bitmapProperties, ID2D1Bitmap **bitmap) PURE;
	CreateSharedBitmap           cgo.Symbol // HRESULT(ID2D1RenderTarget *This, REFIID riid, void *data, const D2D1_BITMAP_PROPERTIES *bitmapProperties, ID2D1Bitmap **bitmap) PURE;
	CreateBitmapBrush            cgo.Symbol // HRESULT(ID2D1RenderTarget *This, ID2D1Bitmap *bitmap, const D2D1_BITMAP_BRUSH_PROPERTIES *bitmapBrushProperties, const D2D1_BRUSH_PROPERTIES *brushProperties, ID2D1BitmapBrush **bitmapBrush) PURE;
	CreateSolidColorBrush        cgo.Symbol // HRESULT(ID2D1RenderTarget *This, const D2D1_COLOR_F *color, const D2D1_BRUSH_PROPERTIES *brushProperties, ID2D1SolidColorBrush **solidColorBrush) PURE;
	CreateGradientStopCollection cgo.Symbol // HRESULT(ID2D1RenderTarget *This, const D2D1_GRADIENT_STOP *gradientStops, UINT gradientStopsCount, D2D1_GAMMA colorInterpolationGamma, D2D1_EXTEND_MODE extendMode, ID2D1GradientStopCollection **gradientStopCollection) PURE;
	CreateLinearGradientBrush    cgo.Symbol // HRESULT(ID2D1RenderTarget *This, const D2D1_LINEAR_GRADIENT_BRUSH_PROPERTIES *linearGradientBrushProperties, const D2D1_BRUSH_PROPERTIES *brushProperties, ID2D1GradientStopCollection *gradientStopCollection, ID2D1LinearGradientBrush **linearGradientBrush) PURE;
	CreateRadialGradientBrush    cgo.Symbol // HRESULT(ID2D1RenderTarget *This, const D2D1_RADIAL_GRADIENT_BRUSH_PROPERTIES *radialGradientBrushProperties, const D2D1_BRUSH_PROPERTIES *brushProperties, ID2D1GradientStopCollection *gradientStopCollection, ID2D1RadialGradientBrush **radialGradientBrush) PURE;
	CreateCompatibleRenderTarget cgo.Symbol // HRESULT(ID2D1RenderTarget *This, const D2D1_SIZE_F *desiredSize, const D2D1_SIZE_U *desiredPixelSize, const D2D1_PIXEL_FORMAT *desiredFormat, D2D1_COMPATIBLE_RENDER_TARGET_OPTIONS options, ID2D1BitmapRenderTarget **bitmapRenderTarget) PURE;
	CreateLayer                  cgo.Symbol // HRESULT(ID2D1RenderTarget *This, const D2D1_SIZE_F *size, ID2D1Layer **layer) PURE;
	CreateMesh                   cgo.Symbol // HRESULT(ID2D1RenderTarget *This, ID2D1Mesh **mesh) PURE;
	DrawLine                     cgo.Symbol //void(ID2D1RenderTarget *This, D2D1_POINT_2F point0, D2D1_POINT_2F point1, ID2D1Brush *brush, FLOAT strokeWidth, ID2D1StrokeStyle *strokeStyle) PURE;
	DrawRectangle                cgo.Symbol //void(ID2D1RenderTarget *This, const D2D1_RECT_F *rect, ID2D1Brush *brush, FLOAT strokeWidth, ID2D1StrokeStyle *strokeStyle) PURE;
	FillRectangle                cgo.Symbol //void(ID2D1RenderTarget *This, const D2D1_RECT_F *rect, ID2D1Brush *brush) PURE;
	DrawRoundedRectangle         cgo.Symbol //void(ID2D1RenderTarget *This, const D2D1_ROUNDED_RECT *roundedRect, ID2D1Brush *brush, FLOAT strokeWidth, ID2D1StrokeStyle *strokeStyle) PURE;
	FillRoundedRectangle         cgo.Symbol //void(ID2D1RenderTarget *This, const D2D1_ROUNDED_RECT *roundedRect, ID2D1Brush *brush) PURE;
	DrawEllipse                  cgo.Symbol //void(ID2D1RenderTarget *This, const D2D1_ELLIPSE *ellipse, ID2D1Brush *brush, FLOAT strokeWidth, ID2D1StrokeStyle *strokeStyle) PURE;
	FillEllipse                  cgo.Symbol //void(ID2D1RenderTarget *This, const D2D1_ELLIPSE *ellipse, ID2D1Brush *brush) PURE;
	DrawGeometry                 cgo.Symbol //void(ID2D1RenderTarget *This, ID2D1Geometry *geometry, ID2D1Brush *brush, FLOAT strokeWidth, ID2D1StrokeStyle *strokeStyle) PURE;
	FillGeometry                 cgo.Symbol //void(ID2D1RenderTarget *This, ID2D1Geometry *geometry, ID2D1Brush *brush, ID2D1Brush *opacityBrush) PURE;
	FillMesh                     cgo.Symbol //void(ID2D1RenderTarget *This, ID2D1Mesh *mesh, ID2D1Brush *brush) PURE;
	FillOpacityMask              cgo.Symbol //void(ID2D1RenderTarget *This, ID2D1Bitmap *opacityMask, ID2D1Brush *brush, D2D1_OPACITY_MASK_CONTENT content, const D2D1_RECT_F *destinationRectangle, const D2D1_RECT_F *sourceRectangle) PURE;
	DrawBitmap                   cgo.Symbol //void(ID2D1RenderTarget *This, ID2D1Bitmap *bitmap, const D2D1_RECT_F *destinationRectangle, FLOAT opacity, D2D1_BITMAP_INTERPOLATION_MODE interpolationMode, const D2D1_RECT_F *sourceRectangle) PURE;
	DrawText                     cgo.Symbol //void(ID2D1RenderTarget *This, const WCHAR *string, UINT stringLength, IDWriteTextFormat *textFormat, const D2D1_RECT_F *layoutRect, ID2D1Brush *defaultForegroundBrush, D2D1_DRAW_TEXT_OPTIONS options, DWRITE_MEASURING_MODE measuringMode) PURE;
	DrawTextLayout               cgo.Symbol //void(ID2D1RenderTarget *This, D2D1_POINT_2F origin, IDWriteTextLayout *textLayout, ID2D1Brush *defaultForegroundBrush, D2D1_DRAW_TEXT_OPTIONS options) PURE;
	DrawGlyphRun                 cgo.Symbol //void(ID2D1RenderTarget *This, D2D1_POINT_2F baselineOrigin, const DWRITE_GLYPH_RUN *glyphRun, ID2D1Brush *foregroundBrush, DWRITE_MEASURING_MODE measuringMode) PURE;
	SetTransform                 cgo.Symbol //void(ID2D1RenderTarget *This, const D2D1_MATRIX_3X2_F *transform) PURE;
	GetTransform                 cgo.Symbol //void(ID2D1RenderTarget *This, D2D1_MATRIX_3X2_F *transform) PURE;
	SetAntialiasMode             cgo.Symbol //void(ID2D1RenderTarget *This, D2D1_ANTIALIAS_MODE antialiasMode) PURE;
	GetAntialiasMode             cgo.Symbol //D2D1_ANTIALIAS_MODE(ID2D1RenderTarget *This) PURE;
	SetTextAntialiasMode         cgo.Symbol //void(ID2D1RenderTarget *This, D2D1_TEXT_ANTIALIAS_MODE textAntialiasMode) PURE;
	GetTextAntialiasMode         cgo.Symbol //D2D1_TEXT_ANTIALIAS_MODE(ID2D1RenderTarget *This) PURE;
	SetTextRenderingParams       cgo.Symbol // void(ID2D1RenderTarget *This, IDWriteRenderingParams *textRenderingParams) PURE;
	GetTextRenderingParams       cgo.Symbol // void(ID2D1RenderTarget *This, IDWriteRenderingParams **textRenderingParams) PURE;
	SetTags                      cgo.Symbol // void(ID2D1RenderTarget *This, D2D1_TAG tag1, D2D1_TAG tag2) PURE;
	GetTags                      cgo.Symbol // void(ID2D1RenderTarget *This, D2D1_TAG *tag1, D2D1_TAG *tag2) PURE;
	PushLayer                    cgo.Symbol // void(ID2D1RenderTarget *This, const D2D1_LAYER_PARAMETERS *layerParameters, ID2D1Layer *layer) PURE;
	PopLayer                     cgo.Symbol // void(ID2D1RenderTarget *This) PURE;
	Flush                        cgo.Symbol //HRESULT(ID2D1RenderTarget *This, D2D1_TAG *tag1, D2D1_TAG *tag2) PURE;
	SaveDrawingState             cgo.Symbol //void(ID2D1RenderTarget *This, ID2D1DrawingStateBlock *drawingStateBlock) PURE;
	RestoreDrawingState          cgo.Symbol //void(ID2D1RenderTarget *This, ID2D1DrawingStateBlock *drawingStateBlock) PURE;
	PushAxisAlignedClip          cgo.Symbol //void(ID2D1RenderTarget *This, const D2D1_RECT_F *clipRect, D2D1_ANTIALIAS_MODE antialiasMode) PURE;
	PopAxisAlignedClip           cgo.Symbol //void(ID2D1RenderTarget *This) PURE;
	Clear                        cgo.Symbol //void(ID2D1RenderTarget *This, const D2D1_COLOR_F *clearColor) PURE;
	BeginDraw                    cgo.Symbol //void(ID2D1RenderTarget *This) PURE;
	EndDraw                      cgo.Symbol //HRESULT(ID2D1RenderTarget *This, D2D1_TAG *tag1, D2D1_TAG *tag2) PURE;
	GetPixelFormat               cgo.Symbol //D2D1_PIXEL_FORMAT(ID2D1RenderTarget *This) PURE;
	SetDpi                       cgo.Symbol //void(ID2D1RenderTarget *This, FLOAT dpiX, FLOAT dpiY) PURE;
	GetDpi                       cgo.Symbol //void(ID2D1RenderTarget *This, FLOAT *dpiX, FLOAT *dpiY) PURE;
	GetSize                      cgo.Symbol //D2D1_SIZE_F(ID2D1RenderTarget *This) PURE;
	GetPixelSize                 cgo.Symbol //D2D1_SIZE_U(ID2D1RenderTarget *This) PURE;
	GetMaximumBitmapSize         cgo.Symbol //UINT32(ID2D1RenderTarget *This) PURE;
	IsSupported                  cgo.Symbol //BOOL(ID2D1RenderTarget *This, const D2D1_RENDER_TARGET_PROPERTIES *renderTargetProperties) PURE;
}

type RenderTarget struct {
	Resource
}

// TODO: more method?

func (this *RenderTarget) CreateBitmap(size SizeU, data []byte, stride int, properties *BitmapProperties) (bitmap *Bitmap, hr com.HRESULT) {
	hr = cgo.CallRet[com.HRESULT](this.class().CreateBitmap, this, size, cgo.CSlice(data), stride, properties, &bitmap)
	return
}

func (this *RenderTarget) CreateBitmapFromWicBitmap(wicBitmapSource *wic.BitmapSource, properties *BitmapProperties) (bitmap *Bitmap, hr com.HRESULT) {
	ret, _, _ := this.class().CreateBitmapFromWicBitmap.CallRaw(uintptr(cgo.Pointer(this)), uintptr(cgo.Pointer(wicBitmapSource)), uintptr(cgo.Pointer(properties)), uintptr(cgo.Pointer(&bitmap)))
	hr = com.HRESULT(ret)
	return
}

func (this *RenderTarget) CreateSolidColorBrush(color *ColorF, brushProperties *BrushProperties) (solidColorBrush *SolidColorBrush, hr com.HRESULT) {
	ret, _, _ := this.class().CreateSolidColorBrush.CallRaw(uintptr(cgo.Pointer(this)), uintptr(cgo.Pointer(color)), uintptr(cgo.Pointer(brushProperties)), uintptr(cgo.Pointer(&solidColorBrush)))
	hr = com.HRESULT(ret)
	return
}

func (this *RenderTarget) CreateGradientStopCollection(stops []GradientStop, gamma Gamma, extendMode ExtendMode) (collection *GradientStopCollection, hr com.HRESULT) {
	ret, _, _ := this.class().CreateGradientStopCollection.CallRaw(
		uintptr(cgo.Pointer(this)), uintptr(cgo.CSlice(stops)), uintptr(len(stops)),
		uintptr(gamma), uintptr(extendMode), uintptr(cgo.Pointer(&collection)),
	)
	runtime.KeepAlive(stops)
	return collection, com.HRESULT(ret)
}

func (this *RenderTarget) CreateLinearGradientBrush(props *LinearGradientBrushProperties, brushProperties *BrushProperties, collection *GradientStopCollection) (brush *LinearGradientBrush, hr com.HRESULT) {
	ret, _, _ := this.class().CreateLinearGradientBrush.CallRaw(
		uintptr(cgo.Pointer(this)), uintptr(cgo.Pointer(props)), uintptr(cgo.Pointer(brushProperties)),
		uintptr(cgo.Pointer(collection)), uintptr(cgo.Pointer(&brush)),
	)
	return brush, com.HRESULT(ret)
}

func (this *RenderTarget) CreateRadialGradientBrush(props *RadialGradientBrushProperties, brushProperties *BrushProperties, collection *GradientStopCollection) (brush *RadialGradientBrush, hr com.HRESULT) {
	ret, _, _ := this.class().CreateRadialGradientBrush.CallRaw(
		uintptr(cgo.Pointer(this)), uintptr(cgo.Pointer(props)), uintptr(cgo.Pointer(brushProperties)),
		uintptr(cgo.Pointer(collection)), uintptr(cgo.Pointer(&brush)),
	)
	return brush, com.HRESULT(ret)
}

func (this *RenderTarget) BeginDraw() {
	this.class().BeginDraw.CallRaw(uintptr(cgo.Pointer(this)))
}

func (this *RenderTarget) EndDraw(tag1, tag2 *Tag) com.HRESULT {
	ret, _, _ := this.class().EndDraw.CallRaw(uintptr(cgo.Pointer(this)), uintptr(cgo.Pointer(tag1)), uintptr(cgo.Pointer(tag2)))
	return com.HRESULT(ret)
}

func (this *RenderTarget) SetDpi(dpiX, dpiY float32) {
	cgo.Call(this.class().SetDpi, this, dpiX, dpiY)
}

func (this *RenderTarget) SetTextAntialiasMode(textAntialiasMode TextAntialiasMode) {
	this.class().SetTextAntialiasMode.CallRaw(uintptr(cgo.Pointer(this)), uintptr(textAntialiasMode))
}

func (this *RenderTarget) Clear(color *ColorF) {
	this.class().Clear.CallRaw(uintptr(cgo.Pointer(this)), uintptr(cgo.Pointer(color)))
}

func (this *RenderTarget) FillRectangle(rect *RectF, brush *Brush) {
	this.class().FillRectangle.CallRaw(uintptr(cgo.Pointer(this)), uintptr(cgo.Pointer(rect)), uintptr(cgo.Pointer(brush)))
}

func (this *RenderTarget) FillRoundedRectangle(roundedRect *RoundRect, brush *Brush) {
	this.class().FillRoundedRectangle.CallRaw(uintptr(cgo.Pointer(this)), uintptr(cgo.Pointer(roundedRect)), uintptr(cgo.Pointer(brush)))
}

func (this *RenderTarget) FillEllipse(ellipse *Ellipse, brush *Brush) {
	this.class().FillEllipse.CallRaw(uintptr(cgo.Pointer(this)), uintptr(cgo.Pointer(ellipse)), uintptr(cgo.Pointer(brush)))
}

func (this *RenderTarget) FillGeometry(geometry *Geometry, brush, opacityBrush *Brush) {
	this.class().FillGeometry.CallRaw(uintptr(cgo.Pointer(this)), uintptr(cgo.Pointer(geometry)), uintptr(cgo.Pointer(brush)), uintptr(cgo.Pointer(opacityBrush)))
}

func (this *RenderTarget) DrawLine(p0, p1 Point2F, brush *Brush, strokeWidth float32, strokeStyle *StrokeStyle) {
	cgo.Call(this.class().DrawLine, this, p0, p1, brush, strokeWidth, strokeStyle)
}

func (this *RenderTarget) DrawRectangle(rect *RectF, brush *Brush, strokeWidth float32, strokeStyle *StrokeStyle) {
	cgo.Call(this.class().DrawRectangle, this, rect, brush, strokeWidth, strokeStyle)
}

func (this *RenderTarget) DrawRoundedRectangle(roundRect *RoundRect, brush *Brush, strokeWidth float32, strokeStyle *StrokeStyle) {
	cgo.Call(this.class().DrawRoundedRectangle, this, roundRect, brush, strokeWidth, strokeStyle)
}

func (this *RenderTarget) DrawEllipse(ellipse *Ellipse, brush *Brush, strokeWidth float32, strokeStyle *StrokeStyle) {
	cgo.Call(this.class().DrawEllipse, this, ellipse, brush, strokeWidth, strokeStyle)
}

func (this *RenderTarget) DrawGeometry(geometry *Geometry, brush *Brush, strokeWidth float32, strokeStyle *StrokeStyle) {
	cgo.Call(this.class().DrawGeometry, this, geometry, brush, strokeWidth, strokeStyle)
}

func (this *RenderTarget) DrawText(text string, format *dwrite.TextFormat, layoutRect *RectF, foreBrush *Brush, options DrawTextOptions, measuringMode dwrite.MeasuringMode) {
	wText, _ := syscall.UTF16FromString(text)
	this.class().DrawText.CallRaw(uintptr(cgo.Pointer(this)), uintptr(cgo.CSlice(wText)), uintptr(len(wText)), uintptr(cgo.Pointer(format)), uintptr(cgo.Pointer(layoutRect)), uintptr(cgo.Pointer(foreBrush)), uintptr(options), uintptr(measuringMode))
	runtime.KeepAlive(wText)
}

func (this *RenderTarget) DrawTextLayout(origin Point2F, layout *dwrite.TextLayout, foreBrush *Brush, options DrawTextOptions) {
	cgo.Call(this.class().DrawTextLayout, this, origin, layout, foreBrush, options)
}

func (this *RenderTarget) DrawBitmap(bitmap *Bitmap, dstRect *RectF, opacity float32, interMode BitmapInterpolationMode, srcRect *RectF) {
	cgo.Call(this.class().DrawBitmap, this, bitmap, dstRect, opacity, interMode, srcRect)
}

func (this *RenderTarget) PushAxisAlignedClip(clipRect *RectF, antialiasMode AntialiasMode) {
	this.class().PushAxisAlignedClip.CallRaw(uintptr(cgo.Pointer(this)), uintptr(cgo.Pointer(clipRect)), uintptr(antialiasMode))
}

func (this *RenderTarget) PopAxisAlignedClip() {
	this.class().PopAxisAlignedClip.CallRaw(uintptr(cgo.Pointer(this)))
}

func (this *RenderTarget) SetTransform(transform *Matrix3x2F) {
	this.class().SetTransform.CallRaw(uintptr(cgo.Pointer(this)), uintptr(cgo.Pointer(transform)))
}

func (this *RenderTarget) GetTransform() (transform Matrix3x2F) {
	this.class().GetTransform.CallRaw(uintptr(cgo.Pointer(this)), uintptr(cgo.Pointer(&transform)))
	return
}

func (this *RenderTarget) class() *RenderTargetClass {
	return (*RenderTargetClass)(this.Class)
}

type HwndRenderTargetClass struct {
	RenderTargetClass

	CheckWindowState cgo.Symbol //D2D1_WINDOW_STATE(ID2D1HwndRenderTarget *This) PURE;
	Resize           cgo.Symbol //HRESULT(ID2D1HwndRenderTarget *This, const D2D1_SIZE_U *pixelSize) PURE;
	GetHwnd          cgo.Symbol //HWND(ID2D1HwndRenderTarget *This) PURE;
}

type HwndRenderTarget struct {
	RenderTarget
}

func (this *HwndRenderTarget) CheckWindowState() WindowState {
	ret, _, _ := this.class().CheckWindowState.CallRaw(uintptr(cgo.Pointer(this)))
	return WindowState(ret)
}

func (this *HwndRenderTarget) Resize(pixelSize *SizeU) com.HRESULT {
	ret, _, _ := this.class().Resize.CallRaw(uintptr(cgo.Pointer(this)), uintptr(cgo.Pointer(pixelSize)))
	return com.HRESULT(ret)
}

func (this *HwndRenderTarget) GetHwnd() uintptr {
	ret, _, _ := this.class().GetHwnd.CallRaw(uintptr(cgo.Pointer(this)))
	return ret
}

func (this *HwndRenderTarget) class() *HwndRenderTargetClass {
	return (*HwndRenderTargetClass)(this.Class)
}

type BrushClass struct {
	ResourceClass

	SetOpacity   cgo.Symbol //void(ID2D1Brush *This, FLOAT opacity) PURE;
	SetTransform cgo.Symbol //void(ID2D1Brush *This, const D2D1_MATRIX_3X2_F *transform) PURE;
	GetOpacity   cgo.Symbol //FLOAT(ID2D1Brush *This) PURE;
	GetTransform cgo.Symbol //void(ID2D1Brush *This, D2D1_MATRIX_3X2_F *transform) PURE;
}

type Brush struct {
	Resource
}

type SolidColorBrushClass struct {
	BrushClass

	SetColor cgo.Symbol //void(ID2D1SolidColorBrush *This, const D2D1_COLOR_F *color) PURE;
	GetColor cgo.Symbol //D2D1_COLOR_F(ID2D1SolidColorBrush *This) PURE;
}

type SolidColorBrush struct {
	Brush
}

func (this *SolidColorBrush) SetColor(color *ColorF) {
	this.class().SetColor.CallRaw(uintptr(cgo.Pointer(this)), uintptr(cgo.Pointer(color)))
}

func (this *SolidColorBrush) class() *SolidColorBrushClass {
	return (*SolidColorBrushClass)(this.Class)
}

type GradientStopCollection struct {
	Resource
}

type LinearGradientBrushClass struct {
	BrushClass

	SetStartPoint cgo.Symbol // void(ID2D1LinearGradientBrush *This, D2D1_POINT_2F startPoint) PURE;
	SetEndPoint   cgo.Symbol // void(ID2D1LinearGradientBrush *This, D2D1_POINT_2F endPoint) PURE;
}

type LinearGradientBrush struct {
	Brush
}

func (this *LinearGradientBrush) SetStartPoint(point Point2F) {
	cgo.Call(this.class().SetStartPoint, this, point)
}

func (this *LinearGradientBrush) SetEndPoint(point Point2F) {
	cgo.Call(this.class().SetEndPoint, this, point)
}

func (this *LinearGradientBrush) class() *LinearGradientBrushClass {
	return (*LinearGradientBrushClass)(this.Class)
}

type RadialGradientBrushClass struct {
	BrushClass

	SetCenter               cgo.Symbol // void(ID2D1RadialGradientBrush *This, D2D1_POINT_2F center) PURE;
	SetGradientOriginOffset cgo.Symbol // void(ID2D1RadialGradientBrush *This, D2D1_POINT_2F gradientOriginOffset) PURE;
	SetRadiusX              cgo.Symbol // void(ID2D1RadialGradientBrush *This, FLOAT radiusX) PURE;
	SetRadiusY              cgo.Symbol // void(ID2D1RadialGradientBrush *This, FLOAT radiusY) PURE;
}

type RadialGradientBrush struct {
	Brush
}

func (this *RadialGradientBrush) SetCenter(point Point2F) {
	cgo.Call(this.class().SetCenter, this, point)
}

func (this *RadialGradientBrush) SetGradientOriginOffset(point Point2F) {
	cgo.Call(this.class().SetGradientOriginOffset, this, point)
}

func (this *RadialGradientBrush) SetRadiusX(radius float32) {
	cgo.Call(this.class().SetRadiusX, this, radius)
}

func (this *RadialGradientBrush) SetRadiusY(radius float32) {
	cgo.Call(this.class().SetRadiusY, this, radius)
}

func (this *RadialGradientBrush) class() *RadialGradientBrushClass {
	return (*RadialGradientBrushClass)(this.Class)
}

type GeometryClass struct {
	ResourceClass

	GetBounds            cgo.Symbol //HRESULT(ID2D1Geometry *This, const D2D1_MATRIX_3X2_F *worldTransform, D2D1_RECT_F *bounds) PURE;
	GetWidenedBounds     cgo.Symbol //HRESULT(ID2D1Geometry *This, FLOAT strokeWidth, ID2D1StrokeStyle *strokeStyle, const D2D1_MATRIX_3X2_F *worldTransform, FLOAT flatteningTolerance, D2D1_RECT_F *bounds) PURE;
	StrokeContainsPoint  cgo.Symbol //HRESULT(ID2D1Geometry *This, D2D1_POINT_2F point, FLOAT strokeWidth, ID2D1StrokeStyle *strokeStyle, const D2D1_MATRIX_3X2_F *worldTransform, FLOAT flatteningTolerance, BOOL *contains) PURE;
	FillContainsPoint    cgo.Symbol //HRESULT(ID2D1Geometry *This, D2D1_POINT_2F point, const D2D1_MATRIX_3X2_F *worldTransform, FLOAT flatteningTolerance, BOOL *contains) PURE;
	CompareWithGeometry  cgo.Symbol //HRESULT(ID2D1Geometry *This, ID2D1Geometry *inputGeometry, const D2D1_MATRIX_3X2_F *inputGeometryTransform, FLOAT flatteningTolerance, D2D1_GEOMETRY_RELATION *relation) PURE;
	Simplify             cgo.Symbol //HRESULT(ID2D1Geometry *This, D2D1_GEOMETRY_SIMPLIFICATION_OPTION simplificationOption, const D2D1_MATRIX_3X2_F *worldTransform, FLOAT flatteningTolerance, ID2D1SimplifiedGeometrySink *geometrySink) PURE;
	Tessellate           cgo.Symbol //HRESULT(ID2D1Geometry *This, const D2D1_MATRIX_3X2_F *worldTransform, FLOAT flatteningTolerance, ID2D1TessellationSink *tessellationSink) PURE;
	CombineWithGeometry  cgo.Symbol //HRESULT(ID2D1Geometry *This, ID2D1Geometry *inputGeometry, D2D1_COMBINE_MODE combineMode, const D2D1_MATRIX_3X2_F *inputGeometryTransform, FLOAT flatteningTolerance, ID2D1SimplifiedGeometrySink *geometrySink) PURE;
	Outline              cgo.Symbol //HRESULT(ID2D1Geometry *This, const D2D1_MATRIX_3X2_F *worldTransform, FLOAT flatteningTolerance, ID2D1SimplifiedGeometrySink *geometrySink) PURE;
	ComputeArea          cgo.Symbol //HRESULT(ID2D1Geometry *This, const D2D1_MATRIX_3X2_F *worldTransform, FLOAT flatteningTolerance, FLOAT *area) PURE;
	ComputeLength        cgo.Symbol //HRESULT(ID2D1Geometry *This, const D2D1_MATRIX_3X2_F *worldTransform, FLOAT flatteningTolerance, FLOAT *length) PURE;
	ComputePointAtLength cgo.Symbol //HRESULT(ID2D1Geometry *This, FLOAT length, const D2D1_MATRIX_3X2_F *worldTransform, FLOAT flatteningTolerance, D2D1_POINT_2F *point, D2D1_POINT_2F *unitTangentVector) PURE;
	Widen                cgo.Symbol //HRESULT(ID2D1Geometry *This, FLOAT strokeWidth, ID2D1StrokeStyle *strokeStyle, const D2D1_MATRIX_3X2_F *worldTransform, FLOAT flatteningTolerance, ID2D1SimplifiedGeometrySink *geometrySink) PURE;
}

type Geometry struct {
	Resource
}

type PathGeometryClass struct {
	GeometryClass

	Open            cgo.Symbol //HRESULT(ID2D1PathGeometry *This, ID2D1GeometrySink **geometrySink) PURE;
	Stream          cgo.Symbol //HRESULT(ID2D1PathGeometry *This, ID2D1GeometrySink *geometrySink) PURE;
	GetSegmentCount cgo.Symbol //HRESULT(ID2D1PathGeometry *This, UINT32 *count) PURE;
	GetFigureCount  cgo.Symbol //HRESULT(ID2D1PathGeometry *This, UINT32 *count) PURE;
}

type PathGeometry struct {
	Geometry
}

func (this *PathGeometry) Open() (sink *GeometrySink, hr com.HRESULT) {
	ret, _, _ := this.class().Open.CallRaw(uintptr(cgo.Pointer(this)), uintptr(cgo.Pointer(&sink)))
	hr = com.HRESULT(ret)
	return
}

func (this *PathGeometry) class() *PathGeometryClass {
	return (*PathGeometryClass)(this.Class)
}

type SimplifiedGeometrySinkClass struct {
	com.UnknownClass

	SetFillMode     cgo.Symbol // void(ID2D1SimplifiedGeometrySink *This, D2D1_FILL_MODE fillMode) PURE;
	SetSegmentFlags cgo.Symbol // void(ID2D1SimplifiedGeometrySink *This, D2D1_PATH_SEGMENT vertexFlags) PURE;
	BeginFigure     cgo.Symbol // void(ID2D1SimplifiedGeometrySink *This, D2D1_POINT_2F startPoint, D2D1_FIGURE_BEGIN figureBegin) PURE;
	AddLines        cgo.Symbol // void(ID2D1SimplifiedGeometrySink *This, const D2D1_POINT_2F *points, UINT pointsCount) PURE;
	AddBeziers      cgo.Symbol // void(ID2D1SimplifiedGeometrySink *This, const D2D1_BEZIER_SEGMENT *beziers, UINT beziersCount) PURE;
	EndFigure       cgo.Symbol // void(ID2D1SimplifiedGeometrySink *This, D2D1_FIGURE_END figureEnd) PURE;
	Close           cgo.Symbol // HRESULT(ID2D1SimplifiedGeometrySink *This) PURE;
}

type SimplifiedGeometrySink struct {
	com.Unknown
}

func (this *SimplifiedGeometrySink) BeginFigure(startPoint Point2F, figureBegin FigureBegin) {
	cgo.Call(this.class().BeginFigure, this, startPoint, figureBegin)
}

func (this *SimplifiedGeometrySink) EndFigure(figureEnd FigureEnd) {
	this.class().EndFigure.CallRaw(uintptr(cgo.Pointer(this)), uintptr(figureEnd))
}

func (this *SimplifiedGeometrySink) AddLines(points []Point2F) {
	this.class().AddLines.CallRaw(uintptr(cgo.Pointer(this)), uintptr(cgo.CSlice(points)), uintptr(len(points)))
}

func (this *SimplifiedGeometrySink) Close() {
	this.class().Close.CallRaw(uintptr(cgo.Pointer(this)))
}

func (this *SimplifiedGeometrySink) class() *SimplifiedGeometrySinkClass {
	return (*SimplifiedGeometrySinkClass)(this.Class)
}

type GeometrySinkClass struct {
	SimplifiedGeometrySinkClass

	AddLine             cgo.Symbol //void(ID2D1GeometrySink *This, D2D1_POINT_2F point) PURE;
	AddBezier           cgo.Symbol //void(ID2D1GeometrySink *This, const D2D1_BEZIER_SEGMENT *bezier) PURE;
	AddQuadraticBezier  cgo.Symbol //void(ID2D1GeometrySink *This, const D2D1_QUADRATIC_BEZIER_SEGMENT *bezier) PURE;
	AddQuadraticBeziers cgo.Symbol //void(ID2D1GeometrySink *This, const D2D1_QUADRATIC_BEZIER_SEGMENT *beziers, UINT beziersCount) PURE;
	AddArc              cgo.Symbol //void(ID2D1GeometrySink *This, const D2D1_ARC_SEGMENT *arc) PURE;
}

type GeometrySink struct {
	SimplifiedGeometrySink
}

func (this *GeometrySink) AddLine(point Point2F) {
	cgo.Call(this.class().AddLine, this, point)
}

func (this *GeometrySink) AddBezier(bezier *BezierSegment) {
	this.class().AddBezier.CallRaw(uintptr(cgo.Pointer(this)), uintptr(cgo.Pointer(bezier)))
}

func (this *GeometrySink) AddArc(arc *ArcSegment) {
	this.class().AddArc.CallRaw(uintptr(cgo.Pointer(this)), uintptr(cgo.Pointer(arc)))
}

func (this *GeometrySink) class() *GeometrySinkClass {
	return (*GeometrySinkClass)(this.Class)
}

var IID_ID2D1Bitmap = com.DefineGuid(0xa2296057, 0xea42, 0x4099, 0x98, 0x3b, 0x53, 0x9f, 0xb6, 0x50, 0x54, 0x26)

type BitmapClass struct {
	com.UnknownClass

	GetSize              cgo.Symbol //D2D1_SIZE_F(ID2D1Bitmap *This)
	GetPixelSize         cgo.Symbol //D2D1_SIZE_U(ID2D1Bitmap *This)
	GetPixelFormat       cgo.Symbol //D2D1_PIXEL_FORMAT(ID2D1Bitmap *This)
	GetDpi               cgo.Symbol //void(ID2D1Bitmap *This, FLOAT *dpiX, FLOAT *dpiY)
	CopyFromBitmap       cgo.Symbol // HRESULT(ID2D1Bitmap *This, const D2D1_POINT_2U *destPoint, ID2D1Bitmap *bitmap, const D2D1_RECT_U *srcRect)
	CopyFromRenderTarget cgo.Symbol // HRESULT(ID2D1Bitmap *This, const D2D1_POINT_2U *destPoint, ID2D1RenderTarget *renderTarget, const D2D1_RECT_U *srcRect)
	CopyFromMemory       cgo.Symbol // HRESULT(ID2D1Bitmap *This, const D2D1_RECT_U *dstRect, const void *srcData, UINT32 pitch)
}

type Bitmap struct {
	com.Unknown
}

type DeviceClass struct {
	ResourceClass
	CreateDeviceContext     cgo.Symbol
	CreatePrintControl      cgo.Symbol
	SetMaximumTextureMemory cgo.Symbol
	GetMaximumTextureMemory cgo.Symbol
	ClearResources          cgo.Symbol
}

type Device struct{ Resource }

func (this *Device) CreateDeviceContext(options DeviceContextOptions) (context *DeviceContext, hr com.HRESULT) {
	ret, _, _ := (*DeviceClass)(this.Class).CreateDeviceContext.CallRaw(uintptr(cgo.Pointer(this)), uintptr(options), uintptr(cgo.Pointer(&context)))
	return context, com.HRESULT(ret)
}

type DeviceContextClass struct {
	RenderTargetClass
	CreateBitmap1                    cgo.Symbol
	CreateBitmapFromWicBitmap1       cgo.Symbol
	CreateColorContext               cgo.Symbol
	CreateColorContextFromFilename   cgo.Symbol
	CreateColorContextFromWic        cgo.Symbol
	CreateBitmapFromDxgiSurface      cgo.Symbol
	CreateEffect                     cgo.Symbol
	CreateGradientStopCollection1    cgo.Symbol
	CreateImageBrush                 cgo.Symbol
	CreateBitmapBrush1               cgo.Symbol
	CreateCommandList                cgo.Symbol
	IsDxgiFormatSupported            cgo.Symbol
	IsBufferPrecisionSupported       cgo.Symbol
	GetImageLocalBounds              cgo.Symbol
	GetImageWorldBounds              cgo.Symbol
	GetGlyphRunWorldBounds           cgo.Symbol
	GetDevice                        cgo.Symbol
	SetTarget                        cgo.Symbol
	GetTarget                        cgo.Symbol
	SetRenderingControls             cgo.Symbol
	GetRenderingControls             cgo.Symbol
	SetPrimitiveBlend                cgo.Symbol
	GetPrimitiveBlend                cgo.Symbol
	SetUnitMode                      cgo.Symbol
	GetUnitMode                      cgo.Symbol
	DrawGlyphRun1                    cgo.Symbol
	DrawImage                        cgo.Symbol
	DrawGdiMetafile                  cgo.Symbol
	DrawBitmap1                      cgo.Symbol
	PushLayer1                       cgo.Symbol
	InvalidateEffectInputRectangle   cgo.Symbol
	GetEffectInvalidRectangleCount   cgo.Symbol
	GetEffectInvalidRectangles       cgo.Symbol
	GetEffectRequiredInputRectangles cgo.Symbol
	FillOpacityMask1                 cgo.Symbol
}

type DeviceContext struct{ RenderTarget }

func (this *DeviceContext) CreateBitmapFromDxgiSurface(surface *dxgi.Surface, props *BitmapProperties1) (bitmap *Bitmap1, hr com.HRESULT) {
	ret, _, _ := (*DeviceContextClass)(this.Class).CreateBitmapFromDxgiSurface.CallRaw(uintptr(cgo.Pointer(this)), uintptr(cgo.Pointer(surface)), uintptr(cgo.Pointer(props)), uintptr(cgo.Pointer(&bitmap)))
	return bitmap, com.HRESULT(ret)
}

func (this *DeviceContext) CreateEffect(clsid com.CLSID) (effect *Effect, hr com.HRESULT) {
	ret, _, _ := (*DeviceContextClass)(this.Class).CreateEffect.CallRaw(uintptr(cgo.Pointer(this)), uintptr(cgo.Pointer(&clsid)), uintptr(cgo.Pointer(&effect)))
	return effect, com.HRESULT(ret)
}

func (this *DeviceContext) CreateCommandList() (commandList *CommandList, hr com.HRESULT) {
	ret, _, _ := (*DeviceContextClass)(this.Class).CreateCommandList.CallRaw(uintptr(cgo.Pointer(this)), uintptr(cgo.Pointer(&commandList)))
	return commandList, com.HRESULT(ret)
}

func (this *DeviceContext) GetImageLocalBounds(image *Image) (bounds RectF, hr com.HRESULT) {
	ret, _, _ := (*DeviceContextClass)(this.Class).GetImageLocalBounds.CallRaw(uintptr(cgo.Pointer(this)), uintptr(cgo.Pointer(image)), uintptr(cgo.Pointer(&bounds)))
	return bounds, com.HRESULT(ret)
}

func (this *DeviceContext) SetTarget(image *Image) {
	(*DeviceContextClass)(this.Class).SetTarget.CallRaw(uintptr(cgo.Pointer(this)), uintptr(cgo.Pointer(image)))
}

func (this *DeviceContext) DrawImage(image *Image, targetOffset *Point2F, imageRect *RectF, interpolation InterpolationMode, composite CompositeMode) {
	(*DeviceContextClass)(this.Class).DrawImage.CallRaw(
		uintptr(cgo.Pointer(this)), uintptr(cgo.Pointer(image)), uintptr(cgo.Pointer(targetOffset)),
		uintptr(cgo.Pointer(imageRect)), uintptr(interpolation), uintptr(composite),
	)
}

type Image struct{ Resource }

type Bitmap1Class struct {
	BitmapClass
	GetColorContext cgo.Symbol
	GetOptions      cgo.Symbol
	GetSurface      cgo.Symbol
	Map             cgo.Symbol
	Unmap           cgo.Symbol
}

type Bitmap1 struct{ Bitmap }

type PropertiesClass struct {
	com.UnknownClass
	GetPropertyCount      cgo.Symbol
	GetPropertyName       cgo.Symbol
	GetPropertyNameLength cgo.Symbol
	GetType               cgo.Symbol
	GetPropertyIndex      cgo.Symbol
	SetValueByName        cgo.Symbol
	SetValue              cgo.Symbol
	GetValueByName        cgo.Symbol
	GetValue              cgo.Symbol
	GetValueSize          cgo.Symbol
	GetSubProperties      cgo.Symbol
}

type Properties struct{ com.Unknown }

type EffectClass struct {
	PropertiesClass
	SetInput      cgo.Symbol
	SetInputCount cgo.Symbol
	GetInput      cgo.Symbol
	GetInputCount cgo.Symbol
	GetOutput     cgo.Symbol
}

type Effect struct{ Properties }

func (this *Effect) SetValue(index uint32, propertyType PropertyType, data cgo.Pointer, size uint32) com.HRESULT {
	ret, _, _ := (*EffectClass)(this.Class).SetValue.CallRaw(uintptr(cgo.Pointer(this)), uintptr(index), uintptr(propertyType), uintptr(data), uintptr(size))
	return com.HRESULT(ret)
}

func (this *Effect) SetInput(index uint32, image *Image, invalidate bool) {
	value := uintptr(0)
	if invalidate {
		value = 1
	}
	(*EffectClass)(this.Class).SetInput.CallRaw(uintptr(cgo.Pointer(this)), uintptr(index), uintptr(cgo.Pointer(image)), value)
}

func (this *Effect) GetOutput() (image *Image) {
	(*EffectClass)(this.Class).GetOutput.CallRaw(uintptr(cgo.Pointer(this)), uintptr(cgo.Pointer(&image)))
	return
}

type CommandListClass struct {
	ResourceClass
	Stream cgo.Symbol
	Close  cgo.Symbol
}

type CommandList struct{ Image }

func (this *CommandList) Close() com.HRESULT {
	ret, _, _ := (*CommandListClass)(this.Class).Close.CallRaw(uintptr(cgo.Pointer(this)))
	return com.HRESULT(ret)
}

func (this *Bitmap) CopyFromMemory(rect *RectU, data []byte, stride int) com.HRESULT {
	ret, _, _ := this.class().CopyFromMemory.CallRaw(uintptr(cgo.Pointer(this)), uintptr(cgo.Pointer(rect)), uintptr(cgo.CSlice(data)), uintptr(stride))
	return com.HRESULT(ret)
}

func (this *Bitmap) class() *BitmapClass {
	return (*BitmapClass)(this.Class)
}

type StrokeStyle struct {
	// TODO: add method
}
