package textedit

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/golang-gui/goui/platform/typography"
	"github.com/rivo/uniseg"
)

// WrapMeasurements owns width-independent advances for one already shaped
// paragraph without forced line breaks. It neither shapes text nor owns a native layout. The caller must
// supply clusters measured with the complete shaping and bidi context. Copies
// share immutable storage and remain valid after the native layout is destroyed.
//
// Advances describe logical progression, not visual X coordinates. The result
// is a line-break candidate index, not visual geometry: line-boundary shaping,
// bidi reordering and final painting must use the caller's confirmed line data.
type WrapMeasurements struct {
	data *wrapMeasurements
}

type wrapMeasurements struct {
	text     string
	offsets  []int
	advances []float64
	breaks   []int // cluster indices with a Unicode line-break opportunity
	tabs     []int // cluster indices containing a single tab
}

// NewWrapMeasurements copies a complete, non-overlapping partition of text's
// shaping clusters. Logical order need not match input order. Widths must be
// nonnegative finite advances. A tab must form its own cluster; its advance is
// recomputed by the iterator because it depends on the new line's origin.
//
// This function does not assert that individually measured characters compose
// correctly. Context-dependent glyphs must have been shaped together first.
func NewWrapMeasurements(text string, clusters []typography.TextCluster) (WrapMeasurements, error) {
	if !utf8.ValidString(text) || strings.ContainsAny(text, "\r\n\v\f\u0085\u2028\u2029") {
		return WrapMeasurements{}, fmt.Errorf("text wrapping: expected one UTF-8 paragraph")
	}
	ordered := append([]typography.TextCluster(nil), clusters...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].Start < ordered[j].Start })
	d := &wrapMeasurements{text: strings.Clone(text), offsets: make([]int, 1, len(ordered)+1), advances: make([]float64, 1, len(ordered)+1)}
	end := 0
	for _, c := range ordered {
		if c.Start != end || c.Length <= 0 || c.Length > len(text)-end || !utf8.RuneStart(text[end]) ||
			end+c.Length < len(text) && !utf8.RuneStart(text[end+c.Length]) ||
			c.Width < 0 || math.IsNaN(float64(c.Width)) || math.IsInf(float64(c.Width), 0) {
			return WrapMeasurements{}, fmt.Errorf("text wrapping: invalid cluster at %d", c.Start)
		}
		part := text[end : end+c.Length]
		if strings.ContainsRune(part, '\t') {
			if part != "\t" {
				return WrapMeasurements{}, fmt.Errorf("text wrapping: tab shares a cluster at %d", end)
			}
		}
		end += c.Length
		d.offsets = append(d.offsets, end)
		d.advances = append(d.advances, d.advances[len(d.advances)-1]+float64(c.Width))
	}
	if end != len(text) {
		return WrapMeasurements{}, fmt.Errorf("text wrapping: clusters do not cover the paragraph")
	}
	// Intersect shaping and grapheme boundaries for emergency wrapping too:
	// fallback fonts can expose several native clusters for one emoji/ZWJ.
	offsets, advances := d.offsets, d.advances
	d.offsets, d.advances = []int{0}, []float64{0}
	g := uniseg.NewGraphemes(text)
	cluster := 1
	for g.Next() {
		_, end := g.Positions()
		for cluster < len(offsets) && offsets[cluster] < end {
			cluster++
		}
		if cluster < len(offsets) && offsets[cluster] == end {
			if text[d.offsets[len(d.offsets)-1]:end] == "\t" {
				d.tabs = append(d.tabs, len(d.offsets)-1)
			}
			d.offsets = append(d.offsets, end)
			d.advances = append(d.advances, advances[cluster])
			if g.LineBreak() != uniseg.LineDontBreak {
				d.breaks = append(d.breaks, len(d.offsets)-1)
			}
		}
	}
	return WrapMeasurements{d}, nil
}

