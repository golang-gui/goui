package pango

import (
	"errors"
	"fmt"
	"image"
	"image/color"
	"slices"
	"unicode/utf8"

	"github.com/goexlib/cgo"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/platform/typography"

	"github.com/golang-gui/goui/platform/linux/libs/cairo"
	"github.com/golang-gui/goui/platform/linux/libs/pango"
	"github.com/golang-gui/goui/platform/linux/libs/pango_cairo"
)

type Context struct {
	fontMap pango.FontMap
	context pango.Context
}

func NewContext() (_ *Context, err error) {
	c := new(Context)
	c.fontMap = pango_cairo.FontMapNew()
	if c.fontMap.IsNull() {
		return nil, errors.New("create pango cairo font map failed")
	}

	c.context = c.fontMap.CreateContext()
	if c.context.IsNull() {
		return nil, errors.New("create pango context from font map failed")
	}

	return c, nil
}

func (c *Context) Name() string {
	return "Pango"
}

func (c *Context) Destroy() {
	if c.context.Valid() {
		c.context.Unref()
		c.context.GObject = 0
	}
	if c.fontMap.Valid() {
		c.fontMap.Unref()
		c.fontMap.GObject = 0
	}
}

func (c *Context) AddFont(fontFile string) error {
	return errors.New("not implement")
}

func (c *Context) NewTextLayout(text string, format typography.TextFormat, width, height float32) (typography.TextLayout, error) {
	desc := pango.FontDescriptionNew()
	if desc == 0 {
		return nil, errors.New("create pango font description failed")
	}
	defer desc.Free()

	desc.SetFamily(format.Font.Family)
	desc.SetAbsoluteSize(float64(format.Font.Size) * pango.Scale)
	// TODO: other font param

	layout := pango.LayoutNew(c.context)
	if layout.IsNull() {
		return nil, errors.New("create pango layout failed")
	}
	layout.SetText(text)
	layout.SetFontDescription(desc)

	return newTextLayout(c, layout, text, format, width, height), nil
}

func (c *Context) DrawText(text string, format typography.TextFormat, width, height float32, foreground color.Color, buf []byte) (bitmap typography.TextBitmap, err error) {
	layout, err := c.NewTextLayout(text, format, width, height)
	if err != nil {
		return
	}
	return c.DrawTextLayout(layout, foreground, buf)
}

func (c *Context) DrawTextLayout(layout typography.TextLayout, foreground color.Color, buf []byte) (bitmap typography.TextBitmap, err error) {
	return layout.(*TextLayout).DrawBitmap(foreground, buf)
}

type TextLayout struct {
	ctx     *Context
	layout  pango.Layout
	text    string
	format  typography.TextFormat
	size    graphics.Size
	attrs   pango.AttrList
	painter textPainter
	chars   int
}

func newTextLayout(c *Context, layout pango.Layout, text string, format typography.TextFormat, width, height float32) (t *TextLayout) {
	t = &TextLayout{
		ctx:    c,
		layout: layout,
		text:   text,
		format: format,
		size:   graphics.Size{Width: width, Height: height},
		attrs:  pango.AttrListNew(),
		chars:  utf8.RuneCountInString(text),
	}
	t.SetSize(width, height)
	t.SetTextAlignment(format.TextAlign)
	t.SetWrapMode(format.WrapMode)
	t.layout.SetAttributes(t.attrs)
	return
}

func (t *TextLayout) Destroy() {
	t.painter.Destroy()
	if t.layout.Valid() {
		t.layout.Unref()
		t.layout.GObject = 0
	}
	t.attrs.Unref()
}

func (t *TextLayout) Text() string {
	return t.text
}

func (t *TextLayout) Format() typography.TextFormat {
	return t.format
}

func (t *TextLayout) Size() (maxWidth, maxHeight float32) {
	return t.size.Width, t.size.Height
}

func (t *TextLayout) SetSize(maxWidth, maxHeight float32) {
	t.size.Width = maxWidth
	t.size.Height = maxHeight
	if t.format.WrapMode != typography.WrapNone {
		t.layout.SetWidth(roundToPixel(t.size.Width) * pango.Scale)
	}
}

func (t *TextLayout) SetTextAlignment(align typography.TextAlignment) {
	t.format.TextAlign = align
	switch align {
	case typography.TextAlignBegin, typography.TextAlignFill:
		t.layout.SetAlignment(pango.AlignLeft)
	case typography.TextAlignEnd:
		t.layout.SetAlignment(pango.AlignRight)
	case typography.TextAlignCenter:
		t.layout.SetAlignment(pango.AlignCenter)
	}
	t.layout.SetJustify(align == typography.TextAlignFill)
}

