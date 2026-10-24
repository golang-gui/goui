package gui

import (
	"fmt"
	"image/color"
	"math"
	"slices"
	"sort"
	"time"
	"unicode/utf8"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui/textedit"
	"github.com/golang-gui/goui/platform/typography"
	"github.com/rivo/uniseg"
	"golang.org/x/text/unicode/bidi"
)

const paragraphBlockBytes = 4096
const paragraphPageLimit = 12

// ParagraphLayout retains a bounded native working set for a long logical
// paragraph. It belongs to one GUI-thread consumer and must be destroyed by that
// consumer. read uses paragraph-local UTF-8 coordinates and must remain valid
// until Destroy; update the length with Change after editing its source.
//
// Global extents are estimates until the corresponding blocks are measured.
// Geometry and painting always use the same measured local windows. A block
// boundary is not a document newline. Native calls never receive the paragraph
// as a whole. This is GUI layout policy, independent of typography backends.
type ParagraphLayout struct {
	context                             typography.Context
	format                              typography.TextFormat
	width                               float32
	length                              int
	read                                func(textedit.Range) (string, error)
	pages                               map[int]*paragraphPage
	starts                              map[int]int
	heights                             textedit.HeightIndex
	lineHeight, blockHeight, blockWidth float32
	runeAdvance                         float32
	viewport                            geometry.Rectangle
	active                              []int
	geometry                            textedit.Geometry
	geometryVersion                     uint64
	geometryIndices                     []int
	editGeometry                        *textedit.Geometry
	editVersion                         uint64
	editIndices                         []int
	clock, version                      uint64
	checkpoints                         []paragraphContext
	scanned                             int
	state                               paragraphContext
	err                                 error
	destroyed                           bool
}

type paragraphPage struct {
	index          int
	runesBefore    int
	range_         textedit.Range
	layout         typography.TextLayout
	prefix         int
	lines          []typography.TextLine
	clusters       []typography.TextCluster
	height, width  float32
	used           uint64
	geometry       *textedit.Geometry
	geometryOrigin geometry.Point
}

type paragraphContext struct {
	runes        int
	at           int
	base, strong rune
	stack        []rune
}

// NewParagraphLayout creates an empty per-view cache. No text is read or native
// resource allocated until Prepare or GeometryAt requests a bounded window.
func NewParagraphLayout(context typography.Context, format typography.TextFormat, width float32, length int, read func(textedit.Range) (string, error)) *ParagraphLayout {
	p := &ParagraphLayout{context: context, format: format, width: max(1, width), length: length, read: read, pages: make(map[int]*paragraphPage), starts: map[int]int{0: 0}, lineHeight: 20}
	p.blockHeight = 20 * float32(math.Ceil(float64(paragraphBlockBytes)*8/float64(p.width)))
	if format.WrapMode == typography.WrapNone {
		p.blockHeight = 20
	}
	p.blockWidth = paragraphBlockBytes * 8
	p.heights.Reset(p.blockCount(), p.blockHeight)
	p.checkpoints = []paragraphContext{{}}
	return p
}

func (p *ParagraphLayout) blockCount() int {
	return max(1, (p.length+paragraphBlockBytes-1)/paragraphBlockBytes)
}

// Destroy releases all native windows. It is safe to call more than once.
func (p *ParagraphLayout) Destroy() {
	p.destroyed = true
	p.version++
	p.clearPages()
}

func (p *ParagraphLayout) clearPages() {
	old := p.pages
	p.pages = make(map[int]*paragraphPage)
	p.active = nil
	p.geometryIndices = nil
	p.editGeometry = nil
	for _, page := range old {
		page.layout.Destroy()
	}
}

// Version changes when cached native windows or their coordinates change.
func (p *ParagraphLayout) Version() uint64 { return p.version }

// Err returns the first failure for the current input. Change permits a retry.
func (p *ParagraphLayout) Err() error { return p.err }

