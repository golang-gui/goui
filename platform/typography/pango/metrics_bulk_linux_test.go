package pango

import (
	"fmt"
	"image/color"
	"runtime"
	"strings"
	"testing"
	"unicode/utf8"

	native "github.com/golang-gui/goui/platform/linux/libs/pango"
	"github.com/golang-gui/goui/platform/typography"
)

// The native cluster iterator is an independent geometry oracle. The optimized
// exporter walks native glyph arrays without NextCluster/GetClusterExtents per
// cluster; compare its exact integer-unit origins and advances with those APIs.
func TestBulkMetricsMatchNativeClusters(t *testing.T) {
	for _, text := range []string{"", "\n\n", "abc\n\n中\n", strings.Repeat("AV ffi e\u0301 👩‍💻 中文\t", 40), strings.Repeat("עברית (123) العربية \u2067abc\u2069 ", 30), "\u202e" + strings.Repeat("ab e\u0301 (123) ", 300) + "\u202c end", "a\ufffcb"} {
		for _, align := range []typography.TextAlignment{typography.TextAlignBegin, typography.TextAlignCenter, typography.TextAlignEnd, typography.TextAlignFill} {
			t.Run(fmt.Sprintf("%q/%d", text[:min(len(text), 10)], align), func(t *testing.T) {
				runtime.LockOSThread()
				defer runtime.UnlockOSThread()
				ctx, err := NewContext()
				if err != nil {
					t.Fatal(err)
				}
				defer ctx.Destroy()
				l, err := ctx.NewTextLayout(text, typography.TextFormat{Font: typography.FontInfo{Family: "DejaVu Sans", Size: 12}, WrapMode: typography.WrapWordChar, TextAlign: align, TabWidth: 40}, 173, 1<<24)
				if err != nil {
					t.Fatal(err)
				}
				defer l.Destroy()
				if len(text) > 20 {
					_, first := utf8.DecodeRuneInString(text)
					_, second := utf8.DecodeRuneInString(text[first:])
					l.SetTextFont(0, first, typography.FontInfo{Family: "serif", Size: 22})
					l.SetTextColor(first, second, color.NRGBA{R: 200, A: 255})
					l.SetUnderline(0, first+second, true)
				}
				if at := strings.IndexRune(text, '\ufffc'); at >= 0 {
					l.SetPlaceholder(at, &typography.PlaceholderMetrics{Width: 31, Height: 47, Baseline: 30})
				}
				lines, clusters := l.MeasureMetrics()
				whole := l.(*TextLayout)
				byStart := make(map[int]typography.TextCluster, len(clusters))
				for _, c := range clusters {
					byStart[c.Start] = c
				}
				iter := whole.layout.GetIter()
				defer iter.Free()
				visited := 0
				for {
					run := iter.GetRunReadonly()
					if run != nil && run.Item != nil {
						// Native indices for RTL clusters point at the final code point. Find
						// the exported logical range that contains that byte index.
						index := iter.GetIndex()
						var got typography.TextCluster
						found := false
						for _, c := range lines[nativeClusterLine(lines, index)].Clusters {
							if c.Start <= index && index < c.Start+c.Length {
								got = c
								found = true
								break
							}
						}
						if !found {
							t.Fatalf("native cluster at %d missing", index)
						}
						var rect native.Rectangle
						iter.GetClusterExtents(nil, &rect)
						if got.X != float32(rect.X)/native.Scale || got.Width != float32(rect.Width)/native.Scale {
							t.Fatalf("cluster %d geometry=(%v,%v), native=(%v,%v)", got.Start, got.X, got.Width, float32(rect.X)/native.Scale, float32(rect.Width)/native.Scale)
						}
						delete(byStart, got.Start)
						visited++
					}
					if !iter.NextCluster() {
						break
					}
				}
				if visited != len(clusters) || len(byStart) != 0 {
					t.Fatalf("visited=%d exported=%d missing=%v", visited, len(clusters), byStart)
				}
			})
		}
	}
}
func nativeClusterLine(lines []typography.TextLine, index int) int {
	for i, l := range lines {
		if l.Start <= index && index < l.Start+l.Length {
			return i
		}
	}
	return 0
}
