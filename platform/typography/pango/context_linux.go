package pango

import (
	"errors"
	"fmt"
	"image/color"
	"math"
	"slices"
	"unicode/utf8"

	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/platform/typography"
	"github.com/golang-gui/goui/platform/typography/internal/lifecycle"
	"github.com/golang-gui/goui/platform/typography/utils"

	"github.com/golang-gui/goui/platform/linux/libs/cairo"
	"github.com/golang-gui/goui/platform/linux/libs/pango"
	"github.com/golang-gui/goui/platform/linux/libs/pango_cairo"

	"github.com/goexlib/cgo"
)

type Context struct {
	fontMap pango.FontMap
}

func NewContext() (_ typography.Context, err error) {
	c := new(Context)
	c.fontMap = pango_cairo.FontMapNew()
	if c.fontMap.IsNull() {
		return nil, errors.New("create pango cairo font map failed")
	}

	return c, nil
}

func (c *Context) Name() string {
	return "Pango"
}

func (c *Context) Destroy() {
	if c.fontMap.Valid() {
		c.fontMap.Unref()
		c.fontMap.GObject = 0
	}
}

func (c *Context) AddFont(fontFile string) error {
	ok := c.fontMap.GetConfig().AppFontAddFile(fontFile)
	if !ok {
		return errors.New("add font file failed")
	}
	return nil
}

func (c *Context) NewTextLayout(text string, format typography.TextFormat, width, height float32) (typography.TextLayout, error) {
	desc := pango.FontDescriptionNew()
	if desc == 0 {
		return nil, errors.New("create pango font description failed")
	}
	defer desc.Free()

	desc.SetFamily(format.Font.Family)
	desc.SetSize(int(format.Font.Size * pango.Scale))
	// TODO: other font param

	layoutContext := c.fontMap.CreateContext()
	if layoutContext.IsNull() {
		return nil, errors.New("create pango context from font map failed")
	}
	// Fix raster options on this layout's own shaping context. Pango installs
	// its font options when drawing, so setting only the destination Cairo
	// context would not guarantee grayscale alpha on transparent bitmaps.
	options := cairo.FontOptionsCreate()
	if options != 0 {
		options.SetAntialias(cairo.AntialiasGray)
		pango_cairo.ContextSetFontOptions(layoutContext, options)
		options.Destroy()
	}

	layout := pango.LayoutNew(layoutContext)
	if layout.IsNull() {
		layoutContext.Unref()
		return nil, errors.New("create pango layout failed")
	}
	layout.SetText(text)
	layout.SetFontDescription(desc)

	return newTextLayout(layoutContext, layout, text, format, width, height), nil
}

type TextLayout struct {
	context pango.Context
	layout  pango.Layout
	text    string
	format  typography.TextFormat
	width   float32
	height  float32
	attrs   pango.AttrList
	chars   int
	painter textPainter
	life    lifecycle.Layout
}

func newTextLayout(context pango.Context, layout pango.Layout, text string, format typography.TextFormat, width, height float32) (t *TextLayout) {
	t = &TextLayout{
		context: context,
		layout:  layout,
		text:    text,
		format:  format,
		width:   width,
		height:  height,
		attrs:   pango.AttrListNew(),
		chars:   utf8.RuneCountInString(text),
	}
	// Force the public setters to apply the initial native values. No listener
	// can observe these construction-time notifications.
	t.format.TextAlign = typography.TextAlignment(-1)
	t.format.WrapMode = typography.WrapMode(-1)
	t.SetSize(width, height)
	t.SetTextAlignment(format.TextAlign)
	t.SetWrapMode(format.WrapMode)
	t.layout.SetAttributes(t.attrs)
	return
}

func (t *TextLayout) Destroy() {
	if !t.life.BeginDestroy() {
		return
	}
	t.painter.Destroy()
	if t.layout.Valid() {
		t.layout.Unref()
		t.layout.GObject = 0
	}
	t.attrs.Unref()
	if t.context.Valid() {
		t.context.Unref()
		t.context.GObject = 0
	}
}

func (t *TextLayout) ConnectChanged(fn func()) signal.Handle {
	return t.life.ConnectChanged(fn)
}

func (t *TextLayout) ConnectDestroy(fn func()) signal.Handle {
	return t.life.ConnectDestroy(fn)
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
	t.width = maxWidth
	t.height = maxHeight
	if t.format.WrapMode != typography.WrapNone {
		t.layout.SetWidth(roundToPixel(t.width) * pango.Scale)
	}
	t.life.Changed()
}

func (t *TextLayout) SetTextAlignment(align typography.TextAlignment) {
	if t.format.TextAlign == align {
		return
	}
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
	t.life.Changed()
}

