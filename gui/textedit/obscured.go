package textedit

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/golang-gui/goui/platform/typography"
	"github.com/rivo/uniseg"
)

// ValidObscureChar reports whether r is a standalone graphic rune suitable
// for repeated masking. Zero requests the default U+2022 bullet.
func ValidObscureChar(r rune) bool {
	return r == 0 || utf8.ValidRune(r) && unicode.IsGraphic(r) && !unicode.IsSpace(r) && !unicode.Is(unicode.M, r)
}

// ObscuredText owns display text and source byte boundaries, not the source
// string. Each extended grapheme becomes one mask, except an optional reveal.
type ObscuredText struct {
	text       string
	boundaries []int
	charBytes  int
	revealed   Range // display byte range; empty when fully obscured
}

// NewObscuredText maps normalized UTF-8 text to masking text. Invalid masking
// runes panic; zero uses U+2022. The source string is not retained.
func NewObscuredText(text string, char rune) *ObscuredText {
	return NewRevealedText(text, char, Range{})
}

// NewRevealedText obscures text except the single complete grapheme containing
// reveal. An empty/invalid range, or one spanning graphemes, reveals nothing.
// The display string copies that grapheme; it never retains the source string.
func NewRevealedText(text string, char rune, reveal Range) *ObscuredText {
	if !ValidObscureChar(char) {
		panic("text: invalid obscuring character")
	}
	if char == 0 {
		char = '•'
	}
	m := &ObscuredText{boundaries: []int{0}, charBytes: utf8.RuneLen(char)}
	var display strings.Builder
	g := uniseg.NewGraphemes(text)
	for g.Next() {
		start, end := g.Positions()
		if reveal.Start >= start && reveal.End <= end && reveal.Start < reveal.End {
			m.revealed.Start = display.Len()
			display.WriteString(text[start:end])
			m.revealed.End = display.Len()
		} else {
			display.WriteRune(char)
		}
		m.boundaries = append(m.boundaries, end)
	}
	m.text = display.String()
	return m
}

func (m *ObscuredText) Text() string      { return m.text }
func (m *ObscuredText) SourceLength() int { return m.boundaries[len(m.boundaries)-1] }

func (m *ObscuredText) sourceOffset(offset int, trailing bool) int {
	if r := m.revealed; r.Start < r.End {
		switch {
		case offset >= r.End:
			offset -= r.End - r.Start - m.charBytes
		case offset > r.Start:
			offset = r.Start
			if trailing {
				offset += m.charBytes
			}
		}
	}
	return m.boundaries[min(max(0, offset/m.charBytes), len(m.boundaries)-1)]
}

// Metrics copies native masking metrics into source byte coordinates. Geometry
// continues using real selection offsets, while native layouts keep only masks.
// Input slices remain owned by the native layout and are never modified.
func (m *ObscuredText) Metrics(lines []typography.TextLine, clusters []typography.TextCluster) ([]typography.TextLine, []typography.TextCluster) {
	mappedClusters := func(values []typography.TextCluster) []typography.TextCluster {
		out := make([]typography.TextCluster, 0, len(values))
		for _, value := range values {
			end := m.sourceOffset(value.Start+value.Length, true)
			value.Start = m.sourceOffset(value.Start, false)
			value.Length = end - value.Start
			// Font fallback may split a revealed emoji/combining sequence into
			// several native clusters. Keep one source grapheme and its bounds.
			if len(out) > 0 {
				last := &out[len(out)-1]
				if last.Start == value.Start && last.Length == value.Length && last.LineIndex == value.LineIndex {
					right, bottom := max(last.X+last.Width, value.X+value.Width), max(last.Y+last.Height, value.Y+value.Height)
					last.X, last.Y = min(last.X, value.X), min(last.Y, value.Y)
					last.Width, last.Height = right-last.X, bottom-last.Y
					continue
				}
			}
			out = append(out, value)
		}
		return out
	}
	out := make([]typography.TextLine, len(lines))
	for i, value := range lines {
		out[i] = value
		out[i].Start = m.sourceOffset(value.Start, false)
		out[i].Length = m.sourceOffset(value.Start+value.Length, true) - out[i].Start
		out[i].Clusters = mappedClusters(value.Clusters)
	}
	return out, mappedClusters(clusters)
}
