package dwrite

import (
	"runtime"
	"syscall"

	"github.com/goexlib/cgo"
	"github.com/golang-gui/goui/platform/win32/sdk/com"
	"github.com/golang-gui/goui/platform/win32/winapi"
)

var (
	dwrite              = cgo.NewLazyLibrary("dwrite.dll")
	dwriteCreateFactory = dwrite.NewSymbol("DWriteCreateFactory")
)

func CreateFactory[F isFactory](factoryType FactoryType, iid com.IID) (factory *F, err error) {
	if err = dwriteCreateFactory.Find(); err == nil {
		ret, _, _ := dwriteCreateFactory.CallRaw(uintptr(factoryType), uintptr(cgo.Pointer(&iid)), uintptr(cgo.Pointer(&factory)))
		if com.HRESULT(ret).Failed() {
			err = com.HRESULT(ret)
		}
	}
	return
}

type isFactory interface {
	isFactory()
}

var IID_IDWriteFactory = com.DefineGuid(0xb859ee5a, 0xd838, 0x4b5b, 0xa2, 0xe8, 0x1a, 0xdc, 0x7d, 0x93, 0xdb, 0x48)

type FactoryClass struct {
	com.UnknownClass
	GetSystemFontCollection        cgo.Symbol //HRESULT(IDWriteFactory *This,	IDWriteFontCollection **collection,	WINBOOL check_for_updates)
	CreateCustomFontCollection     cgo.Symbol //HRESULT(IDWriteFactory *This, IDWriteFontCollectionLoader *loader, const void *key, UINT32 key_size, IDWriteFontCollection **collection)
	RegisterFontCollectionLoader   cgo.Symbol //HRESULT(IDWriteFactory *This, IDWriteFontCollectionLoader *loader)
	UnregisterFontCollectionLoader cgo.Symbol //HRESULT(IDWriteFactory *This, IDWriteFontCollectionLoader *loader)
	CreateFontFileReference        cgo.Symbol //HRESULT(IDWriteFactory *This, const WCHAR *path, const FILETIME *writetime, IDWriteFontFile **font_file)
	CreateCustomFontFileReference  cgo.Symbol //HRESULT(IDWriteFactory *This, const void *reference_key, UINT32 key_size, IDWriteFontFileLoader *loader, IDWriteFontFile **font_file)
	CreateFontFace                 cgo.Symbol //HRESULT(IDWriteFactory *This, DWRITE_FONT_FACE_TYPE facetype, UINT32 files_number, IDWriteFontFile *const *font_files, UINT32 index, DWRITE_FONT_SIMULATIONS sim_flags, IDWriteFontFace **font_face)
	CreateRenderingParams          cgo.Symbol //HRESULT(IDWriteFactory *This, IDWriteRenderingParams **params)
	CreateMonitorRenderingParams   cgo.Symbol //HRESULT(IDWriteFactory *This, HMONITOR monitor, IDWriteRenderingParams **params)
	CreateCustomRenderingParams    cgo.Symbol //HRESULT(IDWriteFactory *This, FLOAT gamma, FLOAT enhancedContrast, FLOAT cleartype_level, DWRITE_PIXEL_GEOMETRY geometry, DWRITE_RENDERING_MODE mode, IDWriteRenderingParams **params)
	RegisterFontFileLoader         cgo.Symbol //HRESULT(IDWriteFactory *This, IDWriteFontFileLoader *loader)
	UnregisterFontFileLoader       cgo.Symbol //HRESULT(IDWriteFactory *This, IDWriteFontFileLoader *loader)
	CreateTextFormat               cgo.Symbol //HRESULT(IDWriteFactory *This, const WCHAR *family_name, IDWriteFontCollection *collection, DWRITE_FONT_WEIGHT weight, DWRITE_FONT_STYLE style, DWRITE_FONT_STRETCH stretch, FLOAT size, const WCHAR *locale, IDWriteTextFormat **format)
	CreateTypography               cgo.Symbol //HRESULT(IDWriteFactory *This, IDWriteTypography **typography)
	GetGdiInterop                  cgo.Symbol //HRESULT(IDWriteFactory *This, IDWriteGdiInterop **gdi_interop)
	CreateTextLayout               cgo.Symbol //HRESULT(IDWriteFactory *This, const WCHAR *string, UINT32 len, IDWriteTextFormat *format, FLOAT max_width, FLOAT max_height, IDWriteTextLayout **layout)
	CreateGdiCompatibleTextLayout  cgo.Symbol //HRESULT(IDWriteFactory *This, const WCHAR *string, UINT32 len, IDWriteTextFormat *format, FLOAT layout_width, FLOAT layout_height, FLOAT pixels_per_dip, const DWRITE_MATRIX *transform, WINBOOL use_gdi_natural, IDWriteTextLayout **layout)
	CreateEllipsisTrimmingSign     cgo.Symbol //HRESULT(IDWriteFactory *This, IDWriteTextFormat *format, IDWriteInlineObject **trimming_sign)
	CreateTextAnalyzer             cgo.Symbol //HRESULT(IDWriteFactory *This, IDWriteTextAnalyzer **analyzer)
	CreateNumberSubstitution       cgo.Symbol //HRESULT(IDWriteFactory *This, DWRITE_NUMBER_SUBSTITUTION_METHOD method, const WCHAR *locale, WINBOOL ignore_user_override, IDWriteNumberSubstitution **substitution)
	CreateGlyphRunAnalysis         cgo.Symbol //HRESULT(IDWriteFactory *This, const DWRITE_GLYPH_RUN *glyph_run, FLOAT pixels_per_dip, const DWRITE_MATRIX *transform, DWRITE_RENDERING_MODE rendering_mode, DWRITE_MEASURING_MODE measuring_mode, FLOAT baseline_x, FLOAT baseline_y, IDWriteGlyphRunAnalysis **analysis)
}

type Factory struct {
	com.Unknown
}

func (this Factory) isFactory() {}

func (this *Factory) RegisterFontFileLoader(loader *FontFileLoader) com.HRESULT {
	ret, _, _ := this.class().RegisterFontFileLoader.CallRaw(uintptr(cgo.Pointer(this)), uintptr(cgo.Pointer(loader)))
	return com.HRESULT(ret)
}

func (this *Factory) UnregisterFontFileLoader(loader *FontFileLoader) com.HRESULT {
	ret, _, _ := this.class().UnregisterFontFileLoader.CallRaw(uintptr(cgo.Pointer(this)), uintptr(cgo.Pointer(loader)))
	return com.HRESULT(ret)
}

func (this *Factory) CreateTextFormat(fontFamily string, collection *FontCollection, weight FontWeight, style FontStyle, stretch FontStretch, size float32, locale string) (format *TextFormat, hr com.HRESULT) {
	wFamily, _ := syscall.UTF16PtrFromString(fontFamily)
	wLocale, _ := syscall.UTF16PtrFromString(locale)
	hr = cgo.CallRet[com.HRESULT](this.class().CreateTextFormat, this, wFamily, collection, weight, style, stretch, size, wLocale, &format)
	runtime.KeepAlive(wFamily)
	runtime.KeepAlive(wLocale)
	return
}

func (this *Factory) CreateTextLayout(text string, format *TextFormat, maxWidth, maxHeight float32) (layout *TextLayout, hr com.HRESULT) {
	wText, _ := syscall.UTF16FromString(text)
	hr = cgo.CallRet[com.HRESULT](this.class().CreateTextLayout, this, cgo.CSlice(wText), len(wText), format, maxWidth, maxHeight, &layout)
	runtime.KeepAlive(wText)
	return
}

func (this *Factory) class() *FactoryClass {
	return (*FactoryClass)(this.Class)
}

var IID_IDWriteFactory1 = com.DefineGuid(0x30572f99, 0xdac6, 0x41db, 0xa1, 0x6e, 0x04, 0x86, 0x30, 0x7e, 0x60, 0x6a)

type Factory1Class struct {
	FactoryClass
	GetEudcFontCollection       cgo.Symbol //HRESULT(IDWriteFactory1 *This, IDWriteFontCollection **collection, WINBOOL check_for_updates = FALSE)
	CreateCustomRenderingParams cgo.Symbol //HRESULT(IDWriteFactory1 *This, FLOAT gamma, FLOAT enhcontrast, FLOAT enhcontrast_grayscale, FLOAT cleartype_level, DWRITE_PIXEL_GEOMETRY geometry, DWRITE_RENDERING_MODE mode, IDWriteRenderingParams1 **params) = 0
}

var IID_IDWriteFactory2 = com.DefineGuid(0x0439fc60, 0xca44, 0x4994, 0x8d, 0xee, 0x3a, 0x9a, 0xf7, 0xb7, 0x32, 0xec)