func (t *TextLayout) SetWrapMode(wrap typography.WrapMode) {
	if t.format.WrapMode == wrap {
		return
	}
	t.format.WrapMode = wrap
	switch wrap {
	case typography.WrapChar:
		t.layout.SetWrap(pango.WrapChar)
	case typography.WrapWordChar:
		t.layout.SetWrap(pango.WrapWordChar)
	}
	if wrap != typography.WrapNone {
		t.layout.SetWidth(roundToPixel(t.width) * pango.Scale)
	} else {
		t.layout.SetWidth(-1)
	}
	t.life.Changed()
}

func (t *TextLayout) SetTextFont(start, length int, font typography.FontInfo) {
	if 0 <= start && start < len(t.text) {
		if length < 0 {
			length = len(t.text)
		}

		familyAttr := pango.AttrFamilyNew(font.Family)
		familyAttr.StartIndex = uint32(start)
		familyAttr.EndIndex = uint32(start + length)
		t.attrs.Change(familyAttr)

		sizeAttr := pango.AttrSizeNew(int(font.Size * pango.Scale))
		sizeAttr.StartIndex = uint32(start)
		sizeAttr.EndIndex = uint32(start + length)
		t.attrs.Change(sizeAttr)

		t.layout.ContextChanged()
		t.life.Changed()
	}
}

func (t *TextLayout) SetTextColor(start, length int, foreground color.Color) {
	if 0 <= start && start < len(t.text) {
		if length < 0 {
			length = len(t.text)
		}

		if foreground == nil {
			foreground = typography.DefaultTextColor()
		}
		c := color.NRGBA64Model.Convert(foreground).(color.NRGBA64)
		attr := pango.AttrForegroundNew(c.R, c.G, c.B)
		attr.StartIndex = uint32(start)
		attr.EndIndex = uint32(start + length)
		t.attrs.Change(attr)
		// PangoCairo treats renderer alpha 0 as "use the default", not fully
		// transparent. The smallest explicit alpha rounds to zero in our 8-bit
		// text bitmap and avoids accidentally painting an opaque black run.
		alpha := pango.AttrForegroundAlphaNew(max(c.A, 1))
		alpha.StartIndex = uint32(start)
		alpha.EndIndex = uint32(start + length)
		t.attrs.Change(alpha)

		t.layout.ContextChanged()
		t.life.Changed()
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
		t.attrs.Change(attr)

		t.layout.ContextChanged()
		t.life.Changed()
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
		t.attrs.Change(attr)

		t.layout.ContextChanged()
		t.life.Changed()
	}
}

func (t *TextLayout) MeasureSize() (width, height float32) {
	_, _, width, height = t.getExtents()
	return
}

func (t *TextLayout) MeasureMetrics() (lines []typography.TextLine, clusters []typography.TextCluster) {
	lineCount := t.layout.GetLineCount()
	if lineCount == 0 {
		return
	}
	lines = make([]typography.TextLine, 0, lineCount)
	clusters = make([]typography.TextCluster, 0, t.chars)
	xOffset, yOffset, _, _ := t.getExtents()
	iter := t.layout.GetIter()
	defer iter.Free()
	// Keep independent line/cluster cursors. NextCluster skips empty lines,
	// while copying a line cursor on Pango 1.50.6 loses end_x_offset and can
	// produce uninitialized X positions when advancing to the next font run.
	clusterIter := t.layout.GetIter()
	defer clusterIter.Free()
	for {
		nativeLine := iter.GetLineReadonly()
		if nativeLine == nil {
			break
		}
		_, rect := iter.GetLineExtents()
		line := typography.TextLine{
			Start: int(nativeLine.StartIndex), Length: int(nativeLine.Length),
			X:     float32(rect.X)/pango.Scale - xOffset,
			Y:     float32(rect.Y)/pango.Scale - yOffset,
			Width: float32(rect.Width) / pango.Scale, Height: float32(rect.Height) / pango.Scale,
			Baseline: float32(iter.GetBaseline())/pango.Scale - yOffset,
		}
		beg := len(clusters)
		// NextCluster skips empty lines. Keep a separate line iterator so empty
		// and trailing lines retain their native metrics and stable LineIndex.
		for clusterIter.GetLineReadonly() == nativeLine {
			run := clusterIter.GetRunReadonly()
			if run != nil && run.Item != nil {
				_, clusterRect := clusterIter.GetClusterExtents()
				clusters = append(clusters, typography.TextCluster{
					Start: clusterIter.GetIndex(),
					X:     float32(clusterRect.X)/pango.Scale - xOffset, Y: line.Y,
					Width: float32(clusterRect.Width) / pango.Scale, Height: line.Height,
					LineIndex: len(lines),
					Direction: typography.TextDirection(run.Item.Analysis.Level & 1),
				})
			}
			if !clusterIter.NextCluster() {
				break
			}
		}
		line.Clusters = clusters[beg:]
		slices.SortFunc(line.Clusters, func(a, b typography.TextCluster) int { return a.Start - b.Start })
		for i := range line.Clusters {
			end := line.Start + line.Length
			if i+1 < len(line.Clusters) {
				end = line.Clusters[i+1].Start
			}
			line.Clusters[i].Length = end - line.Clusters[i].Start
		}
		lines = append(lines, line)
		if !iter.NextLine() {
			break
		}
	}
	return
}

