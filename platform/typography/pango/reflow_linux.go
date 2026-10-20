package pango

import (
	"fmt"
	"image"
	"math"
	"slices"
	"sort"

	"github.com/goexlib/cgo"
	"github.com/golang-gui/goui/platform/linux/libs/cairo"
	native "github.com/golang-gui/goui/platform/linux/libs/pango"
	"github.com/golang-gui/goui/platform/linux/libs/pango_cairo"
	"github.com/golang-gui/goui/platform/typography"
)

// Retained analysis uses the complete paragraph. Display lines are reshaped
// with its full text and resolved levels, never independently itemized.
type retainedParagraph struct {
	text                   string
	ptr                    *byte
	positions              []int
	attrs                  []native.LogAttr
	widths                 []int32
	runs                   []*native.GlyphItem
	tab                    int64
	margin                 int64
	segments               []retainedSegment
	advances               []int64
	lineBreaks, charBreaks []int
	defaultTabs            bool
	paintAttrs             native.AttrList
	color                  [4]float64
}

type retainedSegment struct {
	start, end, next int
	direction        native.Direction
	height, baseline float32
}

func retainParagraph(l *TextLayout) (*retainedParagraph, error) {
	m := &retainedParagraph{text: l.text, ptr: (*byte)(cgo.CString(l.text)), tab: int64(math.Round(float64(l.format.TabWidth * native.Scale)))}
	analysis := l.layout.Copy()
	defer analysis.Unref()
	analysis.SetWidth(-1)
	shaping := l.attrs.Copy()
	defer shaping.Unref()
	for _, kind := range []native.AttrType{native.AttrForeground, native.AttrForegroundAlpha, native.AttrUnderline, native.AttrStrikethrough} {
		shaping.RemoveType(kind)
	}
	analysis.SetAttributes(shaping)
	m.paintAttrs = l.attrs.Copy()
	r, g, b, a := toColor(l.format.TextColor)
	m.color = [4]float64{r, g, b, a}
	if l.format.TabWidth > 0 && m.tab == 0 {
		m.tab = 1
	}
	if m.tab == 0 {
		desc, err := createFontDescription(l.format.Font)
		if err != nil {
			m.destroy()
			return nil, err
		}
		aux := native.LayoutNew(l.context)
		aux.SetFontDescription(desc)
		desc.Free()
		aux.SetText("\t")
		aux.SetAttributes(l.attrs)
		_, rect := aux.GetExtents()
		m.tab = int64(rect.Width)
		aux.Unref()
		m.defaultTabs = true
	}
	for i := range l.text {
		m.positions = append(m.positions, i)
	}
	m.positions = append(m.positions, len(l.text))
	m.widths = make([]int32, len(m.positions)-1)
	m.attrs = slices.Clone(analysis.LogAttrs())
	for i, a := range m.attrs {
		if a.LineBreak() {
			m.lineBreaks = append(m.lineBreaks, i)
		}
		if a.CharBreak() {
			m.charBreaks = append(m.charBreaks, i)
		}
	}
	if len(m.attrs) != len(m.positions) {
		m.destroy()
		return nil, fmt.Errorf("logical attribute count differs")
	}
	for row := 0; row < analysis.GetLineCount(); row++ {
		line := analysis.GetLineReadonly(row)
		start, _ := slices.BinarySearch(m.positions, int(line.StartIndex))
		end, _ := slices.BinarySearch(m.positions, int(line.StartIndex+line.Length))
		next := len(m.widths)
		if row+1 < analysis.GetLineCount() {
			next, _ = slices.BinarySearch(m.positions, int(analysis.GetLineReadonly(row+1).StartIndex))
		}
		_, box := line.GetExtents()
		m.segments = append(m.segments, retainedSegment{start: start, end: end, next: next, direction: line.ResolvedDirection(), height: float32(box.Height) / native.Scale, baseline: -float32(box.Y) / native.Scale})
		for _, run := range line.GetRuns() {
			copy := run.Copy()
			if copy == nil {
				m.destroy()
				return nil, fmt.Errorf("copy glyph item failed")
			}
			m.runs = append(m.runs, copy)
			metrics := native.Font(copy.Item.Analysis.Font).GetMetrics(copy.Item.Analysis.Language)
			m.margin = max(m.margin, 3*int64(metrics.ApproximateCharWidth()))
			metrics.Unref()
			at, _ := slices.BinarySearch(m.positions, int(copy.Item.Offset))
			copy.LogicalWidths(m.ptr, m.widths[at:at+int(copy.Item.NumChars)])
		}
	}
	slices.SortFunc(m.runs, func(a, b *native.GlyphItem) int { return int(a.Item.Offset - b.Item.Offset) })
	m.advances = make([]int64, len(m.widths)+1)
	for i, w := range m.widths {
		m.advances[i+1] = m.advances[i] + int64(w)
	}
	return m, nil
}

