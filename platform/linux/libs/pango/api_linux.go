package pango

import (
	"runtime"

	"github.com/goexlib/cgo"
	"github.com/golang-gui/goui/platform/linux/libs/fontconfig"
	"github.com/golang-gui/goui/platform/linux/libs/glib"
)

var (
	libpango = cgo.NewLazyLibrary("libpango-1.0.so.0")

	// PangoLayout
	pangoLayoutNew                = libpango.NewSymbol("pango_layout_new")
	pangoLayoutSetText            = libpango.NewSymbol("pango_layout_set_text")
	pangoLayoutSetWidth           = libpango.NewSymbol("pango_layout_set_width")
	pangoLayoutSetWrap            = libpango.NewSymbol("pango_layout_set_wrap")
	pangoLayoutSetAlignment       = libpango.NewSymbol("pango_layout_set_alignment")
	pangoLayoutSetJustify         = libpango.NewSymbol("pango_layout_set_justify")
	pangoLayoutSetAutoDir         = libpango.NewSymbol("pango_layout_set_auto_dir")
	pangoLayoutSetSpacing         = libpango.NewSymbol("pango_layout_set_spacing")
	pangoLayoutSetAttributes      = libpango.NewSymbol("pango_layout_set_attributes")
	pangoLayoutSetFontDescription = libpango.NewSymbol("pango_layout_set_font_description")
	pangoLayoutGetContext         = libpango.NewSymbol("pango_layout_get_context")
	pangoLayoutGetSize            = libpango.NewSymbol("pango_layout_get_size")
	pangoLayoutGetIter            = libpango.NewSymbol("pango_layout_get_iter")
	pangoLayoutGetExtents         = libpango.NewSymbol("pango_layout_get_extents")
	pangoLayoutGetLineCount       = libpango.NewSymbol("pango_layout_get_line_count")
	pangoLayoutGetLineReadonly    = libpango.NewSymbol("pango_layout_get_line_readonly")
	pangoLayoutGetCursorPos       = libpango.NewSymbol("pango_layout_get_cursor_pos")
	pangoLayoutIndexToLineX       = libpango.NewSymbol("pango_layout_index_to_line_x")
	pangoLayoutContextChanged     = libpango.NewSymbol("pango_layout_context_changed")

	// PangoLayoutIter
	pangoLayoutIterGetIndex          = libpango.NewSymbol("pango_layout_iter_get_index")
	pangoLayoutIterGetBaseline       = libpango.NewSymbol("pango_layout_iter_get_baseline")
	pangoLayoutIterGetLineReadonly   = libpango.NewSymbol("pango_layout_iter_get_line_readonly")
	pangoLayoutIterGetLineExtents    = libpango.NewSymbol("pango_layout_iter_get_line_extents")
	pangoLayoutIterGetRunReadonly    = libpango.NewSymbol("pango_layout_iter_get_run_readonly")
	pangoLayoutIterGetClusterExtents = libpango.NewSymbol("pango_layout_iter_get_cluster_extents")
	pangoLayoutIterNextLine          = libpango.NewSymbol("pango_layout_iter_next_line")
	pangoLayoutIterNextCluster       = libpango.NewSymbol("pango_layout_iter_next_cluster")

	// PangoLayoutLine
	pangoLayoutLineGetExtents = libpango.NewSymbol("pango_layout_line_get_extents")
	pangoLayoutLineXToIndex   = libpango.NewSymbol("pango_layout_line_x_to_index")

	// PangoAttrList
	pangoAttrListNew    = libpango.NewSymbol("pango_attr_list_new")
	pangoAttrListUnref  = libpango.NewSymbol("pango_attr_list_unref")
	pangoAttrListInsert = libpango.NewSymbol("pango_attr_list_insert")

	// PangoAttribute constructors
	pangoAttributeDestroy     = libpango.NewSymbol("pango_attribute_destroy")
	pangoAttrFamilyNew        = libpango.NewSymbol("pango_attr_family_new")
	pangoAttrSizeNewAbsolute  = libpango.NewSymbol("pango_attr_size_new_absolute")
	pangoAttrWeightNew        = libpango.NewSymbol("pango_attr_weight_new")
	pangoAttrStretchNew       = libpango.NewSymbol("pango_attr_stretch_new")
	pangoAttrFontFeaturesNew  = libpango.NewSymbol("pango_attr_font_features_new")
	pangoAttrForegroundNew    = libpango.NewSymbol("pango_attr_foreground_new")
	pangoAttrUnderlineNew     = libpango.NewSymbol("pango_attr_underline_new")
	pangoAttrStrikethroughNew = libpango.NewSymbol("pango_attr_strikethrough_new")

	// PangoFontDescription
	pangoFontDescriptionNew             = libpango.NewSymbol("pango_font_description_new")
	pangoFontDescriptionFree            = libpango.NewSymbol("pango_font_description_free")
	pangoFontDescriptionSetFamily       = libpango.NewSymbol("pango_font_description_set_family")
	pangoFontDescriptionSetSize         = libpango.NewSymbol("pango_font_description_set_size")
	pangoFontDescriptionSetAbsoluteSize = libpango.NewSymbol("pango_font_description_set_absolute_size")

	// PangoContext
	pangoContextSetFontDescription = libpango.NewSymbol("pango_context_set_font_description")
	pangoContextSetLanguage        = libpango.NewSymbol("pango_context_set_language")
	pangoContextSetBaseDir         = libpango.NewSymbol("pango_context_set_base_dir")
	pangoContextSetFontMap         = libpango.NewSymbol("pango_context_set_font_map")

	// PangoLanguage
	pangoLanguageFromString = libpango.NewSymbol("pango_language_from_string")

	// PangoFontMap
	pangoFontMapCreateContext = libpango.NewSymbol("pango_font_map_create_context")
	pangoFcFontMapGetConfig   = libpango.NewSymbol("pango_fc_font_map_get_config")
	pangoFcFontMapSetConfig   = libpango.NewSymbol("pango_fc_font_map_set_config")
)

