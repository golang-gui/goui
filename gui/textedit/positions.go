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
	if rng.End-rng.Start > 8192 {
		for radius := 2048; ; radius *= 2 {
			start, end := max(rng.Start, offset-radius), min(rng.End, offset+radius)
			for !m.IsRuneBoundary(start) {
				start++
			}
			for !m.IsRuneBoundary(end) {
				end--
			}
			text, _ := m.Slice(Range{start, end})
			at, state := start, -1
			for len(text) > 0 {
				word, rest, next := uniseg.FirstWordInString(text, state)
				last := at + len(word)
				if offset < last || last == end {
					if (at > start || start == rng.Start) && (last < end || end == rng.End) {
						return Range{at, last}
					}
					// A single very long ASCII identifier can be scanned in
					// bounded buffers; never allocate its paragraph as a string.
					if asciiWord(word) {
						if r, safe := m.asciiWordRange(offset, rng); safe {
							return r
						}
					}
					break
				}
				at, text, state = last, rest, next
			}
			if start == rng.Start && end == rng.End {
				return rng
			}
		}
	}
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

func asciiWord(text string) bool {
	if text == "" {
		return false
	}
	for i := 0; i < len(text); i++ {
		c := text[i]
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_') {
			return false
		}
	}
	return true
}

func (m *Model) asciiWordRange(offset int, line Range) (Range, bool) {
	start, end := offset, offset
	for start > line.Start {
		a := max(line.Start, start-4096)
		for !m.IsRuneBoundary(a) {
			a++
		}
		s, _ := m.Slice(Range{a, start})
		i := len(s)
		for i > 0 && asciiWord(s[i-1:i]) {
			i--
		}
		start = a + i
		if i > 0 {
			break
		}
	}
	for end < line.End {
		z := min(line.End, end+4096)
		for !m.IsRuneBoundary(z) {
			z--
		}
		s, _ := m.Slice(Range{end, z})
		i := 0
		for i < len(s) && asciiWord(s[i:i+1]) {
			i++
		}
		end += i
		if i < len(s) {
			break
		}
	}
	// Apostrophes, mid-letter punctuation and Unicode may join an ASCII run
	// under UAX #29. Only independent separators prove this fast path safe.
	for _, edge := range []struct {
		at     int
		before bool
	}{{start, true}, {end, false}} {
		if edge.before && edge.at == line.Start || !edge.before && edge.at == line.End {
			continue
		}
		a, b := edge.at, min(line.End, edge.at+1)
		if edge.before {
			a, b = edge.at-1, edge.at
		}
		if !m.IsRuneBoundary(a) || !m.IsRuneBoundary(b) {
			return Range{}, false
		}
		s, err := m.Slice(Range{a, b})
		if err != nil || len(s) != 1 || !strings.ContainsRune(" \t\r\n\"`[]{}()<>/\\+-=*%&|!?#@$^~;,", rune(s[0])) {
			return Range{}, false
		}
	}
	return Range{start, end}, true
}