type Factory2Class struct {
	Factory1Class
	GetSystemFontFallback                       cgo.Symbol //HRESULT(IDWriteFactory2 *This, IDWriteFontFallback **fallback)
	CreateFontFallbackBuilder                   cgo.Symbol //HRESULT(IDWriteFactory2 *This, IDWriteFontFallbackBuilder **fallbackbuilder)
	TranslateColorGlyphRun                      cgo.Symbol //HRESULT(IDWriteFactory2 *This, FLOAT originX, FLOAT originY, const DWRITE_GLYPH_RUN *run, const DWRITE_GLYPH_RUN_DESCRIPTION *rundescr, DWRITE_MEASURING_MODE mode, const DWRITE_MATRIX *transform, UINT32 palette_index, IDWriteColorGlyphRunEnumerator **colorlayers)
	IDWriteFactory2_CreateCustomRenderingParams cgo.Symbol //HRESULT(IDWriteFactory2 *This, FLOAT gamma, FLOAT contrast, FLOAT grayscalecontrast, FLOAT cleartypeLevel, DWRITE_PIXEL_GEOMETRY pixelGeometry, DWRITE_RENDERING_MODE renderingMode, DWRITE_GRID_FIT_MODE gridFitMode, IDWriteRenderingParams2 **params)
	IDWriteFactory2_CreateGlyphRunAnalysis      cgo.Symbol //HRESULT(IDWriteFactory2 *This, const DWRITE_GLYPH_RUN *run, const DWRITE_MATRIX *transform, DWRITE_RENDERING_MODE renderingMode, DWRITE_MEASURING_MODE measuringMode, DWRITE_GRID_FIT_MODE gridFitMode, DWRITE_TEXT_ANTIALIAS_MODE antialiasMode, FLOAT originX, FLOAT originY, IDWriteGlyphRunAnalysis **analysis)
}

var IID_IDWriteFactory3 = com.DefineGuid(0x9a1b41c3, 0xd3bb, 0x466a, 0x87, 0xfc, 0xfe, 0x67, 0x55, 0x6a, 0x3b, 0x65)

type Factory3Class struct {
	Factory2Class
	IDWriteFactory3_CreateGlyphRunAnalysis      cgo.Symbol //HRESULT(IDWriteFactory3 *This, const DWRITE_GLYPH_RUN *run, const DWRITE_MATRIX *transform, DWRITE_RENDERING_MODE1 rendering_mode, DWRITE_MEASURING_MODE measuring_mode, DWRITE_GRID_FIT_MODE gridfit_mode, DWRITE_TEXT_ANTIALIAS_MODE antialias_mode, FLOAT origin_x, FLOAT origin_y, IDWriteGlyphRunAnalysis **analysis)
	IDWriteFactory3_CreateCustomRenderingParams cgo.Symbol //HRESULT(IDWriteFactory3 *This, FLOAT gamma, FLOAT enhanced_contrast, FLOAT grayscale_enhanced_contrast, FLOAT cleartype_level, DWRITE_PIXEL_GEOMETRY pixel_geometry, DWRITE_RENDERING_MODE1 rendering_mode, DWRITE_GRID_FIT_MODE gridfit_mode, IDWriteRenderingParams3 **params)
	CreateFontFaceReference_                    cgo.Symbol //HRESULT(IDWriteFactory3 *This, IDWriteFontFile *file, UINT32 index, DWRITE_FONT_SIMULATIONS simulations, IDWriteFontFaceReference **reference)
	CreateFontFaceReference                     cgo.Symbol //HRESULT(IDWriteFactory3 *This, const WCHAR *path, const FILETIME *writetime, UINT32 index, DWRITE_FONT_SIMULATIONS simulations, IDWriteFontFaceReference **reference)
	GetSystemFontSet                            cgo.Symbol //HRESULT(IDWriteFactory3 *This, IDWriteFontSet **fontset)
	CreateFontSetBuilder                        cgo.Symbol //HRESULT(IDWriteFactory3 *This, IDWriteFontSetBuilder **builder)
	CreateFontCollectionFromFontSet             cgo.Symbol //HRESULT(IDWriteFactory3 *This, IDWriteFontSet *fontset, IDWriteFontCollection1 **collection)
	IDWriteFactory3_GetSystemFontCollection     cgo.Symbol //HRESULT(IDWriteFactory3 *This, WINBOOL include_downloadable, IDWriteFontCollection1 **collection, WINBOOL check_for_updates)
	GetFontDownloadQueue                        cgo.Symbol //HRESULT(IDWriteFactory3 *This, IDWriteFontDownloadQueue **queue)
}

var IID_IDWriteFactory4 = com.DefineGuid(0x4b0b5bd3, 0x0797, 0x4549, 0x8a, 0xc5, 0xfe, 0x91, 0x5c, 0xc5, 0x38, 0x56)

type Factory4Class struct {
	Factory3Class
	IDWriteFactory4_TranslateColorGlyphRun cgo.Symbol //HRESULT(IDWriteFactory4 *This, D2D1_POINT_2F baseline_origin, const DWRITE_GLYPH_RUN *run, const DWRITE_GLYPH_RUN_DESCRIPTION *run_desc, DWRITE_GLYPH_IMAGE_FORMATS desired_formats, DWRITE_MEASURING_MODE measuring_mode, const DWRITE_MATRIX *transform, UINT32 palette, IDWriteColorGlyphRunEnumerator1 **layers)
	ComputeGlyphOrigins_                   cgo.Symbol //HRESULT(IDWriteFactory4 *This, const DWRITE_GLYPH_RUN *run, D2D1_POINT_2F baseline_origin, D2D1_POINT_2F *origins)
	ComputeGlyphOrigins                    cgo.Symbol //HRESULT(IDWriteFactory4 *This, const DWRITE_GLYPH_RUN *run, DWRITE_MEASURING_MODE measuring_mode, D2D1_POINT_2F baseline_origin, const DWRITE_MATRIX *transform, D2D1_POINT_2F *origins)
}

var IID_IDWriteFactory5 = com.DefineGuid(0x958db99a, 0xbe2a, 0x4f09, 0xaf, 0x7d, 0x65, 0x18, 0x98, 0x03, 0xd1, 0xd3)

type Factory5Class struct {
	IDWriteFactory5_CreateFontSetBuilder cgo.Symbol //HRESULT(IDWriteFactory5 *This, IDWriteFontSetBuilder1 **fontset_builder)
	CreateInMemoryFontFileLoader         cgo.Symbol //HRESULT(IDWriteFactory5 *This, IDWriteInMemoryFontFileLoader **loader)
	CreateHttpFontFileLoader             cgo.Symbol //HRESULT(IDWriteFactory5 *This, const WCHAR *referrer_url, const WCHAR *extra_headers, IDWriteRemoteFontFileLoader **loader)
	AnalyzeContainerType                 cgo.Symbol //DWRITE_CONTAINER_TYPE(IDWriteFactory5 *This, const void *data, UINT32 data_size)
	UnpackFontFile                       cgo.Symbol //HRESULT(IDWriteFactory5 *This, DWRITE_CONTAINER_TYPE container_type, const void *data, UINT32 data_size, IDWriteFontFileStream **stream)
}

type Factory5 struct {
	Factory
}

func (this *Factory5) CreateFontSetBuilder() (fontSetBuilder *FontSetBuilder1, hr com.HRESULT) {
	ret, _, _ := this.class().IDWriteFactory5_CreateFontSetBuilder.CallRaw(uintptr(cgo.Pointer(this)), uintptr(cgo.Pointer(&fontSetBuilder)))
	hr = com.HRESULT(ret)
	return
}

func (this *Factory5) CreateInMemoryFontFileLoader() (loader *InMemoryFontFileLoader, hr com.HRESULT) {
	ret, _, _ := this.class().CreateInMemoryFontFileLoader.CallRaw(uintptr(cgo.Pointer(this)), uintptr(cgo.Pointer(&loader)))
	hr = com.HRESULT(ret)
	return
}

func (this *Factory5) class() *Factory5Class {
	return (*Factory5Class)(this.Class)
}

var IID_IDWriteFontFallbackBuilder = com.DefineGuid(0xfd882d06, 0x8aba, 0x4fb8, 0xb8, 0x49, 0x8b, 0xe8, 0xb7, 0x3e, 0x14, 0xde)

type FontFallbackBuilderClass struct {
	com.UnknownClass
	AddMapping         cgo.Symbol //HRESULT(IDWriteFontFallbackBuilder *This,	const DWRITE_UNICODE_RANGE *ranges,	UINT32 rangesCount,	const WCHAR **targetFamilyNames,	UINT32 targetFamilyNamesCount,	IDWriteFontCollection *collection,	const WCHAR *localeName,	const WCHAR *baseFamilyName,	FLOAT scale)
	AddMappings        cgo.Symbol //HRESULT(IDWriteFontFallbackBuilder *This,	IDWriteFontFallback *fallback)
	CreateFontFallback cgo.Symbol //HRESULT(IDWriteFontFallbackBuilder *This,	IDWriteFontFallback **fallback)
}

type FontFallbackBuilder struct {
	com.Unknown
}

