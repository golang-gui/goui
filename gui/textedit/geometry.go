package textedit

import (
	"math"
	"sort"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform/typography"
)

// Position identifies a UTF-8 byte boundary and its visual affinity.
// A byte boundary can have two visual positions, at a soft wrap or a bidi
// transition. Upstream chooses the trailing edge of the preceding cluster;
// downstream chooses the leading edge of the following cluster. Neither the
// affinity nor native layout coordinates belong in the shared Model.
type Position struct {
	Offset   int
	Upstream bool
}

type textStop struct {
	Position
	x         float32
	precision float32
}

type textVisualLine struct {
	metrics  typography.TextLine
	clusters []typography.TextCluster
	stops    []textStop // visual order, left to right
	first    int        // smallest logical stop offset
	last     int        // largest logical stop offset
}

// Geometry indexes supplied paragraph metrics for hit testing, navigation and
// selection. Construct it with NewGeometry or Fragment.Geometry, not its zero
// value. Fragment geometry caches detailed pages on demand. Geometry owns no
// native layout; rebuild it whenever the paragraph's text or metrics change.
type Geometry struct {
	pages      *fragmentGeometry
	lines      []textVisualLine
	boundaries []int // logical order, independent of visual order
	// Ordinary native lines partition the paragraph in logical order. Keep
	// the general query behavior for overlapping or unordered supplied metrics.
	orderedOffsets bool
	orderedY       bool
}

// LineCount returns the number of visual lines.
func (g *Geometry) LineCount() int {
	if g.pages != nil {
		return g.pages.fragment.LineCount()
	}
	return len(g.lines)
}

// LineMetrics returns a visual line's metrics. index must be in [0, LineCount()).
// Returned slice fields, if present, must be treated as read-only.
func (g *Geometry) LineMetrics(index int) typography.TextLine {
	if g.pages != nil {
		return g.pages.lineMetrics(index)
	}
	return g.lines[index].metrics
}

// LineIndex finds the visual line at pos, honoring its boundary affinity.
func (g *Geometry) LineIndex(pos Position) int {
	if g.pages != nil {
		line, _, _ := g.pages.locate(pos)
		return line
	}
	line, _ := g.locate(pos)
	return line
}

// LineAtY finds the visual line at paragraph-local y, clamping above and below
// the paragraph. A gap belongs to the following line, as in Hit.
func (g *Geometry) LineAtY(y float32) int {
	if g.pages != nil {
		return g.pages.lineAtY(y)
	}
	if g.orderedY {
		return min(len(g.lines)-1, sort.Search(len(g.lines), func(i int) bool {
			line := g.lines[i].metrics
			return !(y >= line.Y+line.Height)
		}))
	}
	i := 0
	for i+1 < len(g.lines) && y >= g.lines[i].metrics.Y+g.lines[i].metrics.Height {
		i++
	}
	return i
}

// LineEdge returns the leftmost or rightmost visual stop of a line.
// index must be in [0, LineCount()).
func (g *Geometry) LineEdge(index int, right bool) Position {
	if g.pages != nil {
		return g.pages.lineEdge(index, right)
	}
	stops := g.lines[index].stops
	if right {
		return stops[len(stops)-1].Position
	}
	return stops[0].Position
}