// Size returns the currently known extent, including estimates for unseen text.
func (p *ParagraphLayout) Size() geometry.Size {
	if p.format.WrapMode == typography.WrapNone {
		return geometry.Size{Width: float32(p.length) / paragraphBlockBytes * p.blockWidth, Height: p.lineHeight}
	}
	return geometry.Size{Width: p.width, Height: p.heights.Total()}
}

// SetWidth invalidates wrapping windows while preserving the text context index.
func (p *ParagraphLayout) SetWidth(width float32) {
	width = max(1, width)
	if p.width == width {
		return
	}
	if p.format.WrapMode == typography.WrapNone {
		p.width = width
		return
	}
	ratio := p.width / width
	p.clearPages()
	p.width = width
	p.starts = map[int]int{0: 0}
	p.blockHeight = max(p.lineHeight, p.blockHeight*ratio)
	p.heights.Reset(p.blockCount(), p.blockHeight)
	p.version++
}

// Change invalidates the edited window and its contextual suffix. Earlier
// native windows stay reusable. Edits use actual execution order and paragraph
// coordinates; callers must notify every source change before requesting layout.
func (p *ParagraphLayout) Change(length int, edits []textedit.Edit) {
	for _, edit := range edits {
		for index, page := range p.pages {
			if page.range_.End >= edit.Range.Start {
				delete(p.pages, index)
				page.layout.Destroy()
			}
		}
		// Context before an edit remains confirmed; later context is rescanned
		// in bounded turns. No edit scans the suffix or copies the paragraph.
		keep := sort.Search(len(p.checkpoints), func(i int) bool { return p.checkpoints[i].at > edit.Range.Start })
		p.checkpoints = p.checkpoints[:max(1, keep)]
		p.state = p.checkpoints[len(p.checkpoints)-1]
		p.state.stack = append([]rune(nil), p.state.stack...)
		p.scanned = p.state.at
	}
	p.length = length
	p.starts = map[int]int{0: 0}
	p.active = nil
	p.heights.Reset(p.blockCount(), p.blockHeight)
	for index, page := range p.pages {
		p.starts[index] = page.range_.Start
		p.starts[index+1] = page.range_.End
		p.heights.Set(index, page.height)
	}
	p.version++
	p.err = nil
}

func (p *ParagraphLayout) readRange(start, end int) (textedit.Range, string, error) {
	start = max(0, min(start, p.length))
	end = max(start, min(end, p.length))
	// Model readers reject UTF-8 interior byte positions. At most three bytes
	// on either edge are needed to align a requested range.
	var last error
	for a := 0; a < 4 && start-a >= 0; a++ {
		for b := 0; b < 4 && end-b >= start-a; b++ {
			r := textedit.Range{Start: start - a, End: end - b}
			s, err := p.read(r)
			if err == nil && utf8.ValidString(s) {
				return r, s, nil
			}
			last = err
		}
	}
	if last == nil {
		last = fmt.Errorf("paragraph reader returned invalid UTF-8")
	}
	return textedit.Range{}, "", last
}

func (s *paragraphContext) consume(text string) {
	// Long ASCII payloads need no Unicode state-machine steps. Checking eight
	// bytes at a time keeps prefix indexing cheap without shaping unseen text.
	if paragraphASCII(text) {
		s.runes += len(text)
		if s.base == 0 || s.strong != '\u200e' {
			for i := 0; i < len(text); i++ {
				c := text[i]
				if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' {
					if s.base == 0 {
						s.base = '\u200e'
					}
					s.strong = '\u200e'
					break
				}
			}
		}
		return
	}
	for _, r := range text {
		s.runes++
		switch r {
		case '\u202a', '\u202b', '\u202d', '\u202e', '\u2066', '\u2067', '\u2068':
			if len(s.stack) < 125 {
				s.stack = append(s.stack, r)
			}
		case '\u202c':
			if n := len(s.stack); n > 0 && s.stack[n-1] < 0x2066 {
				s.stack = s.stack[:n-1]
			}
		case '\u2069':
			for len(s.stack) > 0 {
				n := len(s.stack) - 1
				r := s.stack[n]
				s.stack = s.stack[:n]
				if r >= 0x2066 {
					break
				}
			}
		default:
			if r < 128 {
				if r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' {
					if s.base == 0 {
						s.base = '\u200e'
					}
					s.strong = '\u200e'
				}
				continue
			}
			props, _ := bidi.LookupRune(r)
			switch props.Class() {
			case bidi.L:
				if s.base == 0 {
					s.base = '\u200e'
				}
				s.strong = '\u200e'
			case bidi.R, bidi.AL:
				if s.base == 0 {
					s.base = '\u200f'
				}
				s.strong = '\u200f'
			}
		}
	}
}