func (this *FontFallbackBuilder) AddMapping(ranges []UnicodeRange, familyNames []string, collection *FontCollection, locale, baseFamily string, scale float32) com.HRESULT {
	wFamilys := make([]*uint16, len(familyNames))
	for i := range familyNames {
		wFamilys[i], _ = syscall.UTF16PtrFromString(familyNames[i])
	}
	wLocale, _ := syscall.UTF16PtrFromString(locale)
	wBaseFamily, _ := syscall.UTF16PtrFromString(baseFamily)
	ret := cgo.CallRet[com.HRESULT](this.class().AddMapping, this, cgo.CSlice(ranges), len(ranges), cgo.CSlice(wFamilys), len(familyNames), collection, wLocale, wBaseFamily, scale)
	runtime.KeepAlive(wFamilys)
	runtime.KeepAlive(wLocale)
	runtime.KeepAlive(wBaseFamily)
	return ret
}

func (this *FontFallbackBuilder) AddMappings(fallback *FontFallback) com.HRESULT {
	ret, _, _ := this.class().AddMappings.CallRaw(uintptr(cgo.Pointer(this)), uintptr(cgo.Pointer(fallback)))
	return com.HRESULT(ret)
}

func (this *FontFallbackBuilder) CreateFontFallback() (fallback *FontFallback, hr com.HRESULT) {
	ret, _, _ := this.class().CreateFontFallback.CallRaw(uintptr(cgo.Pointer(this)), uintptr(cgo.Pointer(&fallback)))
	hr = com.HRESULT(ret)
	return
}

func (this *FontFallbackBuilder) class() *FontFallbackBuilderClass {
	return (*FontFallbackBuilderClass)(this.Class)
}

var IID_IDWriteFontFallback = com.DefineGuid(0xefa008f9, 0xf7a1, 0x48bf, 0xb0, 0x5c, 0xf2, 0x24, 0x71, 0x3c, 0xc0, 0xff)

type FontFallbackClass struct {
	com.UnknownClass
	MapCharacters cgo.Symbol //HRESULT(IDWriteFontFallback *This,	IDWriteTextAnalysisSource *source,	UINT32 position,	UINT32 length,	IDWriteFontCollection *basecollection,	const WCHAR *baseFamilyName,	DWRITE_FONT_WEIGHT baseWeight,	DWRITE_FONT_STYLE baseStyle,	DWRITE_FONT_STRETCH baseStretch,	UINT32 *mappedLength,	IDWriteFont **mappedFont,	FLOAT *scale);
}

type FontFallback struct {
	com.Unknown
}

func (this *FontFallback) MapCharacters(source *TextAnalysisSource, position, length int, baseCollection *FontCollection, baseFamily string, baseWeight FontWeight, baseStyle FontStyle, baseStretch FontStretch, mappedLength *int, mappedFont **Font, scale *float32) com.HRESULT {
	wBaseFamily, _ := syscall.UTF16PtrFromString(baseFamily)
	ret := cgo.CallRet[com.HRESULT](this.class().MapCharacters, this, source, position, length, baseCollection, wBaseFamily, baseWeight, baseStyle, baseStretch, mappedLength, mappedFont, scale)
	runtime.KeepAlive(wBaseFamily)
	return ret
}

func (this *FontFallback) class() *FontFallbackClass {
	return (*FontFallbackClass)(this.Class)
}

var IID_IDWriteFontFallback1 = com.DefineGuid(0x2397599d, 0xdd0d, 0x4681, 0xbd, 0x6a, 0xf4, 0xf3, 0x1e, 0xaa, 0xde, 0x77)

type FontFallback1Class struct {
	FontFallbackClass
	IDWriteFontFallback1_MapCharacters cgo.Symbol //HRESULT(IDWriteFontFallback1 *This,	IDWriteTextAnalysisSource *source,	UINT32 pos,	UINT32 length,	IDWriteFontCollection *base_collection,	const WCHAR *familyname,	const DWRITE_FONT_AXIS_VALUE *axis_values,	UINT32 num_values,	UINT32 *mapped_length,	FLOAT *scale,	IDWriteFontFace5 **fontface)
}

type FontFallback1 struct {
	FontFallback
}

func (this *FontFallback1) MapCharacters(source *TextAnalysisSource, position, length int, baseCollection *FontCollection, family string, axisValues []FontAxisValue, mappedLength *int, mappedFont **Font, scale *float32, fontFace **FontFace5) com.HRESULT {
	wFamily, _ := syscall.UTF16PtrFromString(family)
	ret := cgo.CallRet[com.HRESULT](this.class().IDWriteFontFallback1_MapCharacters, this, source, position, length, baseCollection, wFamily, cgo.CSlice(axisValues), len(axisValues), mappedLength, mappedFont, scale, fontFace)
	runtime.KeepAlive(wFamily)
	return ret
}

func (this *FontFallback1) class() *FontFallback1Class {
	return (*FontFallback1Class)(this.Class)
}

type TextAnalysisSource struct {
}

var IID_IDWriteFontFileLoader = com.DefineGuid(0x727cad4e, 0xd6af, 0x4c9e, 0x8a, 0x08, 0xd6, 0x95, 0xb1, 0x1c, 0xaa, 0x49)

type FontFileLoaderClass struct {
	com.UnknownClass
	CreateStreamFromKey cgo.Symbol //HRESULT(IDWriteFontFileLoader *This, const void *key, UINT32 key_size, IDWriteFontFileStream **stream)
}

type FontFileLoader struct {
	com.Unknown
}

var IID_IDWriteInMemoryFontFileLoader = com.DefineGuid(0xdc102f47, 0xa12d, 0x4b1c, 0x82, 0x2d, 0x9e, 0x11, 0x7e, 0x33, 0x04, 0x3f)

type InMemoryFontFileLoaderClass struct {
	FontFileLoaderClass
	CreateInMemoryFontFileReference cgo.Symbol //HRESULT(IDWriteInMemoryFontFileLoader *This, IDWriteFactory *factory, const void *data, UINT32 data_size, IUnknown *owner, IDWriteFontFile **fontfile)
	GetFileCount                    cgo.Symbol //UINT32(IDWriteInMemoryFontFileLoader *This)
}

type InMemoryFontFileLoader struct {
	FontFileLoader
}

func (this *InMemoryFontFileLoader) CreateInMemoryFontFileReference(factory *Factory, data []byte, owner *com.Unknown) (fontFile *FontFile, hr com.HRESULT) {
	ret, _, _ := this.class().CreateInMemoryFontFileReference.CallRaw(uintptr(cgo.Pointer(this)), uintptr(cgo.Pointer(factory)), uintptr(cgo.CSlice(data)), uintptr(len(data)), uintptr(cgo.Pointer(owner)), uintptr(cgo.Pointer(&fontFile)))
	hr = com.HRESULT(ret)
	return
}

func (this *InMemoryFontFileLoader) class() *InMemoryFontFileLoaderClass {
	return (*InMemoryFontFileLoaderClass)(this.Class)
}

var IID_IDWriteFontCollection = com.DefineGuid(0xa84cee02, 0x3eea, 0x4eee, 0xa8, 0x27, 0x87, 0xc1, 0xa0, 0x2a, 0x0f, 0xcc)

type FontCollectionClass struct {
	com.UnknownClass
	GetFontFamilyCount  cgo.Symbol //UINT32(IDWriteFontCollection *This)
	GetFontFamily       cgo.Symbol //HRESULT(IDWriteFontCollection *This, UINT32 index, IDWriteFontFamily **family)
	FindFamilyName      cgo.Symbol //HRESULT(IDWriteFontCollection *This, const WCHAR *name, UINT32 *index, WINBOOL *exists)
	GetFontFromFontFace cgo.Symbol //HRESULT(IDWriteFontCollection *This,IDWriteFontFace *face,IDWriteFont **font)
}

type FontCollection struct {
	com.Unknown
}

func (this *FontCollection) GetFontFamilyCount() int {
	ret, _, _ := this.class().GetFontFamilyCount.CallRaw(uintptr(cgo.Pointer(this)))
	return int(ret)
}

func (this *FontCollection) GetFontFamily(index int) (family *FontFamily, hr com.HRESULT) {
	ret, _, _ := this.class().GetFontFamily.CallRaw(uintptr(cgo.Pointer(this)), uintptr(index), uintptr(cgo.Pointer(&family)))
	hr = com.HRESULT(ret)
	return
}

func (this *FontCollection) class() *FontCollectionClass {
	return (*FontCollectionClass)(this.Class)
}

var IID_IDWriteFontFace = com.DefineGuid(0x5f49804d, 0x7024, 0x4d43, 0xbf, 0xa9, 0xd2, 0x59, 0x84, 0xf5, 0x38, 0x49)