// NewGeometry builds an index from paragraph-local DIP metrics. length is the
// UTF-8 byte length. Empty metrics produce one line of at least emptyHeight.
// Input metrics must remain unchanged for the lifetime of the index.
func NewGeometry(lines []typography.TextLine, clusters []typography.TextCluster, length int, emptyHeight float32) Geometry {
	g := Geometry{boundaries: []int{0, length}, orderedOffsets: true, orderedY: true}
	if len(lines) == 0 {
		lines = []typography.TextLine{{Height: max(1, emptyHeight)}}
	}
	g.lines = make([]textVisualLine, len(lines))
	for i, line := range lines {
		if line.Height <= 0 {
			line.Height = max(1, emptyHeight)
		}
		g.lines[i].metrics = line
	}
	if len(clusters) == 0 {
		for i, line := range lines {
			for _, c := range line.Clusters {
				c.LineIndex = i
				clusters = append(clusters, c)
			}
		}
	}
	for _, c := range clusters {
		if c.LineIndex < 0 || c.LineIndex >= len(lines) || c.Length <= 0 || c.Start < 0 || c.Start+c.Length > length {
			continue
		}
		line := &g.lines[c.LineIndex]
		line.clusters = append(line.clusters, c)
		leading, trailing := c.X, c.X+c.Width
		precision := textStopPrecision(c.X, c.Width)
		if c.Direction == typography.TextRightToLeft {
			leading, trailing = trailing, leading
		}
		line.stops = append(line.stops,
			textStop{Position{c.Start, false}, leading, precision},
			textStop{Position{c.Start + c.Length, true}, trailing, precision})
		g.boundaries = append(g.boundaries, c.Start, c.Start+c.Length)
	}
	for i := range g.lines {
		line := &g.lines[i]
		if len(line.stops) == 0 {
			line.stops = []textStop{{Position{min(length, line.metrics.Start), false}, line.metrics.X, 0}}
		}
		sort.SliceStable(line.stops, func(i, j int) bool {
			a, b := line.stops[i], line.stops[j]
			if a.x != b.x {
				return a.x < b.x
			}
			if a.Offset != b.Offset {
				return a.Offset < b.Offset
			}
			return !a.Upstream && b.Upstream
		})
		// Coincident edges at an ordinary boundary are one visual stop. Prefer
		// downstream; retain different byte boundaries at bidi transitions.
		out := line.stops[:0]
		for _, stop := range line.stops {
			if len(out) > 0 {
				last := &out[len(out)-1]
				if last.Offset == stop.Offset && math.Abs(float64(last.x)-float64(stop.x)) <= float64(last.precision)+float64(stop.precision) {
					// Native origins and advances are rounded independently.
					// At one byte boundary, their addition can differ by a
					// representable unit. Prefer the downstream edge as for
					// exact coincidences; do not add a visual key stop for it.
					if !stop.Upstream {
						*last = stop
					}
					continue
				}
			}
			out = append(out, stop)
		}
		line.stops = out
		line.first, line.last = line.stops[0].Offset, line.stops[0].Offset
		for _, stop := range line.stops[1:] {
			line.first = min(line.first, stop.Offset)
			line.last = max(line.last, stop.Offset)
		}
		if i > 0 {
			prev := &g.lines[i-1]
			// Strict progress at both ends permits checking just neighboring
			// lines, including both affinities at a shared soft-wrap boundary.
			g.orderedOffsets = g.orderedOffsets && prev.last <= line.first &&
				prev.first < line.first && prev.last < line.last
			g.orderedY = g.orderedY && prev.metrics.Y <= line.metrics.Y &&
				prev.metrics.Y+prev.metrics.Height <= line.metrics.Y+line.metrics.Height
		}
	}
	sort.Ints(g.boundaries)
	out := g.boundaries[:0]
	for _, boundary := range g.boundaries {
		if len(out) == 0 || out[len(out)-1] != boundary {
			out = append(out, boundary)
		}
	}
	g.boundaries = out
	return g
}

// Bound the rounding of the native origin, width and computed endpoint in
// float32 coordinates. This scales with their actual representation, not a
// fixed fraction of a pixel; cancellation across zero retains the operand
// precision. Only two stops at the same byte boundary are compared with it.
func textStopPrecision(x, width float32) float32 {
	magnitude := float32(math.Max(math.Abs(float64(x)), math.Max(math.Abs(float64(width)), math.Abs(float64(x+width)))))
	return 2 * (math.Nextafter32(magnitude, float32(math.Inf(1))) - magnitude)
}

// Snap clamps offset to a Cluster boundary, toward forward when inside one.
func (g *Geometry) Snap(offset int, forward bool) int {
	if g.pages != nil {
		return g.pages.boundary(offset, forward, false)
	}
	i := sort.SearchInts(g.boundaries, offset)
	if i == len(g.boundaries) {
		return g.boundaries[len(g.boundaries)-1]
	}
	if i > 0 && !forward && g.boundaries[i] != offset {
		i--
	}
	return g.boundaries[i]
}

// Adjacent finds the preceding or following logical Cluster boundary.
func (g *Geometry) Adjacent(offset int, forward bool) int {
	if g.pages != nil {
		return g.pages.boundary(offset, forward, true)
	}
	if forward {
		i := sort.Search(len(g.boundaries), func(i int) bool { return g.boundaries[i] > offset })
		return g.boundaries[min(i, len(g.boundaries)-1)]
	}
	i := sort.SearchInts(g.boundaries, offset)
	return g.boundaries[max(0, i-1)]
}

func (g *Geometry) locate(pos Position) (int, int) {
	first, end := 0, len(g.lines)
	if g.orderedOffsets {
		i := sort.Search(len(g.lines), func(i int) bool { return g.lines[i].last >= pos.Offset })
		// Include the neighbors for missing affinities, positions inside a
		// cluster, gaps, and equidistant stops. Iterate in the original visual
		// order so ties retain exactly the same answer as the general scan.
		first, end = max(0, i-1), min(len(g.lines), i+2)
	}
	lineIndex, stopIndex, best := 0, 0, int(^uint(0)>>1)
	for i := first; i < end; i++ {
		line := &g.lines[i]
		for j, stop := range line.stops {
			if stop.Position == pos {
				return i, j
			}
			distance := stop.Offset - pos.Offset
			if distance < 0 {
				distance = -distance
			}
			if distance < best {
				lineIndex, stopIndex, best = i, j, distance
			}
		}
	}
	return lineIndex, stopIndex
}

