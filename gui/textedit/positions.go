package textedit

import (
	"strings"
	"unicode/utf8"

	"github.com/rivo/uniseg"
)

// NormalizeSingleLine normalizes UTF-8 and replaces line breaks with spaces.
func NormalizeSingleLine(text string) string {
	return strings.ReplaceAll(Normalize(text), "\n", " ")
}

// ClampOffset clamps index to a UTF-8 rune boundary at or before it.
// text must already be valid UTF-8; document normalization is separate.
func ClampOffset(text string, index int) int {
	if index <= 0 {
		return 0
	}
	if index >= len(text) {
		return len(text)
	}
	for index > 0 && !utf8.RuneStart(text[index]) {
		index--
	}
	return index
}

// MapOffset maps a byte position through one edit. Positions in the replaced
// range, including endpoints, move to the end of the inserted text.
func MapOffset(offset int, edit Edit) int {
	if offset < edit.Range.Start {
		return offset
	}
	if offset <= edit.Range.End {
		return edit.Range.Start + len(edit.Text)
	}
	return offset + len(edit.Text) - (edit.Range.End - edit.Range.Start)
}

// WordAt returns the Unicode word segment at offset within its logical line.
// Whitespace and punctuation may be segments; this is not dictionary-based
// segmentation. Layout users must additionally snap to their Cluster boundaries.
func (m *Model) WordAt(offset int) Range {
	index := m.LineAt(offset)
	rng, _ := m.LineRange(index)
	text, _ := m.Slice(rng)
	start, state := rng.Start, -1
	for len(text) > 0 {
		word, rest, next := uniseg.FirstWordInString(text, state)
		end := start + len(word)
		if offset < end || end == rng.End {
			return Range{start, end}
		}
		start, text, state = end, rest, next
	}
	return rng
}
