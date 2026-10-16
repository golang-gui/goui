package directwrite

import (
	"errors"
	"fmt"
	"image/color"
	"math"
	"path/filepath"
	"sync/atomic"
	"unsafe"

	"github.com/goexlib/cgo"
	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/platform/typography"
	"github.com/golang-gui/goui/platform/typography/internal/lifecycle"
	"github.com/golang-gui/goui/platform/typography/utils"

	"github.com/golang-gui/goui/platform/windows/sdk/com"
	"github.com/golang-gui/goui/platform/windows/sdk/d2d1"
	"github.com/golang-gui/goui/platform/windows/sdk/dwrite"
	"github.com/golang-gui/goui/platform/windows/sdk/dxgi"
	"github.com/golang-gui/goui/platform/windows/sdk/wic"
)

type Context struct {
	dwFactory      *dwrite.Factory
	d2dFactory     *d2d1.Factory
	imgFactory     *wic.ImagingFactory
	ownsCOM        bool // true if we called com.Initialize (not skipped)
	fontFactory    *dwrite.Factory3
	fontSet        *dwrite.FontSet
	fontCollection *dwrite.FontCollection1
	fontFiles      map[string]bool
}

func NewContext() (_ typography.Context, err error) {
	// COM is already initialized as STA by the platform layer (newPlatform).
	// com.Initialize is a no-op when already initialized; we must not
	// Uninitialize on destroy since the platform owns the lifetime.
	already := com.IsInitialized()
	hr := com.Initialize(com.COINIT_APARTMENTTHREADED)
	if hr.Failed() {
		return nil, fmt.Errorf("com initialize err: %v", hr)
	}

	c := new(Context)
	c.ownsCOM = !already
	c.dwFactory, err = dwrite.CreateFactory[dwrite.Factory](dwrite.DWRITE_FACTORY_TYPE_SHARED, dwrite.IID_IDWriteFactory)
	if err != nil {
		c.Destroy()
		return nil, fmt.Errorf("create dwrite factory err: %v", err)
	}
	return c, nil
}

func (c *Context) Destroy() {
	c.destroyDraw()
	if c.fontCollection != nil {
		c.fontCollection.Release()
		c.fontCollection = nil
	}
	if c.fontSet != nil {
		c.fontSet.Release()
		c.fontSet = nil
	}
	if c.fontFactory != nil {
		c.fontFactory.Release()
		c.fontFactory = nil
	}
	if c.dwFactory != nil {
		c.dwFactory.Release()
		c.dwFactory = nil
	}
	if c.ownsCOM {
		com.Uninitialize()
	}
}

func (c *Context) Name() string {
	return "DirectWrite"
}