func (m *retainedParagraph) segment(start int) retainedSegment {
	i := sort.Search(len(m.segments), func(i int) bool { return m.segments[i].next > start })
	if i == len(m.segments) {
		i--
	}
	return m.segments[i]
}

func (m *retainedParagraph) wrapped(start, end int) bool { return end < m.segment(start).end }

func (m *retainedParagraph) destroy() {
	for _, run := range m.runs {
		run.Free()
	}
	if m.ptr != nil {
		cgo.Free(cgo.Pointer(m.ptr))
	}
	if m.paintAttrs != 0 {
		m.paintAttrs.Unref()
		m.paintAttrs = 0
	}
	m.runs, m.ptr = nil, nil
}

type retainedRun struct {
	item   native.Item
	glyphs *native.GlyphString
	owned  bool
	source *native.Item
	owner  *native.GlyphItem
}

func freeRetainedRuns(runs []retainedRun) {
	for _, r := range runs {
		if r.owner != nil {
			r.owner.Free()
		} else if r.owned {
			r.glyphs.Free()
		}
	}
}

func (m *retainedParagraph) shape(start, end int, wrapped bool, boxWidth int64) (runs []retainedRun) {
	var total int64
	first, last := m.positions[start], m.positions[end]
	index := sort.Search(len(m.runs), func(i int) bool { return int(m.runs[i].Item.Offset+m.runs[i].Item.Length) > first })
	if index == len(m.runs) {
		return nil
	}
	for _, run := range m.runs[index:] {
		if int(run.Item.Offset) >= last {
			break
		}
		a, b := max(first, int(run.Item.Offset)), min(last, int(run.Item.Offset+run.Item.Length))
		if a >= b {
			continue
		}
		runeStart, _ := slices.BinarySearch(m.positions, a)
		runeEnd, _ := slices.BinarySearch(m.positions, b)
		item := *run.Item
		item.Offset, item.Length, item.NumChars = int32(a), int32(b-a), int32(runeEnd-runeStart)
		item.Analysis.Flags &^= 1 << 2
		if wrapped && runeEnd == end && m.attrs[end].InsertsHyphen() {
			item.Analysis.Flags |= 1 << 2
		}
		var glyphs *native.GlyphString
		owned := true
		if m.text[a] == '\t' && b == a+1 && m.tab > 0 {
			glyphs = run.Glyphs.Copy()
			gs := glyphs.GlyphInfos()
			if len(gs) != 1 {
				panic("tab must have one native glyph")
			}
			at := min(total, boxWidth)
			gap := int64(1)
			if m.defaultTabs {
				gap = max(1, m.tab/8)
			}
			position := ((at + gap + m.tab - 1) / m.tab) * m.tab
			gs[0].Geometry.Width = int32(position - at)
		} else if a == int(run.Item.Offset) && b == int(run.Item.Offset+run.Item.Length) && item.Analysis.Flags == run.Item.Analysis.Flags {
			if wrapped && runeEnd == end && m.attrs[end-1].White() {
				glyphs = run.Glyphs.Copy()
			} else {
				glyphs = run.Glyphs
				owned = false
			}
		} else {
			glyphs = native.GlyphStringNew()
			native.ShapeItem(&item, m.ptr, len(m.text), &m.attrs[runeStart], glyphs, 1)
		}
		if wrapped && runeEnd == end && m.attrs[end-1].White() {
			gs := glyphs.GlyphInfos()
			if len(gs) > 0 {
				index := len(gs) - 1
				if item.Analysis.Level&1 != 0 {
					index = 0
				}
				logs := glyphs.LogClusters()
				adjacent := index - 1
				if index == 0 {
					adjacent = 1
				}
				if adjacent < 0 || adjacent >= len(gs) || logs[index] != logs[adjacent] {
					gs[index].Geometry.Width = 0
					gs[index].Glyph = 0x0fffffff
				}
			}
		}
		total += int64(glyphs.Width())
		runs = append(runs, retainedRun{item: item, glyphs: glyphs, owned: owned, source: run.Item})
	}
	return runs
}

