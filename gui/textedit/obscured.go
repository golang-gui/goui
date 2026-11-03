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

// ObscuredText contains only masking text and source byte boundaries, never
// the source string. Each extended grapheme becomes one masking rune.
type ObscuredText struct {
	text       string
	boundaries []int
	charBytes  int
}

// NewObscuredText maps normalized UTF-8 text to masking text. Invalid masking
// runes panic; zero uses U+2022. The source string is not retained.
func NewObscuredText(text string, char rune) *ObscuredText {
	if !ValidObscureChar(char) {
		panic("text: invalid obscuring character")
	}
	if char == 0 {
		char = '•'
	}
	m := &ObscuredText{boundaries: []int{0}, charBytes: utf8.RuneLen(char)}
	g := uniseg.NewGraphemes(text)
	for g.Next() {
		_, end := g.Positions()
		m.boundaries = append(m.boundaries, end)
	}
	m.text = strings.Repeat(string(char), len(m.boundaries)-1)
	return m
}

func (m *ObscuredText) Text() string      { return m.text }
func (m *ObscuredText) SourceLength() int { return m.boundaries[len(m.boundaries)-1] }

func (m *ObscuredText) sourceOffset(offset int) int {
	return m.boundaries[min(max(0, offset/m.charBytes), len(m.boundaries)-1)]
}

// Metrics copies native masking metrics into source byte coordinates. Geometry
// continues using real selection offsets, while native layouts keep only masks.
// Input slices remain owned by the native layout and are never modified.
func (m *ObscuredText) Metrics(lines []typography.TextLine, clusters []typography.TextCluster) ([]typography.TextLine, []typography.TextCluster) {
	mappedClusters := func(values []typography.TextCluster) []typography.TextCluster {
		out := make([]typography.TextCluster, len(values))
		for i, value := range values {
			out[i] = value
			out[i].Start = m.sourceOffset(value.Start)
			out[i].Length = m.sourceOffset(value.Start+value.Length) - out[i].Start
		}
		return out
	}
	out := make([]typography.TextLine, len(lines))
	for i, value := range lines {
		out[i] = value
		out[i].Start = m.sourceOffset(value.Start)
		out[i].Length = m.sourceOffset(value.Start+value.Length) - out[i].Start
		out[i].Clusters = mappedClusters(value.Clusters)
	}
	return out, mappedClusters(clusters)
}