func (c *Context) AddFont(fontFile string) error {
	path, err := filepath.Abs(fontFile)
	if err != nil {
		return err
	}
	if c.fontFiles[path] {
		return nil
	}
	if c.fontFactory == nil {
		c.fontFactory, err = dwrite.CreateFactory[dwrite.Factory3](dwrite.DWRITE_FACTORY_TYPE_SHARED, dwrite.IID_IDWriteFactory3)
		if err != nil {
			return fmt.Errorf("create font factory: %w", err)
		}
	}
	file, hr := c.fontFactory.CreateFontFileReference(path)
	if hr.Failed() {
		return fmt.Errorf("create font file: %w", hr)
	}
	defer file.Release()
	supported, _, _, count, hr := file.Analyze()
	if hr.Failed() {
		return fmt.Errorf("analyze font file: %w", hr)
	}
	if !supported || count == 0 {
		return errors.New("unsupported font file")
	}
	builder, hr := c.fontFactory.CreateFontSetBuilder()
	if hr.Failed() {
		return fmt.Errorf("create font set builder: %w", hr)
	}
	defer builder.Release()
	// Application faces precede the previously available system/application faces.
	for index := uint32(0); index < count; index++ {
		ref, hr := c.fontFactory.CreateFontFaceReference(file, index, 0)
		if hr.Failed() {
			return fmt.Errorf("create font face: %w", hr)
		}
		hr = builder.AddFontFaceReference(ref)
		ref.Release()
		if hr.Failed() {
			return fmt.Errorf("add font face: %w", hr)
		}
	}
	previous := c.fontSet
	if previous == nil {
		previous, hr = c.fontFactory.GetSystemFontSet()
		if hr.Failed() {
			return fmt.Errorf("get system fonts: %w", hr)
		}
		defer previous.Release()
	}
	if hr = builder.AddFontSet(previous); hr.Failed() {
		return fmt.Errorf("add existing fonts: %w", hr)
	}
	set, hr := builder.CreateFontSet()
	if hr.Failed() {
		return fmt.Errorf("create font set: %w", hr)
	}
	collection, hr := c.fontFactory.CreateFontCollectionFromFontSet(set)
	if hr.Failed() {
		set.Release()
		return fmt.Errorf("create font collection: %w", hr)
	}
	// Publish only the complete pair. Native layouts retain their old collection.
	if c.fontCollection != nil {
		c.fontCollection.Release()
	}
	if c.fontSet != nil {
		c.fontSet.Release()
	}
	c.fontSet, c.fontCollection = set, collection
	if c.fontFiles == nil {
		c.fontFiles = make(map[string]bool)
	}
	c.fontFiles[path] = true
	return nil
}

func (c *Context) fonts() *dwrite.FontCollection {
	if c.fontCollection == nil {
		return nil
	}
	return &c.fontCollection.FontCollection
}

func (c *Context) NewTextLayout(text string, format typography.TextFormat, width, height float32) (typography.TextLayout, error) {
	if err := utils.ValidateTabWidth(format.TabWidth); err != nil {
		return nil, err
	}
	textFormat, err := c.createTextFormat(format)
	if err != nil {
		return nil, fmt.Errorf("create dwrite text format err: %v", err)
	}
	defer textFormat.Release()

	textLayout, hr := c.dwFactory.CreateTextLayout(text, textFormat, width, height)
	if hr.Failed() {
		return nil, fmt.Errorf("create dwrite text layout err: %v", hr)
	}

	return newTextLayout(c, textLayout, text, format, width, height), nil
}

func (c *Context) createTextFormat(format typography.TextFormat) (textFormat *dwrite.TextFormat, err error) {
	font, err := utils.ResolveFont(format.Font)
	if err != nil {
		return nil, err
	}
	if font.Family == "" {
		font.Family = "Segoe UI"
	}
	textFormat, hr := c.dwFactory.CreateTextFormat(font.Family, c.fonts(), dwrite.FontWeight(font.Weight), nativeFontStyle(font.Style), dwrite.FontStretch(font.Width), ptToDip(font.Size), "")
	if hr.Failed() {
		return nil, hr
	}

	switch format.WrapMode {
	case typography.WrapNone:
		textFormat.SetWordWrapping(dwrite.DWRITE_WORD_WRAPPING_NO_WRAP)
	case typography.WrapChar:
		textFormat.SetWordWrapping(dwrite.DWRITE_WORD_WRAPPING_CHARACTER)
	case typography.WrapWordChar:
		textFormat.SetWordWrapping(dwrite.DWRITE_WORD_WRAPPING_EMERGENCY_BREAK)
	}

	switch format.TextAlign {
	case typography.TextAlignBegin:
		textFormat.SetTextAlignment(dwrite.DWRITE_TEXT_ALIGNMENT_LEADING)
	case typography.TextAlignEnd:
		textFormat.SetTextAlignment(dwrite.DWRITE_TEXT_ALIGNMENT_TRAILING)
	case typography.TextAlignCenter:
		textFormat.SetTextAlignment(dwrite.DWRITE_TEXT_ALIGNMENT_CENTER)
	case typography.TextAlignFill:
		textFormat.SetTextAlignment(dwrite.DWRITE_TEXT_ALIGNMENT_JUSTIFIED)
	}

	return
}

