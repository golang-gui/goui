package directwrite

import (
	"errors"
	"fmt"
	"image/color"

	"github.com/golang-gui/goui/platform/typography"
	"github.com/golang-gui/goui/platform/typography/utils"

	"github.com/golang-gui/goui/platform/windows/sdk/com"
	"github.com/golang-gui/goui/platform/windows/sdk/d2d1"
	"github.com/golang-gui/goui/platform/windows/sdk/dwrite"
	"github.com/golang-gui/goui/platform/windows/sdk/dxgi"
	"github.com/golang-gui/goui/platform/windows/sdk/wic"
	"github.com/golang-gui/goui/platform/windows/sdk/winapi"
)

type Context struct {
	dwFactory  *dwrite.Factory
	d2dFactory *d2d1.Factory
	imgFactory *wic.ImagingFactory
}

func NewContext() (_ typography.Context, err error) {
	hr := com.Initialize(com.COINIT_MULTITHREADED)
	if hr.Failed() {
		return nil, fmt.Errorf("com initialize err: %v", hr)
	}

	c := new(Context)
	c.dwFactory, err = dwrite.CreateFactory[dwrite.Factory](dwrite.DWRITE_FACTORY_TYPE_SHARED, dwrite.IID_IDWriteFactory)
	if err != nil {
		return nil, fmt.Errorf("create dwrite factory err: %v", err)
	}
	return c, nil
}

func (c *Context) Destroy() {
	c.destroyDraw()
	if c.dwFactory != nil {
		c.dwFactory.Release()
		c.dwFactory = nil
	}
	com.Uninitialize()
}

func (c *Context) Name() string {
	return "DirectWrite"
}

func (c *Context) AddFont(fontFile string) error {
	num := winapi.AddFontResourceEx(fontFile, winapi.FR_PRIVATE)
	if num == 0 {
		return errors.New("add font resource failed")
	}
	return nil
}

func (c *Context) NewTextLayout(text string, format typography.TextFormat, width, height float32) (typography.TextLayout, error) {
	textFormat, err := c.createTextFormat(format)
	if err != nil {
		return nil, fmt.Errorf("create dwrite text format err: %v", err)
	}

	textLayout, hr := c.dwFactory.CreateTextLayout(text, textFormat, width, height)
	if hr.Failed() {
		return nil, fmt.Errorf("create dwrite text layout err: %v", hr)
	}

	return newTextLayout(c, textLayout, text, format, width, height), nil
}

func (c *Context) DrawTextLayout(layout typography.TextLayout, buf []byte) (bitmap typography.TextBitmap, err error) {
	if err = c.prepareDraw(); err != nil {
		return
	}
	return layout.(*TextLayout).DrawBitmap(buf)
}

func (c *Context) createTextFormat(format typography.TextFormat) (textFormat *dwrite.TextFormat, err error) {
	textFormat, hr := c.dwFactory.CreateTextFormat(format.Font.Family, nil, dwrite.DWRITE_FONT_WEIGHT_NORMAL, dwrite.DWRITE_FONT_STYLE_NORMAL, dwrite.DWRITE_FONT_STRETCH_NORMAL, format.Font.Size, "")
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
	ctx      *Context
	layout   *dwrite.TextLayout
	text     string
	format   typography.TextFormat
	width    float32
	height   float32
	position utils.StringPosition
	colors   []textColorAttr
	painter  textPainter
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
	return
}

func (t *TextLayout) Destroy() {
	if t.layout != nil {
		t.layout.Release()
		t.layout = nil
	}
	t.painter.Destroy()
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
	t.width, t.height = maxWidth, maxHeight
	t.layout.SetMaxWidth(maxWidth)
	t.layout.SetMaxHeight(maxHeight)
}

func (t *TextLayout) SetTextAlignment(align typography.TextAlignment) {
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
}

func (t *TextLayout) SetWrapMode(wrap typography.WrapMode) {
	t.format.WrapMode = wrap
	switch wrap {
	case typography.WrapNone:
		t.layout.SetWordWrapping(dwrite.DWRITE_WORD_WRAPPING_NO_WRAP)
	case typography.WrapChar:
		t.layout.SetWordWrapping(dwrite.DWRITE_WORD_WRAPPING_CHARACTER)
	case typography.WrapWordChar:
		t.layout.SetWordWrapping(dwrite.DWRITE_WORD_WRAPPING_EMERGENCY_BREAK)
	}
}