type FontFaceClass struct {
	com.UnknownClass
	GetType                      cgo.Symbol //DWRITE_FONT_FACE_TYPE(IDWriteFontFace *This)
	GetFiles                     cgo.Symbol //HRESULT(IDWriteFontFace *This,	UINT32 *number_of_files,	IDWriteFontFile **fontfiles)
	GetIndex                     cgo.Symbol //UINT32(IDWriteFontFace *This)
	GetSimulations               cgo.Symbol //DWRITE_FONT_SIMULATIONS(IDWriteFontFace *This)
	IsSymbolFont                 cgo.Symbol //WINBOOL(IDWriteFontFace *This)
	GetMetrics                   cgo.Symbol //void(IDWriteFontFace *This,	DWRITE_FONT_METRICS *metrics)
	GetGlyphCount                cgo.Symbol //UINT16(IDWriteFontFace *This)
	GetDesignGlyphMetrics        cgo.Symbol //HRESULT(IDWriteFontFace *This,	const UINT16 *glyph_indices,	UINT32 glyph_count,	DWRITE_GLYPH_METRICS *metrics,	WINBOOL is_sideways)
	GetGlyphIndices              cgo.Symbol //HRESULT(IDWriteFontFace *This,	const UINT32 *codepoints,	UINT32 count,	UINT16 *glyph_indices)
	TryGetFontTable              cgo.Symbol //HRESULT(IDWriteFontFace *This,	UINT32 table_tag,	const void **table_data,	UINT32 *table_size,	void **context,	WINBOOL *exists)
	ReleaseFontTable             cgo.Symbol //void(IDWriteFontFace *This,	void *table_context)
	GetGlyphRunOutline           cgo.Symbol //HRESULT(IDWriteFontFace *This,	FLOAT emSize,	const UINT16 *glyph_indices,	const FLOAT *glyph_advances,	const DWRITE_GLYPH_OFFSET *glyph_offsets,	UINT32 glyph_count,	WINBOOL is_sideways,	WINBOOL is_rtl,	IDWriteGeometrySink *geometrysink)
	GetRecommendedRenderingMode  cgo.Symbol //HRESULT(IDWriteFontFace *This,	FLOAT emSize,	FLOAT pixels_per_dip,	DWRITE_MEASURING_MODE mode,	IDWriteRenderingParams *params,	DWRITE_RENDERING_MODE *rendering_mode)
	GetGdiCompatibleMetrics      cgo.Symbol //HRESULT(IDWriteFontFace *This,	FLOAT emSize,	FLOAT pixels_per_dip,	const DWRITE_MATRIX *transform,	DWRITE_FONT_METRICS *metrics)
	GetGdiCompatibleGlyphMetrics cgo.Symbol //HRESULT(IDWriteFontFace *This,	FLOAT emSize,	FLOAT pixels_per_dip,	const DWRITE_MATRIX *transform,	WINBOOL use_gdi_natural,	const UINT16 *glyph_indices,	UINT32 glyph_count,	DWRITE_GLYPH_METRICS *metrics,	WINBOOL is_sideways)
}

type FontFace struct {
	com.Unknown
}

func (this *FontFace) GetType() FontFaceType {
	ret, _, _ := this.class().GetType.CallRaw(uintptr(cgo.Pointer(this)))
	return FontFaceType(ret)
}

func (this *FontFace) GetFiles() ([]*FontFile, com.HRESULT) {
	var count int
	var fontFiles *FontFile
	ret, _, _ := this.class().GetFiles.CallRaw(uintptr(cgo.Pointer(this)), uintptr(cgo.Pointer(&count)), uintptr(cgo.Pointer(&fontFiles)))
	if ret == 0 {
		return cgo.GoSliceN[*FontFile](cgo.Pointer(fontFiles), count), 0
	}
	return nil, com.HRESULT(ret)
}

func (this *FontFace) GetIndex() int {
	ret, _, _ := this.class().GetIndex.CallRaw(uintptr(cgo.Pointer(this)))
	return int(ret)
}

func (this *FontFace) class() *FontFaceClass {
	return (*FontFaceClass)(this.Class)
}

var IID_IDWriteFontFace1 = com.DefineGuid(0xa71efdb4, 0x9fdb, 0x4838, 0xad, 0x90, 0xcf, 0xc3, 0xbe, 0x8c, 0x3d, 0xaf)

type FontFace1Class struct {
	FontFaceClass
	IDWriteFontFace1_GetMetrics                  cgo.Symbol //void(IDWriteFontFace1 *This,	DWRITE_FONT_METRICS1 *metrics)
	IDWriteFontFace1_GetGdiCompatibleMetrics     cgo.Symbol //HRESULT(IDWriteFontFace1 *This,	FLOAT em_size,	FLOAT pixels_per_dip,	const DWRITE_MATRIX *transform,	DWRITE_FONT_METRICS1 *metrics)
	GetCaretMetrics                              cgo.Symbol //void(IDWriteFontFace1 *This,	DWRITE_CARET_METRICS *metrics)
	GetUnicodeRanges                             cgo.Symbol //HRESULT(IDWriteFontFace1 *This,	UINT32 max_count,	DWRITE_UNICODE_RANGE *ranges,	UINT32 *count)
	IsMonospacedFont                             cgo.Symbol //WINBOOL(IDWriteFontFace1 *This)
	GetDesignGlyphAdvances                       cgo.Symbol //HRESULT(IDWriteFontFace1 *This,	UINT32 glyph_count,	const UINT16 *indices,	INT32 *advances,	WINBOOL is_sideways)
	GetGdiCompatibleGlyphAdvances                cgo.Symbol //HRESULT(IDWriteFontFace1 *This,	FLOAT em_size,	FLOAT pixels_per_dip,	const DWRITE_MATRIX *transform,	WINBOOL use_gdi_natural,	WINBOOL is_sideways,	UINT32 glyph_count,	const UINT16 *indices,	INT32 *advances)
	GetKerningPairAdjustments                    cgo.Symbol //HRESULT(IDWriteFontFace1 *This,	UINT32 glyph_count,	const UINT16 *indices,	INT32 *adjustments)
	HasKerningPairs                              cgo.Symbol //WINBOOL(IDWriteFontFace1 *This)
	IDWriteFontFace1_GetRecommendedRenderingMode cgo.Symbol //HRESULT(IDWriteFontFace1 *This,	FLOAT font_emsize,	FLOAT dpiX,	FLOAT dpiY,	const DWRITE_MATRIX *transform,	WINBOOL is_sideways,	DWRITE_OUTLINE_THRESHOLD threshold,	DWRITE_MEASURING_MODE measuring_mode,	DWRITE_RENDERING_MODE *rendering_mode)
	GetVerticalGlyphVariants                     cgo.Symbol //HRESULT(IDWriteFontFace1 *This,	UINT32 glyph_count,	const UINT16 *nominal_indices,	UINT16 *vertical_indices)
	HasVerticalGlyphVariants                     cgo.Symbol //WINBOOL(IDWriteFontFace1 *This)
}

type FontFace1 struct {
	FontFace
}

var IID_IDWriteFontFace2 = com.DefineGuid(0xd8b768ff, 0x64bc, 0x4e66, 0x98, 0x2b, 0xec, 0x8e, 0x87, 0xf6, 0x93, 0xf7)

type FontFace2Class struct {
	FontFace1Class
	IsColorFont                                  cgo.Symbol //WINBOOL(IDWriteFontFace2 *This)
	GetColorPaletteCount                         cgo.Symbol //UINT32(IDWriteFontFace2 *This)
	GetPaletteEntryCount                         cgo.Symbol //UINT32(IDWriteFontFace2 *This)
	GetPaletteEntries                            cgo.Symbol //HRESULT(IDWriteFontFace2 *This,	UINT32 palette_index,	UINT32 first_entry_index,	UINT32 entry_count,	DWRITE_COLOR_F *entries)
	IDWriteFontFace2_GetRecommendedRenderingMode cgo.Symbol //HRESULT(IDWriteFontFace2 *This,	FLOAT fontEmSize,	FLOAT dpiX,	FLOAT dpiY,	const DWRITE_MATRIX *transform,	WINBOOL is_sideways,	DWRITE_OUTLINE_THRESHOLD threshold,	DWRITE_MEASURING_MODE measuringmode,	IDWriteRenderingParams *params,	DWRITE_RENDERING_MODE *renderingmode,	DWRITE_GRID_FIT_MODE *gridfitmode)
}

type FontFace2 struct {
	FontFace1
}

var IID_IDWriteFontFace3 = com.DefineGuid(0xd37d7598, 0x09be, 0x4222, 0xa2, 0x36, 0x20, 0x81, 0x34, 0x1c, 0xc1, 0xf2)