func (c *Context) prepareDraw() (err error) {
	if c.d2dFactory == nil {
		c.d2dFactory, err = d2d1.CreateFactory[d2d1.Factory](d2d1.D2D1_FACTORY_TYPE_SINGLE_THREADED, d2d1.IID_ID2D1Factory1, nil)
		if err != nil {
			return fmt.Errorf("create d2d factory err: %v", err)
		}
	}
	if c.imgFactory == nil {
		c.imgFactory, err = wic.CreateImagingFactory[wic.ImagingFactory](wic.CLSID_WICImagingFactory2, wic.IID_IWICImagingFactory)
		if err != nil {
			return fmt.Errorf("create wic factory err: %v", err)
		}
	}
	return nil
}

func (c *Context) destroyDraw() {
	if c.d2dFactory != nil {
		c.d2dFactory.Release()
		c.d2dFactory = nil
	}
	if c.imgFactory != nil {
		c.imgFactory.Release()
		c.imgFactory = nil
	}
}

type TextLayout struct {
	ctx             *Context
	layout          *dwrite.TextLayout
	text            string
	format          typography.TextFormat
	width           float32
	height          float32
	position        utils.StringPosition
	colors          []textColorAttr
	painter         textPainter
	life            lifecycle.Layout
	placeholders    map[int]typography.PlaceholderMetrics
	defaultTabWidth float32
}

type textColorAttr struct {
	Range dwrite.TextRange
	Color color.RGBA
}

func newTextLayout(ctx *Context, layout *dwrite.TextLayout, text string, format typography.TextFormat, width, height float32) (t *TextLayout) {
	t = new(TextLayout)
	t.layout = layout
	t.ctx = ctx
	t.text = text
	t.format = format
	t.width = width
	t.height = height
	t.position = utils.CalcStringPosition(text)
	t.defaultTabWidth = layout.GetIncrementalTabStop()
	t.format.TabWidth = -1
	t.SetTabWidth(format.TabWidth)
	return
}

func (t *TextLayout) SetTabWidth(width float32) {
	if err := utils.ValidateTabWidth(width); err != nil {
		panic(err)
	}
	if t.format.TabWidth == width {
		return
	}
	native := width
	if native == 0 {
		native = t.defaultTabWidth
	}
	if hr := t.layout.SetIncrementalTabStop(native); hr.Failed() {
		panic(hr)
	}
	t.format.TabWidth = width
	t.life.Changed()
}

func (t *TextLayout) Destroy() {
	if !t.life.BeginDestroy() {
		return
	}
	t.painter.Destroy()
	if t.layout != nil {
		t.layout.Release()
		t.layout = nil
	}
}

func (t *TextLayout) ConnectChanged(fn func()) signal.Handle {
	return t.life.ConnectChanged(fn)
}

func (t *TextLayout) ConnectDestroy(fn func()) signal.Handle {
	return t.life.ConnectDestroy(fn)
}

func (*TextLayout) Name() string {
	return "DirectWrite"
}

func (t *TextLayout) Text() string {
	return t.text
}

func (t *TextLayout) Format() typography.TextFormat {
	return t.format
}

func (t *TextLayout) Size() (maxWidth, maxHeight float32) {
	return t.width, t.height
}

func (t *TextLayout) SetSize(maxWidth, maxHeight float32) {
	if t.width == maxWidth && t.height == maxHeight {
		return
	}
	t.width, t.height = maxWidth, maxHeight
	t.layout.SetMaxWidth(maxWidth)
	t.layout.SetMaxHeight(maxHeight)
	t.life.Changed()
}

