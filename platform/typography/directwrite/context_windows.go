package directwrite

import (
	"errors"
	"fmt"
	"math"

	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/platform/typography"
	"github.com/golang-gui/goui/platform/typography/typodraw"
	"github.com/golang-gui/goui/platform/typography/utils"

	"github.com/golang-gui/goui/platform/win32/sdk/com"
	"github.com/golang-gui/goui/platform/win32/sdk/d2d1"
	"github.com/golang-gui/goui/platform/win32/sdk/dwrite"
	"github.com/golang-gui/goui/platform/win32/sdk/dxgi"
	"github.com/golang-gui/goui/platform/win32/sdk/wic"
)

type Context struct {
	dwFactory  *dwrite.Factory
	d2dFactory *d2d1.Factory
	imgFactory *wic.ImagingFactory
}

func NewContext() (_ *Context, err error) {
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
}

func (c *Context) Name() string {
	return "DirectWrite"
}

func (c *Context) AddFont(fontFile string) error {
	return fmt.Errorf("not implement")
}

func (c *Context) NewTextLayout(text string, format typography.TextFormat, width, height float32) (typography.TextLayout, error) {
	textFormat, err := c.CreateTextFormat(format)
	if err != nil {
		return nil, fmt.Errorf("create dwrite text format err: %v", err)
	}

	textLayout, hr := c.dwFactory.CreateTextLayout(text, textFormat, width, height)
	if hr.Failed() {
		return nil, fmt.Errorf("create dwrite text layout err: %v", hr)
	}

	return newTextLayout(c, textLayout, text, format, width, height), nil
}

func (c *Context) DrawText(text string, format typography.TextFormat, brush graphics.Brush, size graphics.Size, pixelFormat graphics.PixelFormat, buf []byte) (bitmap typodraw.TextBitmap, err error) {
	if err = c.prepareDraw(); err != nil {
		return
	}

	fgColor, ok := brush.(graphics.Color)
	if !ok {
		return bitmap, errors.New("unsupported brush")
	}

	var painter textPainter
	err = painter.Init(c.d2dFactory, c.imgFactory, graphics.Rectangle{Size: size}, pixelFormat, fgColor)
	if err != nil {
		return
	}
	defer painter.Destroy()

	textFormat, err := c.CreateTextFormat(format)
	if err != nil {
		return bitmap, fmt.Errorf("create dwrite text format err: %v", err)
	}
	defer textFormat.Release()

	return painter.DrawText(text, textFormat, buf)
}

func (c *Context) DrawTextLayout(layout typography.TextLayout, brush graphics.Brush, pixelFormat graphics.PixelFormat, buf []byte) (bitmap typodraw.TextBitmap, err error) {
	if err = c.prepareDraw(); err != nil {
		return
	}
	return layout.(*TextLayout).DrawBitmap(brush, pixelFormat, buf)
}