// Text returns the measured input. It is immutable and independent of the
// supplying layout's lifetime.
func (m WrapMeasurements) Text() string {
	if m.data == nil {
		return ""
	}
	return m.data.text
}

// WrapIterator advances through a paragraph without rescanning shaped clusters
// or allocating native resources. Each Next query uses prefix advances and
// binary searches; tab-separated spans are processed in order. Iterators have
// independent positions and are sequential, not safe for concurrent use.
type WrapIterator struct {
	measurements WrapMeasurements
	cluster      int
	tab          int
	empty        bool
}

// Iterator starts at the beginning of the measured paragraph. Keeping an
// iterator between event-loop turns allows line breaking to yield after any
// completed candidate. Width may change between queries; callers normally
// start a new iterator when reflowing the same paragraph at another width.
func (m WrapMeasurements) Iterator() WrapIterator { return WrapIterator{measurements: m} }

// WrapLine is a candidate line in the original paragraph's UTF-8 coordinates.
// Width includes the supplied advances and origin-dependent tab expansion.
// It does not include line-end reshaping, alignment or justification.
type WrapLine struct {
	Range Range
	Width float32
}

// Next returns the next nonempty candidate, or the empty paragraph once.
// width must be finite and nonnegative. A non-fitting first cluster is included
// whole to ensure progress. word selects Unicode opportunities before using a
// whole-cluster emergency break. A zero tabWidth preserves supplied tab widths;
// otherwise tabs advance to the next positive repeating stop from line X=0.
func (it *WrapIterator) Next(width, tabWidth float32, word bool) (WrapLine, bool) {
	if width < 0 || tabWidth < 0 || math.IsNaN(float64(width)) || math.IsInf(float64(width), 0) ||
		math.IsNaN(float64(tabWidth)) || math.IsInf(float64(tabWidth), 0) {
		panic("text wrapping: invalid width or tab interval")
	}
	d := it.measurements.data
	if d == nil {
		return WrapLine{}, false
	}
	n := len(d.offsets) - 1
	if n == 0 {
		if it.empty {
			return WrapLine{}, false
		}
		it.empty = true
		return WrapLine{}, true
	}
	if it.cluster == n {
		return WrapLine{}, false
	}
	start := it.cluster
	end, advance := it.fit(n, float64(width), float64(tabWidth))
	if end == start || advance == 0 && end < n {
		end++
		advance = it.advance(end, float64(tabWidth))
	}
	if word && end < n {
		i := sort.SearchInts(d.breaks, end+1) - 1
		if i >= 0 && d.breaks[i] > start && d.breaks[i] != end {
			end = d.breaks[i]
			advance = it.advance(end, float64(tabWidth))
		}
	}
	it.cluster = end
	for it.tab < len(d.tabs) && d.tabs[it.tab] < end {
		it.tab++
	}
	return WrapLine{Range: Range{Start: d.offsets[start], End: d.offsets[end]}, Width: float32(advance)}, true
}

func (it *WrapIterator) fit(end int, width, tabWidth float64) (int, float64) {
	d := it.measurements.data
	at, advance := it.cluster, float64(0)
	for tab := it.tab; ; {
		limit := end
		if tabWidth > 0 && tab < len(d.tabs) && d.tabs[tab] < end {
			limit = d.tabs[tab]
		}
		base := d.advances[at]
		fit := at + sort.Search(limit-at, func(i int) bool { return advance+d.advances[at+i+1]-base > width })
		advance += d.advances[fit] - base
		if fit < limit || limit == end {
			return fit, advance
		}
		next := (math.Floor(advance/tabWidth) + 1) * tabWidth
		if next > width {
			return limit, advance
		}
		advance, at = next, limit+1
		tab++
	}
}

func (it *WrapIterator) advance(end int, tabWidth float64) float64 {
	_, advance := it.fit(end, math.MaxFloat64, tabWidth)
	return advance
}
