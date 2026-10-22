package textedit

import (
	"sort"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform/typography"
)

const geometryPageLines = 32
const geometryPageLimit = 4

// Detailed caret stops are a small working set. Complete native metrics remain
// ordinary immutable values; preparing a page does not shape or measure text.
type fragmentGeometry struct {
	fragment    Fragment
	emptyHeight float32
	pages       map[int]*geometryPage
	clock       uint64
	orderedY    bool
}

type geometryPage struct {
	geometry   Geometry
	boundaries []int // actual cluster boundaries, without artificial page edges
	used       uint64
}

func newFragmentGeometry(f Fragment, height float32) *fragmentGeometry {
	p := &fragmentGeometry{fragment: f, emptyHeight: height, pages: make(map[int]*geometryPage), orderedY: true}
	for i := 1; i < f.LineCount(); i++ {
		a, b := p.line(i-1), p.line(i)
		p.orderedY = p.orderedY && a.Y+a.Height <= b.Y
	}
	return p
}

func (p *fragmentGeometry) line(index int) typography.TextLine {
	f := p.fragment
	line := f.data.lines[f.first+index]
	line.Start -= f.Range().Start - f.data.text.Start
	line.Y -= f.Origin().Y
	line.Baseline -= f.Origin().Y
	return line
}

func (p *fragmentGeometry) lineMetrics(index int) typography.TextLine {
	f := p.fragment
	line := p.line(index)
	start, y := f.Range().Start-f.data.text.Start, f.Origin().Y
	for _, c := range f.data.lineClusters(f.first + index) {
		c.Start -= start
		c.Y -= y
		c.LineIndex = index
		line.Clusters = append(line.Clusters, c)
	}
	return line
}

func (p *fragmentGeometry) page(number int) *geometryPage {
	p.clock++
	if page := p.pages[number]; page != nil {
		page.used = p.clock
		return page
	}
	f := p.fragment
	first, end := number*geometryPageLines, min((number+1)*geometryPageLines, f.LineCount())
	lines := make([]typography.TextLine, end-first)
	var clusters []typography.TextCluster
	page := &geometryPage{used: p.clock}
	start, y := f.Range().Start-f.data.text.Start, f.Origin().Y
	for i := first; i < end; i++ {
		lines[i-first] = p.line(i)
		begin := len(clusters)
		for _, c := range f.data.lineClusters(f.first + i) {
			c.Start -= start
			c.Y -= y
			c.LineIndex = i - first
			clusters = append(clusters, c)
			page.boundaries = append(page.boundaries, c.Start, c.Start+c.Length)
		}
		lines[i-first].Clusters = clusters[begin:len(clusters)]
	}
	length := f.Range().End - f.Range().Start
	if first == 0 {
		page.boundaries = append(page.boundaries, 0)
	}
	if end == f.LineCount() {
		page.boundaries = append(page.boundaries, length)
	}
	sort.Ints(page.boundaries)
	page.boundaries = compactBoundaries(page.boundaries)
	page.geometry = NewGeometry(lines, clusters, length, p.emptyHeight)
	if len(p.pages) >= geometryPageLimit {
		oldest := number
		var used uint64 = ^uint64(0)
		for key, candidate := range p.pages {
			if candidate.used < used {
				oldest, used = key, candidate.used
			}
		}
		delete(p.pages, oldest)
	}
	p.pages[number] = page
	return page
}

func compactBoundaries(values []int) []int {
	out := values[:0]
	for _, n := range values {
		if len(out) == 0 || out[len(out)-1] != n {
			out = append(out, n)
		}
	}
	return out
}

func (p *fragmentGeometry) lineAtY(y float32) int {
	count := p.fragment.LineCount()
	if p.orderedY {
		return min(count-1, sort.Search(count, func(i int) bool {
			l := p.line(i)
			return !(y >= l.Y+l.Height)
		}))
	}
	i := 0
	for i+1 < count {
		l := p.line(i)
		if !(y >= l.Y+l.Height) {
			break
		}
		i++
	}
	return i
}