func paragraphASCII(text string) bool {
	i := 0
	for ; i+8 <= len(text); i += 8 {
		if (text[i]|text[i+1]|text[i+2]|text[i+3]|text[i+4]|text[i+5]|text[i+6]|text[i+7])&0x80 != 0 {
			return false
		}
	}
	for ; i < len(text); i++ {
		if text[i] >= 128 {
			return false
		}
	}
	return true
}

func (p *ParagraphLayout) contextAt(at int, deadline time.Time) (paragraphContext, bool) {
	advanced := false
	for p.scanned < at {
		if advanced && !time.Now().Before(deadline) {
			return paragraphContext{}, false
		}
		r, text, err := p.readRange(p.scanned, min(at, p.scanned+64*1024))
		if err != nil {
			p.err = err
			return paragraphContext{}, false
		}
		if r.End <= p.scanned {
			return paragraphContext{}, false
		}
		p.state.consume(text)
		advanced = true
		p.scanned = r.End
		p.state.at = p.scanned
		copy := p.state
		copy.stack = append([]rune(nil), copy.stack...)
		p.checkpoints = append(p.checkpoints, copy)
	}
	i := sort.Search(len(p.checkpoints), func(i int) bool { return p.checkpoints[i].at > at }) - 1
	s := p.checkpoints[max(0, i)]
	s.stack = append([]rune(nil), s.stack...)
	if s.at < at {
		_, text, err := p.readRange(s.at, at)
		if err != nil {
			p.err = err
			return s, false
		}
		s.consume(text)
	}
	return s, true
}

