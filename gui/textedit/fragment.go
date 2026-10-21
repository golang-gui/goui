package textedit

import (
	"fmt"
	"math"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform/typography"
)

// Fragment owns a snapshot of measured text. It contains no native objects and
// remains valid after the supplying layout is changed or destroyed. A fragment
// can cover a complete input or a range of complete measured display lines.
//
// Copies and LineRange views share immutable storage. Metrics returns independent
// slices. Range and Origin map fragment-local byte offsets and DIP coordinates
// back to the supplying input; neither changes the document or inserts breaks.
type Fragment struct {
	data       *fragmentMetrics
	first, end int
}

type fragmentMetrics struct {
	text                Range
	size                geometry.Size
	lines               []typography.TextLine
	clusters            [][]typography.TextCluster
	firstStop, lastStop []int
	orderedOffsets      bool
}

// NewFragment copies metrics whose byte offsets and coordinates are local to
// text. text identifies that input in its parent's byte coordinates. size is the
// occupied size, not the layout box. An empty lines slice describes an empty
// input; it does not assert that an unmeasured nonempty input is ready.
//
// No shaping or boundary inference is performed here. The caller must only
// publish metrics already confirmed for their complete text context.
func NewFragment(text Range, size geometry.Size, lines []typography.TextLine, clusters []typography.TextCluster) (Fragment, error) {
	finite := func(v float32) bool { return !math.IsNaN(float64(v)) && !math.IsInf(float64(v), 0) }
	if text.Start < 0 || text.End < text.Start || !finite(size.Width) || !finite(size.Height) || size.Width < 0 || size.Height < 0 {
		return Fragment{}, fmt.Errorf("text fragment: invalid text range or size")
	}
	length := text.End - text.Start
	if len(lines) == 0 && length != 0 {
		return Fragment{}, fmt.Errorf("text fragment: nonempty input has no measured lines")
	}
	if len(lines) > 0 && lines[0].Start != 0 {
		return Fragment{}, fmt.Errorf("text fragment: first measured line does not start at input")
	}
	d := &fragmentMetrics{text: text, size: size, lines: make([]typography.TextLine, len(lines)), clusters: make([][]typography.TextCluster, len(lines))}
	counts := make([]int, len(lines))
	for i, line := range lines {
		if line.Start < 0 || line.Length < 0 || line.Start > length-line.Length ||
			!finite(line.X) || !finite(line.Y) || !finite(line.Width) || !finite(line.Height) || !finite(line.Baseline) ||
			line.Width < 0 || line.Height <= 0 || i > 0 && line.Start < lines[i-1].Start+lines[i-1].Length {
			return Fragment{}, fmt.Errorf("text fragment: invalid or unordered line %d", i)
		}
		d.lines[i] = line
		d.lines[i].Clusters = nil
	}
	countCluster := func(c typography.TextCluster, index int) error {
		if index < 0 || index >= len(lines) || c.Start < 0 || c.Length <= 0 || c.Start > length-c.Length ||
			!finite(c.X) || !finite(c.Y) || !finite(c.Width) || !finite(c.Height) || c.Width < 0 || c.Height < 0 {
			return fmt.Errorf("text fragment: invalid cluster at %d", c.Start)
		}
		// A shaping cluster cannot cross a completed display-line boundary.
		line := lines[index]
		end := length
		if index+1 < len(lines) {
			end = lines[index+1].Start
		}
		if c.Start < line.Start || c.Start+c.Length > end {
			return fmt.Errorf("text fragment: cluster crosses line %d", index)
		}
		counts[index]++
		return nil
	}
	if len(clusters) != 0 {
		for _, c := range clusters {
			if err := countCluster(c, c.LineIndex); err != nil {
				return Fragment{}, err
			}
		}
	} else {
		for i, line := range lines {
			for _, c := range line.Clusters {
				if err := countCluster(c, i); err != nil {
					return Fragment{}, err
				}
			}
		}
	}
	count := 0
	for _, n := range counts {
		count += n
	}
	storage := make([]typography.TextCluster, count)
	at := 0
	for i, n := range counts {
		d.clusters[i] = storage[at : at+n : at+n]
		at += n
		counts[i] = 0
	}
	copyCluster := func(c typography.TextCluster, index int) {
		c.LineIndex = index
		d.clusters[index][counts[index]] = c
		counts[index]++
	}
	if len(clusters) != 0 {
		for _, c := range clusters {
			copyCluster(c, c.LineIndex)
		}
	} else {
		for i, line := range lines {
			for _, c := range line.Clusters {
				copyCluster(c, i)
			}
		}
	}
	d.firstStop, d.lastStop = make([]int, len(lines)), make([]int, len(lines))
	d.orderedOffsets = true
	for i, cs := range d.clusters {
		first, last := length, 0
		if len(cs) == 0 {
			first, last = lines[i].Start, lines[i].Start
		}
		for _, c := range cs {
			first, last = min(first, c.Start), max(last, c.Start+c.Length)
		}
		d.firstStop[i], d.lastStop[i] = first, last
		if i > 0 {
			d.orderedOffsets = d.orderedOffsets && d.lastStop[i-1] <= first && d.firstStop[i-1] < first && d.lastStop[i-1] < last
		}
	}
	return Fragment{data: d, end: len(lines)}, nil
}