func (c *Context) CreateTextFormat(format typography.TextFormat) (textFormat *dwrite.TextFormat, err error) {
	textFormat, hr := c.dwFactory.CreateTextFormat(format.Font.Family, nil, dwrite.DWRITE_FONT_WEIGHT_NORMAL, dwrite.DWRITE_FONT_STYLE_NORMAL, dwrite.DWRITE_FONT_STRETCH_NORMAL, format.Font.Size, "")
	if hr.Failed() {
		return nil, hr
	}

	switch format.WordWrap {
	case typography.WrapNone:
		textFormat.SetWordWrapping(dwrite.DWRITE_WORD_WRAPPING_NO_WRAP)
	case typography.WrapWord:
		textFormat.SetWordWrapping(dwrite.DWRITE_WORD_WRAPPING_WHOLE_WORD)
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

	switch format.LineAlign {
	case typography.LineAlignBegin:
		textFormat.SetParagraphAlignment(dwrite.DWRITE_PARAGRAPH_ALIGNMENT_NEAR)
	case typography.LineAlignEnd:
		textFormat.SetParagraphAlignment(dwrite.DWRITE_PARAGRAPH_ALIGNMENT_FAR)
	case typography.LineAlignCenter:
		textFormat.SetParagraphAlignment(dwrite.DWRITE_PARAGRAPH_ALIGNMENT_CENTER)
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

	rect    graphics.Rectangle
	painter textPainter
}

type textColorAttr struct {
	Range dwrite.TextRange
	Color graphics.Color
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

func (t *TextLayout) SetLineAlignment(align typography.LineAlignment) {
	t.format.LineAlign = align
	switch align {
	case typography.LineAlignBegin:
		t.layout.SetParagraphAlignment(dwrite.DWRITE_PARAGRAPH_ALIGNMENT_NEAR)
	case typography.LineAlignEnd:
		t.layout.SetParagraphAlignment(dwrite.DWRITE_PARAGRAPH_ALIGNMENT_FAR)
	case typography.LineAlignCenter:
		t.layout.SetParagraphAlignment(dwrite.DWRITE_PARAGRAPH_ALIGNMENT_CENTER)
	}
}

func (t *TextLayout) SetWordWrap(wrap typography.WrapMode) {
	t.format.WordWrap = wrap
	switch wrap {
	case typography.WrapNone:
		t.layout.SetWordWrapping(dwrite.DWRITE_WORD_WRAPPING_NO_WRAP)
	case typography.WrapWord:
		t.layout.SetWordWrapping(dwrite.DWRITE_WORD_WRAPPING_WHOLE_WORD)
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

func (t *TextLayout) SetTextColor(start, length int, color graphics.Color) {
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
			Color: color,
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

	for _, metrics := range lineMetrics {
		var line typography.TextLine
		endPos := pos + int(metrics.Length)
		line.Start = t.position.ToUtf8(pos)
		line.Length = t.position.ToUtf8(endPos) - line.Start
		line.X, line.Y, _, _ = t.layout.HitTestTextPosition(pos, false)
		endX, _, _, _ := t.layout.HitTestTextPosition(endPos-1, true)
		line.Width = endX - line.X
		line.Height = metrics.Height
		line.Baseline = metrics.Baseline

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

func (t *TextLayout) MeasureRect() (x, y, width, height float32) {
	hitMetrics, hr := t.layout.HitTestTextRange(0, -1, 0, 0)
	if hr.Failed() {
		return
	}
	startX := float32(math.MaxFloat32)
	startY := float32(math.MaxFloat32)
	endX := float32(0)
	endY := float32(0)
	for _, metrics := range hitMetrics {
		startX = min(startX, metrics.Left)
		startY = min(startY, metrics.Top)
		endX = max(endX, metrics.Left+metrics.Width)
		endY = max(endY, metrics.Top+metrics.Height)
	}
	return startX, startY, endX - startX, endY - startY
}

func (t *TextLayout) Draw(render *d2d1.RenderTarget, origin d2d1.Point2F, brush *d2d1.Brush, drawOptions d2d1.DrawTextOptions) (err error) {
	if len(t.colors) != 0 {
		type RGBA struct{ R, G, B, A byte }
		var (
			rgba     RGBA
			d2dColor d2d1.ColorF
		)

		brushs := make(map[RGBA]*d2d1.Brush, len(t.colors))
		defer func() {
			for _, b := range brushs {
				b.Release()
			}
		}()

		getColorBrush := func(color graphics.Color) (*d2d1.Brush, error) {
			rgba.R, rgba.G, rgba.B, rgba.A = color.RGBA8()
			d2dBrush, exist := brushs[rgba]
			if !exist {
				d2dColor.R, d2dColor.G, d2dColor.B, d2dColor.A = color.R, color.G, color.B, color.A
				b, hr := render.CreateSolidColorBrush(&d2dColor, nil)
				if hr.Failed() {
					return nil, fmt.Errorf("create d2d solid color brush err: %v", err)
				}
				d2dBrush = &b.Brush
			}
			return d2dBrush, nil
		}

		for _, attr := range t.colors {
			d2dBrush, err := getColorBrush(attr.Color)
			if err != nil {
				return err
			}
			t.layout.SetDrawingEffect(&d2dBrush.Unknown, attr.Range)
		}
	}

	render.DrawTextLayout(origin, t.layout, brush, drawOptions)
	return nil
}

func (t *TextLayout) DrawBitmap(brush graphics.Brush, pixelFormat graphics.PixelFormat, buf []byte) (bitmap typodraw.TextBitmap, err error) {
	fgColor, ok := brush.(graphics.Color)
	if !ok {
		return bitmap, errors.New("unsupported brush")
	}

	x, y, width, height := t.MeasureRect()
	if width == 0 || height == 0 {
		return
	}

	rect := graphics.Rect(x, y, width, height)
	if t.rect != rect {
		t.rect = rect
		t.painter.Destroy()
		err = t.painter.Init(t.ctx.d2dFactory, t.ctx.imgFactory, rect, pixelFormat, fgColor)
		if err != nil {
			return
		}
	}

	return t.painter.DrawTextLayout(t, buf)
}

func roundPixel(v float32) int {
	return int(v + 0.99)
}

type textPainter struct {
	rect   graphics.Rectangle
	format graphics.PixelFormat
	bitmap *wic.Bitmap
	render *d2d1.RenderTarget
	brush  *d2d1.SolidColorBrush
}

func (p *textPainter) Init(d2dFactory *d2d1.Factory, imgFactory *wic.ImagingFactory, rect graphics.Rectangle, pixelFormat graphics.PixelFormat, fgColor graphics.Color) (err error) {
	p.rect = rect
	p.format = pixelFormat

	wicPixelFormat := wic.GUID_WICPixelFormat32bppPBGRA
	d2dPixelFormat := d2d1.PixelFormat{
		Format:    dxgi.DXGI_FORMAT_B8G8R8A8_UNORM,
		AlphaMode: d2d1.D2D1_ALPHA_MODE_PREMULTIPLIED,
	}
	switch pixelFormat {
	case graphics.PixelFormatRGBA:
		wicPixelFormat = wic.GUID_WICPixelFormat32bppPRGBA
		d2dPixelFormat.Format = dxgi.DXGI_FORMAT_R8G8B8A8_UNORM
	case graphics.PixelFormatGray:
		wicPixelFormat = wic.GUID_WICPixelFormat8bppGray
		d2dPixelFormat.Format = dxgi.DXGI_FORMAT_A8_UNORM
	}

	var hr com.HRESULT
	p.bitmap, hr = imgFactory.CreateBitmap(roundPixel(rect.X+rect.Width), roundPixel(rect.Y+rect.Height), wicPixelFormat, wic.WICBitmapCacheOnLoad)
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

	p.brush, hr = p.render.CreateSolidColorBrush(&d2d1.ColorF{R: fgColor.R, G: fgColor.G, B: fgColor.B, A: fgColor.A}, nil)
	if hr.Failed() {
		return fmt.Errorf("create d2d color brush err: %v", hr)
	}

	return nil
}

func (p *textPainter) Destroy() {
	if p.brush != nil {
		p.brush.Release()
		p.brush = nil
	}
	if p.render != nil {
		p.render.Release()
		p.render = nil
	}
	if p.bitmap != nil {
		p.bitmap.Release()
		p.bitmap = nil
	}
}

func (p *textPainter) DrawText(text string, format *dwrite.TextFormat, buf []byte) (typodraw.TextBitmap, error) {
	p.render.BeginDraw()
	p.render.Clear(&d2d1.ColorF{})
	p.render.DrawText(text, format, &d2d1.RectF{Right: p.rect.Width, Bottom: p.rect.Height}, &p.brush.Brush, d2d1.D2D1_DRAW_TEXT_OPTIONS_ENABLE_COLOR_FONT|d2d1.D2D1_DRAW_TEXT_OPTIONS_CLIP, 0)
	p.render.EndDraw(nil, nil)
	return p.getBitmap(buf)
}

func (p *textPainter) DrawTextLayout(layout *TextLayout, buf []byte) (bitmap typodraw.TextBitmap, err error) {
	p.render.BeginDraw()
	p.render.Clear(&d2d1.ColorF{})
	err = layout.Draw(p.render, d2d1.Point2F{}, &p.brush.Brush, d2d1.D2D1_DRAW_TEXT_OPTIONS_ENABLE_COLOR_FONT|d2d1.D2D1_DRAW_TEXT_OPTIONS_CLIP)
	p.render.EndDraw(nil, nil)
	if err != nil {
		return
	}
	return p.getBitmap(buf)
}

func (p *textPainter) getBitmap(buf []byte) (bitmap typodraw.TextBitmap, err error) {
	bitmap.Bitmap = graphics.MakeBitmap(0, 0, roundPixel(p.rect.Width), roundPixel(p.rect.Height), p.format, buf)
	rect := wic.Rect{
		X:      int32(p.rect.X),
		Y:      int32(p.rect.Y),
		Width:  int32(bitmap.Bitmap.Width),
		Height: int32(bitmap.Bitmap.Height),
	}
	hr := p.bitmap.CopyPixels(&rect, bitmap.Bitmap.Stride, bitmap.Bitmap.Pixels)
	if hr.Failed() {
		return bitmap, hr
	}

	return bitmap, nil
}