func (p *ParagraphLayout) page(index int, deadline time.Time) *paragraphPage {
	if p.destroyed || p.err != nil {
		return nil
	}
	index = max(0, min(index, p.blockCount()-1))
	p.clock++
	if page := p.pages[index]; page != nil {
		page.used = p.clock
		return page
	}
	start := index * paragraphBlockBytes
	if at, ok := p.starts[index]; ok {
		start = at
	} else {
		// A random window needs a confirmed grapheme boundary. ASCII/ASCII
		// and ASCII/CJK transitions provide independent safe scan anchors.
		// Otherwise advance the nearest confirmed predecessor on later turns.
		if at, ok := p.alignStart(start); ok {
			start = at
		} else {
			next := 0
			for i := range p.starts {
				if i <= index && i > next {
					next = i
				}
			}
			if next < index {
				p.page(next, deadline)
			}
			return nil
		}
	}
	r, text, err := p.readRange(start, min(p.length, (index+1)*paragraphBlockBytes))
	if err != nil {
		p.err = err
		return nil
	}
	state, ready := p.contextAt(r.Start, deadline)
	if !ready {
		return nil
	}
	var prefix string
	if r.Start > 0 && (len(state.stack) > 0 || state.base == '\u200f') {
		base := state.base
		if base == 0 {
			base = '\u200e'
		}
		prefix = string(base) + string(state.stack)
		if state.strong != 0 {
			prefix += string(state.strong)
		}
	}
	version := p.version
	native, err := p.context.NewTextLayout(prefix+text, p.format, p.width, 1<<20)
	if err != nil {
		p.err = err
		return nil
	}
	if p.destroyed || p.version != version {
		native.Destroy()
		return nil
	}
	w, h := native.MeasureSize()
	lines, clusters := native.MeasureMetrics()
	if p.destroyed || p.version != version {
		native.Destroy()
		return nil
	}
	if len(lines) == 0 {
		native.Destroy()
		return nil
	}
	validLines := len(lines)
	end := r.End
	if p.format.WrapMode != typography.WrapNone && end < p.length && len(lines) > 1 {
		validLines--
		end = r.Start + lines[validLines].Start - len(prefix)
		h = lines[validLines].Y
	}
	if end <= r.Start {
		native.Destroy()
		return nil
	}
	page := &paragraphPage{index: index, range_: textedit.Range{Start: r.Start, End: end}, layout: native, prefix: len(prefix), height: h, width: w, used: p.clock, runesBefore: state.runes}
	for _, line := range lines[:validLines] {
		line.Start += r.Start - len(prefix)
		line.Clusters = nil
		if line.Start < r.Start {
			line.Length -= r.Start - line.Start
			line.Start = r.Start
		}
		line.Length = max(0, min(line.Length, end-line.Start))
		page.lines = append(page.lines, line)
	}
	for _, c := range clusters {
		c.Start += r.Start - len(prefix)
		if c.Start < r.Start || c.Start >= end || c.LineIndex >= validLines {
			continue
		}
		page.clusters = append(page.clusters, c)
	}
	if page.height <= 0 {
		page.height = max(1, lines[0].Height)
	}
	p.lineHeight = max(1, lines[0].Height)
	if index == 0 && len(p.pages) == 0 {
		if len(page.clusters) > 0 {
			advance := page.clusters[0].Width
			uniform := advance > 0
			for _, c := range page.clusters {
				a := c.Start - r.Start
				b := a + c.Length
				if c.Width != advance || a < 0 || b > len(text) || utf8.RuneCountInString(text[a:b]) != 1 {
					uniform = false
					break
				}
			}
			if uniform {
				p.runeAdvance = advance
			}
		}
		p.blockHeight = max(p.lineHeight, page.height*float32(paragraphBlockBytes)/float32(max(1, end-r.Start)))
		p.blockWidth = max(1, w*float32(paragraphBlockBytes)/float32(max(1, end-r.Start)))
		p.heights.Reset(p.blockCount(), p.blockHeight)
	}
	if next := p.pages[index+1]; next != nil && next.range_.Start != end {
		delete(p.pages, index+1)
		next.layout.Destroy()
	}
	p.starts[index+1] = end
	p.pages[index] = page
	p.heights.Set(index, page.height)
	p.version++
	p.trim(index)
	return page
}

func (p *ParagraphLayout) alignStart(at int) (int, bool) {
	r, text, err := p.readRange(max(0, at-paragraphBlockBytes), min(p.length, at+utf8.UTFMax))
	if err != nil {
		p.err = err
		return 0, false
	}
	anchor := -1
	previous := rune(-1)
	if r.Start == 0 {
		anchor = 0
	}
	for i, current := range text {
		if r.Start+i > at {
			break
		}
		if previous >= 0 && previous < 128 && (current < 128 && !(previous == '\r' && current == '\n') || current >= 0x4e00 && current <= 0x9fff) {
			anchor = i
		}
		previous = current
	}
	if anchor < 0 {
		return 0, false
	}
	g := uniseg.NewGraphemes(text[anchor:])
	for g.Next() {
		a, b := g.Positions()
		if r.Start+anchor+b > at {
			return r.Start + anchor + a, true
		}
	}
	return at, true
}

func (p *ParagraphLayout) trim(keep int) {
	for len(p.pages) > paragraphPageLimit {
		oldest := -1
		clock := ^uint64(0)
		for index, page := range p.pages {
			if index != keep && !containsParagraphIndex(p.active, index) && page.used < clock {
				oldest = index
				clock = page.used
			}
		}
		if oldest < 0 {
			break
		}
		page := p.pages[oldest]
		delete(p.pages, oldest)
		page.layout.Destroy()
	}
}

