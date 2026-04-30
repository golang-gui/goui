package directwrite

import (
	"errors"
	"fmt"
	"image/color"
	"math"
	"slices"

	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/platform/typography"
	"github.com/golang-gui/goui/platform/typography/utils"
	"github.com/golang-gui/goui/platform/win32/sdk/com"
	"github.com/golang-gui/goui/platform/win32/sdk/d2d1"
	"github.com/golang-gui/goui/platform/win32/sdk/dwrite"
	"github.com/golang-gui/goui/platform/win32/sdk/dxgi"
	"github.com/golang-gui/goui/platform/win32/sdk/wic"
)

type Context struct {
	dwFactory *dwrite.Factory
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
	textFormat, err := CreateTextFormat(c.dwFactory, format)
	if err != nil {
		return nil, fmt.Errorf("create dwrite text format err: %v", err)
	}

	textLayout, hr := c.dwFactory.CreateTextLayout(text, textFormat, width, height)
	if hr.Failed() {
		return nil, fmt.Errorf("create dwrite text layout err: %v", hr)
	}

	return newTextLayout(text, textLayout), nil
}

type TextLayout struct {
	*dwrite.TextLayout
	text     string
	position utils.StringPosition
	attrs    []typography.TextAttribute

	width      int
	height     int
	d2dFactory *d2d1.Factory1
	imgFactory *wic.ImagingFactory
	bitmap     *wic.Bitmap
	render     *d2d1.RenderTarget
	brush      *d2d1.SolidColorBrush
}

func newTextLayout(text string, layout *dwrite.TextLayout) (t *TextLayout) {
	t = new(TextLayout)
	t.TextLayout = layout
	t.text = text
	t.position = utils.CalcStringPosition(text)
	return
}

func (t *TextLayout) Destroy() {
	if t.TextLayout != nil {
		t.TextLayout.Release()
		t.TextLayout = nil
	}
	t.destroyRender()
}

func (*TextLayout) Name() string {
	return "DirectWrite"
}

func (t *TextLayout) SetSize(maxWidth, maxHeight float32) {
	t.TextLayout.SetMaxWidth(maxWidth)
	t.TextLayout.SetMaxHeight(maxHeight)
}

func (t *TextLayout) SetTextAlignment(align typography.TextAlignment) {
	switch align {
	case typography.TextAlignBegin:
		t.TextLayout.SetTextAlignment(dwrite.DWRITE_TEXT_ALIGNMENT_LEADING)
	case typography.TextAlignEnd:
		t.TextLayout.SetTextAlignment(dwrite.DWRITE_TEXT_ALIGNMENT_TRAILING)
	case typography.TextAlignCenter:
		t.TextLayout.SetTextAlignment(dwrite.DWRITE_TEXT_ALIGNMENT_CENTER)
	case typography.TextAlignFill:
		t.TextLayout.SetTextAlignment(dwrite.DWRITE_TEXT_ALIGNMENT_JUSTIFIED)
	}
}

func (t *TextLayout) SetLineAlignment(align typography.LineAlignment) {
	switch align {
	case typography.LineAlignBegin:
		t.TextLayout.SetParagraphAlignment(dwrite.DWRITE_PARAGRAPH_ALIGNMENT_NEAR)
	case typography.LineAlignEnd:
		t.TextLayout.SetParagraphAlignment(dwrite.DWRITE_PARAGRAPH_ALIGNMENT_FAR)
	case typography.LineAlignCenter:
		t.TextLayout.SetParagraphAlignment(dwrite.DWRITE_PARAGRAPH_ALIGNMENT_CENTER)
	}
}

func (t *TextLayout) SetWordWrap(wrap typography.WrapMode) {
	switch wrap {
	case typography.WrapNone:
		t.TextLayout.SetWordWrapping(dwrite.DWRITE_WORD_WRAPPING_NO_WRAP)
	case typography.WrapWord:
		t.TextLayout.SetWordWrapping(dwrite.DWRITE_WORD_WRAPPING_WHOLE_WORD)
	case typography.WrapChar:
		t.TextLayout.SetWordWrapping(dwrite.DWRITE_WORD_WRAPPING_CHARACTER)
	case typography.WrapWordChar:
		t.TextLayout.SetWordWrapping(dwrite.DWRITE_WORD_WRAPPING_EMERGENCY_BREAK)
	}
}