type FontFace3Class struct {
	FontFace2Class
	GetFontFaceReference                         cgo.Symbol //HRESULT(IDWriteFontFace3 *This,	IDWriteFontFaceReference **reference)
	GetPanose                                    cgo.Symbol //void(IDWriteFontFace3 *This,	DWRITE_PANOSE *panose)
	GetWeight                                    cgo.Symbol //DWRITE_FONT_WEIGHT(IDWriteFontFace3 *This)
	GetStretch                                   cgo.Symbol //DWRITE_FONT_STRETCH(IDWriteFontFace3 *This)
	GetStyle                                     cgo.Symbol //DWRITE_FONT_STYLE(IDWriteFontFace3 *This)
	GetFamilyNames                               cgo.Symbol //HRESULT(IDWriteFontFace3 *This,	IDWriteLocalizedStrings **names)
	GetFaceNames                                 cgo.Symbol //HRESULT(IDWriteFontFace3 *This,	IDWriteLocalizedStrings **names)
	GetInformationalStrings                      cgo.Symbol //HRESULT(IDWriteFontFace3 *This,	DWRITE_INFORMATIONAL_STRING_ID stringid,	IDWriteLocalizedStrings **strings,	WINBOOL *exists)
	HasCharacter                                 cgo.Symbol //WINBOOL(IDWriteFontFace3 *This,	UINT32 character)
	IDWriteFontFace3_GetRecommendedRenderingMode cgo.Symbol //HRESULT(IDWriteFontFace3 *This,	FLOAT emsize,	FLOAT dpi_x,	FLOAT dpi_y,	const DWRITE_MATRIX *transform,	WINBOOL is_sideways,	DWRITE_OUTLINE_THRESHOLD threshold,	DWRITE_MEASURING_MODE measuring_mode,	IDWriteRenderingParams *params,	DWRITE_RENDERING_MODE1 *rendering_mode,	DWRITE_GRID_FIT_MODE *gridfit_mode)
	IsCharacterLocal                             cgo.Symbol //WINBOOL(IDWriteFontFace3 *This,	UINT32 character)
	IsGlyphLocal                                 cgo.Symbol //WINBOOL(IDWriteFontFace3 *This,	UINT16 glyph)
	AreCharactersLocal                           cgo.Symbol //HRESULT(IDWriteFontFace3 *This,	const WCHAR *characters,	UINT32 count,	WINBOOL enqueue_if_not,	WINBOOL *are_local)
	AreGlyphsLocal                               cgo.Symbol //HRESULT(IDWriteFontFace3 *This,	const UINT16 *glyphs,	UINT32 count,	WINBOOL enqueue_if_not,	WINBOOL *are_local)
}

type FontFace3 struct {
	FontFace2
}

var IID_IDWriteFontFace4 = com.DefineGuid(0x27f2a904, 0x4eb8, 0x441d, 0x96, 0x78, 0x05, 0x63, 0xf5, 0x3e, 0x3e, 0x2f)

type FontFace4Class struct {
	FontFace3Class
	GetGlyphImageFormats_ cgo.Symbol //HRESULT(IDWriteFontFace4 *This,	UINT16 glyph,	UINT32 ppem_first,	UINT32 ppem_last,	DWRITE_GLYPH_IMAGE_FORMATS *formats)
	GetGlyphImageFormats  cgo.Symbol //DWRITE_GLYPH_IMAGE_FORMATS(IDWriteFontFace4 *This)
	GetGlyphImageData     cgo.Symbol //HRESULT(IDWriteFontFace4 *This,	UINT16 glyph,	UINT32 ppem,	DWRITE_GLYPH_IMAGE_FORMATS format,	DWRITE_GLYPH_IMAGE_DATA *data,	void **context)
	ReleaseGlyphImageData cgo.Symbol //void(IDWriteFontFace4 *This,	void *context)
}
type FontFace4 struct {
	FontFace3
}

var IID_IDWriteFontFace5 = com.DefineGuid(0x98eff3a5, 0xb667, 0x479a, 0xb1, 0x45, 0xe2, 0xfa, 0x5b, 0x9f, 0xdc, 0x29)

type FontFace5Class struct {
	FontFace4Class
	GetFontAxisValueCount cgo.Symbol //UINT32(IDWriteFontFace5 *This)
	GetFontAxisValues     cgo.Symbol //HRESULT(IDWriteFontFace5 *This,	DWRITE_FONT_AXIS_VALUE *values,	UINT32 value_count)
	HasVariations         cgo.Symbol //WINBOOL(IDWriteFontFace5 *This)
	GetFontResource       cgo.Symbol //HRESULT(IDWriteFontFace5 *This,	IDWriteFontResource **resource)
	Equals                cgo.Symbol //WINBOOL(IDWriteFontFace5 *This,	IDWriteFontFace *fontface)
}

type FontFace5 struct {
	FontFace4
}

var IID_IDWriteFontFile = com.DefineGuid(0x739d886a, 0xcef5, 0x47dc, 0x87, 0x69, 0x1a, 0x8b, 0x41, 0xbe, 0xbb, 0xb0)

type FontFileClass struct {
	com.UnknownClass
	GetReferenceKey cgo.Symbol //HRESULT(IDWriteFontFile *This,	const void **key,	UINT32 *key_size)
	GetLoader       cgo.Symbol //HRESULT(IDWriteFontFile *This,	IDWriteFontFileLoader **loader)
	Analyze         cgo.Symbol //HRESULT(IDWriteFontFile *This,	WINBOOL *is_supported_fonttype,	DWRITE_FONT_FILE_TYPE *file_type,	DWRITE_FONT_FACE_TYPE *face_type,	UINT32 *faces_num)
}

type FontFile struct {
	// TODO: add method
}

var IID_IDWriteFontList = com.DefineGuid(0x1a0d8438, 0x1d97, 0x4ec1, 0xae, 0xf9, 0xa2, 0xfb, 0x86, 0xed, 0x6a, 0xcb)

type FontListClass struct {
	com.UnknownClass
	GetFontCollection cgo.Symbol //HRESULT(IDWriteFontList *This, IDWriteFontCollection **collection)
	GetFontCount      cgo.Symbol //UINT32(IDWriteFontList *This)
	GetFont           cgo.Symbol //HRESULT(IDWriteFontList *This, UINT32 index, IDWriteFont **font)
}

type FontList struct {
	com.Unknown
}

var IID_IDWriteFontFamily = com.DefineGuid(0xda20d8ef, 0x812a, 0x4c43, 0x98, 0x02, 0x62, 0xec, 0x4a, 0xbd, 0x7a, 0xdd)

type FontFamilyClass struct {
	FontListClass
	GetFamilyNames       cgo.Symbol //HRESULT(IDWriteFontFamily *This, IDWriteLocalizedStrings **names)
	GetFirstMatchingFont cgo.Symbol //HRESULT(IDWriteFontFamily *This, DWRITE_FONT_WEIGHT weight, DWRITE_FONT_STRETCH stretch, DWRITE_FONT_STYLE style, IDWriteFont **font)
	GetMatchingFonts     cgo.Symbol //HRESULT(IDWriteFontFamily *This, DWRITE_FONT_WEIGHT weight, DWRITE_FONT_STRETCH stretch, DWRITE_FONT_STYLE style, IDWriteFontList **fonts)
}

type FontFamily struct {
	FontList
}

func (this *FontFamily) GetFamilyNames() (names *LocalizedStrings, hr com.HRESULT) {
	ret, _, _ := this.class().GetFamilyNames.CallRaw(uintptr(cgo.Pointer(this)), uintptr(cgo.Pointer(&names)))
	hr = com.HRESULT(ret)
	return
}

func (this *FontFamily) class() *FontFamilyClass {
	return (*FontFamilyClass)(this.Class)
}

var IID_IDWriteLocalizedStrings = com.DefineGuid(0x08256209, 0x099a, 0x4b34, 0xb8, 0x6d, 0xc2, 0x2b, 0x11, 0x0e, 0x77, 0x71)

type LocalizedStringsClass struct {
	com.UnknownClass
	GetCount            cgo.Symbol //UINT32(IDWriteLocalizedStrings *This)
	FindLocaleName      cgo.Symbol //HRESULT(IDWriteLocalizedStrings *This, const WCHAR *locale_name, UINT32 *index, WINBOOL *exists)
	GetLocaleNameLength cgo.Symbol //HRESULT(IDWriteLocalizedStrings *This, UINT32 index, UINT32 *length)
	GetLocaleName       cgo.Symbol //HRESULT(IDWriteLocalizedStrings *This, UINT32 index, WCHAR *locale_name, UINT32 size)
	GetStringLength     cgo.Symbol //HRESULT(IDWriteLocalizedStrings *This, UINT32 index, UINT32 *length)
	GetString           cgo.Symbol //HRESULT(IDWriteLocalizedStrings *This, UINT32 index, WCHAR *buffer, UINT32 size)
}

type LocalizedStrings struct {
	com.Unknown
}

func (this *LocalizedStrings) GetString(index int) string {
	length := this.GetStringLength(index)
	if length != 0 {
		buf := make([]uint16, length)
		ret, _, _ := this.class().GetString.CallRaw(uintptr(cgo.Pointer(this)), uintptr(index), uintptr(cgo.CSlice(buf)), uintptr(length))
		if ret == 0 {
			return syscall.UTF16ToString(buf)
		}
	}
	return ""
}

func (this *LocalizedStrings) GetStringLength(index int) (length int) {
	this.class().GetStringLength.CallRaw(uintptr(cgo.Pointer(this)), uintptr(index), uintptr(cgo.Pointer(&length)))
	return
}

func (this *LocalizedStrings) class() *LocalizedStringsClass {
	return (*LocalizedStringsClass)(this.Class)
}

var IID_IDWriteFontSetBuilder = com.DefineGuid(0x2f642afe, 0x9c68, 0x4f40, 0xb8, 0xbe, 0x45, 0x74, 0x01, 0xaf, 0xcb, 0x3d)

