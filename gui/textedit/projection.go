package textedit

import "sort"

// Projection is a view-local projection, not an edit in Model. Only the
// surviving ends of the affected paragraphs are copied. Selecting a megabyte
// range does not copy that range, or shape its hidden paragraphs.
type Projection struct {
	selection      Selection
	replacement    Range
	first, last    int // affected model paragraph indexes, inclusive
	origin         int // byte offset of the first affected paragraph
	prefix, suffix string
	text, display  string
	caret          int     // byte offset within normalized preedit text
	lines          []Range // paragraph ranges within display, excluding LF
}

// NewProjection hides a valid model selection with initially empty preedit.
// All subsequent queries must use the same, unchanged model. Discard the
// projection when the model changes; it does not subscribe to model signals.
func NewProjection(model *Model, selection Selection) *Projection {
	rng := selection.Range()
	first, last := model.LineAt(rng.Start), model.LineAt(rng.End)
	firstRange, _ := model.LineRange(first)
	lastRange, _ := model.LineRange(last)
	prefix, _ := model.Slice(Range{firstRange.Start, rng.Start})
	suffix, _ := model.Slice(Range{rng.End, lastRange.End})
	p := &Projection{selection: selection, replacement: rng, first: first, last: last,
		origin: firstRange.Start, prefix: prefix, suffix: suffix}
	p.Update("", 0)
	return p
}

// Update replaces preedit text. text must be normalized UTF-8 with LF line
// breaks; caret is a byte offset in that text and is clamped to a rune boundary.
func (p *Projection) Update(text string, caret int) {
	p.text, p.caret = text, ClampOffset(text, caret)
	p.display = p.prefix + text + p.suffix
	p.lines = p.lines[:0]
	start := 0
	for i := 0; i < len(p.display); i++ {
		if p.display[i] == '\n' {
			p.lines = append(p.lines, Range{start, i})
			start = i + 1
		}
	}
	p.lines = append(p.lines, Range{start, len(p.display)})
}

func (p *Projection) lineDelta() int { return len(p.lines) - (p.last - p.first + 1) }
func (p *Projection) byteDelta() int { return len(p.text) - (p.replacement.End - p.replacement.Start) }

// ModelOffset maps a display byte offset to the model. Inside preedit text it
// chooses the start or, when trailing, the end of the replaced model range.
func (p *Projection) ModelOffset(displayOffset int, trailing bool) int {
	if displayOffset <= p.replacement.Start {
		return displayOffset
	}
	if displayOffset >= p.replacement.Start+len(p.text) {
		return displayOffset - p.byteDelta()
	}
	if trailing {
		return p.replacement.End
	}
	return p.replacement.Start
}

// Selection returns the selection replaced by this temporary projection.
func (p *Projection) Selection() Selection { return p.selection }

// Replacement returns the model range hidden by preedit text.
func (p *Projection) Replacement() Range { return p.replacement }

// FirstLine and LastLine identify affected model paragraphs, inclusive.
func (p *Projection) FirstLine() int { return p.first }
func (p *Projection) LastLine() int  { return p.last }

// ParagraphCount returns the number of temporary display paragraphs.
func (p *Projection) ParagraphCount() int { return len(p.lines) }

// Text returns the uncommitted preedit text.
func (p *Projection) Text() string { return p.text }

// Caret returns a UTF-8 byte offset inside Text.
func (p *Projection) Caret() int { return p.caret }

// SetCaret moves the preedit caret without rebuilding its projection.
func (p *Projection) SetCaret(offset int) { p.caret = ClampOffset(p.text, offset) }

// LineCount returns the projected document's logical line count.
func (p *Projection) LineCount(model *Model) int {
	return model.LineCount() + p.lineDelta()
}

// LineRange returns a valid display paragraph's range, excluding LF.
func (p *Projection) LineRange(model *Model, index int) Range {
	if index >= p.first {
		if index < p.first+len(p.lines) {
			rng := p.lines[index-p.first]
			return Range{rng.Start + p.origin, rng.End + p.origin}
		}
		rng, _ := model.LineRange(index - p.lineDelta())
		return Range{rng.Start + p.byteDelta(), rng.End + p.byteDelta()}
	}
	rng, _ := model.LineRange(index)
	return rng
}

// Paragraph returns a valid display paragraph's text, excluding LF.
func (p *Projection) Paragraph(model *Model, index int) string {
	if index >= p.first {
		if index < p.first+len(p.lines) {
			rng := p.lines[index-p.first]
			return p.display[rng.Start:rng.End]
		}
		index -= p.lineDelta()
	}
	rng, _ := model.LineRange(index)
	text, _ := model.Slice(rng)
	return text
}

// LineAt finds the display paragraph containing offset.
func (p *Projection) LineAt(model *Model, offset int) int {
	if offset >= p.origin {
		if offset <= p.origin+len(p.display) {
			line := sort.Search(len(p.lines), func(i int) bool { return p.lines[i].Start > offset-p.origin }) - 1
			return p.first + max(0, line)
		}
		return model.LineAt(offset-p.byteDelta()) + p.lineDelta()
	}
	return model.LineAt(offset)
}