// Caret returns a 1 DIP wide caret rectangle at the nearest visual stop.
func (g *Geometry) Caret(pos Position) geometry.Rectangle {
	if g.pages != nil {
		return g.pages.caret(pos)
	}
	i, j := g.locate(pos)
	line := &g.lines[i]
	return geometry.Rect(line.stops[j].x, line.metrics.Y, 1, line.metrics.Height)
}

// Hit finds the nearest caret position in paragraph-local coordinates.
func (g *Geometry) Hit(point geometry.Point) Position {
	return g.HitLine(g.LineAtY(point.Y), point.X)
}

// HitLine finds a caret position on a valid visual line at local x.
func (g *Geometry) HitLine(index int, x float32) Position {
	if g.pages != nil {
		return g.pages.hitLine(index, x)
	}
	line := &g.lines[index]
	// First choose the half of the actual cluster, not an inferred logical
	// advance. This also disambiguates coincident bidi-run edges.
	for _, c := range line.clusters {
		if c.Width > 0 && x >= c.X && x < c.X+c.Width {
			trailing := x >= c.X+c.Width/2
			if c.Direction == typography.TextRightToLeft {
				trailing = !trailing
			}
			if trailing {
				return Position{c.Start + c.Length, true}
			}
			return Position{c.Start, false}
		}
	}
	best := line.stops[0]
	for _, stop := range line.stops[1:] {
		if math.Abs(float64(stop.x-x)) < math.Abs(float64(best.x-x)) {
			best = stop
		}
	}
	return best.Position
}

// Visual moves one visible boundary. Equal-X bidi stops do not cause a key
// press to leave the Caret stationary. Crossing a soft wrap preserves affinity.
// false means the caller must continue into the previous/next paragraph.
func (g *Geometry) Visual(pos Position, right bool) (Position, bool) {
	if g.pages != nil {
		return g.pages.visual(pos, right)
	}
	i, j := g.locate(pos)
	line := &g.lines[i]
	x := line.stops[j].x
	if right {
		for j++; j < len(line.stops); j++ {
			if line.stops[j].x > x {
				return line.stops[j].Position, true
			}
		}
		if i+1 < len(g.lines) {
			return g.lines[i+1].stops[0].Position, true
		}
	} else {
		for j--; j >= 0; j-- {
			if line.stops[j].x < x {
				return line.stops[j].Position, true
			}
		}
		if i > 0 {
			stops := g.lines[i-1].stops
			return stops[len(stops)-1].Position, true
		}
	}
	return pos, false
}

// Selection returns the selected Cluster rectangles, merging adjacent spans
// on each visual line but retaining disjoint bidirectional spans.
func (g *Geometry) Selection(rng Range) []geometry.Rectangle {
	return g.SelectionLines(rng, 0, g.LineCount())
}

// SelectionLines returns the selected Cluster rectangles on visual lines
// [first, end), using the same merging and bidi semantics as Selection. Bounds
// must satisfy 0 <= first <= end <= LineCount(). Returned rectangles retain
// their full paragraph-local geometry; the painter still applies its clip.
//
// Use LineAtY to restrict painting to the viewport without traversing the
// rest of a wrapped paragraph. This only queries supplied metrics; it does not
// perform native shaping. NewGeometry indexes its input eagerly; a geometry
// obtained from Fragment prepares only the detailed pages needed by queries.
func (g *Geometry) SelectionLines(rng Range, first, end int) []geometry.Rectangle {
	if g.pages != nil {
		return g.pages.selection(rng, first, end)
	}
	if first < 0 || first > end || end > len(g.lines) {
		panic("textedit: invalid visual line range")
	}
	if rng.Start >= rng.End {
		return nil
	}
	if g.orderedOffsets {
		first = max(first, sort.Search(len(g.lines), func(i int) bool { return g.lines[i].last > rng.Start }))
		end = min(end, sort.Search(len(g.lines), func(i int) bool { return g.lines[i].first >= rng.End }))
	}
	var result []geometry.Rectangle
	for i := first; i < end; i++ {
		line := &g.lines[i]
		var rects []geometry.Rectangle
		for _, c := range line.clusters {
			if c.Start < rng.End && c.Start+c.Length > rng.Start && c.Width > 0 {
				rects = append(rects, geometry.Rect(c.X, line.metrics.Y, c.Width, line.metrics.Height))
			}
		}
		sort.Slice(rects, func(i, j int) bool { return rects[i].X < rects[j].X })
		first := len(result)
		for _, rect := range rects {
			if len(result) > first {
				last := &result[len(result)-1]
				if rect.X <= last.X+last.Width {
					last.Width = max(last.X+last.Width, rect.X+rect.Width) - last.X
					continue
				}
			}
			result = append(result, rect)
		}
	}
	return result
}