func (t *TextLayout) SetTextAlignment(align typography.TextAlignment) {
	if t.format.TextAlign == align {
		return
	}
	t.format.TextAlign = align
	switch align {
	case typography.TextAlignBegin:
		t.layout.SetTextAlignment(dwrite.DWRITE_TEXT_ALIGNMENT_LEADING)
	case typography.TextAlignEnd:
		t.layout.SetTextAlignment(dwrite.DWRITE_TEXT_ALIGNMENT_TRAILING)
	case typography.TextAlignCenter:
		t.layout.SetTextAlignment(dwrite.DWRITE_TEXT_ALIGNMENT_CENTER)
	case typography.TextAlignFill:
		t.layout.SetTextAlignment(dwrite.DWRITE_TEXT_ALIGNMENT_JUSTIFIED)
	}
	t.life.Changed()
}

func (t *TextLayout) SetWrapMode(wrap typography.WrapMode) {
	if t.format.WrapMode == wrap {
		return
	}
	t.format.WrapMode = wrap
	switch wrap {
	case typography.WrapNone:
		t.layout.SetWordWrapping(dwrite.DWRITE_WORD_WRAPPING_NO_WRAP)
	case typography.WrapChar:
		t.layout.SetWordWrapping(dwrite.DWRITE_WORD_WRAPPING_CHARACTER)
	case typography.WrapWordChar:
		t.layout.SetWordWrapping(dwrite.DWRITE_WORD_WRAPPING_EMERGENCY_BREAK)
	}
	t.life.Changed()
}

func (t *TextLayout) SetTextFont(start, length int, font typography.FontInfo) {
	utils.CheckTextRange(t.text, start, length)
	font, err := utils.ResolveFont(font)
	if err != nil {
		panic(err)
	}
	if font.Family == "" {
		font.Family = "Segoe UI"
	}
	startPos, endPos := t.position.ToUtf16(start), t.position.ToUtf16(start+length)
	textRange := dwrite.TextRange{StartPosition: uint32(startPos), Length: uint32(endPos - startPos)}
	t.layout.SetFontFamilyName(font.Family, textRange)
	t.layout.SetFontSize(ptToDip(font.Size), textRange)
	t.layout.SetFontWeight(dwrite.FontWeight(font.Weight), textRange)
	t.layout.SetFontStretch(dwrite.FontStretch(font.Width), textRange)
	t.layout.SetFontStyle(nativeFontStyle(font.Style), textRange)
	t.life.Changed()
}

func nativeFontStyle(style typography.FontStyle) dwrite.FontStyle {
	return []dwrite.FontStyle{dwrite.DWRITE_FONT_STYLE_NORMAL, dwrite.DWRITE_FONT_STYLE_ITALIC, dwrite.DWRITE_FONT_STYLE_OBLIQUE}[style]
}

func (t *TextLayout) SetTextColor(start, length int, color color.Color) {
	if 0 <= start && 0 < length {
		startPos := t.position.ToUtf16(start)
		endPos := t.position.ToUtf16(start + length)
		if endPos < 0 {
			endPos = t.position.ToUtf16(len(t.text))
		}

		textRange := dwrite.TextRange{
			StartPosition: uint32(startPos),
			Length:        uint32(endPos - startPos),
		}

		// A full foreground replacement makes every older range obsolete.
		// Retaining them would grow memory and rendering work on every recolor.
		if startPos == 0 && endPos == t.position.ToUtf16(len(t.text)) {
			t.colors = t.colors[:0]
		}
		t.colors = append(t.colors, textColorAttr{
			Range: textRange,
			Color: toRGBAColor(color),
		})
		t.life.Changed()
	}
}

func (t *TextLayout) SetUnderline(start, length int, underline bool) {
	if 0 <= start && 0 < length {
		startPos := t.position.ToUtf16(start)
		endPos := t.position.ToUtf16(start + length)
		if endPos < 0 {
			endPos = t.position.ToUtf16(len(t.text))
		}

		textRange := dwrite.TextRange{
			StartPosition: uint32(startPos),
			Length:        uint32(endPos - startPos),
		}

		t.layout.SetUnderline(underline, textRange)
		t.life.Changed()
	}
}