func (t *TextLayout) SetAttribute(attr typography.TextAttribute) {
	existed := slices.ContainsFunc(t.attrs, func(elem typography.TextAttribute) bool {
		return elem.Start == attr.Start && elem.Length == attr.Length && elem.Type == attr.Type
	})
	if !existed {
		start := t.position.ToUtf16(attr.Start)
		if start < 0 {
			// TODO: error
			return
		}
		end := t.position.ToUtf16(attr.Start + attr.Length)
		if end < 0 {
			end = t.position.ToUtf16(len(t.text))
		}

		textRange := dwrite.TextRange{
			StartPosition: uint32(start),
			Length:        uint32(end - start),
		}

		switch attr.Type {
		case typography.TextFont:
			font := attr.Value.(typography.FontInfo)
			t.TextLayout.SetFontFamilyName(font.Family, textRange)
			t.TextLayout.SetFontSize(font.Size, textRange)
			// TODO: set other font arg: weight, kern ...
		case typography.TextFgColor, typography.TextBgColor:
			// lazy to render
		case typography.TextUnderline:
			underline := attr.Value.(bool)
			t.TextLayout.SetUnderline(underline, textRange)
		case typography.TextStrike:
			strike := attr.Value.(bool)
			t.TextLayout.SetStrikethrough(strike, textRange)
		}

		t.attrs = append(t.attrs, attr)
	}
}

func (t *TextLayout) GetAttributes() []typography.TextAttribute {
	return slices.Clone(t.attrs)
}

func (t *TextLayout) GetLineRuns() (lines []typography.TextLine, runs []typography.TextRun) {
	lineMetrics, _ := t.TextLayout.GetLineMetrics()
	lines = make([]typography.TextLine, 0, len(lineMetrics))

	clusters, _ := t.TextLayout.GetClusterMetrics()
	runs = make([]typography.TextRun, 0, len(clusters))

	var lastLine typography.TextLine
	pos := 0
	index := 0
	start := 0
	runsCount := 0

	for _, metrics := range lineMetrics {
		var line typography.TextLine
		line.Start = t.position.ToUtf8(pos)
		line.X = math.MaxFloat32
		line.Y = lastLine.Y + lastLine.Height
		line.Height = metrics.Height
		line.Baseline = metrics.Baseline

		var lastRun typography.TextRun
		runsBeg := runsCount

		end := start + int(metrics.Length)
		for ; index < len(clusters); index++ {
			if pos < end {
				cluster := clusters[index]
				if cluster.Width != 0 {
					var run typography.TextRun
					run.Start = t.position.ToUtf8(pos)
					run.Length = t.position.ToUtf8(pos+int(cluster.Length)) - run.Start
					run.X = lastRun.X + lastRun.Width
					run.Y = line.Y
					run.Width = cluster.Width
					run.Height = line.Height
					if cluster.IsRightToLeft() {
						run.Direction = typography.TextRightToLeft
					}
					lastRun = run

					line.Length += run.Length
					line.X = min(line.X, run.X)
					line.Width = run.X + run.Width
					runs = append(runs, run)
					runsCount++
				}
				pos += int(cluster.Length)
				continue
			}
			break
		}
		line.Runs = runs[runsBeg:runsCount]
		lastLine = line
		lines = append(lines, line)

		start = end
	}

	return
}

func (t *TextLayout) Measure() (width, height float32) {
	metrics, _ := t.GetMetrics()
	width, height = metrics.WidthIncludingTrailingWhitespace, metrics.Height
	return
}

