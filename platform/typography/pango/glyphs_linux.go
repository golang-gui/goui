package pango

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/goexlib/cgo"
	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform/linux/libs/pango"
	"github.com/golang-gui/goui/platform/linux/libs/pango_cairo"
	"github.com/golang-gui/goui/platform/typography"
	"github.com/golang-gui/goui/platform/typography/utils"
)

var _ typography.GlyphContext = (*Context)(nil)

func (c *Context) NewGlyphLayout(text string, format typography.TextFormat) (typography.GlyphLayout, error) {
	if !utf8.ValidString(text) || strings.ContainsAny(text, "\r\n\v\f\u0085\u2028\u2029") {
		return nil, fmt.Errorf("pango: expected one UTF-8 paragraph")
	}
	format.WrapMode, format.TextAlign = typography.WrapNone, typography.TextAlignBegin
	l, err := c.NewTextLayout(text, format, 0, 0)
	if err != nil {
		return nil, err
	}
	return l.(*TextLayout), nil
}

type glyphCluster struct {
	start, run, first, end, visual int
	advance                        int64
}

type glyphSnapshot struct {
	serial   uint32
	metrics  typography.GlyphMetrics
	runs     []pango.GlyphItem // borrowed until native serial changes
	clusters []glyphCluster    // logical order, same indices as metrics.Clusters
}

func (t *TextLayout) GlyphMetrics() (typography.GlyphMetrics, error) {
	if err := t.prepareGlyphs(); err != nil {
		return typography.GlyphMetrics{}, err
	}
	return t.glyphs.metrics, nil
}