func (m *retainedParagraph) decorate(runs []retainedRun) []retainedRun {
	var out []retainedRun
	for _, run := range runs {
		item := run.source.Copy()
		a, _ := slices.BinarySearch(m.positions, int(item.Offset))
		b, _ := slices.BinarySearch(m.positions, int(run.item.Offset))
		if run.item.Offset > item.Offset {
			item.Split(int(run.item.Offset-item.Offset), b-a).Free()
		}
		if run.item.Length < item.Length {
			prefix := item.Split(int(run.item.Length), int(run.item.NumChars))
			item.Free()
			item = prefix
		}
		item.Analysis.Flags = run.item.Analysis.Flags
		glyphs := run.glyphs
		if !run.owned {
			glyphs = glyphs.Copy()
		}
		g := native.GlyphItemNew(item, glyphs)
		list := g.ApplyAttributes(m.ptr, m.paintAttrs)
		for p := list; p != nil; p = p.Next {
			out = append(out, retainedRun{item: *p.Data.Item, glyphs: p.Data.Glyphs, owner: p.Data})
		}
		list.Free()
	}
	return out
}

func (r *retainedReflow) measure(start, end int, wrapped bool) int64 {
	m := r.paragraph
	runs := m.shape(start, end, wrapped, r.width)
	defer freeRetainedRuns(runs)
	var w int64
	for _, run := range runs {
		w += int64(run.glyphs.Width())
	}
	return w
}

func visualRetainedRuns(runs []retainedRun) {
	maxLevel, minOdd := uint8(0), uint8(255)
	for _, r := range runs {
		level := r.item.Analysis.Level
		maxLevel = max(maxLevel, level)
		if level&1 != 0 {
			minOdd = min(minOdd, level)
		}
	}
	if minOdd == 255 {
		return
	}
	for level := int(maxLevel); level >= int(minOdd); level-- {
		for at := 0; at < len(runs); {
			if int(runs[at].item.Analysis.Level) < level {
				at++
				continue
			}
			end := at + 1
			for end < len(runs) && int(runs[end].item.Analysis.Level) >= level {
				end++
			}
			slices.Reverse(runs[at:end])
			at = end
		}
	}
}

type retainedReflow struct {
	paragraph             *retainedParagraph
	width                 int64
	wrap                  typography.WrapMode
	next                  int
	lines                 []typography.TextLine
	clusters              []typography.TextCluster
	resources             map[int][]retainedRun
	sizeWidth, sizeHeight float32
	minX, maxX            float32
}

func (r *retainedReflow) destroy() {
	for _, runs := range r.resources {
		freeRetainedRuns(runs)
	}
	r.resources = nil
}

func (r *retainedReflow) clearPages() {
	for _, runs := range r.resources {
		freeRetainedRuns(runs)
	}
	r.resources = nil
}