func (t *TextLayout) SetStrikethrough(start, length int, strike bool) {
	if 0 <= start && 0 < length {
		startPos := t.position.ToUtf16(start)
		endPos := t.position.ToUtf16(start + length)
		if endPos < 0 {
			endPos = t.position.ToUtf16(len(t.text))
		}

		textRange := dwrite.TextRange{
			StartPosition: uint32(startPos),
			Length:        uint32(endPos - startPos),
		}

		t.layout.SetStrikethrough(strike, textRange)
		t.life.Changed()
	}
}

func (t *TextLayout) MeasureMetrics() (lines []typography.TextLine, clusters []typography.TextCluster) {
	lineMetrics, _ := t.layout.GetLineMetrics()
	lines = make([]typography.TextLine, 0, len(lineMetrics))

	clusterMetrics, _ := t.layout.GetClusterMetrics()
	clusters = make([]typography.TextCluster, 0, len(clusterMetrics))

	pos, index := 0, 0
	xOffset, yOffset, _, _ := t.getExtends()
	lineY := -yOffset
	totalUnits := t.position.ToUtf16(len(t.text))

	for lineIndex, metrics := range lineMetrics {
		var line typography.TextLine
		endPos := pos + int(metrics.Length)
		// DirectWrite counts an implicit end-of-text unit in the last line's
		// length without reporting it as a newline. Clamp the content to the
		// actual text so the phantom unit never becomes a cluster.
		contentEnd := min(endPos-int(metrics.NewlineLength), totalUnits)
		line.Start = t.position.ToUtf8(min(pos, totalUnits))
		line.Length = t.position.ToUtf8(contentEnd) - line.Start
		lineX, _, _, _ := t.layout.HitTestTextPosition(pos, false)
		line.X = lineX - xOffset
		line.Y = lineY
		line.Height = metrics.Height
		line.Baseline = line.Y + metrics.Baseline
		clustersBeg := len(clusters)
		for index < len(clusterMetrics) && pos < endPos {
			dwCluster := clusterMetrics[index]
			clusterEnd := pos + int(dwCluster.Length)
			// Newline positions belong to the line boundary, not the preceding
			// visible cluster. DirectWrite also reports an implicit end-of-text
			// cluster that is not always flagged, so only clusters fully inside
			// the content range are emitted. Positions are still consumed so
			// the next line starts aligned. Other zero-width clusters still
			// carry text ranges.
			if clusterEnd > pos && clusterEnd <= contentEnd && !dwCluster.IsNewLine() {
				_, _, hit, _ := t.layout.HitTestTextPosition(pos, false)
				cluster := typography.TextCluster{
					Start: t.position.ToUtf8(pos),
					X:     hit.Left - xOffset, Y: line.Y,
					Width: dwCluster.Width, Height: line.Height, LineIndex: lineIndex,
				}
				cluster.Length = t.position.ToUtf8(pos+int(dwCluster.Length)) - cluster.Start
				if placeholder, ok := t.placeholders[cluster.Start]; ok {
					cluster.Y, cluster.Height = line.Baseline-placeholder.Baseline, placeholder.Height
				}
				// Inline objects have no RTL glyph flag. Hit testing reports the
				// actual resolved bidi level for both text and inline objects.
				if hit.BidiLevel&1 != 0 {
					cluster.Direction = typography.TextRightToLeft
				}
				// Logical cluster order is not visual order in a bidi line.
				if len(clusters) == clustersBeg {
					line.X, line.Width = cluster.X, cluster.Width
				} else {
					right := max(line.X+line.Width, cluster.X+cluster.Width)
					line.X = min(line.X, cluster.X)
					line.Width = right - line.X
				}
				clusters = append(clusters, cluster)
			}
			pos = clusterEnd
			index++
		}
		line.Clusters = clusters[clustersBeg:]
		lines = append(lines, line)
		lineY += metrics.Height
	}

	return
}