// --- PangoLayout ---

type Layout struct {
	glib.Object
}

// LayoutNew creates a new PangoLayout object with attributes initialized to default values.
func LayoutNew(context Context) (l Layout) {
	// PangoLayout* pango_layout_new(PangoContext* context)
	l.GObject, _, _ = pangoLayoutNew.CallRaw(context.GObject)
	return
}

// SetText sets the text of the layout.
func (l Layout) SetText(text string) {
	// void pango_layout_set_text(PangoLayout* layout, const char* text, int length)
	cText := cgo.CString(text)
	pangoLayoutSetText.CallRaw(l.GObject, uintptr(cText), uintptr(len(text)))
	runtime.KeepAlive(cText)
}

func (l Layout) SetFontDescription(desc FontDescription) {
	pangoLayoutSetFontDescription.CallRaw(l.GObject, uintptr(desc))
}

// SetWidth sets the width to which the lines of the PangoLayout should wrap or ellipsized.
// width is in Pango units (pixels * Scale). Pass -1 to disable wrapping.
func (l Layout) SetWidth(width int) {
	// void pango_layout_set_width(PangoLayout* layout, int width)
	pangoLayoutSetWidth.CallRaw(l.GObject, uintptr(width))
}

// SetWrap sets the wrap mode.
func (l Layout) SetWrap(wrap WrapMode) {
	// void pango_layout_set_wrap(PangoLayout* layout, PangoWrapMode wrap)
	pangoLayoutSetWrap.CallRaw(l.GObject, uintptr(wrap))
}

// SetAlignment sets the alignment for the layout.
func (l Layout) SetAlignment(alignment Alignment) {
	// void pango_layout_set_alignment(PangoLayout* layout, PangoAlignment alignment)
	pangoLayoutSetAlignment.CallRaw(l.GObject, uintptr(alignment))
}

func (l Layout) SetJustify(justify bool) {
	pangoLayoutSetJustify.CallRaw(l.GObject, uintptr(cgo.CBool(justify)))
}

// SetAutoDir sets whether to calculate the bidirectional base direction for the layout automatically.
func (l Layout) SetAutoDir(autoDir bool) {
	// void pango_layout_set_auto_dir(PangoLayout* layout, gboolean auto_dir)
	pangoLayoutSetAutoDir.CallRaw(l.GObject, uintptr(cgo.CBool(autoDir)))
}

// SetSpacing sets the amount of spacing in Pango units between the lines of the layout.
func (l Layout) SetSpacing(spacing int) {
	// void pango_layout_set_spacing(PangoLayout* layout, int spacing)
	pangoLayoutSetSpacing.CallRaw(l.GObject, uintptr(spacing))
}

// SetAttributes sets the text attributes for a layout object.
func (l Layout) SetAttributes(attrs AttrList) {
	// void pango_layout_set_attributes(PangoLayout* layout, PangoAttrList* attrs)
	pangoLayoutSetAttributes.CallRaw(l.GObject, uintptr(attrs))
}