func (t *TextLayout) Render(brush graphics.Brush, buf []byte) (bitmap graphics.Bitmap, err error) {
	fgColor, ok := brush.(graphics.Color)
	if !ok {
		return bitmap, errors.New("unsupported brush")
	}

	width, height := t.Measure()
	bitmap.Width = roundPixel(width)
	bitmap.Height = roundPixel(height)
	bitmap.Format = graphics.PixelFormatBGRA
	bitmap.Stride = bitmap.Width * bitmap.Format.BytesPerPixel()

	if bitmap.Width == 0 || bitmap.Height == 0 {
		return bitmap, nil
	}

	err = t.prepareRender(bitmap.Width, bitmap.Height, fgColor)
	if err != nil {
		return
	}

	t.render.BeginDraw()
	t.render.Clear(&d2d1.ColorF{})
	RenderTextLayout(t.render, d2d1.Point2F{}, t, &t.brush.Brush, d2d1.D2D1_DRAW_TEXT_OPTIONS_ENABLE_COLOR_FONT|d2d1.D2D1_DRAW_TEXT_OPTIONS_CLIP)
	t.render.EndDraw(nil, nil)

	byteSize := bitmap.Stride * bitmap.Height
	bitmap.Pixels = slices.Grow(buf, byteSize)[:byteSize]
	hr := t.bitmap.CopyPixels(nil, bitmap.Stride, bitmap.Pixels)
	if hr.Failed() {
		return bitmap, hr
	}

	return bitmap, nil
}

func (t *TextLayout) prepareRender(width, height int, fgColor graphics.Color) (err error) {
	if t.d2dFactory == nil {
		t.d2dFactory, err = d2d1.CreateFactory[d2d1.Factory1](d2d1.D2D1_FACTORY_TYPE_SINGLE_THREADED, d2d1.IID_ID2D1Factory1, nil)
		if err != nil {
			return fmt.Errorf("create d2d factory err: %v", err)
		}

		t.imgFactory, err = wic.CreateImagingFactory[wic.ImagingFactory](wic.CLSID_WICImagingFactory2, wic.IID_IWICImagingFactory)
		if err != nil {
			t.d2dFactory.Release()
			t.d2dFactory = nil
			return fmt.Errorf("create wic factory err: %v", err)
		}
	}

	if t.width != width || t.height != height {
		t.destroyBitmapRender()
		t.width, t.height = width, height

		var hr com.HRESULT
		t.bitmap, hr = t.imgFactory.CreateBitmap(width, height, wic.GUID_WICPixelFormat32bppPBGRA, wic.WICBitmapCacheOnLoad)
		if hr.Failed() {
			return fmt.Errorf("create wic bitmap err: %v", hr)
		}

		props := d2d1.RenderTargetProperties{
			PixelFormat: d2d1.PixelFormat{
				Format:    dxgi.DXGI_FORMAT_B8G8R8A8_UNORM,
				AlphaMode: d2d1.D2D1_ALPHA_MODE_PREMULTIPLIED,
			},
			DpiX: 96,
			DpiY: 96,
		}

		t.render, hr = t.d2dFactory.CreateWicBitmapRenderTarget(t.bitmap, &props)
		if hr.Failed() {
			return fmt.Errorf("create d2d wic bitmap render target err: %v", hr)
		}

		t.brush, hr = t.render.CreateSolidColorBrush(&d2d1.ColorF{R: fgColor.R, G: fgColor.G, B: fgColor.B, A: fgColor.A}, nil)
		if hr.Failed() {
			return fmt.Errorf("create d2d color brush err: %v", hr)
		}

		return nil
	}

	t.brush.SetColor(&d2d1.ColorF{R: fgColor.R, G: fgColor.G, B: fgColor.B, A: fgColor.A})
	return nil
}

func (t *TextLayout) destroyRender() {
	t.destroyBitmapRender()
	if t.imgFactory != nil {
		t.imgFactory.Release()
		t.imgFactory = nil
	}
	if t.d2dFactory != nil {
		t.d2dFactory.Release()
		t.d2dFactory = nil
	}
}

func (t *TextLayout) destroyBitmapRender() {
	if t.brush != nil {
		t.brush.Release()
		t.brush = nil
	}
	if t.render != nil {
		t.render.Release()
		t.render = nil
	}
	if t.bitmap != nil {
		t.bitmap.Release()
		t.bitmap = nil
	}
}