func (t *TextLayout) MeasureSize() (width, height float32) {
	_, _, width, height = t.getExtends()
	return
}

func (t *TextLayout) Draw(render *d2d1.RenderTarget, origin d2d1.Point2F, drawOptions d2d1.DrawTextOptions) (err error) {
	var d2dColor d2d1.ColorF

	brushs := make(map[color.RGBA]*d2d1.Brush, len(t.colors))
	defer func() {
		for _, b := range brushs {
			b.Release()
		}
	}()

	getColorBrush := func(rgba color.RGBA) (*d2d1.Brush, error) {
		d2dBrush, exist := brushs[rgba]
		if !exist {
			d2dColor = toD2dColor(rgba)
			b, hr := render.CreateSolidColorBrush(&d2dColor, nil)
			if hr.Failed() {
				return nil, fmt.Errorf("create d2d solid color brush err: %v", hr)
			}
			d2dBrush = &b.Brush
			brushs[rgba] = d2dBrush
		}
		return d2dBrush, nil
	}

	fgColorBrush, err := getColorBrush(toRGBAColor(t.format.TextColor))
	if err != nil {
		return err
	}

	for _, attr := range t.colors {
		d2dBrush, err := getColorBrush(attr.Color)
		if err != nil {
			return err
		}
		t.layout.SetDrawingEffect(&d2dBrush.Unknown, attr.Range)
	}

	x, y, _, _ := t.getExtends()
	pos := d2d1.Point2F{
		X: origin.X - x,
		Y: origin.Y - y,
	}
	render.DrawTextLayout(pos, t.layout, fgColorBrush, drawOptions)
	return nil
}

func (t *TextLayout) Rasterize(scale float32, buf []byte) (bitmap typography.TextBitmap, err error) {
	if t.life.IsDestroyed() {
		return bitmap, errors.New("directwrite: rasterize destroyed text layout")
	}
	if scale <= 0 || math.IsNaN(float64(scale)) || math.IsInf(float64(scale), 0) {
		return bitmap, fmt.Errorf("directwrite: invalid raster scale %v", scale)
	}
	return t.rasterize(scale, buf)
}

func (t *TextLayout) rasterize(scale float32, buf []byte) (bitmap typography.TextBitmap, err error) {
	if err = t.ctx.prepareDraw(); err != nil {
		return
	}

	_, _, width, height := t.getExtends()
	if width <= 0 || height <= 0 {
		return
	}

	width = min(width, t.width) * scale
	height = min(height, t.height) * scale
	if width <= 0 || height <= 0 {
		return
	}

	if t.painter.width < width || t.painter.height < height {
		widthCapacity := max(width, t.painter.width)
		heightCapacity := max(height, t.painter.height)
		t.painter.Destroy()
		err = t.painter.Init(t.ctx.d2dFactory, t.ctx.imgFactory, widthCapacity, heightCapacity)
		if err != nil {
			return
		}
	}

	err = t.painter.DrawTextLayout(t, scale)
	if err != nil {
		return
	}

	return t.painter.GetBitmap(width, height, buf)
}

func (t *TextLayout) getExtends() (x, y, width, height float32) {
	metrics, _ := t.layout.GetMetrics()
	x = metrics.Left
	y = metrics.Top
	width = metrics.Width
	height = metrics.Height
	return
}

func ptToDip(pt float32) float32 {
	// DirectWrite font sizes are in DIPs (1 DIP = 1/96 inch); Font.Size is in
	// typographic points (1 pt = 1/72 inch), so 1 pt = 96/72 DIP.
	return pt * 96.0 / 72.0
}

func roundPixel(v float32) int {
	return int(v + 0.99)
}

type textPainter struct {
	width  float32
	height float32
	colorf d2d1.ColorF
	bitmap *wic.Bitmap
	render *d2d1.RenderTarget
}