// GetContext retrieves the PangoContext used for this layout.
func (l Layout) GetContext() (c Context) {
	// PangoContext* pango_layout_get_context(PangoLayout* layout)
	c.GObject, _, _ = pangoLayoutGetContext.CallRaw(l.GObject)
	return
}

// GetSize determines the logical width and height of a PangoLayout in Pango units.
func (l Layout) GetSize() (width, height int) {
	// void pango_layout_get_size(PangoLayout* layout, int* width, int* height)
	var w, h int32
	pangoLayoutGetSize.CallRaw(l.GObject, uintptr(cgo.Pointer(&w)), uintptr(cgo.Pointer(&h)))
	return int(w), int(h)
}

// GetExtents computes the ink and logical extents of the layout.
func (l Layout) GetExtents() (inkRect, logicalRect Rectangle) {
	// void pango_layout_get_extents(PangoLayout* layout, PangoRectangle* ink_rect, PangoRectangle* logical_rect)
	pangoLayoutGetExtents.CallRaw(l.GObject, uintptr(cgo.Pointer(&inkRect)), uintptr(cgo.Pointer(&logicalRect)))
	return
}

func (l Layout) GetIter() LayoutIter {
	// PangoLayoutIter* pango_layout_get_iter(PangoLayout* layout)
	ret, _, _ := pangoLayoutGetIter.CallRaw(l.GObject)
	return LayoutIter(ret)
}

// GetLineCount retrieves the count of lines for the layout.
func (l Layout) GetLineCount() int {
	// int pango_layout_get_line_count(PangoLayout* layout)
	ret, _, _ := pangoLayoutGetLineCount.CallRaw(l.GObject)
	return int(ret)
}

// GetLineReadonly retrieves a particular line from a layout.
// This is a faster alternative to GetLine, but the caller is not expected to modify the contents.
func (l Layout) GetLineReadonly(line int) *LayoutLine {
	// PangoLayoutLine* pango_layout_get_line_readonly(PangoLayout* layout, int line)
	ret, _, _ := pangoLayoutGetLineReadonly.CallRaw(l.GObject, uintptr(line))
	return goPointer[LayoutLine](ret)
}

// GetCursorPos determines the positions of the strong and weak cursors if the insertion point is at the given index.
// inkRect and logicalRect are in layout coordinates (origin at top-left of layout).
func (l Layout) GetCursorPos(index int) (strongPos, weakPos Rectangle) {
	// void pango_layout_get_cursor_pos(PangoLayout* layout, int index_, PangoRectangle* strong_pos, PangoRectangle* weak_pos)
	pangoLayoutGetCursorPos.CallRaw(l.GObject, uintptr(index),
		uintptr(cgo.Pointer(&strongPos)), uintptr(cgo.Pointer(&weakPos)))
	return
}

// IndexToLineX converts from character position to x position.
// trailing: if true, the trailing edge of the character at index is used.
func (l Layout) IndexToLineX(index int, trailing bool) (line, xPos int) {
	// void pango_layout_index_to_line_x(PangoLayout* layout, int index_, gboolean trailing, int* line, int* x_pos)
	var lineOut, xPosOut int32
	pangoLayoutIndexToLineX.CallRaw(l.GObject, uintptr(index), uintptr(cgo.CBool(trailing)),
		uintptr(cgo.Pointer(&lineOut)), uintptr(cgo.Pointer(&xPosOut)))
	return int(lineOut), int(xPosOut)
}

// ContextChanged forces recomputation of any state in the PangoLayout that might depend on the layout's context.
func (l Layout) ContextChanged() {
	// void pango_layout_context_changed(PangoLayout* layout)
	pangoLayoutContextChanged.CallRaw(l.GObject)
}

// --- PangoLayoutIter ---

type LayoutIter uintptr

func (iter LayoutIter) GetIndex() int {
	// int pango_layout_iter_get_index(PangoLayoutIter* iter)
	ret, _, _ := pangoLayoutIterGetIndex.CallRaw(uintptr(iter))
	return int(ret)
}

func (iter LayoutIter) GetBaseline() int {
	// int pango_layout_iter_get_baseline(PangoLayoutIter* iter)
	ret, _, _ := pangoLayoutIterGetBaseline.CallRaw(uintptr(iter))
	return int(ret)
}