func (t *TextLayout) prepareGlyphs() error {
	if t.life.IsDestroyed() {
		return fmt.Errorf("pango: destroyed glyph layout")
	}
	if t.format.WrapMode != typography.WrapNone || t.layout.GetLineCount() != 1 {
		return fmt.Errorf("pango: glyph metrics require an unwrapped paragraph")
	}
	serial := t.layout.GetSerial()
	if t.glyphs != nil && t.glyphs.serial == serial {
		return nil
	}
	it := t.layout.GetIter()
	defer it.Free()
	native := it.GetLineReadonly()
	if native == nil {
		return fmt.Errorf("pango: missing glyph line")
	}
	var rect pango.Rectangle
	it.GetLineExtents(nil, &rect)
	s := &glyphSnapshot{serial: serial, runs: native.GetRuns()}
	if strings.ContainsRune(t.text, '\t') {
		tab := float32(math.Round(float64(t.format.TabWidth*pango.Scale))) / pango.Scale
		if t.format.TabWidth > 0 {
			tab = max(tab, 1.0/pango.Scale)
		} else {
			// Ask the same native context for the default interval at the first
			// effective font. Measuring one tab is bounded and independent of the
			// source paragraph's original tab positions; no magic column estimate.
			desc, err := createFontDescription(t.firstFont)
			if err != nil {
				return err
			}
			probe := pango.LayoutNew(t.context)
			if probe.IsNull() {
				desc.Free()
				return fmt.Errorf("pango: create tab metrics layout")
			}
			probe.SetText("\t")
			probe.SetFontDescription(desc)
			desc.Free()
			pw, _ := probe.GetSize()
			probe.Unref()
			tab = float32(pw) / pango.Scale
		}
		if tab <= 0 {
			return fmt.Errorf("pango: invalid default tab interval")
		}
		s.metrics.TabWidth = tab
	}
	m := &s.metrics
	m.Line = typography.TextLine{Length: len(t.text), Y: float32(rect.Y) / pango.Scale, Height: float32(rect.Height) / pango.Scale, Baseline: float32(it.GetBaseline()) / pango.Scale}
	if native.ResolvedDirection() == pango.DirectionRTL {
		m.BaseDirection = typography.TextRightToLeft
	}
	baseline := int64(it.GetBaseline())
	var total int64
	type glyphKey struct {
		font  uintptr
		glyph uint32
	}
	inkCache := make(map[glyphKey]pango.Rectangle)
	var left, top, right, bottom int64
	for r, run := range s.runs {
		if run.Item == nil || run.Glyphs == nil {
			return fmt.Errorf("pango: missing glyph run")
		}
		infos, logs := run.Glyphs.GlyphInfos(), run.Glyphs.LogClusters()
		for i := 0; i < len(infos); {
			j, advance := i, int64(0)
			for j < len(infos) && logs[j] == logs[i] {
				g := infos[j]
				advance += int64(g.Geometry.Width)
				key := glyphKey{run.Item.Analysis.Font, g.Glyph}
				ink, ok := inkCache[key]
				if !ok {
					ink, _ = pango.Font(key.font).GlyphExtents(key.glyph)
					inkCache[key] = ink
				}
				// Offsets are bounded in both directions, conservatively. A
				// zero-advance mark can extend beyond either range edge.
				dx, dy := int64(g.Geometry.XOffset), int64(g.Geometry.YOffset)
				if dx < 0 {
					dx = -dx
				}
				if dy < 0 {
					dy = -dy
				}
				rise := int64(run.YOffset)
				if rise < 0 {
					rise = -rise
				}
				dy += rise
				left = max(left, -int64(ink.X)+dx)
				right = max(right, int64(ink.X)+int64(ink.Width)+dx-int64(g.Geometry.Width))
				top = max(top, -int64(ink.Y)+dy-baseline)
				bottom = max(bottom, int64(ink.Y)+int64(ink.Height)+dy+baseline-int64(rect.Y)-int64(rect.Height))
				j++
			}
			if advance < 0 {
				return fmt.Errorf("pango: negative cluster advance")
			}
			s.clusters = append(s.clusters, glyphCluster{start: int(run.Item.Offset + logs[i]), run: r, first: i, end: j, advance: advance, visual: len(s.clusters)})
			total += advance
			i = j
		}
	}
	sort.Slice(s.clusters, func(i, j int) bool { return s.clusters[i].start < s.clusters[j].start })
	m.Clusters = make([]typography.TextCluster, len(s.clusters))
	m.VisualOrder = make([]int, len(s.clusters))
	if len(t.text) > 0 && (len(s.clusters) == 0 || s.clusters[0].start != 0) {
		return fmt.Errorf("pango: incomplete shaping coverage")
	}
	for i, c := range s.clusters {
		end := len(t.text)
		if i+1 < len(s.clusters) {
			end = s.clusters[i+1].start
		}
		if c.start >= end || end > len(t.text) {
			return fmt.Errorf("pango: invalid shaping cluster")
		}
		m.Clusters[i] = typography.TextCluster{Start: c.start, Length: end - c.start, Width: float32(c.advance) / pango.Scale, Y: m.Line.Y, Height: m.Line.Height, Direction: typography.TextDirection(s.runs[c.run].Item.Analysis.Level & 1)}
		m.VisualOrder[c.visual] = i
	}
	// Accumulate in int64, not Pango's whole-line int32 coordinate space.
	var x int64
	for _, i := range m.VisualOrder {
		m.Clusters[i].X = float32(x) / pango.Scale
		x += s.clusters[i].advance
	}
	m.Line.Width = float32(total) / pango.Scale
	m.InkPadding.Left = float32(left) / pango.Scale
	m.InkPadding.Top = float32(top) / pango.Scale
	m.InkPadding.Right = float32(right) / pango.Scale
	m.InkPadding.Bottom = float32(bottom) / pango.Scale
	t.glyphs = s
	return nil
}

