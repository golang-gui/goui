package coretext

import (
	"fmt"
	"image/color"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"

	. "github.com/golang-gui/goui/platform/darwin/frameworks/core_foundation"
	. "github.com/golang-gui/goui/platform/darwin/frameworks/core_graphics"
	. "github.com/golang-gui/goui/platform/darwin/frameworks/core_text"
	"github.com/golang-gui/goui/platform/typography"
)

// Preserve the previous native export as a reference. This test compares every
// line/cluster value; sharing scratch storage must not change shaping or output.
func metricsWithPerRunStorage(t *TextLayout) (lines []typography.TextLine, clusters []typography.TextCluster) {
	t.ensureFrame()

	if len(t.lines) == 0 {
		return
	}

	// 坐标先以原生精度转换；这里仍保留逐 run 分配作为 scratch 复用的参考。
	height := t.layoutSize.Height
	lines = make([]typography.TextLine, 0, len(t.lines))
	clusters = make([]typography.TextCluster, 0, t.textLength)

	for lineIndex, li := range t.lines {
		baseline := float32(height - li.origin.Y)
		lineX := float32(li.origin.X)
		lineY := float32(height - (li.origin.Y + li.ascent + li.leading))
		lineWidth := float32(li.width)
		lineHeight := float32(li.ascent + li.descent + li.leading)

		u8Start := t.position.ToUtf8(li.u16Start)
		u8End := t.position.ToUtf8(li.u16Start + li.u16Length)
		if u8End < 0 {
			u8End = len(t.text)
		}
		// CoreText includes the paragraph separator in a line's string range.
		// Keep it out of visible clusters, as in Pango and DirectWrite.
		u8End = u8Start + len(strings.TrimRight(t.text[u8Start:u8End], "\r\n\u2028\u2029"))

		line := typography.TextLine{
			Start:    u8Start,
			Length:   u8End - u8Start,
			X:        lineX,
			Y:        lineY,
			Width:    lineWidth,
			Height:   lineHeight,
			Baseline: baseline,
		}

		clustersBeg := len(clusters)
		clusterByIndex := make(map[int]int)

		// Get clusters from runs
		runs := CTLineGetGlyphRuns(li.line)
		runCount := CFArrayGetCount(runs)
		for ri := 0; ri < runCount; ri++ {
			run := CFArrayGetValueAtIndex(runs, ri)
			status := CTRunGetStatus(run)
			isRTL := (status & KCTRunStatusRightToLeft) != 0

			glyphCount := CTRunGetGlyphCount(run)
			if glyphCount == 0 {
				continue
			}

			positions := make([]CGPoint, glyphCount)
			CTRunGetPositions(run, CFRangeMake(0, 0), positions)
			advances := make([]CGSize, glyphCount)
			CTRunGetAdvances(run, CFRangeMake(0, 0), advances)

			indices := make([]CFIndex, glyphCount)
			CTRunGetStringIndices(run, CFRangeMake(0, 0), indices)

			// A string index may map to several positioned glyphs. Group by
			// that index (not glyph order), preserving native advance geometry.
			for gi := 0; gi < glyphCount; gi++ {
				u16Idx := int(indices[gi])
				u8Pos := t.position.ToUtf8(u16Idx)
				if u8Pos < u8Start || u8Pos >= u8End {
					continue
				}
				x0 := lineX + float32(positions[gi].X)
				x1 := x0 + float32(advances[gi].Width)
				left, right := min(x0, x1), max(x0, x1)
				if ci, ok := clusterByIndex[u8Pos]; ok {
					cluster := &clusters[ci]
					right = max(right, cluster.X+cluster.Width)
					cluster.X = min(left, cluster.X)
					cluster.Width = right - cluster.X
					continue
				}

				cluster := typography.TextCluster{
					Start:     u8Pos,
					X:         left,
					Y:         lineY,
					Width:     right - left,
					Height:    lineHeight,
					LineIndex: lineIndex,
				}
				if placeholder, ok := t.placeholders[u8Pos]; ok {
					cluster.Y, cluster.Height = line.Baseline-placeholder.Baseline, placeholder.Height
				}
				if isRTL {
					cluster.Direction = typography.TextRightToLeft
				}
				clusterByIndex[u8Pos] = len(clusters)
				clusters = append(clusters, cluster)
			}
		}

		line.Clusters = clusters[clustersBeg:]
		slices.SortFunc(line.Clusters, func(a, b typography.TextCluster) int {
			return a.Start - b.Start
		})
		for i := range line.Clusters {
			end := u8End
			if i+1 < len(line.Clusters) {
				end = line.Clusters[i+1].Start
			}
			line.Clusters[i].Length = end - line.Clusters[i].Start
		}
		lines = append(lines, line)
	}
	return
}