func (r *retainedReflow) lineRuns(index int) []retainedRun {
	if runs, ok := r.resources[index]; ok {
		return runs
	}
	line := r.lines[index]
	m := r.paragraph
	start, _ := slices.BinarySearch(m.positions, line.Start)
	end, _ := slices.BinarySearch(m.positions, line.Start+line.Length)
	runs := m.decorate(m.shape(start, end, m.wrapped(start, end), r.width))
	visualRetainedRuns(runs)
	if r.resources == nil {
		r.resources = make(map[int][]retainedRun)
	}
	r.resources[index] = runs
	return runs
}

func (r *retainedReflow) rasterize(scale float32, region image.Rectangle) (typography.TextBitmap, error) {
	var painter textPainter
	if err := painter.Init(region.Dx(), region.Dy()); err != nil {
		return typography.TextBitmap{}, err
	}
	defer painter.Destroy()
	cr := painter.context
	cr.Translate(-float64(region.Min.X), -float64(region.Min.Y))
	cr.Scale(float64(scale), float64(scale))
	low, high := float32(region.Min.Y)/scale, float32(region.Max.Y)/scale
	overscan := high - low
	first := sort.Search(len(r.lines), func(i int) bool { return r.lines[i].Y+r.lines[i].Height > low-overscan })
	last := sort.Search(len(r.lines), func(i int) bool { return r.lines[i].Y >= high+overscan })
	for i, runs := range r.resources {
		if i < first || i >= last {
			freeRetainedRuns(runs)
			delete(r.resources, i)
		}
	}
	for i := first; i < last; i++ {
		line := r.lines[i]
		if line.Y+line.Height <= low || line.Y >= high {
			continue
		}
		x := line.X
		for _, run := range r.lineRuns(i) {
			color := r.paragraph.color
			for attrs := run.item.Analysis.ExtraAttributes(); attrs != nil; attrs = attrs.Next {
				switch attrs.Data.Type() {
				case native.AttrForeground:
					c := attrs.Data.ColorValue()
					color[0], color[1], color[2] = float64(c.Red)/65535, float64(c.Green)/65535, float64(c.Blue)/65535
				case native.AttrForegroundAlpha:
					color[3] = float64(attrs.Data.IntValue()) / 65535
				}
			}
			cr.SetSourceRGBA(color[0], color[1], color[2], color[3])
			cr.MoveTo(float64(x), float64(line.Baseline))
			drawRetainedGlyphs(cr, float64(x), float64(line.Baseline), run)
			x += float32(run.glyphs.Width()) / native.Scale
		}
	}
	if status := cr.Status(); status != 0 {
		return typography.TextBitmap{}, fmt.Errorf("prepared glyph draw: %v", status)
	}
	return painter.GetBitmap(region.Dx(), region.Dy(), nil), nil
}