func (t *TextLayout) Rasterize(scale float32, buf []byte) (bitmap typography.TextBitmap, err error) {
	if t.life.IsDestroyed() {
		return bitmap, errors.New("pango: rasterize destroyed text layout")
	}
	if scale <= 0 || math.IsNaN(float64(scale)) || math.IsInf(float64(scale), 0) {
		return bitmap, fmt.Errorf("pango: invalid raster scale %v", scale)
	}
	return t.rasterize(scale, buf)
}

func (t *TextLayout) rasterize(scale float32, buf []byte) (bitmap typography.TextBitmap, err error) {
	x, y, width, height := t.getExtents()
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
		err = t.painter.Init(widthCapacity, heightCapacity)
		if err != nil {
			return
		}
	}

	err = t.painter.DrawTextLayout(t, -x, -y, scale)
	if err != nil {
		return
	}

	return t.painter.GetBitmap(width, height, buf), nil
}

func (t *TextLayout) getExtents() (x, y, width, height float32) {
	_, rect := t.layout.GetExtents()
	x = float32(rect.X) / pango.Scale
	y = float32(rect.Y) / pango.Scale
	width = float32(rect.Width) / pango.Scale
	height = float32(rect.Height) / pango.Scale
	return
}

type textPainter struct {
	width   float32
	height  float32
	bitmap  typography.TextBitmap
	surface cairo.Surface
	context cairo.Context
}

func (p *textPainter) Init(width, height float32) (err error) {
	p.width = width
	p.height = height
	p.bitmap.Width = roundToPixel(width)
	p.bitmap.Height = roundToPixel(height)
	p.bitmap.Stride = p.bitmap.Width * 4
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

	return nil
}

func (p *textPainter) Destroy() {
	if p.context != 0 {
		p.context.Destroy()
		p.context = 0
	}
	if p.surface != 0 {
		p.surface.Destroy()
		p.surface = 0
	}
}

func (p *textPainter) DrawTextLayout(t *TextLayout, x, y, scale float32) (err error) {
	cgo.Memset(cgo.CSlice(p.bitmap.Pixels), 0, cgo.Sizet(len(p.bitmap.Pixels)))
	p.context.Save()
	defer p.context.Restore()

	r, g, b, a := toColor(t.format.TextColor)
	p.context.Scale(float64(scale), float64(scale))
	p.context.SetSourceRGBA(r, g, b, a)
	p.context.MoveTo(float64(x), float64(y))
	// Raster scale changes the device mapping, not the measured logical layout.
	// UpdateLayout would re-shape with scale-dependent hinted advances and can
	// change wrapping; restoring it afterwards cannot fix the pixels just drawn.
	pango_cairo.ShowLayout(p.context, t.layout)
	if status := p.context.Status(); status != 0 {
		return fmt.Errorf("cairo draw pango layout err: %v", status)
	}
	return
}

func (p *textPainter) GetBitmap(width, height float32, buf []byte) (bitmap typography.TextBitmap) {
	bitmap.Width = roundToPixel(width)
	bitmap.Height = roundToPixel(height)
	bitmap = utils.CopyBitmap(p.bitmap, bitmap.Width, bitmap.Height, buf)
	utils.ReverseBitmap(bitmap)
	return
}

func roundToPixel(num float32) int {
	return int(num + 0.99)
}

func toColor(c color.Color) (r, g, b, a float64) {
	if c == nil {
		c = typography.DefaultTextColor()
	}
	// cairo_set_source_rgba accepts straight components, not Go's RGBA values.
	nrgba := color.NRGBA64Model.Convert(c).(color.NRGBA64)
	r = float64(nrgba.R) / 65535.0
	g = float64(nrgba.G) / 65535.0
	b = float64(nrgba.B) / 65535.0
	a = float64(nrgba.A) / 65535.0
	return
}