func TestMetricsScratchPreservesNativeGeometry(t *testing.T) {
	for _, text := range []string{"", "Aé中", "a\n\nb\n", "a\u2028\u2029b", "abc\tdef\t", "office e\u0301 👩‍👩‍👦 عربى אבג xyz", strings.Repeat("one two אבג e\u0301 \u2067RTL\u2069 word ", 100), "abc \u202eאבג (123)\u202c xyz", "a\ufffcb"} {
		for _, width := range []float32{31, 317, 1 << 20} {
			t.Run(fmt.Sprintf("%q/%g", text[:min(20, len(text))], width), func(t *testing.T) {
				runtime.LockOSThread()
				defer runtime.UnlockOSThread()
				ctx, err := NewContext()
				if err != nil {
					t.Fatal(err)
				}
				defer ctx.Destroy()
				layout, err := ctx.NewTextLayout(text, typography.TextFormat{Font: typography.FontInfo{Family: "Arial", Size: 14}, WrapMode: typography.WrapWordChar, TabWidth: 32}, width, 1<<20)
				if err != nil {
					t.Fatal(err)
				}
				defer layout.Destroy()
				if text == "a\ufffcb" {
					layout.SetPlaceholder(1, &typography.PlaceholderMetrics{Width: 17, Height: 21, Baseline: 16})
				}
				if len(text) > 0 {
					layout.SetTextColor(0, 1, color.RGBA{R: 160, A: 255})
				}
				native := layout.(*TextLayout)
				wantLines, wantClusters := metricsWithPerRunStorage(native)
				gotLines, gotClusters := layout.MeasureMetrics()
				if !reflect.DeepEqual(gotLines, wantLines) || !reflect.DeepEqual(gotClusters, wantClusters) {
					t.Fatal("scratch reuse changed native line or cluster metrics")
				}
				if text == "Aé中" && width > 100 {
					if len(gotClusters) != 3 {
						t.Fatalf("UTF-8 clusters = %d", len(gotClusters))
					}
					for i, start := range []int{0, 1, 3} {
						if gotClusters[i].Start != start || gotClusters[i].Length != i+1 {
							t.Fatalf("cluster %d = %+v", i, gotClusters[i])
						}
					}
				}
				// A subsequent call and changed width may reuse local buffers, but must
				// never mutate ordinary values already returned to an upper-layer cache.
				layout.SetSize(47, 1<<20)
				layout.MeasureMetrics()
				if !reflect.DeepEqual(gotLines, wantLines) || !reflect.DeepEqual(gotClusters, wantClusters) {
					t.Fatal("later measurement mutated returned metrics")
				}
			})
		}
	}
}

func BenchmarkMetricsScratch(b *testing.B) {
	text := strings.Repeat("alpha beta gamma delta ", 8192)
	for _, reuse := range []bool{false, true} {
		b.Run(fmt.Sprint(reuse), func(b *testing.B) {
			runtime.LockOSThread()
			defer runtime.UnlockOSThread()
			ctx, err := NewContext()
			if err != nil {
				b.Fatal(err)
			}
			defer ctx.Destroy()
			layout, err := ctx.NewTextLayout(text, typography.TextFormat{Font: typography.FontInfo{Family: "Arial", Size: 14}, WrapMode: typography.WrapWordChar}, 1000, 1<<24)
			if err != nil {
				b.Fatal(err)
			}
			defer layout.Destroy()
			layout.MeasureSize() // Compare export only, after the same native shaping.
			native := layout.(*TextLayout)
			b.ReportAllocs()
			b.SetBytes(int64(len(text)))
			b.ResetTimer()
			for range b.N {
				if reuse {
					layout.MeasureMetrics()
				} else {
					metricsWithPerRunStorage(native)
				}
			}
		})
	}
}