type FontSetBuilderClass struct {
	com.UnknownClass
	AddFontFaceReference_ cgo.Symbol //HRESULT(IDWriteFontSetBuilder *This, IDWriteFontFaceReference *ref, const DWRITE_FONT_PROPERTY *props, UINT32 prop_count)
	AddFontFaceReference  cgo.Symbol //HRESULT(IDWriteFontSetBuilder *This, IDWriteFontFaceReference *ref)

	AddFontSet    cgo.Symbol //HRESULT(IDWriteFontSetBuilder *This, IDWriteFontSet *fontset)
	CreateFontSet cgo.Symbol //HRESULT(IDWriteFontSetBuilder *This, IDWriteFontSet **fontset)
}

type FontSetBuilder struct {
	com.Unknown
}

func (this *FontSetBuilder) CreateFontSet() (fontSet *FontSet, hr com.HRESULT) {
	ret, _, _ := this.class().CreateFontSet.CallRaw(uintptr(cgo.Pointer(this)), uintptr(cgo.Pointer(&fontSet)))
	hr = com.HRESULT(ret)
	return
}

func (this *FontSetBuilder) class() *FontSetBuilderClass {
	return (*FontSetBuilderClass)(this.Class)
}

var IID_IDWriteFontSetBuilder1 = com.DefineGuid(0x3ff7715f, 0x3cdc, 0x4dc6, 0x9b, 0x72, 0xec, 0x56, 0x21, 0xdc, 0xca, 0xfd)

type FontSetBuilder1Class struct {
	FontSetBuilderClass
	AddFontFile cgo.Symbol //HRESULT(IDWriteFontSetBuilder1 *This, IDWriteFontFile *file)
}

type FontSetBuilder1 struct {
	FontSetBuilder
}

func (this *FontSetBuilder1) AddFontFile(file *FontFile) com.HRESULT {
	ret, _, _ := this.class().AddFontFile.CallRaw(uintptr(cgo.Pointer(this)), uintptr(cgo.Pointer(file)))
	return com.HRESULT(ret)
}

func (this *FontSetBuilder1) class() *FontSetBuilder1Class {
	return (*FontSetBuilder1Class)(this.Class)
}

var IID_IDWriteFontSet = com.DefineGuid(0x53585141, 0xd9f8, 0x4095, 0x83, 0x21, 0xd7, 0x3c, 0xf6, 0xbd, 0x11, 0x6b)

type FontSetClass struct {
	com.UnknownClass
	GetFontCount               cgo.Symbol //UINT32(IDWriteFontSet *This)
	GetFontFaceReference       cgo.Symbol //HRESULT(IDWriteFontSet *This,	UINT32 index,	IDWriteFontFaceReference **reference)
	FindFontFaceReference      cgo.Symbol //HRESULT(IDWriteFontSet *This,	IDWriteFontFaceReference *reference,	UINT32 *index,	WINBOOL *exists)
	FindFontFace               cgo.Symbol //HRESULT(IDWriteFontSet *This,	IDWriteFontFace *fontface,	UINT32 *index,	WINBOOL *exists)
	GetPropertyValues__        cgo.Symbol //HRESULT(IDWriteFontSet *This,	DWRITE_FONT_PROPERTY_ID id,	IDWriteStringList **values)
	GetPropertyValues_         cgo.Symbol //HRESULT(IDWriteFontSet *This,	DWRITE_FONT_PROPERTY_ID id,	const WCHAR *preferred_locales,	IDWriteStringList **values)
	GetPropertyValues          cgo.Symbol //HRESULT(IDWriteFontSet *This,	UINT32 index,	DWRITE_FONT_PROPERTY_ID id,	WINBOOL *exists,	IDWriteLocalizedStrings **values)
	GetPropertyOccurrenceCount cgo.Symbol //HRESULT(IDWriteFontSet *This,	const DWRITE_FONT_PROPERTY *property,	UINT32 *count)
	GetMatchingFonts_          cgo.Symbol //HRESULT(IDWriteFontSet *This,	const WCHAR *family,	DWRITE_FONT_WEIGHT weight,	DWRITE_FONT_STRETCH stretch,	DWRITE_FONT_STYLE style,	IDWriteFontSet **fontset)
	GetMatchingFonts           cgo.Symbol //HRESULT(IDWriteFontSet *This,	const DWRITE_FONT_PROPERTY *props,	UINT32 count,	IDWriteFontSet **fontset)

}

type FontSet struct {
	com.Unknown
}

func (this *FontSet) GetFontCount() int {
	ret, _, _ := this.class().GetFontCount.CallRaw(uintptr(cgo.Pointer(this)))
	return int(ret)
}

func (this *FontSet) GetFontFaceReference(index int) (ref *FontFaceReference, hr com.HRESULT) {
	ret, _, _ := this.class().GetFontFaceReference.CallRaw(uintptr(cgo.Pointer(this)), uintptr(index), uintptr(cgo.Pointer(&ref)))
	hr = com.HRESULT(ret)
	return
}

func (this *FontSet) class() *FontSetClass {
	return (*FontSetClass)(this.Class)
}

type Font struct {
	//TODO: apply
}

var IID_IDWriteFontFaceReference = com.DefineGuid(0x5e7fa7ca, 0xdde3, 0x424c, 0x89, 0xf0, 0x9f, 0xcd, 0x6f, 0xed, 0x58, 0xcd)

type FontFaceReferenceClass struct {
	com.UnknownClass
	CreateFontFace                     cgo.Symbol //HRESULT(IDWriteFontFaceReference *This,	IDWriteFontFace3 **fontface)
	CreateFontFaceWithSimulations      cgo.Symbol //HRESULT(IDWriteFontFaceReference *This,	DWRITE_FONT_SIMULATIONS simulations,	IDWriteFontFace3 **fontface)
	Equals                             cgo.Symbol //WINBOOL(IDWriteFontFaceReference *This,	IDWriteFontFaceReference *reference)
	GetFontFaceIndex                   cgo.Symbol //UINT32(IDWriteFontFaceReference *This)
	GetSimulations                     cgo.Symbol //DWRITE_FONT_SIMULATIONS(IDWriteFontFaceReference *This)
	GetFontFile                        cgo.Symbol //HRESULT(IDWriteFontFaceReference *This,	IDWriteFontFile **fontfile)
	GetLocalFileSize                   cgo.Symbol //UINT64(IDWriteFontFaceReference *This)
	GetFileSize                        cgo.Symbol //UINT64(IDWriteFontFaceReference *This)
	GetFileTime                        cgo.Symbol //HRESULT(IDWriteFontFaceReference *This,	FILETIME *writetime)
	GetLocality                        cgo.Symbol //DWRITE_LOCALITY(IDWriteFontFaceReference *This)
	EnqueueFontDownloadRequest         cgo.Symbol //HRESULT(IDWriteFontFaceReference *This)
	EnqueueCharacterDownloadRequest    cgo.Symbol //HRESULT(IDWriteFontFaceReference *This,	const WCHAR *chars,	UINT32 count)
	EnqueueGlyphDownloadRequest        cgo.Symbol //HRESULT(IDWriteFontFaceReference *This,	const UINT16 *glyphs,	UINT32 count)
	EnqueueFileFragmentDownloadRequest cgo.Symbol //HRESULT(IDWriteFontFaceReference *This,	UINT64 offset,	UINT64 size)
}

type FontFaceReference struct {
	com.Unknown
}

func (this *FontFaceReference) class() *FontFaceReferenceClass {
	return (*FontFaceReferenceClass)(this.Class)
}

var IID_IDWriteTextFormat = com.DefineGuid(0x9c906818, 0x31d7, 0x4fd3, 0xa1, 0x51, 0x7c, 0x5e, 0x22, 0x5d, 0xb5, 0x5a)