func (p *textPainter) Init(d2dFactory *d2d1.Factory, imgFactory *wic.ImagingFactory, width, height float32) (err error) {
	p.width = width
	p.height = height
	wicPixelFormat := wic.GUID_WICPixelFormat32bppPRGBA
	d2dPixelFormat := d2d1.PixelFormat{
		Format:    dxgi.DXGI_FORMAT_R8G8B8A8_UNORM,
		AlphaMode: d2d1.D2D1_ALPHA_MODE_PREMULTIPLIED,
	}

	var hr com.HRESULT
	p.bitmap, hr = imgFactory.CreateBitmap(roundPixel(width), roundPixel(height), wicPixelFormat, wic.WICBitmapCacheOnLoad)
	if hr.Failed() {
		return fmt.Errorf("create wic bitmap err: %v", hr)
	}

	props := d2d1.RenderTargetProperties{
		PixelFormat: d2dPixelFormat,
		DpiX:        96,
		DpiY:        96,
	}

	p.render, hr = d2dFactory.CreateWicBitmapRenderTarget(p.bitmap, &props)
	if hr.Failed() {
		return fmt.Errorf("create d2d wic bitmap render target err: %v", hr)
	}

	p.render.SetTextAntialiasMode(d2d1.D2D1_TEXT_ANTIALIAS_MODE_GRAYSCALE)
	return nil
}

func (p *textPainter) Destroy() {
	if p.render != nil {
		p.render.Release()
		p.render = nil
	}
	if p.bitmap != nil {
		p.bitmap.Release()
		p.bitmap = nil
	}
}

func (p *textPainter) DrawTextLayout(layout *TextLayout, scale float32) (err error) {
	dpi := scale * 96
	p.render.SetDpi(dpi, dpi)
	p.render.BeginDraw()
	p.render.Clear(&p.colorf)
	err = layout.Draw(p.render, d2d1.Point2F{}, d2d1.D2D1_DRAW_TEXT_OPTIONS_ENABLE_COLOR_FONT|d2d1.D2D1_DRAW_TEXT_OPTIONS_CLIP)
	hr := p.render.EndDraw(nil, nil)
	if err != nil {
		return fmt.Errorf("directwrite draw text err: %w", err)
	}
	if hr.Failed() {
		return fmt.Errorf("directwrite end draw err: %w", hr)
	}
	return nil
}

func (p *textPainter) GetBitmap(width, height float32, buf []byte) (bitmap typography.TextBitmap, err error) {
	bitmap.Width = roundPixel(width)
	bitmap.Height = roundPixel(height)
	bitmap.Stride = bitmap.Width * 4
	byteSize := bitmap.Stride * bitmap.Height
	if byteSize <= cap(buf) {
		bitmap.Pixels = buf[:byteSize]
	} else {
		bitmap.Pixels = make([]byte, byteSize)
	}

	rect := wic.Rect{
		Width:  int32(bitmap.Width),
		Height: int32(bitmap.Height),
	}
	hr := p.bitmap.CopyPixels(&rect, bitmap.Stride, bitmap.Pixels)
	if hr.Failed() {
		return bitmap, fmt.Errorf("copy bitmap pixels err: %w", err)
	}

	return bitmap, nil
}

func toD2dColor(c color.Color) (d2dColor d2d1.ColorF) {
	if c == nil {
		c = typography.DefaultTextColor()
	}
	rgba := color.NRGBA64Model.Convert(c).(color.NRGBA64)
	return d2d1.ColorF{
		R: float32(rgba.R) / 65535,
		G: float32(rgba.G) / 65535,
		B: float32(rgba.B) / 65535,
		A: float32(rgba.A) / 65535,
	}
}

func toRGBAColor(c color.Color) (rgba color.RGBA) {
	if c == nil {
		c = typography.DefaultTextColor()
	}
	return color.RGBAModel.Convert(c).(color.RGBA)
}

