package textedit

import (
	"fmt"
	"image/color"
	"math"
	"sort"
	"strings"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform/typography"
	"golang.org/x/text/unicode/bidi"
)

// ShapedText owns one immutable native shaping source and an ordinary-value
// break index. It is sequential and thread-affine, like its native layout.
// Retain/Release permit width projections of ONE view to share the source.
// Finish painting before releasing the retained native reference. Fragment and
// Geometry snapshots remain usable independently of the source lifetime.
type ShapedText struct {
	layout typography.GlyphLayout
	*shapedFacts
	refs int
}

// No native handle or owner is reachable from these immutable facts. Lazy
// Fragment snapshots retain only this data, even after the source is released.
type shapedFacts struct {
	metrics      typography.GlyphMetrics
	measurements WrapMeasurements
	rank         []int
	reset        []bool
}

// NewShapedText shapes the complete paragraph once. A failure leaves no native
// resource owned by the caller. It does not create per-width caret geometry.
func NewShapedText(ctx typography.GlyphContext, text string, format typography.TextFormat) (*ShapedText, error) {
	l, err := ctx.NewGlyphLayout(text, format)
	if err != nil {
		return nil, err
	}
	m, err := l.GlyphMetrics()
	if err != nil {
		l.Destroy()
		return nil, err
	}
	if len(m.VisualOrder) != len(m.Clusters) || m.Line.Height <= 0 || math.IsNaN(float64(m.Line.Height)) || math.IsInf(float64(m.Line.Height), 0) || math.IsNaN(float64(m.Line.Baseline)) || math.IsInf(float64(m.Line.Baseline), 0) {
		l.Destroy()
		return nil, fmt.Errorf("textedit: invalid glyph metrics")
	}
	seen := make([]bool, len(m.Clusters))
	for _, i := range m.VisualOrder {
		if i < 0 || i >= len(seen) || seen[i] {
			l.Destroy()
			return nil, fmt.Errorf("textedit: invalid glyph order")
		}
		seen[i] = true
	}
	for i, c := range m.Clusters {
		if i > 0 && c.Start <= m.Clusters[i-1].Start {
			l.Destroy()
			return nil, fmt.Errorf("textedit: unordered glyph clusters")
		}
	}
	w, err := NewWrapMeasurements(text, m.Clusters)
	if err != nil {
		l.Destroy()
		return nil, err
	}
	s := &ShapedText{layout: l, shapedFacts: &shapedFacts{metrics: m, measurements: w, rank: make([]int, len(m.Clusters)), reset: make([]bool, len(m.Clusters))}, refs: 1}
	for rank, index := range m.VisualOrder {
		s.rank[index] = rank
	}
	for i, c := range m.Clusters {
		reset := true
		for _, r := range text[c.Start : c.Start+c.Length] {
			p, _ := bidi.LookupRune(r)
			switch p.Class() {
			case bidi.WS, bidi.BN, bidi.LRE, bidi.RLE, bidi.LRO, bidi.RLO, bidi.PDF, bidi.LRI, bidi.RLI, bidi.FSI, bidi.PDI:
			default:
				reset = false
			}
		}
		s.reset[i] = reset
	}
	return s, nil
}

func (s *ShapedText) Retain() *ShapedText {
	if s.refs <= 0 {
		panic("textedit: retain released shaping")
	}
	s.refs++
	return s
}
func (s *ShapedText) Release() {
	if s.refs <= 0 {
		panic("textedit: release released shaping")
	}
	s.refs--
	if s.refs == 0 {
		s.layout.Destroy()
	}
}

// Layout is read-only while retained by projections. Callers must not mutate
// it or destroy it directly; Release owns destruction.
func (s *ShapedText) Layout() typography.GlyphLayout { return s.layout }

// WrappedText is a width-specific immutable projection. Only display-line
// ranges are indexed eagerly; detailed cluster positions are produced on demand.
type WrappedText struct {
	shape *ShapedText
	*wrapProjection
}

// This is the lazy Fragment provider. In particular it cannot retain a native
// layout through WrappedText or ShapedText.
type wrapProjection struct {
	facts    *shapedFacts
	fragment Fragment
	tab      float32
}

// Wrap computes breaks from retained advances. It preserves complete shaping
// clusters and graphemes, never independently shapes a substring, and inserts
// no discretionary hyphen. A zero tab interval uses the retained native
// default. height is the minimum uniform display-line height.
func (s *ShapedText) Wrap(width, tab, height float32, word bool) *WrappedText {
	if height <= 0 || math.IsNaN(float64(height)) || math.IsInf(float64(height), 0) {
		panic("textedit: invalid line height")
	}
	if tab == 0 {
		tab = s.metrics.TabWidth
	}
	w := &WrappedText{shape: s, wrapProjection: &wrapProjection{facts: s.shapedFacts, tab: tab}}
	h := max(height, s.metrics.Line.Height)
	d := &fragmentMetrics{text: Range{End: len(s.measurements.Text())}, orderedOffsets: true}
	it := s.measurements.Iterator()
	for row := 0; ; row++ {
		candidate, ok := it.Next(width, tab, word)
		if !ok {
			break
		}
		x := float32(0)
		if s.metrics.BaseDirection == typography.TextRightToLeft {
			x = max(0, width-candidate.Width)
		}
		d.lines = append(d.lines, typography.TextLine{Start: candidate.Range.Start, Length: candidate.Range.End - candidate.Range.Start, Y: float32(row) * h, X: x, Width: candidate.Width, Height: h, Baseline: float32(row)*h + s.metrics.Line.Baseline})
		d.firstStop = append(d.firstStop, candidate.Range.Start)
		d.lastStop = append(d.lastStop, candidate.Range.End)
		d.size.Width = max(d.size.Width, d.lines[row].X+candidate.Width)
	}
	d.size.Height = float32(len(d.lines)) * h
	w.fragment = Fragment{data: d, end: len(d.lines)}
	d.loadClusters = w.wrapProjection.lineClusters
	return w
}