func (iter LayoutIter) GetLineReadonly() *LayoutLine {
	// PangoLayoutLine* pango_layout_iter_get_line_readonly(PangoLayoutIter* iter)
	ret, _, _ := pangoLayoutIterGetLineReadonly.CallRaw(uintptr(iter))
	return goPointer[LayoutLine](ret)
}

func (iter LayoutIter) GetLineExtents() (inkRect, logicalRect Rectangle) {
	// void pango_layout_iter_get_line_extents(PangoLayoutIter* iter, PangoRectangle* ink_rect, PangoRectangle* logical_rect)
	pangoLayoutIterGetLineExtents.CallRaw(uintptr(iter), uintptr(cgo.Pointer(&inkRect)), uintptr(cgo.Pointer(&logicalRect)))
	return
}

func (iter LayoutIter) GetRunReadonly() *LayoutRun {
	// PangoLayoutLine* pango_layout_iter_get_line_readonly(PangoLayoutIter* iter)
	ret, _, _ := pangoLayoutIterGetRunReadonly.CallRaw(uintptr(iter))
	return goPointer[LayoutRun](ret)
}

func (iter LayoutIter) GetClusterExtents() (inkRect, logicalRect Rectangle) {
	// void pango_layout_iter_get_cluster_extents(PangoLayoutIter* iter, PangoRectangle* ink_rect, PangoRectangle* logical_rect)
	pangoLayoutIterGetClusterExtents.CallRaw(uintptr(iter), uintptr(cgo.Pointer(&inkRect)), uintptr(cgo.Pointer(&logicalRect)))
	return
}

func (iter LayoutIter) NextLine() bool {
	// bool pango_layout_iter_next_line(PangoLayoutIter* iter)
	ret, _, _ := pangoLayoutIterNextLine.CallRaw(uintptr(iter))
	return ret != 0
}

func (iter LayoutIter) NextCluster() bool {
	// bool pango_layout_iter_next_cluster(PangoLayoutIter* iter)
	ret, _, _ := pangoLayoutIterNextCluster.CallRaw(uintptr(iter))
	return ret != 0
}

// --- PangoLayoutLine ---

type LayoutLine struct {
	Layout     Layout
	StartIndex int32
	Length     int32
	runs       *glib.GSList[GlyphItem]
	flags      uint32
}

// GetExtents computes the ink and logical extents of a layout line.
func (ll *LayoutLine) GetExtents() (inkRect, logicalRect Rectangle) {
	// void pango_layout_line_get_extents(PangoLayoutLine* line, PangoRectangle* ink_rect, PangoRectangle* logical_rect)
	pangoLayoutLineGetExtents.CallRaw(uintptr(cgo.Pointer(ll)), uintptr(cgo.Pointer(&inkRect)), uintptr(cgo.Pointer(&logicalRect)))
	return
}

func (ll *LayoutLine) GetRuns() (runs []GlyphItem) {
	runs = make([]GlyphItem, 0, ll.Length)
	for run := ll.runs; run != nil; run = run.Next {
		runs = append(runs, *run.Data)
	}
	return
}

// XToIndex converts from x offset to the byte index of the corresponding character within the text of the layout.
// Returns true if the position is inside the layout line.
func (ll *LayoutLine) XToIndex(xPos int) (index, trailing int, inside bool) {
	// gboolean pango_layout_line_x_to_index(PangoLayoutLine* line, int x_pos, int* index_, int* trailing)
	var idx, trail int32
	ret, _, _ := pangoLayoutLineXToIndex.CallRaw(uintptr(cgo.Pointer(ll)), uintptr(xPos),
		uintptr(cgo.Pointer(&idx)), uintptr(cgo.Pointer(&trail)))
	return int(idx), int(trail), ret != 0
}

func (ll *LayoutLine) IsParagraphStart() bool {
	return ll.flags&0x01 != 0
}

// --- LayoutRun ---

type LayoutRun = GlyphItem

// --- GlyphItem ---

type GlyphItem struct {
	Item         *Item
	Glyphs       *GlyphString
	YOffset      int32
	StartXOffset int32
	EndXOffset   int32
}

type GlyphString struct {
	NumGlyphs   int32
	glyphs      uintptr
	logClusters uintptr
}

func (g GlyphString) GetGlyphInfos() []GlyphInfo {
	return cgo.GoSliceN[GlyphInfo](cgo.Pointer(g.glyphs), int(g.NumGlyphs))
}