// LineCount returns the number of measured display lines.
func (f Fragment) LineCount() int { return f.end - f.first }

// Range identifies the fragment's bytes in the original parent's coordinates.
// A line includes its trailing separator up to the next display-line start.
func (f Fragment) Range() Range {
	if f.data == nil {
		return Range{}
	}
	start, end := f.data.text.Start, f.data.text.End
	if f.first < len(f.data.lines) {
		start += f.data.lines[f.first].Start
	} else {
		start = end
	}
	if f.end < len(f.data.lines) {
		end = f.data.text.Start + f.data.lines[f.end].Start
	}
	if f.first == f.end {
		end = start
	}
	return Range{Start: start, End: end}
}

// Origin translates fragment-local coordinates to the original layout. X stays
// unchanged, preserving alignment and RTL line positions relative to the box.
func (f Fragment) Origin() geometry.Point {
	if f.data == nil || f.first == 0 {
		return geometry.Point{}
	}
	if f.first >= len(f.data.lines) {
		return geometry.Point{Y: f.data.size.Height}
	}
	return geometry.Point{Y: f.data.lines[f.first].Y}
}

// Size returns the fragment's occupied extent. The complete snapshot preserves
// the supplied size. A line view reports the rightmost X+Width and the bottom
// edge relative to Origin; native line offsets and inter-line gaps are retained.
func (f Fragment) Size() geometry.Size {
	if f.data == nil {
		return geometry.Size{}
	}
	if f.first == 0 && f.end == len(f.data.lines) {
		return f.data.size
	}
	if f.first == f.end {
		return geometry.Size{}
	}
	var size geometry.Size
	origin := f.Origin()
	for _, l := range f.data.lines[f.first:f.end] {
		size.Width = max(size.Width, l.X+l.Width)
		size.Height = max(size.Height, l.Y+l.Height-origin.Y)
	}
	return size
}

// LineRange returns a zero-copy immutable view of [first,end) display lines.
// The indices are relative to f. It never measures text or guesses a boundary.
func (f Fragment) LineRange(first, end int) (Fragment, error) {
	if f.data == nil || first < 0 || end < first || end > f.LineCount() {
		return Fragment{}, fmt.Errorf("text fragment: invalid display-line range [%d,%d)", first, end)
	}
	return Fragment{data: f.data, first: f.first + first, end: f.first + end}, nil
}

// Metrics returns independent metrics in fragment-local byte/DIP coordinates.
// Work is proportional to this fragment, not the complete measured input.
func (f Fragment) Metrics() (lines []typography.TextLine, clusters []typography.TextCluster) {
	if f.data == nil {
		return nil, nil
	}
	start := f.Range().Start - f.data.text.Start
	y := f.Origin().Y
	lines = make([]typography.TextLine, f.LineCount())
	for index := f.first; index < f.end; index++ {
		line := f.data.lines[index]
		line.Start -= start
		line.Y -= y
		line.Baseline -= y
		local := index - f.first
		begin := len(clusters)
		for _, cluster := range f.data.clusters[index] {
			cluster.Start -= start
			cluster.Y -= y
			cluster.LineIndex = local
			clusters = append(clusters, cluster)
		}
		line.Clusters = clusters[begin:len(clusters)]
		lines[local] = line
	}
	return lines, clusters
}

// Geometry builds editing geometry for this fragment only. The caller translates
// positions with Range and Origin, and owns/caches the resulting ordinary value.
func (f Fragment) Geometry(emptyHeight float32) Geometry {
	if f.LineCount() > geometryPageLines && f.data.orderedOffsets {
		return Geometry{pages: newFragmentGeometry(f, emptyHeight)}
	}
	lines, clusters := f.Metrics()
	rng := f.Range()
	return NewGeometry(lines, clusters, rng.End-rng.Start, emptyHeight)
}