func containsParagraphIndex(indices []int, index int) bool {
	for _, n := range indices {
		if n == index {
			return true
		}
	}
	return false
}

func (p *ParagraphLayout) origin(page *paragraphPage) geometry.Point {
	if p.format.WrapMode == typography.WrapNone {
		if p.runeAdvance > 0 {
			return geometry.Point{X: float32(page.runesBefore) * p.runeAdvance}
		}
		return geometry.Point{X: float32(page.index) * p.blockWidth}
	}
	return geometry.Point{Y: p.heights.Top(page.index)}
}

func (p *ParagraphLayout) snapshot(indices []int) textedit.Geometry {
	var lines []typography.TextLine
	var clusters []typography.TextCluster
	sort.Ints(indices)
	for _, index := range indices {
		page := p.pages[index]
		if page == nil {
			continue
		}
		origin := p.origin(page)
		first := len(lines)
		for _, line := range page.lines {
			line.X += origin.X
			line.Y += origin.Y
			line.Baseline += origin.Y
			lines = append(lines, line)
		}
		for _, c := range page.clusters {
			c.X += origin.X
			c.Y += origin.Y
			c.LineIndex += first
			clusters = append(clusters, c)
		}
	}
	return textedit.NewGeometry(lines, clusters, p.length, p.lineHeight)
}

// Prepare prepares at most one new native window for the latest viewport. It
// returns false while context or additional viewport windows remain pending.
// Call again on a later event-loop turn. Cached work never shapes text.
func (p *ParagraphLayout) Prepare(viewport geometry.Rectangle, measure bool) bool {
	p.viewport = viewport
	if len(p.pages) == 0 && measure {
		if p.page(0, time.Now().Add(2*time.Millisecond)) == nil {
			return false
		}
		measure = false
	}
	first, last := p.heights.At(max(0, viewport.Y-p.lineHeight)), p.heights.At(max(0, viewport.Y+viewport.Height+p.lineHeight))
	if p.format.WrapMode == typography.WrapNone {
		first = int(max(0, viewport.X) / p.blockWidth)
		last = int(max(0, viewport.X+viewport.Width) / p.blockWidth)
	}
	first = max(0, min(first, p.blockCount()-1))
	last = max(first, min(last, p.blockCount()-1))
	p.active = p.active[:0]
	ready := true
	deadline := time.Now().Add(2 * time.Millisecond)
	for i := first; i <= last; i++ {
		page := p.pages[i]
		if page == nil && measure {
			page = p.page(i, deadline)
			measure = false
		}
		if page == nil {
			ready = false
			continue
		}
		p.clock++
		page.used = p.clock
		p.active = append(p.active, i)
	}
	if p.geometryVersion != p.version || !slices.Equal(p.geometryIndices, p.active) {
		p.geometry = p.snapshot(append([]int(nil), p.active...))
		p.geometryIndices = append(p.geometryIndices[:0], p.active...)
		p.geometryVersion = p.version
	}
	return ready
}

// Geometry returns the measured viewport snapshot. Preparing a different
// viewport replaces it; callers must not retain its pointer across Prepare.
func (p *ParagraphLayout) Geometry() *textedit.Geometry { return &p.geometry }

// VisibleRange returns the measured viewport's paragraph-local byte interval.
func (p *ParagraphLayout) VisibleRange() textedit.Range {
	r := textedit.Range{Start: p.length}
	for _, i := range p.active {
		page := p.pages[i]
		r.Start = min(r.Start, page.range_.Start)
		r.End = max(r.End, page.range_.End)
	}
	if r.End < r.Start {
		return textedit.Range{}
	}
	return r
}