type GlyphInfo struct {
	Glyph    uint32
	Geometry GlyphGeometry
	Attr     GlyphVisAttr
}

type GlyphGeometry struct {
	Width   int32
	XOffset int32
	YOffset int32
}

type GlyphVisAttr struct {
	bits uint32
}

func (a GlyphVisAttr) IsClusterStart() bool {
	return a.bits&0x01 != 0
}

func (a GlyphVisAttr) IsColor() bool {
	return a.bits&(0x01<<1) != 0
}

type Item struct {
	Offset   int32
	Length   int32
	NumChars int32
	Analysis Analysis
}

type Analysis struct {
	ShapeEngine uintptr
	LangEngine  uintptr
	Font        uintptr
	Level       uint8
	Gravity     uint8
	Flags       uint8
	Script      uint8
	Language    Language
	ExtraAttrs  uintptr
}

// --- PangoAttrList ---

type AttrList uintptr

// AttrListNew creates a new empty attribute list with a reference count of one.
func AttrListNew() AttrList {
	// PangoAttrList* pango_attr_list_new()
	ret, _, _ := pangoAttrListNew.CallRaw()
	return AttrList(ret)
}

// Unref decrements the reference count of the attribute list.
func (al AttrList) Unref() {
	// void pango_attr_list_unref(PangoAttrList* list)
	pangoAttrListUnref.CallRaw(uintptr(al))
}

// Insert inserts the given attribute into the PangoAttrList. It will be inserted after all other attributes with a matching start_index.
func (al AttrList) Insert(attr *Attribute) {
	// void pango_attr_list_insert(PangoAttrList* list, PangoAttribute* attr)
	pangoAttrListInsert.CallRaw(uintptr(al), uintptr(cgo.Pointer(attr)))
}

// --- PangoAttribute ---

type Attribute struct {
	Class      uintptr
	StartIndex uint32
	EndIndex   uint32
}

func (attr *Attribute) Destroy() {
	// void  pango_attribute_destroy(PangoAttribute *attr);
	pangoAttributeDestroy.CallRaw(uintptr(cgo.Pointer(attr)))
}

// AttrFamilyNew create a new font family attribute.
func AttrFamilyNew(family string) *Attribute {
	// PangoAttribute* pango_attr_family_new(const char *family)
	cFamily := cgo.CString(family)
	ret, _, _ := pangoAttrFamilyNew.CallRaw(uintptr(cFamily))
	runtime.KeepAlive(cFamily)
	return goPointer[Attribute](ret)
}

// AttrSizeNewAbsolute creates a new absolute-font-size attribute in fractional points.
// size is in Pango units (points * Scale).
func AttrSizeNewAbsolute(size int) *Attribute {
	// PangoAttribute* pango_attr_size_new(int size)
	ret, _, _ := pangoAttrSizeNewAbsolute.CallRaw(uintptr(size))
	return goPointer[Attribute](ret)
}

// AttrUnderlineNew create a new underline-style attribute.
func AttrUnderlineNew(underline Underline) *Attribute {
	// PangoAttribute* pango_attr_underline_new(PangoUnderline underline)
	ret, _, _ := pangoAttrUnderlineNew.CallRaw(uintptr(underline))
	return goPointer[Attribute](ret)
}

// AttrStrikethroughNew create a new strike-through attribute.
func AttrStrikethroughNew(strikethrough bool) *Attribute {
	// PangoAttribute* pango_attr_strikethrough_new(gboolean strikethrough)
	ret, _, _ := pangoAttrStrikethroughNew.CallRaw(uintptr(cgo.CBool(strikethrough)))
	return goPointer[Attribute](ret)
}

// AttrFontFeaturesNew creates a new font features tag attribute.
// features is a string of OpenType feature tags (e.g. "liga=1,kern=1").
func AttrFontFeaturesNew(features string) *Attribute {
	// PangoAttribute* pango_attr_font_features_new(const gchar* features)
	cFeatures := cgo.CString(features)
	ret, _, _ := pangoAttrFontFeaturesNew.CallRaw(uintptr(cFeatures))
	runtime.KeepAlive(cFeatures)
	return goPointer[Attribute](ret)
}

// AttrForegroundNew create a new foreground color attribute.
func AttrForegroundNew(red, green, blue uint16) *Attribute {
	// PangoAttribute* pango_attr_foreground_new(guint16 red, guint16 green, guint16 blue)
	ret, _, _ := pangoAttrForegroundNew.CallRaw(uintptr(red), uintptr(green), uintptr(blue))
	return goPointer[Attribute](ret)
}