// A C-allocated, numeric-only COM object. DirectWrite owns references to it;
// no native pointer keeps Go heap/stack memory or a TextLayout alive.
type inlineObject struct {
	class   uintptr
	refs    int32
	metrics dwrite.InlineObjectMetrics
}

var inlineClass = dwrite.InlineObjectClass{
	UnknownClass: com.UnknownClass{
		QueryInterface: cgo.Symbol(cgo.NewCallback(func(this *inlineObject, iid *com.IID, result *uintptr) com.HRESULT {
			if iid == nil || result == nil {
				return com.E_POINTER
			}
			*result = 0
			if *iid != com.IID_IUnknown && *iid != dwrite.IID_IDWriteInlineObject {
				return com.E_NOINTERFACE
			}
			*result = uintptr(unsafe.Pointer(this))
			atomic.AddInt32(&this.refs, 1)
			return com.S_OK
		})),
		AddRef: cgo.Symbol(cgo.NewCallback(func(this *inlineObject) uint32 { return uint32(atomic.AddInt32(&this.refs, 1)) })),
		Release: cgo.Symbol(cgo.NewCallback(func(this *inlineObject) uint32 {
			count := atomic.AddInt32(&this.refs, -1)
			if count == 0 {
				cgo.Free(unsafe.Pointer(this))
			}
			return uint32(count)
		})),
	},
	Draw: cgo.Symbol(cgo.NewCallback(func(this *inlineObject, client, renderer uintptr, x, y float32, sideways, rtl int32, effect uintptr) com.HRESULT {
		return com.S_OK
	})),
	GetMetrics: cgo.Symbol(cgo.NewCallback(func(this *inlineObject, result *dwrite.InlineObjectMetrics) com.HRESULT {
		if result == nil {
			return com.E_POINTER
		}
		*result = this.metrics
		return com.S_OK
	})),
	GetOverhangMetrics: cgo.Symbol(cgo.NewCallback(func(this *inlineObject, result *dwrite.OverhangMetrics) com.HRESULT {
		if result == nil {
			return com.E_POINTER
		}
		*result = dwrite.OverhangMetrics{}
		return com.S_OK
	})),
	GetBreakConditions: cgo.Symbol(cgo.NewCallback(func(this *inlineObject, before, after *dwrite.BreakCondition) com.HRESULT {
		if before == nil || after == nil {
			return com.E_POINTER
		}
		*before, *after = dwrite.DWRITE_BREAK_CONDITION_CAN_BREAK, dwrite.DWRITE_BREAK_CONDITION_CAN_BREAK
		return com.S_OK
	})),
}

func (t *TextLayout) SetPlaceholder(start int, metrics *typography.PlaceholderMetrics) {
	utils.CheckPlaceholder(t.text, start, metrics)
	r := dwrite.TextRange{StartPosition: uint32(t.position.ToUtf16(start)), Length: 1}
	var object *dwrite.InlineObject
	if metrics != nil {
		data := cgo.Malloc(cgo.Sizet(unsafe.Sizeof(inlineObject{})))
		if data == nil {
			panic("directwrite: allocate inline metrics failed")
		}
		value := (*inlineObject)(data)
		value.class = uintptr(unsafe.Pointer(&inlineClass))
		value.refs = 1
		value.metrics = dwrite.InlineObjectMetrics{Width: metrics.Width, Height: metrics.Height, Baseline: metrics.Baseline}
		object = (*dwrite.InlineObject)(data)
	}
	hr := t.layout.SetInlineObject(object, r)
	if object != nil {
		object.Release()
	}
	if hr.Failed() {
		panic(hr)
	}
	if metrics == nil {
		delete(t.placeholders, start)
	} else {
		if t.placeholders == nil {
			t.placeholders = make(map[int]typography.PlaceholderMetrics)
		}
		t.placeholders[start] = *metrics
	}
	t.life.Changed()
}