func (t *TextLayout) SetWrapMode(wrap typography.WrapMode) {
	t.format.WrapMode = wrap
	switch wrap {
	case typography.WrapChar:
		t.layout.SetWrap(pango.WrapChar)
	case typography.WrapWordChar:
		t.layout.SetWrap(pango.WrapWordChar)
	}
	if wrap != typography.WrapNone {
		t.layout.SetWidth(roundToPixel(t.size.Width) * pango.Scale)
	} else {
		t.layout.SetWidth(-1)
	}
}

func (t *TextLayout) SetTextFont(start, length int, font typography.FontInfo) {
	if 0 <= start && start < len(t.text) {
		if length < 0 {
			length = len(t.text)
		}

		familyAttr := pango.AttrFamilyNew(font.Family)
		familyAttr.StartIndex = uint32(start)
		familyAttr.EndIndex = uint32(start + length)
		t.attrs.Insert(familyAttr)

		sizeAttr := pango.AttrSizeNewAbsolute(int(font.Size * pango.Scale))
		sizeAttr.StartIndex = uint32(start)
		sizeAttr.EndIndex = uint32(start + length)
		t.attrs.Insert(sizeAttr)

		t.layout.ContextChanged()
	}
}

func (t *TextLayout) SetTextColor(start, length int, foreground color.Color) {
	if 0 <= start && start < len(t.text) {
		if length < 0 {
			length = len(t.text)
		}

		r, g, b, _ := foreground.RGBA()
		attr := pango.AttrForegroundNew(uint16(r), uint16(g), uint16(b))
		attr.StartIndex = uint32(start)
		attr.EndIndex = uint32(start + length)
		t.attrs.Insert(attr)

		t.layout.ContextChanged()
	}
}

func (t *TextLayout) SetUnderline(start, length int, underline bool) {
	if 0 <= start && start < len(t.text) {
		if length < 0 {
			length = len(t.text)
		}

		value := pango.UnderlineNone
		if underline {
			value = pango.UnderlineSingle
		}
		attr := pango.AttrUnderlineNew(value)
		attr.StartIndex = uint32(start)
		attr.EndIndex = uint32(start + length)
		t.attrs.Insert(attr)

		t.layout.ContextChanged()
	}
}

func (t *TextLayout) SetStrikethrough(start, length int, strike bool) {
	if 0 <= start && start < len(t.text) {
		if length < 0 {
			length = len(t.text)
		}

		attr := pango.AttrStrikethroughNew(strike)
		attr.StartIndex = uint32(start)
		attr.EndIndex = uint32(start + length)
		t.attrs.Insert(attr)

		t.layout.ContextChanged()
	}
}

func (t *TextLayout) MeasureRect() (x, y, width, height float32) {
	x, y, width, height, xOffset, yOffset := t.getExtents()
	x += xOffset
	y += yOffset
	return
}

func (t *TextLayout) MeasureMetrics() (lines []typography.TextLine, clusters []typography.TextCluster) {
	lineCount := t.layout.GetLineCount()
	if lineCount != 0 {
		lines = make([]typography.TextLine, 0, lineCount)
		clusters = make([]typography.TextCluster, 0, t.chars)
		_, _, _, _, xOffset, yOffset := t.getExtents()

		iter := t.layout.GetIter()
		var clustersBeg, clustersEnd int
		var lastLine *pango.LayoutLine
		for {
			line := iter.GetLineReadonly()
			if line != lastLine {
				if linesCount := len(lines); linesCount != 0 {
					last := &lines[linesCount-1]
					last.Clusters = clusters[clustersBeg:clustersEnd]
					slices.SortFunc(last.Clusters, func(a, b typography.TextCluster) int {
						return a.Start - b.Start
					})
					clustersBeg = clustersEnd
				}
				baseline := iter.GetBaseline()
				_, lineRect := iter.GetLineExtents()
				lines = append(lines, typography.TextLine{
					Start:    int(line.StartIndex),
					Length:   int(line.Length),
					X:        xOffset + float32(lineRect.X)/pango.Scale,
					Y:        yOffset + float32(lineRect.Y)/pango.Scale,
					Width:    float32(lineRect.Width) / pango.Scale,
					Height:   float32(lineRect.Height) / pango.Scale,
					Baseline: yOffset + float32(baseline)/pango.Scale,
				})
				lastLine = line
			}

			run := iter.GetRunReadonly()

			index := iter.GetIndex()
			_, clusterRect := iter.GetClusterExtents()
			lineIndex := len(lines) - 1
			currentLine := &lines[lineIndex]
			clusters = append(clusters, typography.TextCluster{
				Start:     index,
				X:         xOffset + float32(clusterRect.X)/pango.Scale,
				Y:         currentLine.Y,
				Width:     float32(clusterRect.Width) / pango.Scale,
				Height:    currentLine.Height,
				LineIndex: lineIndex,
				Direction: typography.TextDirection(run.Item.Analysis.Level),
			})
			clustersEnd++

			if !iter.NextCluster() {
				currentLine.Clusters = clusters[clustersBeg:clustersEnd]
				slices.SortFunc(currentLine.Clusters, func(a, b typography.TextCluster) int {
					return a.Start - b.Start
				})
				break
			}
		}

		for i := 1; i < len(clusters); i++ {
			clusters[i-1].Length = clusters[i].Start - clusters[i-1].Start
		}
	}
	return
}