// The byte ranges partition ordered lines, but some contain only invisible
// separators. Include adjacent pages to preserve nearest-stop/affinity ties.
func (p *fragmentGeometry) locate(pos Position) (int, int, *geometryPage) {
	count := p.fragment.LineCount()
	f := p.fragment
	start := f.Range().Start - f.data.text.Start
	i := sort.Search(count, func(i int) bool {
		return f.data.lastStop[f.first+i]-start >= pos.Offset
	})
	first, end := max(0, min(i, count-1)-1)/geometryPageLines, min(count-1, i+1)/geometryPageLines
	bestDistance := int(^uint(0) >> 1)
	var bestPage *geometryPage
	var bestLine, bestStop int
	for number := first; number <= end; number++ {
		page := p.page(number)
		line, stop := page.geometry.locate(pos)
		candidate := page.geometry.lines[line].stops[stop].Position
		if candidate == pos {
			return line + number*geometryPageLines, stop, page
		}
		distance := candidate.Offset - pos.Offset
		if distance < 0 {
			distance = -distance
		}
		if distance < bestDistance {
			bestDistance, bestPage = distance, page
			bestLine, bestStop = line+number*geometryPageLines, stop
		}
	}
	return bestLine, bestStop, bestPage
}

func (p *fragmentGeometry) boundary(offset int, forward, strict bool) int {
	count := p.fragment.LineCount()
	length := p.fragment.Range().End - p.fragment.Range().Start
	if offset < 0 || offset == 0 && (!forward || !strict) {
		return 0
	}
	if offset > length || offset == length && (forward || !strict) {
		return length
	}
	i := max(0, sort.Search(count, func(i int) bool { return p.line(i).Start > offset })-1)
	for number := i / geometryPageLines; number >= 0 && number <= (count-1)/geometryPageLines; {
		bounds := p.page(number).boundaries
		if forward {
			at := sort.Search(len(bounds), func(i int) bool { return bounds[i] > offset || !strict && bounds[i] == offset })
			if at < len(bounds) {
				return bounds[at]
			}
			number++
		} else {
			at := sort.Search(len(bounds), func(i int) bool { return bounds[i] >= offset && (strict || bounds[i] != offset) })
			if at > 0 {
				return bounds[at-1]
			}
			number--
		}
	}
	if forward {
		return length
	}
	return 0
}

func (p *fragmentGeometry) caret(pos Position) geometry.Rectangle {
	i, stop, page := p.locate(pos)
	line := page.geometry.lines[i%geometryPageLines]
	return geometry.Rect(line.stops[stop].x, line.metrics.Y, 1, line.metrics.Height)
}

func (p *fragmentGeometry) hitLine(index int, x float32) Position {
	return p.page(index/geometryPageLines).geometry.HitLine(index%geometryPageLines, x)
}

func (p *fragmentGeometry) lineEdge(index int, right bool) Position {
	return p.page(index/geometryPageLines).geometry.LineEdge(index%geometryPageLines, right)
}

func (p *fragmentGeometry) visual(pos Position, right bool) (Position, bool) {
	i, _, page := p.locate(pos)
	if next, ok := page.geometry.Visual(pos, right); ok {
		return next, true
	}
	if right && i+1 < p.fragment.LineCount() {
		return p.lineEdge(i+1, false), true
	}
	if !right && i > 0 {
		return p.lineEdge(i-1, true), true
	}
	return pos, false
}

func (p *fragmentGeometry) selection(rng Range, first, end int) []geometry.Rectangle {
	if first < 0 || first > end || end > p.fragment.LineCount() {
		panic("textedit: invalid visual line range")
	}
	if rng.Start >= rng.End {
		return nil
	}
	count := p.fragment.LineCount()
	f := p.fragment
	start := f.Range().Start - f.data.text.Start
	first = max(first, sort.Search(count, func(i int) bool {
		return f.data.lastStop[f.first+i]-start > rng.Start
	}))
	end = min(end, sort.Search(count, func(i int) bool { return f.data.firstStop[f.first+i]-start >= rng.End }))
	var result []geometry.Rectangle
	for first < end {
		number := first / geometryPageLines
		last := min(end, (number+1)*geometryPageLines)
		result = append(result, p.page(number).geometry.SelectionLines(rng, first%geometryPageLines, last-number*geometryPageLines)...)
		first = last
	}
	return result
}
