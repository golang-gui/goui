package pango

import (
	"fmt"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"unsafe"

	"github.com/goexlib/cgo"
	"github.com/golang-gui/goui/platform/linux/libs/pango"
	"github.com/golang-gui/goui/platform/typography"
)

// Verify the missing fact before designing a cross-platform API: paragraph
// itemization is distinct from shaping a finite substring. Unlike replaying a
// stack of directional controls, native analysis preserves neutral/bracket and
// isolate resolution from the complete paragraph.
func TestContextualSubstringShaping(t *testing.T) {
	for _, tc := range []struct{ text, target string }{
		{"start \u202e" + strings.Repeat("abcd 1234 ", 410) + "TARGET text\u202c end", "TARGET"},
		{"אבג \u202d" + strings.Repeat("abcd 1234 ", 410) + "TARGET text\u202c סוף", "TARGET"},
		{"start \u2067" + strings.Repeat("עברית العربية 123 ", 180) + "TARGET text\u2069 end", "TARGET"},
		{strings.Repeat("中文 e\u0301 ffi 👩‍💻 العربية עברית\t ", 130) + "TARGET text", "TARGET"},
		{strings.Repeat("العربية 中文 ", 300) + "العربية", "عرب"},
		{strings.Repeat("office ", 600) + "office", "ffi"},
		{strings.Repeat("text ", 900) + "e\u0301 end", "e\u0301"},
	} {
		text := tc.text
		t.Run(fmt.Sprintf("%q", text[:min(16, len(text))]), func(t *testing.T) {
			runtime.LockOSThread()
			defer runtime.UnlockOSThread()
			ctx, err := NewContext()
			if err != nil {
				t.Fatal(err)
			}
			defer ctx.Destroy()
			format := typography.TextFormat{Font: typography.FontInfo{Family: "DejaVu Sans", Size: 12}}
			layout, err := ctx.NewTextLayout(text, format, 1<<24, 1<<24)
			if err != nil {
				t.Fatal(err)
			}
			defer layout.Destroy()
			whole := layout.(*TextLayout)
			attrs := pango.AttrListNew()
			defer attrs.Unref()
			desc, err := createFontDescription(format.Font)
			if err != nil {
				t.Fatal(err)
			}
			attr := pango.AttrFontDescNew(desc)
			desc.Free()
			attr.StartIndex, attr.EndIndex = 0, uint32(len(text))
			attrs.Insert(attr)
			items := whole.context.Itemize(text, 0, len(text), attrs)
			defer func() {
				for _, item := range items {
					item.Free()
				}
			}()
			start, end := strings.LastIndex(text, tc.target), strings.LastIndex(text, tc.target)+len(tc.target)
			var item *pango.Item
			for _, candidate := range items {
				if int(candidate.Offset) <= start && end <= int(candidate.Offset+candidate.Length) {
					item = candidate
					break
				}
			}
			if item == nil {
				t.Fatal("target has no native analysis item")
			}
			glyphs := pango.GlyphStringNew()
			defer glyphs.Free()
			buffer := cgo.CString(text)
			defer cgo.Free(buffer)
			var flags pango.ShapeFlags
			if whole.context.GetRoundGlyphPositions() {
				flags = pango.ShapeRoundPositions
			}
			pango.ShapeWithFlags(buffer, len(text), start, end-start, &item.Analysis, glyphs, flags)
			got := glyphs.GetGlyphInfos()
			logs := append([]int32(nil), glyphs.LogClusters()...)
			var want []pango.GlyphInfo
			var wantLogs []int32
			line := whole.layout.GetLineReadonly(0)
			for _, run := range line.GetRuns() {
				indices := run.Glyphs.LogClusters()
				for i, glyph := range run.Glyphs.GetGlyphInfos() {
					at := int(run.Item.Offset + indices[i])
					if at >= start && at < end {
						if run.Item.Analysis.Level != item.Analysis.Level {
							t.Fatal("lost paragraph bidi level")
						}
						want = append(want, glyph)
						wantLogs = append(wantLogs, int32(at-start))
					}
				}
			}
			if len(got) == 0 || !reflect.DeepEqual(logs, wantLogs) {
				t.Fatalf("cluster mapping: %v want %v", logs, wantLogs)
			}
			for i, g := range got {
				if g.Glyph != want[i].Glyph || g.Geometry != want[i].Geometry {
					t.Fatalf("glyph %d: %+v want %+v", i, g, want[i])
				}
			}
		})
	}
}

func TestAnalysisBindingPrefixes(t *testing.T) {
	// These prefix layouts are dereferenced by the bindings; native allocations
	// remain native-owned and are never allocated using these Go struct sizes.
	ptr := unsafe.Sizeof(uintptr(0))
	align := func(n uintptr) uintptr { return (n + ptr - 1) &^ (ptr - 1) }
	if unsafe.Offsetof(pango.Item{}.Analysis) != align(12) || unsafe.Sizeof(pango.Analysis{}) != align(5*ptr+4) || unsafe.Sizeof(pango.Item{}) != align(12)+align(5*ptr+4) {
		t.Fatal("unexpected PangoItem/PangoAnalysis ABI")
	}
}

func BenchmarkContextualShaping(b *testing.B) {
	for _, length := range []int{0, 512, 2048} {
		name := fmt.Sprintf("ShapeRange/%d", length)
		if length == 0 {
			name = "AnalyzeParagraph"
		}
		b.Run(name, func(b *testing.B) {
			runtime.LockOSThread()
			defer runtime.UnlockOSThread()
			ctx, err := NewContext()
			if err != nil {
				b.Fatal(err)
			}
			defer ctx.Destroy()
			text := strings.Repeat(`{"title":"hello world","value":1234},`, 4500)
			l, err := ctx.NewTextLayout(text, typography.TextFormat{Font: typography.FontInfo{Family: "monospace", Size: 12}}, 1<<24, 1<<24)
			if err != nil {
				b.Fatal(err)
			}
			defer l.Destroy()
			whole := l.(*TextLayout)
			attrs := pango.AttrListNew()
			defer attrs.Unref()
			desc, err := createFontDescription(l.Format().Font)
			if err != nil {
				b.Fatal(err)
			}
			attr := pango.AttrFontDescNew(desc)
			desc.Free()
			attr.StartIndex, attr.EndIndex = 0, uint32(len(text))
			attrs.Insert(attr)
			items := whole.context.Itemize(text, 0, len(text), attrs)
			defer func() {
				for _, item := range items {
					item.Free()
				}
			}()
			buffer := cgo.CString(text)
			defer cgo.Free(buffer)
			glyphs := pango.GlyphStringNew()
			defer glyphs.Free()
			var flags pango.ShapeFlags
			if whole.context.GetRoundGlyphPositions() {
				flags = pango.ShapeRoundPositions
			}
			b.ReportAllocs()
			if length == 0 {
				b.SetBytes(int64(len(text)))
			} else {
				b.SetBytes(int64(length))
			}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if length == 0 {
					next := whole.context.Itemize(text, 0, len(text), attrs)
					for _, item := range next {
						item.Free()
					}
				} else {
					pango.ShapeWithFlags(buffer, len(text), len(text)/2, length, &items[0].Analysis, glyphs, flags)
				}
			}
		})
	}
}