func (r *retainedReflow) finishLine(start, end int) typography.TextLine {
	m := r.paragraph
	runs := m.shape(start, end, m.wrapped(start, end), r.width)
	defer freeRetainedRuns(runs)
	visualRetainedRuns(runs)
	line := typography.TextLine{Start: m.positions[start], Length: m.positions[end] - m.positions[start]}
	if len(r.lines) > 0 {
		last := r.lines[len(r.lines)-1]
		line.Y = last.Y + last.Height
	}
	var minY, maxY int32
	for _, run := range runs {
		_, box := run.glyphs.Extents(native.Font(run.item.Analysis.Font))
		minY = min(minY, box.Y)
		maxY = max(maxY, box.Y+box.Height)
		line.Width += float32(run.glyphs.Width()) / native.Scale
	}
	line.Height = float32(maxY-minY) / native.Scale
	line.Baseline = line.Y - float32(minY)/native.Scale
	if len(runs) == 0 {
		line.Height = m.segment(start).height
		line.Baseline = line.Y + m.segment(start).baseline
	}
	if m.segment(start).direction == native.DirectionRTL {
		line.X = float32(r.width)/native.Scale - line.Width
	}
	x := line.X
	beg := len(r.clusters)
	for _, run := range runs {
		glyphs, logs := run.glyphs.GlyphInfos(), run.glyphs.LogClusters()
		for i := 0; i < len(glyphs); {
			end := i + 1
			width := glyphs[i].Geometry.Width
			for end < len(glyphs) && logs[end] == logs[i] {
				width += glyphs[end].Geometry.Width
				end++
			}
			r.clusters = append(r.clusters, typography.TextCluster{Start: int(run.item.Offset) + int(logs[i]), X: x, Y: line.Y, Width: float32(width) / native.Scale, Height: line.Height, LineIndex: len(r.lines), Direction: typography.TextDirection(run.item.Analysis.Level & 1)})
			x += float32(width) / native.Scale
			i = end
		}
	}
	line.Clusters = r.clusters[beg:]
	slices.SortFunc(line.Clusters, func(a, b typography.TextCluster) int { return a.Start - b.Start })
	for i := range line.Clusters {
		end := line.Start + line.Length
		if i+1 < len(line.Clusters) {
			end = line.Clusters[i+1].Start
		}
		line.Clusters[i].Length = end - line.Clusters[i].Start
	}
	if len(r.lines) == 0 {
		r.minX, r.maxX = line.X, line.X+line.Width
	} else {
		r.minX = min(r.minX, line.X)
		r.maxX = max(r.maxX, line.X+line.Width)
	}
	r.sizeWidth = r.maxX - r.minX
	r.sizeHeight = line.Y + line.Height
	return line
}

// fitItem chooses within one native analysis item. A completed item's glyph
// width consumes the remaining box before the following item is considered.
func (r *retainedReflow) fitItem(lineStart, from, to int, remaining int64, force, noEnd bool) (int, bool) {
	m := r.paragraph
	prefix := int64(0)
	if from > lineStart {
		prefix = r.measure(lineStart, from, false)
	}
	full := m.advances[to] - m.advances[from]
	atEndBreak := m.attrs[to].LineBreak()
	if r.wrap == typography.WrapChar {
		atEndBreak = m.attrs[to].CharBreak()
	}
	decision := full
	if atEndBreak && full <= remaining+m.margin {
		decision = r.measure(lineStart, to, true) - prefix
	}
	if !noEnd && (decision <= remaining || to-from == 1 && from == lineStart || m.text[m.positions[from]] == '\t') {
		return to, true
	}
	choose := func(charOnly bool) (int, int64) {
		chosen, best := to, full
		limit := to
		if noEnd {
			limit--
		}
		breaks := m.lineBreaks
		if charOnly {
			breaks = m.charBreaks
		}
		index, _ := slices.BinarySearch(breaks, from)
		for _, end := range breaks[index:] {
			if end > limit {
				break
			}
			approximate := m.advances[end] - m.advances[from]
			if end > from || from > lineStart {
				w := approximate
				if end > from && m.attrs[end-1].White() {
					w -= int64(m.widths[end-1])
				}
				if end == from || w >= remaining-m.margin && !(end == to && w > remaining+m.margin) {
					w = r.measure(lineStart, end, true) - prefix
				}
				if chosen == to || w <= remaining || w < best {
					chosen, best = end, w
				}
			}
			if approximate > remaining+m.margin && chosen < to {
				break
			}
		}
		return chosen, best
	}
	end, width := choose(r.wrap == typography.WrapChar)
	if force && r.wrap == typography.WrapWordChar && width > remaining {
		end, width = choose(true)
	}
	if !force && width > remaining {
		return from, false
	}
	return end, true
}