// GeometryAt prepares a bounded input window for explicit editing. A nil result
// means the preceding text context still needs event-loop turns; it never
// falls back to whole-paragraph measurement. It does not change the viewport.
func (p *ParagraphLayout) GeometryAt(offset int) *textedit.Geometry {
	offset = max(0, min(offset, p.length))
	deadline := time.Now().Add(2 * time.Millisecond)
	index := min(p.blockCount()-1, offset/paragraphBlockBytes)
	page := p.page(index, deadline)
	if page == nil {
		return nil
	}
	if offset > page.range_.End || offset == page.range_.End && offset < p.length {
		index++
		page = p.page(index, deadline)
		if page == nil {
			return nil
		}
	}
	if offset < page.range_.Start || offset > page.range_.End {
		return nil
	}
	indices := []int{page.index}
	if offset-page.range_.Start < 64 && page.index > 0 {
		if prev := p.pages[page.index-1]; prev != nil {
			indices = append(indices, prev.index)
		}
	}
	if page.range_.End-offset < 64 && page.index+1 < p.blockCount() {
		if next := p.page(page.index+1, deadline); next != nil {
			indices = append(indices, next.index)
		}
	}
	if len(indices) == 1 {
		return p.pageGeometry(page)
	}
	sort.Ints(indices)
	if p.editGeometry == nil || p.editVersion != p.version || !slices.Equal(p.editIndices, indices) {
		g := p.snapshot(indices)
		p.editGeometry = &g
		p.editVersion = p.version
		p.editIndices = append(p.editIndices[:0], indices...)
	}
	return p.editGeometry
}

// CachedOffset reports whether geometry for offset is already available.
func (p *ParagraphLayout) CachedOffset(offset int) bool {
	for _, i := range p.active {
		r := p.pages[i].range_
		if offset >= r.Start && offset <= r.End {
			return true
		}
	}
	return false
}

// CachedGeometryAt reads an existing editing window without allocating native
// resources. It may be outside the prepared painting viewport.
func (p *ParagraphLayout) CachedGeometryAt(offset int) *textedit.Geometry {
	for i, page := range p.pages {
		if offset >= page.range_.Start && offset <= page.range_.End {
			return p.pageGeometry(p.pages[i])
		}
	}
	return nil
}

func (p *ParagraphLayout) pageGeometry(page *paragraphPage) *textedit.Geometry {
	origin := p.origin(page)
	if page.geometry == nil || page.geometryOrigin != origin {
		g := p.snapshot([]int{page.index})
		page.geometry = &g
		page.geometryOrigin = origin
	}
	return page.geometry
}

// SetTextColor applies a paragraph-local color range to retained native windows.
// Coloring does not read or materialize the source paragraph.
func (p *ParagraphLayout) SetTextColor(r textedit.Range, c color.Color) {
	for _, page := range p.pages {
		a, b := max(r.Start, page.range_.Start), min(r.End, page.range_.End)
		if a < b {
			version := p.version
			page.layout.SetTextColor(page.prefix+a-page.range_.Start, b-a, c)
			if p.destroyed || version != p.version {
				return
			}
		}
	}
}

// Paint draws only prepared viewport windows and clips their uncommitted tails.
func (p *ParagraphLayout) Paint(draw Painter, origin geometry.Point) {
	for _, index := range p.active {
		page := p.pages[index]
		at := p.origin(page)
		at.X += origin.X
		at.Y += origin.Y
		draw.Save()
		draw.SetClipRect(geometry.Rect(at.X, at.Y, max(p.width, page.width), page.height))
		draw.DrawTextLayout(at, page.layout)
		draw.Restore()
	}
}

// SetColor changes the base foreground without invalidating measured windows.
func (p *ParagraphLayout) SetColor(c color.Color) {
	p.format.TextColor = c
	p.SetTextColor(textedit.Range{End: p.length}, c)
}

// Text reads only the requested paragraph-local source range.
func (p *ParagraphLayout) Text(r textedit.Range) string {
	_, text, err := p.readRange(r.Start, r.End)
	if err != nil {
		p.err = err
	}
	return text
}