// --- PangoFontDescription ---

// FontDescriptionNew creates a new font description structure with all fields unset.
func FontDescriptionNew() FontDescription {
	// PangoFontDescription* pango_font_description_new()
	ret, _, _ := pangoFontDescriptionNew.CallRaw()
	return FontDescription(ret)
}

// Free frees a font description.
func (fd FontDescription) Free() {
	// void pango_font_description_free(PangoFontDescription* desc)
	pangoFontDescriptionFree.CallRaw(uintptr(fd))
}

// SetFamily sets the family name field of a font description.
func (fd FontDescription) SetFamily(family string) {
	// void pango_font_description_set_family(PangoFontDescription* desc, const char* family)
	cFamily := cgo.CString(family)
	pangoFontDescriptionSetFamily.CallRaw(uintptr(fd), uintptr(cFamily))
	runtime.KeepAlive(cFamily)
}

// SetSize sets the size field of a font description in fractional points.
// size is in Pango units (points * Scale).
func (fd FontDescription) SetSize(size int) {
	// void pango_font_description_set_size(PangoFontDescription* desc, gint size)
	pangoFontDescriptionSetSize.CallRaw(uintptr(fd), uintptr(size))
}

func (fd FontDescription) SetAbsoluteSize(size float64) {
	//void pango_font_description_set_absolute_size(PangoFontDescription* desc, double size)
	cgo.Call(pangoFontDescriptionSetAbsoluteSize.Addr(), fd, size)
}

// --- PangoContext ---

type Context struct {
	glib.Object
}

// SetFontDescription sets the default font description for a context.
func (c Context) SetFontDescription(desc FontDescription) {
	// void pango_context_set_font_description(PangoContext* context, const PangoFontDescription* desc)
	pangoContextSetFontDescription.CallRaw(c.GObject, uintptr(desc))
}

// SetLanguage sets the global language tag for the context.
func (c Context) SetLanguage(language Language) {
	// void pango_context_set_language(PangoContext* context, PangoLanguage* language)
	pangoContextSetLanguage.CallRaw(c.GObject, uintptr(language))
}

// SetBaseDir sets the base direction for the context.
func (c Context) SetBaseDir(direction Direction) {
	// void pango_context_set_base_dir(PangoContext* context, PangoDirection direction)
	pangoContextSetBaseDir.CallRaw(c.GObject, uintptr(direction))
}

// SetFontMap sets the font map to be searched when fonts are looked-up in this context.
func (c Context) SetFontMap(fontMap FontMap) {
	// void pango_context_set_font_map(PangoContext* context, PangoFontMap* font_map)
	pangoContextSetFontMap.CallRaw(c.GObject, fontMap.GObject)
}

// --- PangoLanguage ---

// LanguageFromString takes a RFC-3066 format language tag as a string and converts it to a PangoLanguage pointer.
func LanguageFromString(language string) Language {
	// PangoLanguage* pango_language_from_string(const char* language)
	cLang := cgo.CString(language)
	ret, _, _ := pangoLanguageFromString.CallRaw(uintptr(cLang))
	runtime.KeepAlive(cLang)
	return Language(ret)
}

// --- PangoFontMap ---

type FontMap struct {
	glib.Object
}

// CreateContext creates a PangoContext connected to the given font map.
func (fm FontMap) CreateContext() (c Context) {
	// PangoContext* pango_font_map_create_context(PangoFontMap* fontmap)
	c.GObject, _, _ = pangoFontMapCreateContext.CallRaw(fm.GObject)
	return
}

func (fm FontMap) GetConfig() fontconfig.Config {
	ret, _, _ := pangoFcFontMapGetConfig.CallRaw(fm.GObject)
	return fontconfig.Config(ret)
}

// FontMapSetConfig sets the FcConfig for a PangoCairoFontMap created with FontMapNewForFontType(cairo.FontTypeFT).
// The fontMap must be a PangoFcFontMap (i.e. created with FontTypeFT).
// config is an *FcConfig from the fontconfig package (passed as uintptr).
func (fm FontMap) SetConfig(config fontconfig.Config) {
	// void pango_fc_font_map_set_config(PangoFcFontMap* fcfontmap, FcConfig* fcconfig)
	pangoFcFontMapSetConfig.CallRaw(fm.GObject, uintptr(config))
}

func goPointer[T any](ptr uintptr) *T {
	return (*T)(cgo.Pointer(ptr))
}