func (r *retainedReflow) advance(lineLimit int) bool {
	m := r.paragraph
	for range lineLimit {
		start := r.next
		if start >= len(m.widths) {
			if len(r.lines) == 0 || m.segments[len(m.segments)-1].start == start && r.lines[len(r.lines)-1].Start != m.positions[start] {
				r.lines = append(r.lines, r.finishLine(start, start))
			}
			return true
		}
		segment := m.segment(start)
		if segment.end == start {
			r.lines = append(r.lines, r.finishLine(start, start))
			r.next = segment.next
			continue
		}
		at, end := start, start
		remaining := r.width
		rollback, rollbackEnd, rollbackRemaining := -1, 0, int64(0)
		for at < segment.end {
			pos := m.positions[at]
			itemIndex := sort.Search(len(m.runs), func(i int) bool { return int(m.runs[i].Item.Offset+m.runs[i].Item.Length) > pos })
			if itemIndex == len(m.runs) {
				end = segment.end
				break
			}
			item := m.runs[itemIndex]
			itemEnd, _ := slices.BinarySearch(m.positions, int(item.Item.Offset+item.Item.Length))
			itemEnd = min(itemEnd, segment.end)
			next, ok := r.fitItem(start, at, itemEnd, remaining, rollback < 0, false)
			if !ok {
				if rollback < 0 {
					panic("missing forced native break")
				}
				end, _ = r.fitItem(start, rollback, rollbackEnd, rollbackRemaining, true, true)
				break
			}
			end = next
			if next < itemEnd {
				break
			}
			if m.text[m.positions[at]] != '\t' {
				for offset := at; offset < itemEnd; offset++ {
					allowed := m.attrs[offset].LineBreak()
					if r.wrap == typography.WrapChar {
						allowed = m.attrs[offset].CharBreak()
					}
					if allowed && (offset > at || at > start) {
						rollback, rollbackEnd, rollbackRemaining = at, itemEnd, remaining
						break
					}
				}
			}
			before := int64(0)
			if at > start {
				before = r.measure(start, at, false)
			}
			consumed := r.measure(start, itemEnd, false) - before
			remaining = max(0, remaining-consumed)
			at = itemEnd
		}
		if end <= start {
			panic("retained reflow made no progress")
		}
		r.lines = append(r.lines, r.finishLine(start, end))
		r.next = end
		if end == segment.end {
			r.next = segment.next
		}
	}
	return r.next == len(m.widths) && (m.segments[len(m.segments)-1].start != r.next || len(r.lines) > 0 && r.lines[len(r.lines)-1].Start == m.positions[r.next])
}

// Draw positioned glyphs with the font captured by Pango. Pango 1.50.6's
// show_glyph_string convenience renderer retains alpha from prior layout draws,
// so it cannot provide isolated per-layout color. Cairo's public scaled-font
// path has explicit context state. Native outlines retain missing-glyph boxes.
func drawRetainedGlyphs(cr cairo.Context, x, y float64, run retainedRun) {
	font := native.Font(run.item.Analysis.Font)
	cr.SetScaledFont(pango_cairo.FontScaledFont(font))
	var glyphs []cairo.Glyph
	unknown := false
	var advance int64
	for _, g := range run.glyphs.GlyphInfos() {
		if g.Glyph&0x80000000 != 0 {
			unknown = true
		} else if g.Glyph != 0x0fffffff {
			glyphs = append(glyphs, cairo.Glyph{Index: uintptr(g.Glyph), X: x + float64(advance+int64(g.Geometry.XOffset))/native.Scale, Y: y + float64(g.Geometry.YOffset)/native.Scale})
		}
		advance += int64(g.Geometry.Width)
	}
	cr.ShowGlyphs(glyphs)
	if unknown {
		missing := run.glyphs.Copy()
		defer missing.Free()
		infos := missing.GlyphInfos()
		for i := range infos {
			g := &infos[i]
			if g.Glyph&0x80000000 == 0 {
				g.Glyph = 0x0fffffff
			}
		}
		cr.MoveTo(x, y)
		pango_cairo.GlyphStringPath(cr, font, missing)
		cr.Fill()
	}
}