func roundPixel(v float32) int {
	return int(v + 0.99)
}

func CreateTextFormat(factory *dwrite.Factory, format typography.TextFormat) (textFormat *dwrite.TextFormat, err error) {
	textFormat, hr := factory.CreateTextFormat(format.Font.Family, nil, dwrite.DWRITE_FONT_WEIGHT_NORMAL, dwrite.DWRITE_FONT_STYLE_NORMAL, dwrite.DWRITE_FONT_STRETCH_NORMAL, format.Font.Size, "")
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

func RenderTextLayout(render *d2d1.RenderTarget, origin d2d1.Point2F, layout *TextLayout, brush *d2d1.Brush, drawOptions d2d1.DrawTextOptions) (err error) {
	fgColorAttrs := make([]typography.TextAttribute, 0, len(layout.attrs))
	bgColorAttrs := make([]typography.TextAttribute, 0, len(layout.attrs))
	for _, attr := range layout.attrs {
		switch attr.Type {
		case typography.TextFgColor:
			fgColorAttrs = append(fgColorAttrs, attr)
		case typography.TextBgColor:
			bgColorAttrs = append(bgColorAttrs, attr)
		}
	}
	if len(fgColorAttrs)+len(bgColorAttrs) != 0 {
		var (
			rgba     color.RGBA
			d2dColor d2d1.ColorF
		)

		brushs := make(map[color.RGBA]*d2d1.Brush, len(fgColorAttrs)+len(bgColorAttrs))
		defer func() {
			for _, b := range brushs {
				b.Release()
			}
		}()

		getColorBrush := func(gColor graphics.Color) (*d2d1.Brush, error) {
			rgba.R, rgba.G, rgba.B, rgba.A = gColor.RGBA8()
			d2dBrush, exist := brushs[rgba]
			if !exist {
				d2dColor.R, d2dColor.G, d2dColor.B, d2dColor.A = gColor.R, gColor.G, gColor.B, gColor.A
				b, hr := render.CreateSolidColorBrush(&d2dColor, nil)
				if hr.Failed() {
					return nil, fmt.Errorf("create d2d solid color brush err: %v", err)
				}
				d2dBrush = &b.Brush
			}
			return d2dBrush, nil
		}

		for _, attr := range fgColorAttrs {
			gColor := attr.Value.(graphics.Color)

			start := layout.position.ToUtf16(attr.Start)
			if start < 0 {
				// TODO: error
				continue
			}
			end := layout.position.ToUtf16(attr.Start + attr.Length)
			if end < 0 {
				end = layout.position.ToUtf16(len(layout.text))
			}

			d2dBrush, err := getColorBrush(gColor)
			if err != nil {
				return err
			}

			textRange := dwrite.TextRange{
				StartPosition: uint32(start),
				Length:        uint32(end - start),
			}
			layout.TextLayout.SetDrawingEffect(&d2dBrush.Unknown, textRange)
		}

		if len(bgColorAttrs) != 0 {
			var rect d2d1.RectF
			lines, _ := layout.GetLineRuns()
			for _, line := range lines {
				for _, attr := range bgColorAttrs {
					gColor := attr.Value.(graphics.Color)
					end := attr.Start + attr.Length
					var fillX float32 = math.MaxFloat32
					var fillY float32 = math.MaxFloat32
					var fillW, fillH float32
					for _, run := range line.Runs {
						if attr.Start <= run.Start && run.Start+run.Length <= end {
							fillX = min(fillX, run.X)
							fillY = min(fillY, run.Y)
							fillW += run.Width
							fillH = max(run.Height)
						}
					}
					if fillW != 0 {
						rect.Left = origin.X + fillX
						rect.Top = origin.Y + fillY
						rect.Right = rect.Left + fillW
						rect.Bottom = rect.Top + fillH
						d2dBrush, err := getColorBrush(gColor)
						if err != nil {
							return err
						}
						render.FillRectangle(&rect, d2dBrush)
					}
				}
			}
		}
	}

	render.DrawTextLayout(origin, layout.TextLayout, brush, drawOptions)
	return nil
}