func (w *WrappedText) Fragment() Fragment { return w.fragment }

func (w *wrapProjection) lineClusters(row int) []typography.TextCluster {
	s := w.facts
	line := w.fragment.data.lines[row]
	source := s.metrics.Clusters
	first := sort.Search(len(source), func(i int) bool { return source[i].Start >= line.Start })
	end := sort.Search(len(source), func(i int) bool { return source[i].Start >= line.Start+line.Length })
	indices := make([]int, end-first)
	cs := append([]typography.TextCluster(nil), source[first:end]...)
	// Advances for tabs depend on this display line, not the source line.
	var advance float64
	for i := range cs {
		indices[i] = first + i
		if w.tab > 0 && s.measurements.Text()[cs[i].Start:cs[i].Start+cs[i].Length] == "\t" {
			cs[i].Width = float32((math.Floor(advance/float64(w.tab))+1)*float64(w.tab) - advance)
		}
		advance += float64(cs[i].Width)
	}
	tail := end
	for tail > first && s.reset[tail-1] {
		tail--
	}
	// UAX #9 L1 resets trailing whitespace/formatting to paragraph level.
	// Restrict the native L2/L3 permutation for the remaining glyphs, preserving
	// native cluster-internal order, mirroring, joining and ligatures.
	sort.Slice(indices, func(i, j int) bool {
		a, b := indices[i], indices[j]
		if a >= tail || b >= tail {
			if a >= tail && b >= tail {
				if s.metrics.BaseDirection == typography.TextRightToLeft {
					return a > b
				}
				return a < b
			}
			if s.metrics.BaseDirection == typography.TextRightToLeft {
				return a >= tail
			}
			return b >= tail
		}
		return s.rank[a] < s.rank[b]
	})
	x := float64(line.X)
	for _, index := range indices {
		c := &cs[index-first]
		c.X = float32(x)
		c.Y = line.Y
		c.Height = line.Height
		c.LineIndex = row
		if index >= tail {
			c.Direction = s.metrics.BaseDirection
		}
		x += float64(c.Width)
	}
	return cs
}

// GlyphPainter is the drawing capability needed by a wrapped projection.
type GlyphPainter interface {
	DrawTextGlyphs(geometry.Point, typography.GlyphLayout, int, int, color.Color)
}

// GlyphColor is an ordered, non-overlapping foreground override in source bytes.
// A shaping cluster takes the color at its leading byte; glyphs are never split
// or reshaped to apply a color inside a ligature.
type GlyphColor struct {
	Range Range
	Color color.Color
}

// Paint draws only lines intersecting clip. The caller owns clipping. Native
// bitmap caches belong to the painter, with source destruction notifications.
func (w *WrappedText) Paint(p GlyphPainter, origin geometry.Point, clip geometry.Rectangle, foreground color.Color, colors []GlyphColor) {
	d := w.fragment.data
	first := sort.Search(len(d.lines), func(i int) bool {
		return origin.Y+d.lines[i].Y+d.lines[i].Height+w.shape.metrics.InkPadding.Bottom > clip.Y
	})
	for row := first; row < len(d.lines) && origin.Y+d.lines[row].Y-w.shape.metrics.InkPadding.Top < clip.Y+clip.Height; row++ {
		cs := w.lineClusters(row)
		sort.Slice(cs, func(i, j int) bool {
			if cs[i].X != cs[j].X {
				return cs[i].X < cs[j].X
			}
			return w.shape.rank[sort.Search(len(w.shape.metrics.Clusters), func(k int) bool { return w.shape.metrics.Clusters[k].Start >= cs[i].Start })] < w.shape.rank[sort.Search(len(w.shape.metrics.Clusters), func(k int) bool { return w.shape.metrics.Clusters[k].Start >= cs[j].Start })]
		})
		at := 0
		for at < len(cs) {
			c := cs[at]
			text := w.shape.measurements.Text()
			if c.Width == 0 || text[c.Start:c.Start+c.Length] == "\t" {
				at++
				continue
			}
			col := glyphColorAt(colors, c.Start, foreground)
			start, end, next := c.Start, c.Start+c.Length, at+1
			for next < len(cs) {
				n := cs[next]
				if n.Width == 0 || strings.ContainsRune(text[n.Start:n.Start+n.Length], '\t') || n.Direction != c.Direction || !sameGlyphColor(col, glyphColorAt(colors, n.Start, foreground)) || n.Start != end && n.Start+n.Length != start {
					break
				}
				start, end = min(start, n.Start), max(end, n.Start+n.Length)
				next++
			}
			p.DrawTextGlyphs(geometry.Point{X: origin.X + c.X, Y: origin.Y + d.lines[row].Y}, w.shape.layout, start, end-start, col)
			at = next
		}
	}
}

func glyphColorAt(colors []GlyphColor, offset int, base color.Color) color.Color {
	i := sort.Search(len(colors), func(i int) bool { return colors[i].Range.End > offset })
	if i < len(colors) && colors[i].Range.Start <= offset {
		return colors[i].Color
	}
	return base
}
func sameGlyphColor(a, b color.Color) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return color.NRGBA64Model.Convert(a) == color.NRGBA64Model.Convert(b)
}

// Baseline is the first display line's baseline in local logical units.
func (w *WrappedText) Baseline() float32 { return w.shape.metrics.Line.Baseline }