func (t *TextLayout) DrawBitmap(fgColor color.Color, buf []byte) (bitmap typography.TextBitmap, err error) {
	x, y, width, height, _, _ := t.getExtents()
	if width == 0 || height == 0 {
		return
	}

	width = min(width, t.size.Width)
	height = min(height, t.size.Height)

	if t.painter.width < width || t.painter.height < height {
		t.painter.Destroy()
		err = t.painter.Init(width, height)
		if err != nil {
			return
		}
	}

	err = t.painter.DrawTextLayout(t, fgColor, -x, -y)
	if err != nil {
		return
	}

	return t.painter.GetBitmap(width, height, buf), nil
}

func (t *TextLayout) getExtents() (x, y, width, height, xOffset, yOffset float32) {
	_, rect := t.layout.GetExtents()
	x = float32(rect.X) / pango.Scale
	y = float32(rect.Y) / pango.Scale
	width = float32(rect.Width) / pango.Scale
	height = float32(rect.Height) / pango.Scale

	if t.format.WrapMode == typography.WrapNone {
		if t.format.TextAlign == typography.TextAlignCenter {
			xOffset = (t.size.Width - width) / 2
		} else if t.format.TextAlign == typography.TextAlignEnd {
			xOffset = t.size.Width - width
		}
	}
	return
}

type textPainter struct {
	width   float32
	height  float32
	bitmap  graphics.Bitmap
	surface cairo.Surface
	context cairo.Context
	options cairo.FontOptions
}

func (p *textPainter) Init(width, height float32) (err error) {
	p.width = width
	p.height = height
	p.bitmap.Width = roundToPixel(width)
	p.bitmap.Height = roundToPixel(height)
	p.bitmap.Stride = p.bitmap.Width * 4
	p.bitmap.Format = graphics.PixelFormatBGRA
	p.bitmap.Pixels = make([]byte, p.bitmap.Stride*p.bitmap.Height)
	p.surface = cairo.ImageSurfaceCreateForData(p.bitmap.Pixels, cairo.FormatARGB32, p.bitmap.Width, p.bitmap.Height, p.bitmap.Stride)
	if status := p.surface.Status(); status != 0 {
		p.Destroy()
		return fmt.Errorf("create cairo image surface err: %v", status)
	}

	p.context = cairo.Create(p.surface)
	if status := p.context.Status(); status != 0 {
		p.Destroy()
		return fmt.Errorf("create cairo context err: %v", status)
	}

	p.options = cairo.FontOptionsCreate()
	if p.options != 0 {
		p.options.SetAntialias(cairo.AntialiasGray)
		p.context.SetFontOptions(p.options)
	}

	return nil
}

func (p *textPainter) Destroy() {
	if p.options != 0 {
		p.options.Destroy()
		p.options = 0
	}
	if p.context != 0 {
		p.context.Destroy()
		p.context = 0
	}
	if p.surface != 0 {
		p.surface.Destroy()
		p.surface = 0
	}
}

func (p *textPainter) DrawTextLayout(t *TextLayout, fgColor color.Color, x, y float32) (err error) {
	gColor := toColor(fgColor)
	cgo.Memset(cgo.CSlice(p.bitmap.Pixels), 0, cgo.Sizet(len(p.bitmap.Pixels)))
	p.context.SetSourceRGBA(float64(gColor.R), float64(gColor.G), float64(gColor.B), float64(gColor.A))
	p.context.MoveTo(float64(x), float64(y))
	pango_cairo.UpdateLayout(p.context, t.layout)
	pango_cairo.ShowLayout(p.context, t.layout)
	if status := p.context.Status(); status != 0 {
		return fmt.Errorf("cairo draw pango layout err: %v", status)
	}
	return
}

func (p *textPainter) GetBitmap(width, height float32, buf []byte) (bitmap typography.TextBitmap) {
	subImage := p.bitmap.SubImage(image.Rect(0, 0, roundToPixel(width), roundToPixel(height)))
	bmp := graphics.CopyToBitmap(subImage, graphics.PixelFormatRGBA, buf)
	bitmap.Width = bmp.Width
	bitmap.Height = bmp.Height
	bitmap.Stride = bmp.Stride
	bitmap.Pixels = bmp.Pixels
	return
}

func roundToPixel(num float32) int {
	return int(num + 0.99)
}

func toColor(c color.Color) graphics.Color {
	r, g, b, a := c.RGBA()
	return graphics.Color{
		R: float32(r) / 65535,
		G: float32(g) / 65535,
		B: float32(b) / 65535,
		A: float32(a) / 65535,
	}
}