type TextFormatClass struct {
	com.UnknownClass
	SetTextAlignment        cgo.Symbol //HRESULT(IDWriteTextFormat *This,DWRITE_TEXT_ALIGNMENT alignment)
	SetParagraphAlignment   cgo.Symbol //HRESULT(IDWriteTextFormat *This,DWRITE_PARAGRAPH_ALIGNMENT alignment)
	SetWordWrapping         cgo.Symbol //HRESULT(IDWriteTextFormat *This,DWRITE_WORD_WRAPPING wrapping)
	SetReadingDirection     cgo.Symbol //HRESULT(IDWriteTextFormat *This,DWRITE_READING_DIRECTION direction)
	SetFlowDirection        cgo.Symbol //HRESULT(IDWriteTextFormat *This,DWRITE_FLOW_DIRECTION direction)
	SetIncrementalTabStop   cgo.Symbol //HRESULT(IDWriteTextFormat *This,FLOAT tabstop)
	SetTrimming             cgo.Symbol //HRESULT(IDWriteTextFormat *This,const DWRITE_TRIMMING *trimming,IDWriteInlineObject *trimming_sign)
	SetLineSpacing          cgo.Symbol //HRESULT(IDWriteTextFormat *This,DWRITE_LINE_SPACING_METHOD spacing,FLOAT line_spacing,FLOAT baseline)
	GetTextAlignment        cgo.Symbol //DWRITE_TEXT_ALIGNMENT (IDWriteTextFormat *This)
	GetParagraphAlignment   cgo.Symbol //DWRITE_PARAGRAPH_ALIGNMENT (IDWriteTextFormat *This)
	GetWordWrapping         cgo.Symbol //DWRITE_WORD_WRAPPING (IDWriteTextFormat *This)
	GetReadingDirection     cgo.Symbol //DWRITE_READING_DIRECTION (IDWriteTextFormat *This)
	GetFlowDirection        cgo.Symbol //DWRITE_FLOW_DIRECTION (IDWriteTextFormat *This)
	GetIncrementalTabStop   cgo.Symbol //FLOAT (IDWriteTextFormat *This)
	GetTrimming             cgo.Symbol //HRESULT (IDWriteTextFormat *This,DWRITE_TRIMMING *options,IDWriteInlineObject **trimming_sign)
	GetLineSpacing          cgo.Symbol //HRESULT (IDWriteTextFormat *This,DWRITE_LINE_SPACING_METHOD *method,FLOAT *spacing,FLOAT *baseline)
	GetFontCollection       cgo.Symbol //HRESULT (IDWriteTextFormat *This,IDWriteFontCollection **collection)
	GetFontFamilyNameLength cgo.Symbol //UINT32 (IDWriteTextFormat *This)
	GetFontFamilyName       cgo.Symbol //HRESULT (IDWriteTextFormat *This,WCHAR *name,UINT32 size)
	GetFontWeight           cgo.Symbol //DWRITE_FONT_WEIGHT (IDWriteTextFormat *This)
	GetFontStyle            cgo.Symbol //DWRITE_FONT_STYLE (IDWriteTextFormat *This)
	GetFontStretch          cgo.Symbol //DWRITE_FONT_STRETCH (IDWriteTextFormat *This)
	GetFontSize             cgo.Symbol //FLOAT (IDWriteTextFormat *This)
	GetLocaleNameLength     cgo.Symbol //UINT32 (IDWriteTextFormat *This)
	GetLocaleName           cgo.Symbol //HRESULT (IDWriteTextFormat *This,WCHAR *name,UINT32 size)
}

type TextFormat struct {
	com.Unknown
}

func (this *TextFormat) SetTextAlignment(alignment TextAlignment) com.HRESULT {
	ret, _, _ := this.class().SetTextAlignment.CallRaw(uintptr(cgo.Pointer(this)), uintptr(alignment))
	return com.HRESULT(ret)
}

func (this *TextFormat) SetParagraphAlignment(alignment ParagraphAlignment) com.HRESULT {
	ret, _, _ := this.class().SetParagraphAlignment.CallRaw(uintptr(cgo.Pointer(this)), uintptr(alignment))
	return com.HRESULT(ret)
}

func (this *TextFormat) SetWordWrapping(wrapping WordWrapping) com.HRESULT {
	ret, _, _ := this.class().SetWordWrapping.CallRaw(uintptr(cgo.Pointer(this)), uintptr(wrapping))
	return com.HRESULT(ret)
}

func (this *TextFormat) SetLineSpacing(spacingMethod LineSpacingMethod, spacing, baseline float32) com.HRESULT {
	return cgo.CallRet[com.HRESULT](this.class().SetLineSpacing, this, spacingMethod, spacing, baseline)
}

func (this *TextFormat) class() *TextFormatClass {
	return (*TextFormatClass)(this.Class)
}

var IID_IDWriteTextLayout = com.DefineGuid(0x53737037, 0x6d14, 0x410b, 0x9b, 0xfe, 0x0b, 0x18, 0x2b, 0xb7, 0x09, 0x61)

type TextLayoutClass struct {
	TextFormatClass
	SetMaxWidth                               cgo.Symbol //HRESULT(IDWriteTextLayout *This, FLOAT maxWidth)
	SetMaxHeight                              cgo.Symbol //HRESULT(IDWriteTextLayout *This, FLOAT maxHeight)
	SetFontCollection                         cgo.Symbol //HRESULT(IDWriteTextLayout *This, IDWriteFontCollection *collection,    DWRITE_TEXT_RANGE range)
	SetFontFamilyName                         cgo.Symbol //HRESULT(IDWriteTextLayout *This, const WCHAR *name,    DWRITE_TEXT_RANGE range)
	SetFontWeight                             cgo.Symbol //HRESULT(IDWriteTextLayout *This, DWRITE_FONT_WEIGHT weight,    DWRITE_TEXT_RANGE range)
	SetFontStyle                              cgo.Symbol //HRESULT(IDWriteTextLayout *This, DWRITE_FONT_STYLE style,    DWRITE_TEXT_RANGE range)
	SetFontStretch                            cgo.Symbol //HRESULT(IDWriteTextLayout *This, DWRITE_FONT_STRETCH stretch,    DWRITE_TEXT_RANGE range)
	SetFontSize                               cgo.Symbol //HRESULT(IDWriteTextLayout *This, FLOAT size,    DWRITE_TEXT_RANGE range)
	SetUnderline                              cgo.Symbol //HRESULT(IDWriteTextLayout *This, WINBOOL underline,    DWRITE_TEXT_RANGE range)
	SetStrikethrough                          cgo.Symbol //HRESULT(IDWriteTextLayout *This, WINBOOL strikethrough,    DWRITE_TEXT_RANGE range)
	SetDrawingEffect                          cgo.Symbol //HRESULT(IDWriteTextLayout *This, IUnknown *effect,    DWRITE_TEXT_RANGE range)
	SetInlineObject                           cgo.Symbol //HRESULT(IDWriteTextLayout *This, IDWriteInlineObject *object,    DWRITE_TEXT_RANGE range)
	SetTypography                             cgo.Symbol //HRESULT(IDWriteTextLayout *This, IDWriteTypography *typography,    DWRITE_TEXT_RANGE range)
	SetLocaleName                             cgo.Symbol //HRESULT(IDWriteTextLayout *This, const WCHAR *locale,    DWRITE_TEXT_RANGE range)
	GetMaxWidth                               cgo.Symbol //FLOAT(IDWriteTextLayout *This)
	GetMaxHeight                              cgo.Symbol //FLOAT(IDWriteTextLayout *This)
	IDWriteTextLayout_GetFontCollection       cgo.Symbol //HRESULT(IDWriteTextLayout *This, UINT32 pos,    IDWriteFontCollection **collection,    DWRITE_TEXT_RANGE *range)
	IDWriteTextLayout_GetFontFamilyNameLength cgo.Symbol //HRESULT(IDWriteTextLayout *This, UINT32 pos,    UINT32 *len,    DWRITE_TEXT_RANGE *range)
	IDWriteTextLayout_GetFontFamilyName       cgo.Symbol //HRESULT(IDWriteTextLayout *This, UINT32 position,    WCHAR *name,    UINT32 name_size,    DWRITE_TEXT_RANGE *range)
	IDWriteTextLayout_GetFontWeight           cgo.Symbol //HRESULT(IDWriteTextLayout *This, UINT32 position,    DWRITE_FONT_WEIGHT *weight,    DWRITE_TEXT_RANGE *range)
	IDWriteTextLayout_GetFontStyle            cgo.Symbol //HRESULT(IDWriteTextLayout *This, UINT32 currentPosition,    DWRITE_FONT_STYLE *style,    DWRITE_TEXT_RANGE *range)
	IDWriteTextLayout_GetFontStretch          cgo.Symbol //HRESULT(IDWriteTextLayout *This, UINT32 position,    DWRITE_FONT_STRETCH *stretch,    DWRITE_TEXT_RANGE *range)
	IDWriteTextLayout_GetFontSize             cgo.Symbol //HRESULT(IDWriteTextLayout *This, UINT32 position,    FLOAT *size,    DWRITE_TEXT_RANGE *range)
	GetUnderline                              cgo.Symbol //HRESULT(IDWriteTextLayout *This, UINT32 position,    WINBOOL *has_underline,    DWRITE_TEXT_RANGE *range)
	GetStrikethrough                          cgo.Symbol //HRESULT(IDWriteTextLayout *This, UINT32 position,    WINBOOL *has_strikethrough,    DWRITE_TEXT_RANGE *range)
	GetDrawingEffect                          cgo.Symbol //HRESULT(IDWriteTextLayout *This, UINT32 position,    IUnknown **effect,    DWRITE_TEXT_RANGE *range)
	GetInlineObject                           cgo.Symbol //HRESULT(IDWriteTextLayout *This, UINT32 position,    IDWriteInlineObject **object,    DWRITE_TEXT_RANGE *range)
	GetTypography                             cgo.Symbol //HRESULT(IDWriteTextLayout *This, UINT32 position,    IDWriteTypography **typography,    DWRITE_TEXT_RANGE *range)
	IDWriteTextLayout_GetLocaleNameLength     cgo.Symbol //HRESULT(IDWriteTextLayout *This, UINT32 position,    UINT32 *length,    DWRITE_TEXT_RANGE *range)
	IDWriteTextLayout_GetLocaleName           cgo.Symbol //HRESULT(IDWriteTextLayout *This, UINT32 position,    WCHAR *name,    UINT32 name_size,    DWRITE_TEXT_RANGE *range)
	Draw                                      cgo.Symbol //HRESULT(IDWriteTextLayout *This, void *context,    IDWriteTextRenderer *renderer,    FLOAT originX,    FLOAT originY)
	GetLineMetrics                            cgo.Symbol //HRESULT(IDWriteTextLayout *This, DWRITE_LINE_METRICS *metrics,    UINT32 max_count,    UINT32 *actual_count)
	GetMetrics                                cgo.Symbol //HRESULT(IDWriteTextLayout *This, DWRITE_TEXT_METRICS *metrics)
	GetOverhangMetrics                        cgo.Symbol //HRESULT(IDWriteTextLayout *This, DWRITE_OVERHANG_METRICS *overhangs)
	GetClusterMetrics                         cgo.Symbol //HRESULT(IDWriteTextLayout *This, DWRITE_CLUSTER_METRICS *metrics,    UINT32 max_count,    UINT32 *act_count)
	DetermineMinWidth                         cgo.Symbol //HRESULT(IDWriteTextLayout *This, FLOAT *min_width)
	HitTestPoint                              cgo.Symbol //HRESULT(IDWriteTextLayout *This, FLOAT pointX,    FLOAT pointY,    WINBOOL *is_trailinghit,    WINBOOL *is_inside,    DWRITE_HIT_TEST_METRICS *metrics)
	HitTestTextPosition                       cgo.Symbol //HRESULT(IDWriteTextLayout *This, UINT32 textPosition,    WINBOOL is_trailinghit,    FLOAT *pointX,    FLOAT *pointY,    DWRITE_HIT_TEST_METRICS *metrics)
	HitTestTextRange                          cgo.Symbol //HRESULT(IDWriteTextLayout *This, UINT32 textPosition,    UINT32 textLength,    FLOAT originX,    FLOAT originY,    DWRITE_HIT_TEST_METRICS *metrics,    UINT32 max_metricscount,    UINT32 *actual_metricscount)
}

