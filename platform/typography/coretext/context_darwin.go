package coretext

import (
	"errors"
	"fmt"
	"image/color"
	"math"
	"slices"
	"strings"

	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/platform/typography"
	"github.com/golang-gui/goui/platform/typography/internal/lifecycle"
	"github.com/golang-gui/goui/platform/typography/utils"

	"github.com/golang-gui/goui/platform/darwin/frameworks"
	. "github.com/golang-gui/goui/platform/darwin/frameworks/core_foundation"
	. "github.com/golang-gui/goui/platform/darwin/frameworks/core_graphics"
	. "github.com/golang-gui/goui/platform/darwin/frameworks/core_text"
)

type Context struct {
	fonts []CTFontRef // registered custom fonts (for cleanup if needed)
}

func NewContext() (typography.Context, error) {
	err := frameworks.Init()
	if err != nil {
		return nil, err
	}
	return &Context{}, nil
}

func (c *Context) Name() string {
	return "CoreText"
}

func (c *Context) Destroy() {
	for _, f := range c.fonts {
		if f != 0 {
			CFRelease(f)
		}
	}
	c.fonts = nil
}

func (c *Context) AddFont(fontFile string) error {
	fontURL := CFURLCreateWithFileSystemPath(fontFile, KCFURLPOSIXPathStyle, false)
	if fontURL == 0 {
		return fmt.Errorf("failed to create URL for font file: %s", fontFile)
	}
	defer CFRelease(fontURL)

	return CTFontManagerRegisterFontsForURL(fontURL, KCTFontManagerScopeProcess)
}

func (c *Context) NewTextLayout(text string, format typography.TextFormat, width, height float32) (typography.TextLayout, error) {
	return newTextLayout(c, text, format, width, height)
}

// TextLayout implements typography.TextLayout using Core Text.
type TextLayout struct {
	text     string
	format   typography.TextFormat
	width    float32
	height   float32
	position utils.StringPosition

	attrString CFMutableAttributedStringRef
	textLength int // UTF-16 length

	colorSpace CGColorSpaceRef

	dirty      bool // whether attrString has been modified and CTFrame need rebuild
	frame      CTFrameRef
	layoutSize CGSize
	lines      []lineInfo
	painter    textPainter
	life       lifecycle.Layout
}

type lineInfo struct {
	line      CTLineRef
	origin    CGPoint
	u16Start  int
	u16Length int
	ascent    float64
	descent   float64
	leading   float64
	width     float64
}

func newTextLayout(c *Context, text string, format typography.TextFormat, width, height float32) (*TextLayout, error) {
	textStr := CFStringCreateWithString(text)
	if textStr == 0 {
		return nil, errors.New("create text string failed")
	}
	defer CFRelease(textStr)

	ctFont := createCTFont(format.Font)
	if ctFont == 0 {
		return nil, errors.New("create font failed")
	}
	defer CFRelease(ctFont)

	t := &TextLayout{
		text:     text,
		format:   format,
		width:    width,
		height:   height,
		position: utils.CalcStringPosition(text),
		dirty:    true,
	}

	t.colorSpace = CGColorSpaceCreateDeviceRGB()
	if t.colorSpace == 0 {
		return nil, errors.New("create device RGB color space failed ")
	}

	fgColor := createCGColor(t.colorSpace, toRGBAColor(format.TextColor))
	if fgColor == 0 {
		return nil, errors.New("create text color failed")
	}
	defer CGColorRelease(fgColor)

	t.attrString = CFAttributedStringCreateMutable(0, 0)
	if t.attrString == 0 {
		return nil, errors.New("create attributed string failed")
	}

	CFAttributedStringReplaceString(t.attrString, CFRangeMake(0, 0), textStr)
	t.textLength = CFAttributedStringGetLength(t.attrString)

	textRange := CFRangeMake(0, t.textLength)

	CFAttributedStringBeginEditing(t.attrString)
	CFAttributedStringSetAttribute(t.attrString, textRange, KCTFontAttributeName, ctFont)
	CFAttributedStringSetAttribute(t.attrString, textRange, KCTForegroundColorAttributeName, fgColor)
	CFAttributedStringEndEditing(t.attrString)

	t.updateParagraphStyle()

	return t, nil
}

