package pango

import (
	"testing"
	"unsafe"
)

// Public Pango headers: pango-glyph-item.h, pango-glyph.h and pango-item.h.
// GlyphItem is allocated with GLib slice allocation. The other two structs
// below are borrowed public prefixes; Pango alone allocates their private tails.
func TestGlyphStorageABI(t *testing.T) {
	word := unsafe.Sizeof(uintptr(0))
	aligned := func(n uintptr) uintptr { return (n + word - 1) / word * word }
	checks := []struct {
		name      string
		got, want uintptr
	}{
		{"GlyphItem.size", unsafe.Sizeof(GlyphItem{}), aligned(2*word + 12)},
		{"GlyphItem.glyphs", unsafe.Offsetof(GlyphItem{}.Glyphs), word},
		{"GlyphItem.y_offset", unsafe.Offsetof(GlyphItem{}.YOffset), 2 * word},
		{"GlyphString.glyphs", unsafe.Offsetof(GlyphString{}.glyphs), aligned(4)},
		{"GlyphString.log_clusters", unsafe.Offsetof(GlyphString{}.logClusters), aligned(4) + word},
		{"Item.analysis", unsafe.Offsetof(Item{}.Analysis), aligned(12)},
		{"Analysis.level", unsafe.Offsetof(Analysis{}.Level), 3 * word},
		{"Analysis.language", unsafe.Offsetof(Analysis{}.Language), aligned(3*word + 4)},
		{"GlyphInfo.size", unsafe.Sizeof(GlyphInfo{}), 20},
		{"LogAttr.size", unsafe.Sizeof(LogAttr(0)), 4},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s=%d want%d", c.name, c.got, c.want)
		}
	}
}