type TextLayout struct {
	TextFormat
}

func (this *TextLayout) SetMaxWidth(maxWidth float32) com.HRESULT {
	return cgo.CallRet[com.HRESULT](this.class().SetMaxWidth, this, maxWidth)
}

func (this *TextLayout) SetMaxHeight(maxHeight float32) com.HRESULT {
	return cgo.CallRet[com.HRESULT](this.class().SetMaxHeight, this, maxHeight)
}

func (this *TextLayout) SetFontCollection(collection *FontCollection, textRange TextRange) com.HRESULT {
	return cgo.CallRet[com.HRESULT](this.class().SetFontCollection, this, collection, textRange)
}

func (this *TextLayout) SetFontFamilyName(familyName string, textRange TextRange) com.HRESULT {
	wFamily, _ := syscall.UTF16PtrFromString(familyName)
	ret := cgo.CallRet[com.HRESULT](this.class().SetFontFamilyName, this, wFamily, textRange)
	runtime.KeepAlive(wFamily)
	return ret
}

func (this *TextLayout) SetFontWeight(weight FontWeight, textRange TextRange) com.HRESULT {
	return cgo.CallRet[com.HRESULT](this.class().SetFontWeight, this, weight, textRange)
}

func (this *TextLayout) SetFontStyle(style FontStyle, textRange TextRange) com.HRESULT {
	return cgo.CallRet[com.HRESULT](this.class().SetFontStyle, this, style, textRange)
}

func (this *TextLayout) SetFontStretch(stretch FontStretch, textRange TextRange) com.HRESULT {
	return cgo.CallRet[com.HRESULT](this.class().SetFontStretch, this, stretch, textRange)
}

func (this *TextLayout) SetFontSize(size float32, textRange TextRange) com.HRESULT {
	return cgo.CallRet[com.HRESULT](this.class().SetFontSize, this, size, textRange)
}

func (this *TextLayout) SetUnderline(underline bool, textRange TextRange) com.HRESULT {
	return cgo.CallRet[com.HRESULT](this.class().SetUnderline, this, underline, textRange)
}

func (this *TextLayout) SetStrikethrough(strike bool, textRange TextRange) com.HRESULT {
	return cgo.CallRet[com.HRESULT](this.class().SetStrikethrough, this, strike, textRange)
}

func (this *TextLayout) SetDrawingEffect(effect *com.Unknown, textRange TextRange) com.HRESULT {
	return cgo.CallRet[com.HRESULT](this.class().SetDrawingEffect, this, effect, textRange)
}

func (this *TextLayout) GetMetrics() (metrics TextMetrics, hr com.HRESULT) {
	ret, _, _ := this.class().GetMetrics.CallRaw(uintptr(cgo.Pointer(this)), uintptr(cgo.Pointer(&metrics)))
	hr = com.HRESULT(ret)
	return
}

func (this *TextLayout) GetLineMetrics() (lineMetrics []LineMetrics, hr com.HRESULT) {
	var count int
	this.class().GetLineMetrics.CallRaw(uintptr(cgo.Pointer(this)), 0, 0, uintptr(cgo.Pointer(&count)))
	lineMetrics = make([]LineMetrics, count)
	ret, _, _ := this.class().GetLineMetrics.CallRaw(uintptr(cgo.Pointer(this)), uintptr(cgo.CSlice(lineMetrics)), uintptr(len(lineMetrics)), uintptr(cgo.Pointer(&count)))
	hr = com.HRESULT(ret)
	return
}

func (this *TextLayout) GetClusterMetrics() (clusterMetrics []ClusterMetrics, hr com.HRESULT) {
	var count int
	this.class().GetClusterMetrics.CallRaw(uintptr(cgo.Pointer(this)), 0, 0, uintptr(cgo.Pointer(&count)))
	clusterMetrics = make([]ClusterMetrics, count)
	ret, _, _ := this.class().GetClusterMetrics.CallRaw(uintptr(cgo.Pointer(this)), uintptr(cgo.CSlice(clusterMetrics)), uintptr(len(clusterMetrics)), uintptr(cgo.Pointer(&count)))
	hr = com.HRESULT(ret)
	return
}

func (this *TextLayout) DetermineMinWidth() (minWidth float32) {
	this.class().DetermineMinWidth.CallRaw(uintptr(cgo.Pointer(this)), uintptr(cgo.Pointer(&minWidth)))
	return
}

func (this *TextLayout) HitTestPoint(x, y float32, isTrialing bool) (isInside bool, hitTestMetrics HitTestMetrics, hr com.HRESULT) {
	hr = cgo.CallRet[com.HRESULT](this.class().HitTestPoint, this, x, y, isTrialing, &isInside, &hitTestMetrics)
	return
}

func (this *TextLayout) HitTestTextPosition(position int, isTrialing bool) (x, y float32, hitTestMetrics HitTestMetrics, hr com.HRESULT) {
	hr = cgo.CallRet[com.HRESULT](this.class().HitTestTextPosition, this, position, isTrialing, &x, &y, &hitTestMetrics)
	return
}

func (this *TextLayout) HitTestTextRange(position, length int, x, y float32) (hitTestMetrics []HitTestMetrics, hr com.HRESULT) {
	var count int
	cgo.Call(this.class().HitTestTextRange, this, position, length, x, y, 0, 0, &count)
	hitTestMetrics = make([]HitTestMetrics, count)
	hr = cgo.CallRet[com.HRESULT](this.class().HitTestTextRange, this, position, length, x, y, cgo.CSlice(hitTestMetrics), len(hitTestMetrics), &count)
	return
}

func (this *TextLayout) class() *TextLayoutClass {
	return (*TextLayoutClass)(this.Class)
}

type TextRange struct {
	StartPosition uint32
	Length        uint32
}

type TextMetrics struct {
	Left                             float32
	Top                              float32
	Width                            float32
	WidthIncludingTrailingWhitespace float32
	Height                           float32
	LayoutWidth                      float32
	LayoutHeight                     float32
	MaxBidiReorderingDepth           uint32
	LineCount                        uint32
}

type LineMetrics struct {
	Length                   uint32
	TrailingWhitespaceLength uint32
	NewlineLength            uint32
	Height                   float32
	Baseline                 float32
	IsTrimmed                winapi.BOOL
}

type HitTestMetrics struct {
	TextPosition uint32
	Length       uint32
	Left         float32
	Top          float32
	Width        float32
	Height       float32
	BidLevel     float32
	IsText       winapi.BOOL
	IsTrimmed    winapi.BOOL
}

type ClusterMetrics struct {
	Width  float32
	Length uint16
	bits   uint16
}

func (m ClusterMetrics) CanWrapLineAfter() bool {
	return m.bits&1 != 0
}

func (m ClusterMetrics) IsWhitespace() bool {
	return m.bits&(1<<1) != 0
}

func (m ClusterMetrics) IsNewLine() bool {
	return m.bits&(1<<2) != 0
}

func (m ClusterMetrics) IsSoftHyphen() bool {
	return m.bits&(1<<3) != 0
}

func (m ClusterMetrics) IsRightToLeft() bool {
	return m.bits&(1<<4) != 0
}

type UnicodeRange struct {
	First rune
	Last  rune
}

type FontAxisValue struct {
	AxisTag FontAxisTag
	Value   float32
}
