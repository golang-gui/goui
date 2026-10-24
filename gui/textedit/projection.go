package textedit

import (
	"fmt"
	"sort"
	"strings"
)

// Projection is a view-local range projection, not an edit in Model. Surviving
// model prefixes and suffixes remain range references. Short inputs may also
// retain a bounded materialization; long lines are read through Slice.
type Projection struct {
	selection                  Selection
	replacement                Range
	first, last                int    // affected model paragraph indexes, inclusive
	origin                     int    // byte offset of the first affected paragraph
	prefix, suffix             string // bounded materialization for short projected paragraphs
	text, display              string
	prefixLength, suffixLength int
	caret                      int     // byte offset within normalized preedit text
	lines                      []Range // paragraph ranges within display, excluding LF
}

// NewProjection hides a valid model selection with initially empty preedit.
// All subsequent queries must use the same, unchanged model. Discard the
// projection when the model changes; it does not subscribe to model signals.
func NewProjection(model *Model, selection Selection) *Projection {
	rng := selection.Range()
	first, last := model.LineAt(rng.Start), model.LineAt(rng.End)
	firstRange, _ := model.LineRange(first)
	lastRange, _ := model.LineRange(last)
	p := &Projection{selection: selection, replacement: rng, first: first, last: last,
		origin: firstRange.Start, prefixLength: rng.Start - firstRange.Start, suffixLength: lastRange.End - rng.End}
	if p.prefixLength+p.suffixLength <= 8192 {
		p.prefix, _ = model.Slice(Range{firstRange.Start, rng.Start})
		p.suffix, _ = model.Slice(Range{rng.End, lastRange.End})
	}
	p.Update("", 0)
	return p
}

// Update replaces preedit text. text must be normalized UTF-8 with LF line
// breaks; caret is a byte offset in that text and is clamped to a rune boundary.
func (p *Projection) Update(text string, caret int) {
	p.text, p.caret = text, ClampOffset(text, caret)
	p.display = ""
	if len(p.prefix)+len(p.suffix) == p.prefixLength+p.suffixLength {
		p.display = p.prefix + text + p.suffix
	}
	p.lines = p.lines[:0]
	start := 0
	for i := 0; i < len(text); i++ {
		if text[i] == '\n' {
			p.lines = append(p.lines, Range{start, p.prefixLength + i})
			start = p.prefixLength + i + 1
		}
	}
	p.lines = append(p.lines, Range{start, p.prefixLength + len(text) + p.suffixLength})
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
	text, _ := p.Slice(model, p.LineRange(model, index))
	return text
}

// Slice reads a bounded range in projected document coordinates. The unchanged
// model's prefix and suffix are referenced rather than copied during Update.
func (p *Projection) Slice(model *Model, rng Range) (string, error) {
	length := model.Len() + p.byteDelta()
	if rng.Start < 0 || rng.End < rng.Start || rng.End > length {
		return "", fmt.Errorf("text projection: invalid range %v", rng)
	}
	for _, at := range []int{rng.Start, rng.End} {
		if at > p.replacement.Start && at < p.replacement.Start+len(p.text) {
			if ClampOffset(p.text, at-p.replacement.Start) != at-p.replacement.Start {
				return "", fmt.Errorf("text projection: range splits UTF-8")
			}
		} else if !model.IsRuneBoundary(p.ModelOffset(at, false)) {
			return "", fmt.Errorf("text projection: range splits UTF-8")
		}
	}
	start, end := p.replacement.Start, p.replacement.Start+len(p.text)
	var out strings.Builder
	out.Grow(rng.End - rng.Start)
	if rng.Start < start {
		s, err := model.Slice(Range{rng.Start, min(rng.End, start)})
		if err != nil {
			return "", err
		}
		out.WriteString(s)
	}
	if a, b := max(rng.Start, start), min(rng.End, end); a < b {
		a -= start
		b -= start
		if ClampOffset(p.text, a) != a || ClampOffset(p.text, b) != b {
			return "", fmt.Errorf("text projection: range splits UTF-8")
		}
		out.WriteString(p.text[a:b])
	}
	if rng.End > end {
		a, b := max(rng.Start, end)-p.byteDelta(), rng.End-p.byteDelta()
		s, err := model.Slice(Range{a, b})
		if err != nil {
			return "", err
		}
		out.WriteString(s)
	}
	return out.String(), nil
}

// LineAt finds the display paragraph containing offset.
func (p *Projection) LineAt(model *Model, offset int) int {
	if offset >= p.origin {
		if offset <= p.origin+p.prefixLength+len(p.text)+p.suffixLength {
			line := sort.Search(len(p.lines), func(i int) bool { return p.lines[i].Start > offset-p.origin }) - 1
			return p.first + max(0, line)
		}
		return model.LineAt(offset-p.byteDelta()) + p.lineDelta()
	}
	return model.LineAt(offset)
}