func (t *TextLayout) Destroy() {
	if !t.life.BeginDestroy() {
		return
	}
	t.painter.Destroy()
	t.releaseFrame()
	if t.attrString != 0 {
		CFRelease(t.attrString)
		t.attrString = 0
	}
	if t.colorSpace != 0 {
		CGColorSpaceRelease(t.colorSpace)
		t.colorSpace = 0
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
	t.width, t.height = maxWidth, maxHeight
	t.dirty = true
	t.life.Changed()
}

func (t *TextLayout) SetTextAlignment(align typography.TextAlignment) {
	if t.format.TextAlign == align {
		return
	}
	t.format.TextAlign = align
	t.updateParagraphStyle()
	t.life.Changed()
}

func (t *TextLayout) SetWrapMode(wrap typography.WrapMode) {
	if t.format.WrapMode == wrap {
		return
	}
	t.format.WrapMode = wrap
	t.updateParagraphStyle()
	t.life.Changed()
}

func (t *TextLayout) SetTextFont(start, length int, font typography.FontInfo) {
	if 0 <= start && 0 < length && (len(font.Family) != 0 || font.Size != 0) {
		u16Start := t.position.ToUtf16(start)
		u16End := t.position.ToUtf16(start + length)
		if u16End < 0 {
			u16End = t.textLength
		}

		ctFont := createCTFont(font)
		if ctFont != 0 {
			defer CFRelease(ctFont)
			CFAttributedStringBeginEditing(t.attrString)
			CFAttributedStringSetAttribute(t.attrString, CFRangeMake(u16Start, u16End-u16Start), KCTFontAttributeName, ctFont)
			CFAttributedStringEndEditing(t.attrString)
			t.dirty = true
			t.life.Changed()
		}
	}
}

func (t *TextLayout) SetTextColor(start, length int, c color.Color) {
	if 0 <= start && 0 < length {
		u16Start := t.position.ToUtf16(start)
		u16End := t.position.ToUtf16(start + length)
		if u16End < 0 {
			u16End = t.textLength
		}

		fgColor := createCGColor(t.colorSpace, toRGBAColor(c))
		if fgColor != 0 {
			defer CFRelease(fgColor)
			CFAttributedStringBeginEditing(t.attrString)
			CFAttributedStringSetAttribute(t.attrString, CFRangeMake(u16Start, u16End-u16Start), KCTForegroundColorAttributeName, fgColor)
			CFAttributedStringEndEditing(t.attrString)
			t.dirty = true
			t.life.Changed()
		}
	}
}

func (t *TextLayout) SetUnderline(start, length int, underline bool) {
	if 0 <= start && 0 < length {
		u16Start := t.position.ToUtf16(start)
		u16End := t.position.ToUtf16(start + length)
		if u16End < 0 {
			u16End = t.textLength
		}

		style := KCTUnderlineStyleNone
		if underline {
			style = KCTUnderlineStyleSingle
		}
		cfNum := CFNumberCreateInt32(int32(style))
		if cfNum != 0 {
			defer CFRelease(cfNum)
			CFAttributedStringBeginEditing(t.attrString)
			CFAttributedStringSetAttribute(t.attrString, CFRangeMake(u16Start, u16End-u16Start), KCTUnderlineStyleAttributeName, cfNum)
			CFAttributedStringEndEditing(t.attrString)
			t.dirty = true
			t.life.Changed()
		}
	}
}

func (t *TextLayout) SetStrikethrough(start, length int, strike bool) {
	if 0 <= start && 0 < length && KCTStrikethroughStyleAttributeName != 0 {
		u16Start := t.position.ToUtf16(start)
		u16End := t.position.ToUtf16(start + length)
		if u16End < 0 {
			u16End = t.textLength
		}

		val := int32(0)
		if strike {
			val = int32(KCTUnderlineStyleSingle)
		}
		cfNum := CFNumberCreateInt32(val)
		if cfNum != 0 {
			defer CFRelease(cfNum)
			CFAttributedStringBeginEditing(t.attrString)
			CFAttributedStringSetAttribute(t.attrString, CFRangeMake(u16Start, u16End-u16Start), KCTStrikethroughStyleAttributeName, cfNum)
			CFAttributedStringEndEditing(t.attrString)
			t.dirty = true
			t.life.Changed()
		}
	}
}

func (t *TextLayout) MeasureSize() (width, height float32) {
	t.ensureFrame()
	_, _, width, height = t.getExtents()
	return
}

func (t *TextLayout) MeasureMetrics() (lines []typography.TextLine, clusters []typography.TextCluster) {
	t.ensureFrame()

	if len(t.lines) == 0 {
		return
	}

	height := float32(t.layoutSize.Height)
	lines = make([]typography.TextLine, 0, len(t.lines))
	clusters = make([]typography.TextCluster, 0, t.textLength)

	for lineIndex, li := range t.lines {
		baseline := float32(li.origin.Y)
		lineX := float32(li.origin.X)
		lineY := float32(li.origin.Y + li.ascent + li.leading)
		lineWidth := float32(li.width)
		lineHeight := float32(li.ascent + li.descent + li.leading)

		u8Start := t.position.ToUtf8(li.u16Start)
		u8End := t.position.ToUtf8(li.u16Start + li.u16Length)
		if u8End < 0 {
			u8End = len(t.text)
		}
		// CoreText includes the paragraph separator in a line's string range.
		// Keep it out of visible clusters, as in Pango and DirectWrite.
		u8End = u8Start + len(strings.TrimRight(t.text[u8Start:u8End], "\r\n\u2028\u2029"))

		line := typography.TextLine{
			Start:    u8Start,
			Length:   u8End - u8Start,
			X:        lineX,
			Y:        height - lineY, // convert Y
			Width:    lineWidth,
			Height:   lineHeight,
			Baseline: height - baseline, // convert Y
		}

		clustersBeg := len(clusters)
		clusterByIndex := make(map[int]int)

		// Get clusters from runs
		runs := CTLineGetGlyphRuns(li.line)
		runCount := CFArrayGetCount(runs)
		for ri := 0; ri < runCount; ri++ {
			run := CFArrayGetValueAtIndex(runs, ri)
			status := CTRunGetStatus(run)
			isRTL := (status & KCTRunStatusRightToLeft) != 0

			glyphCount := CTRunGetGlyphCount(run)
			if glyphCount == 0 {
				continue
			}

			positions := make([]CGPoint, glyphCount)
			CTRunGetPositions(run, CFRangeMake(0, 0), positions)
			advances := make([]CGSize, glyphCount)
			CTRunGetAdvances(run, CFRangeMake(0, 0), advances)

			indices := make([]CFIndex, glyphCount)
			CTRunGetStringIndices(run, CFRangeMake(0, 0), indices)

			// A string index may map to several positioned glyphs. Group by
			// that index (not glyph order), preserving native advance geometry.
			for gi := 0; gi < glyphCount; gi++ {
				u16Idx := int(indices[gi])
				u8Pos := t.position.ToUtf8(u16Idx)
				if u8Pos < u8Start || u8Pos >= u8End {
					continue
				}
				x0 := lineX + float32(positions[gi].X)
				x1 := x0 + float32(advances[gi].Width)
				left, right := min(x0, x1), max(x0, x1)
				if ci, ok := clusterByIndex[u8Pos]; ok {
					cluster := &clusters[ci]
					right = max(right, cluster.X+cluster.Width)
					cluster.X = min(left, cluster.X)
					cluster.Width = right - cluster.X
					continue
				}

				cluster := typography.TextCluster{
					Start:     u8Pos,
					X:         left,
					Y:         height - lineY, // convert Y
					Width:     right - left,
					Height:    lineHeight,
					LineIndex: lineIndex,
				}
				if isRTL {
					cluster.Direction = typography.TextRightToLeft
				}
				clusterByIndex[u8Pos] = len(clusters)
				clusters = append(clusters, cluster)
			}
		}

		line.Clusters = clusters[clustersBeg:]
		slices.SortFunc(line.Clusters, func(a, b typography.TextCluster) int {
			return a.Start - b.Start
		})
		for i := range line.Clusters {
			end := u8End
			if i+1 < len(line.Clusters) {
				end = line.Clusters[i+1].Start
			}
			line.Clusters[i].Length = end - line.Clusters[i].Start
		}
		lines = append(lines, line)
	}
	return
}

func (t *TextLayout) Rasterize(scale float32, buf []byte) (bitmap typography.TextBitmap, err error) {
	if t.life.IsDestroyed() {
		return bitmap, errors.New("coretext: rasterize destroyed text layout")
	}
	if scale <= 0 || math.IsNaN(float64(scale)) || math.IsInf(float64(scale), 0) {
		return bitmap, fmt.Errorf("coretext: invalid raster scale %v", scale)
	}
	return t.rasterize(scale, buf)
}

func (t *TextLayout) rasterize(scale float32, buf []byte) (bitmap typography.TextBitmap, err error) {
	t.ensureFrame()

	_, _, width, height := t.getExtents()
	if width <= 0 || height <= 0 {
		return
	}

	width = min(width, t.width) * scale
	height = min(height, t.height) * scale
	if width <= 0 || height <= 0 {
		return
	}

	pw := roundToPixel(width)
	ph := roundToPixel(height)

	if t.painter.bitmap.Width < pw || t.painter.bitmap.Height < ph {
		widthCapacity := max(pw, t.painter.bitmap.Width)
		heightCapacity := max(ph, t.painter.bitmap.Height)
		t.painter.Destroy()
		err = t.painter.Init(float32(widthCapacity), float32(heightCapacity))
		if err != nil {
			return
		}
	}

	err = t.painter.DrawTextLayout(t, scale)
	if err != nil {
		return
	}

	return t.painter.GetBitmap(width, height, buf), nil
}

func (t *TextLayout) updateParagraphStyle() {
	alignment := convertAlignment(t.format.TextAlign)
	lineBreak := convertLineBreakMode(t.format.WrapMode)
	paraStyle := CreateParagraphStyle(alignment, lineBreak)
	if paraStyle != 0 {
		defer CFRelease(paraStyle)
		CFAttributedStringBeginEditing(t.attrString)
		CFAttributedStringSetAttribute(t.attrString, CFRangeMake(0, t.textLength), KCTParagraphStyleAttributeName, paraStyle)
		CFAttributedStringEndEditing(t.attrString)
		t.dirty = true
	}
}

func (t *TextLayout) releaseFrame() {
	if t.frame != 0 {
		CFRelease(t.frame)
		t.frame = 0
		t.layoutSize = CGSize{}
	}
}

func (t *TextLayout) ensureFrame() {
	if !t.dirty {
		return
	}

	t.dirty = false
	t.releaseFrame()

	if t.textLength == 0 {
		return
	}

	framesetter := CTFramesetterCreateWithAttributedString(t.attrString)
	if framesetter != 0 {
		defer CFRelease(framesetter)

		textRange := CFRangeMake(0, 0)
		size := CGSize{Width: float64(t.width), Height: 20000}
		t.layoutSize, _ = CTFramesetterSuggestFrameSizeWithConstraints(framesetter, textRange, 0, size)
		if t.layoutSize.Width != 0 && t.layoutSize.Height != 0 {
			path := CGPathCreateWithRect(CGRect{Size: t.layoutSize}, nil)
			if path != 0 {
				defer CFRelease(path)

				t.frame = CTFramesetterCreateFrame(framesetter, textRange, path, 0)
				if t.frame != 0 {
					lines := CTFrameGetLines(t.frame)
					lineCount := CFArrayGetCount(lines)
					t.lines = make([]lineInfo, lineCount)
					origins := make([]CGPoint, lineCount)
					CTFrameGetLineOrigins(t.frame, CFRange{}, origins)
					for i := 0; i < lineCount; i++ {
						line := CFArrayGetValueAtIndex(lines, i)
						lineRange := CTLineGetStringRange(line)
						width, ascent, descent, leading := CTLineGetTypographicBounds(line)
						t.lines[i].line = line
						t.lines[i].origin = origins[i]
						t.lines[i].u16Start = lineRange.Location
						t.lines[i].u16Length = lineRange.Length
						t.lines[i].width = width
						t.lines[i].ascent = ascent
						t.lines[i].descent = descent
						t.lines[i].leading = leading
					}
				}
			}
		}
	}
}

func (t *TextLayout) getExtents() (x, y, width, height float32) {
	if t.frame == 0 {
		return
	}

	minX := float32(math.MaxFloat32)
	minY := float32(math.MaxFloat32)
	maxX := float32(0)
	maxY := float32(0)

	for _, line := range t.lines {
		ox := float32(line.origin.X)
		oy := float32(t.layoutSize.Height - (line.origin.Y + line.ascent + line.leading))
		lineRight := ox + float32(line.width)
		lineBottom := oy + float32(line.ascent+line.descent+line.leading)
		minX = min(minX, ox)
		minY = min(minY, oy)
		maxX = max(maxX, lineRight)
		maxY = max(maxY, lineBottom)
	}

	x = minX
	y = minY
	width = maxX - minX
	height = maxY - minY

	return
}

// ========== Text Painter ==========

type textPainter struct {
	width  float32
	height float32
	bitmap typography.TextBitmap
	cgCtx  CGContextRef
	cs     CGColorSpaceRef
}

func (p *textPainter) Init(width, height float32) (err error) {
	p.width = width
	p.height = height
	p.bitmap.Width = roundToPixel(width)
	p.bitmap.Height = roundToPixel(height)
	p.bitmap.Stride = p.bitmap.Width * 4
	p.bitmap.Pixels = make([]byte, p.bitmap.Stride*p.bitmap.Height)
	p.cs = CGColorSpaceCreateDeviceRGB()

	p.cgCtx = CGBitmapContextCreate(
		p.bitmap.Pixels,
		p.bitmap.Width,
		p.bitmap.Height,
		8,
		p.bitmap.Stride,
		p.cs,
		CGImageAlphaPremultipliedLast, // RGBA
	)
	if p.cgCtx == 0 {
		p.Destroy()
		return errors.New("failed to create bitmap context")
	}

	CGContextSetShouldAntialias(p.cgCtx, true)
	CGContextSetAllowsAntialiasing(p.cgCtx, true)
	return nil
}

func (p *textPainter) Destroy() {
	if p.cgCtx != 0 {
		CGContextRelease(p.cgCtx)
		p.cgCtx = 0
	}
	if p.cs != 0 {
		CGColorSpaceRelease(p.cs)
		p.cs = 0
	}
}

func (p *textPainter) DrawTextLayout(t *TextLayout, scale float32) error {
	CGContextSaveGState(p.cgCtx)
	defer CGContextRestoreGState(p.cgCtx)

	CGContextClearRect(p.cgCtx, CGRectMake(0, 0, float64(p.bitmap.Width), float64(p.bitmap.Height)))
	CGContextScaleCTM(p.cgCtx, float64(scale), float64(scale))
	CGContextSetTextMatrix(p.cgCtx, CGAffineTransformIdentity)
	CTFrameDraw(t.frame, p.cgCtx)
	return nil
}

func (p *textPainter) GetBitmap(width, height float32, buf []byte) (bitmap typography.TextBitmap) {
	bitmap.Width = min(roundToPixel(width), p.bitmap.Width)
	bitmap.Height = min(roundToPixel(height), p.bitmap.Height)
	bitmap.Stride = bitmap.Width * 4
	byteSize := bitmap.Stride * bitmap.Height
	if byteSize <= cap(buf) {
		bitmap.Pixels = buf[:byteSize]
	} else {
		bitmap.Pixels = make([]byte, byteSize)
	}
	// CoreText draws from the CGContext's bottom-left origin. When the
	// retained bitmap is taller than this rasterization, the active rows
	// are at the bottom of its top-to-bottom pixel buffer.
	for y := 0; y < bitmap.Height; y++ {
		dstOffset := bitmap.PixOffset(0, y)
		srcOffset := p.bitmap.PixOffset(0, p.bitmap.Height-bitmap.Height+y)
		copy(bitmap.Pixels[dstOffset:dstOffset+bitmap.Stride], p.bitmap.Pixels[srcOffset:])
	}
	return
}

func createCTFont(font typography.FontInfo) CTFontRef {
	size := float64(font.Size)
	if size == 0 {
		size = 12
	}

	if font.Family == "" {
		return CTFontCreateWithName(".AppleSystemUIFont", size, nil)
	}

	if font.Weight != 0 || font.Width != 0 {
		keys := make([]CFTypeRef, 0, 3)
		values := make([]CFTypeRef, 0, 3)

		cfFamily := CFStringCreateWithString(font.Family)
		keys = append(keys, KCTFontFamilyNameAttribute)
		values = append(values, cfFamily)

		cfSize := CFNumberCreateFloat64(size)
		keys = append(keys, KCTFontSizeAttribute)
		values = append(values, cfSize)

		if font.Weight != 0 {
			// Build traits dictionary with weight
			traitKeys := []CFTypeRef{KCTFontWeightTrait}
			weightVal := CFNumberCreateFloat64(float64(font.Weight))
			traitValues := []CFTypeRef{weightVal}
			traitsDict := CFDictionaryCreate(traitKeys, traitValues)
			CFRelease(weightVal)

			keys = append(keys, KCTFontTraitsAttribute)
			values = append(values, traitsDict)
			defer CFRelease(traitsDict)
		}

		attrs := CFDictionaryCreate(keys, values)
		CFRelease(cfFamily)
		CFRelease(cfSize)

		desc := CTFontDescriptorCreateWithAttributes(attrs)
		CFRelease(attrs)
		if desc != 0 {
			ctFont := CTFontCreateWithFontDescriptor(desc, size, nil)
			CFRelease(desc)
			return ctFont
		}
	}

	return CTFontCreateWithName(font.Family, size, nil)
}

func convertAlignment(align typography.TextAlignment) CTTextAlignment {
	switch align {
	case typography.TextAlignEnd:
		return KCTTextAlignmentRight
	case typography.TextAlignCenter:
		return KCTTextAlignmentCenter
	case typography.TextAlignFill:
		return KCTTextAlignmentJustified
	default:
		return KCTTextAlignmentLeft
	}
}

func convertLineBreakMode(wrap typography.WrapMode) CTLineBreakMode {
	switch wrap {
	case typography.WrapNone:
		return KCTLineBreakByClipping
	case typography.WrapChar:
		return KCTLineBreakByCharWrapping
	default:
		return KCTLineBreakByWordWrapping
	}
}

func toRGBAColor(c color.Color) color.RGBA {
	if c == nil {
		c = typography.DefaultTextColor()
	}
	return color.RGBAModel.Convert(c).(color.RGBA)
}

func createCGColor(colorSpace CGColorSpaceRef, rgba color.RGBA) CFTypeRef {
	// CGColor components are straight; the bitmap target remains premultiplied.
	c := color.NRGBA64Model.Convert(rgba).(color.NRGBA64)
	components := []CGFloat{
		float64(c.R) / 65535.0,
		float64(c.G) / 65535.0,
		float64(c.B) / 65535.0,
		float64(c.A) / 65535.0,
	}
	return CFTypeRef(CGColorCreate(colorSpace, components))
}

func roundToPixel(v float32) int {
	return int(v + 0.99)
}