func (t *TextLayout) SetTextFont(start, length int, font typography.FontInfo) {
	if 0 <= start && 0 < length && (len(font.Family) != 0 || font.Size != 0) {
		startPos := t.position.ToUtf16(start)
		endPos := t.position.ToUtf16(start + length)
		if endPos < 0 {
			endPos = t.position.ToUtf16(len(t.text))
		}

		textRange := dwrite.TextRange{
			StartPosition: uint32(startPos),
			Length:        uint32(endPos - startPos),
		}

		if len(font.Family) != 0 {
			t.layout.SetFontFamilyName(font.Family, textRange)
		}
		if font.Size != 0 {
			t.layout.SetFontSize(font.Size, textRange)
		}
	}
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

		t.colors = append(t.colors, textColorAttr{
			Range: textRange,
			Color: toRGBAColor(color),
		})
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
	}
}

func (t *TextLayout) MeasureMetrics() (lines []typography.TextLine, clusters []typography.TextCluster) {
	lineMetrics, _ := t.layout.GetLineMetrics()
	lines = make([]typography.TextLine, 0, len(lineMetrics))

	clusterMetrics, _ := t.layout.GetClusterMetrics()
	clusters = make([]typography.TextCluster, 0, len(clusterMetrics))

	pos := 0
	index := 0
	start := 0
	clustersEnd := 0

	xOffset, _, _, _ := t.getExtends()

	for lineIndex, metrics := range lineMetrics {
		var line typography.TextLine
		endPos := pos + int(metrics.Length)
		endU8Pos := t.position.ToUtf8(endPos)
		if endU8Pos == -1 {
			endU8Pos = len(t.text)
		}
		line.Start = t.position.ToUtf8(pos)
		line.Length = endU8Pos - line.Start
		lineX, lineY, _, _ := t.layout.HitTestTextPosition(pos, false)
		lineRangeMetrics, _ := t.layout.HitTestTextRange(pos, 1, 0, 0)
		for _, rangeMetrics := range lineRangeMetrics {
			lineX = min(lineX, rangeMetrics.Left)
			lineY = min(lineY, rangeMetrics.Top)
		}
		lineEndX, _, _, _ := t.layout.HitTestTextPosition(endPos-1, true)
		line.X = lineX - xOffset
		line.Y = lineY
		line.Width = lineEndX - lineX
		line.Height = metrics.Height
		line.Baseline = line.Y + metrics.Baseline

		lastCluster := typography.TextCluster{
			X: line.X,
		}
		clustersBeg := clustersEnd

		end := start + int(metrics.Length)
		for ; index < len(clusterMetrics); index++ {
			if pos < end {
				dwCluster := clusterMetrics[index]
				if dwCluster.Width != 0 {
					var cluster typography.TextCluster
					cluster.Start = t.position.ToUtf8(pos)
					cluster.Length = t.position.ToUtf8(pos+int(dwCluster.Length)) - cluster.Start
					cluster.X = lastCluster.X + lastCluster.Width
					cluster.Y = line.Y
					cluster.Width = dwCluster.Width
					cluster.Height = line.Height
					cluster.LineIndex = lineIndex
					if dwCluster.IsRightToLeft() {
						cluster.Direction = typography.TextRightToLeft
					}
					lastCluster = cluster

					clusters = append(clusters, cluster)
					clustersEnd++
				}
				pos += int(dwCluster.Length)
				continue
			}
			break
		}
		line.Clusters = clusters[clustersBeg:clustersEnd]
		lines = append(lines, line)

		start = end
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
				return nil, fmt.Errorf("create d2d solid color brush err: %v", err)
			}
			d2dBrush = &b.Brush
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

func (t *TextLayout) DrawBitmap(buf []byte) (bitmap typography.TextBitmap, err error) {
	_, _, width, height := t.getExtends()
	if width == 0 || height == 0 {
		return
	}

	width = min(width, t.width)
	height = min(height, t.height)

	if t.painter.width < width || t.painter.height < height {
		t.painter.Destroy()
		err = t.painter.Init(t.ctx.d2dFactory, t.ctx.imgFactory, width, height)
		if err != nil {
			return
		}
	}

	err = t.painter.DrawTextLayout(t)
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

func (p *textPainter) DrawTextLayout(layout *TextLayout) (err error) {
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
	rgba := toRGBAColor(c)
	return d2d1.ColorF{
		R: float32(rgba.R) / 255,
		G: float32(rgba.G) / 255,
		B: float32(rgba.B) / 255,
		A: float32(rgba.A) / 255,
	}
}

func toRGBAColor(c color.Color) (rgba color.RGBA) {
	if c == nil {
		c = typography.DefaultTextColor()
	}
	return color.RGBAModel.Convert(c).(color.RGBA)
}