func (t *TextLayout) RasterizeGlyphs(start, length int, foreground color.Color, offset geometry.Point, scale float32, region image.Rectangle, buf []byte) (typography.TextBitmap, error) {
	if math.IsNaN(float64(offset.X)) || math.IsNaN(float64(offset.Y)) || math.IsInf(float64(offset.X), 0) || math.IsInf(float64(offset.Y), 0) || scale <= 0 || math.IsNaN(float64(scale)) || math.IsInf(float64(scale), 0) {
		return typography.TextBitmap{}, fmt.Errorf("pango: invalid glyph raster scale")
	}
	if err := t.prepareGlyphs(); err != nil {
		return typography.TextBitmap{}, err
	}
	s := t.glyphs
	end := start + length
	if start < 0 || length < 0 || start > len(t.text)-length {
		return typography.TextBitmap{}, fmt.Errorf("pango: invalid glyph range")
	}
	first := sort.Search(len(s.clusters), func(i int) bool { return s.clusters[i].start >= start })
	last := sort.Search(len(s.clusters), func(i int) bool { return s.clusters[i].start >= end })
	boundary := func(at, index int) bool {
		return at == len(t.text) || index < len(s.clusters) && s.clusters[index].start == at
	}
	if !boundary(start, first) || !boundary(end, last) {
		return typography.TextBitmap{}, fmt.Errorf("pango: range splits a shaping cluster")
	}
	w, h, err := utils.RasterDimensions(region)
	if err != nil || w == 0 || h == 0 {
		return typography.TextBitmap{}, err
	}
	if t.painter.bitmap.Width < w || t.painter.bitmap.Height < h {
		var next textPainter
		if err := next.Init(w, h); err != nil {
			return typography.TextBitmap{}, err
		}
		t.painter.Destroy()
		t.painter = next
	}
	p := &t.painter
	if p.scratchGlyphs == nil {
		p.scratchGlyphs = pango.GlyphStringNew()
	}
	if p.scratchGlyphs == nil {
		return typography.TextBitmap{}, fmt.Errorf("pango: allocate glyph string")
	}
	cgo.Memset(cgo.CSlice(p.bitmap.Pixels), 0, cgo.Sizet(len(p.bitmap.Pixels)))
	p.context.Save()
	defer p.context.Restore()
	p.context.Translate(-float64(region.Min.X), -float64(region.Min.Y))
	p.context.Translate(float64(offset.X), float64(offset.Y))
	p.context.Scale(float64(scale), float64(scale))
	// Locate only runs intersecting the logical range, then restore their native
	// visual order. A short range never scans all glyphs in a huge paragraph.
	type portion struct{ run, first, end int }
	parts := make([]portion, 0, 4)
	for _, c := range s.clusters[first:last] {
		found := -1
		for i := range parts {
			if parts[i].run == c.run {
				found = i
				break
			}
		}
		if found < 0 {
			parts = append(parts, portion{c.run, c.first, c.end})
		} else {
			parts[found].first = min(parts[found].first, c.first)
			parts[found].end = max(parts[found].end, c.end)
		}
	}
	sort.Slice(parts, func(i, j int) bool { return parts[i].run < parts[j].run })
	var x int64
	for _, part := range parts {
		run := s.runs[part.run]
		infos := run.Glyphs.GlyphInfos()[part.first:part.end]
		logs := run.Glyphs.LogClusters()[part.first:part.end]
		p.scratchGlyphs.SetGlyphs(infos, logs)
		r, g, b, a := toColor(t.format.TextColor)
		if foreground != nil {
			r, g, b, a = toColor(foreground)
		} else {
			for attrs := run.Item.Analysis.ExtraAttributes(); attrs != nil; attrs = attrs.Next {
				switch attrs.Data.Type() {
				case pango.AttrForeground:
					c := attrs.Data.ColorValue()
					r, g, b = float64(c.Red)/65535, float64(c.Green)/65535, float64(c.Blue)/65535
				case pango.AttrForegroundAlpha:
					a = float64(attrs.Data.IntValue()) / 65535
				}
			}
		}
		p.context.SetSourceRGBA(r, g, b, a)
		p.context.Save()
		p.context.Translate(float64(x+int64(run.StartXOffset))/pango.Scale, float64(s.metrics.Line.Baseline)-float64(run.YOffset)/pango.Scale)
		p.context.NewPath()
		pango_cairo.ShowGlyphString(p.context, pango.Font(run.Item.Analysis.Font), p.scratchGlyphs)
		p.context.Restore()
		for _, g := range infos {
			x += int64(g.Geometry.Width)
		}
		x += int64(run.StartXOffset) + int64(run.EndXOffset)
	}
	if status := p.context.Status(); status != 0 {
		return typography.TextBitmap{}, fmt.Errorf("pango: draw glyphs: %v", status)
	}
	return p.GetBitmap(w, h, buf), nil
}
