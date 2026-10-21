package pango

import (
	"fmt"
	"image/color"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/golang-gui/goui/platform/linux/libs/pango"
	"github.com/golang-gui/goui/platform/typography"
)

// The previous native cursor path is retained as a reference for reducing FFI
// queries. Every cluster position still comes from Pango; this does not shape
// independent substrings or infer bidi positions from glyph advances.
func metricsByClusterQueries(t *TextLayout) (lines []typography.TextLine, clusters []typography.TextCluster) {
	lineCount := t.layout.GetLineCount()
	if lineCount == 0 {
		return
	}
	lines = make([]typography.TextLine, 0, lineCount)
	clusters = make([]typography.TextCluster, 0, t.chars)
	iter := t.layout.GetIter()
	defer iter.Free()
	// Keep independent line/cluster cursors. NextCluster skips empty lines,
	// while copying a line cursor on Pango 1.50.6 loses end_x_offset and can
	// produce uninitialized X positions when advancing to the next font run.
	clusterIter := t.layout.GetIter()
	defer clusterIter.Free()
	var currentRun *pango.GlyphItem
	var clusterStarts []int
	clusterOrdinal := 0
	for {
		nativeLine := iter.GetLineReadonly()
		if nativeLine == nil {
			break
		}
		var ink, rect pango.Rectangle
		iter.GetLineExtents(&ink, &rect)
		line := typography.TextLine{
			Start: int(nativeLine.StartIndex), Length: int(nativeLine.Length),
			X:     float32(rect.X) / pango.Scale,
			Y:     float32(rect.Y) / pango.Scale,
			Width: float32(rect.Width) / pango.Scale, Height: float32(rect.Height) / pango.Scale,
			Baseline: float32(iter.GetBaseline()) / pango.Scale,
		}
		beg := len(clusters)
		// NextCluster skips empty lines. Keep a separate line iterator so empty
		// and trailing lines retain their native metrics and stable LineIndex.
		for clusterIter.GetLineReadonly() == nativeLine {
			run := clusterIter.GetRunReadonly()
			if run != nil && run.Item != nil {
				if run != currentRun {
					currentRun, clusterOrdinal = run, 0
					clusterStarts = clusterStarts[:0]
					logs := run.Glyphs.LogClusters()
					for i, offset := range logs {
						if i == 0 || offset != logs[i-1] {
							clusterStarts = append(clusterStarts, int(run.Item.Offset+offset))
						}
					}
				}
				// Pango's RTL iterator index identifies the last character of a
				// cluster, not its logical start. Take byte starts from the shaped
				// glyph log instead, in the iterator's same visual cluster order.
				start := clusterStarts[clusterOrdinal]
				clusterOrdinal++
				var ink, clusterRect pango.Rectangle
				clusterIter.GetClusterExtents(&ink, &clusterRect)
				clusters = append(clusters, typography.TextCluster{
					Start: start,
					X:     float32(clusterRect.X) / pango.Scale, Y: line.Y,
					Width: float32(clusterRect.Width) / pango.Scale, Height: line.Height,
					LineIndex: len(lines),
					Direction: typography.TextDirection(run.Item.Analysis.Level & 1),
				})
				if placeholder, ok := t.placeholders[start]; ok {
					cluster := &clusters[len(clusters)-1]
					cluster.Y, cluster.Height = line.Baseline-placeholder.Baseline, placeholder.Height
				}
			}
			if !clusterIter.NextCluster() {
				break
			}
		}
		line.Clusters = clusters[beg:]
		slices.SortFunc(line.Clusters, func(a, b typography.TextCluster) int { return a.Start - b.Start })
		for i := range line.Clusters {
			end := line.Start + line.Length
			if i+1 < len(line.Clusters) {
				end = line.Clusters[i+1].Start
			}
			line.Clusters[i].Length = end - line.Clusters[i].Start
		}
		lines = append(lines, line)
		if !iter.NextLine() {
			break
		}
	}
	return
}

func TestMetricsByRunsPreserveNativeClusterExtents(t *testing.T) {
	for _, text := range []string{"", "Aé中", "a\n\nb\n", "a\u2028\u2029b", "abc\tdef\t", "office e\u0301 👩‍👩‍👦 عربى אבג xyz", strings.Repeat("one two אבג e\u0301 \u2067RTL\u2069 word ", 100), "abc \u202eאבג (123)\u202c xyz"} {
		for _, width := range []float32{31, 317, 1 << 20} {
			t.Run(text[:min(20, len(text))]+"/"+fmt.Sprint(width), func(t *testing.T) {
				runtime.LockOSThread()
				defer runtime.UnlockOSThread()
				c, err := NewContext()
				if err != nil {
					t.Fatal(err)
				}
				defer c.Destroy()
				native, err := c.NewTextLayout(text, typography.TextFormat{Font: typography.FontInfo{Family: "Go", Size: 14}, WrapMode: typography.WrapWordChar}, width, 1<<24)
				if err != nil {
					t.Fatal(err)
				}
				defer native.Destroy()
				layout := native.(*TextLayout)
				// Split native attribute/font runs as well as fallback/script runs.
				if len(text) > 3 {
					native.SetTextColor(1, 2, color.NRGBA{R: 255, A: 255})
				}
				wantLines, wantClusters := metricsByClusterQueries(layout)
				lines, clusters := native.MeasureMetrics()
				if !reflect.DeepEqual(lines, wantLines) || !reflect.DeepEqual(clusters, wantClusters) {
					t.Fatal("run traversal changed native lines or cluster extents")
				}
				if text == "Aé中" && width == 1<<20 {
					if len(clusters) != 3 {
						t.Fatal("missing independent rune clusters", clusters)
					}
					for i, r := range [][2]int{{0, 1}, {1, 2}, {3, 3}} {
						if clusters[i].Start != r[0] || clusters[i].Length != r[1] || clusters[i].Direction != typography.TextLeftToRight {
							t.Fatal("wrong independent UTF-8 boundary", clusters)
						}
					}
				}
			})
		}
	}
}

func BenchmarkMetricsRunQueries(b *testing.B) {
	for _, reference := range []bool{true, false} {
		name := "runs"
		if reference {
			name = "per-cluster"
		}
		b.Run(name, func(b *testing.B) {
			runtime.LockOSThread()
			defer runtime.UnlockOSThread()
			c, err := NewContext()
			if err != nil {
				b.Fatal(err)
			}
			defer c.Destroy()
			text := strings.Repeat("one two three four five six seven ", 4096)
			native, err := c.NewTextLayout(text, typography.TextFormat{Font: typography.FontInfo{Family: "Go", Size: 14}, WrapMode: typography.WrapWordChar}, 1000, 1<<24)
			if err != nil {
				b.Fatal(err)
			}
			defer native.Destroy()
			native.MeasureSize()
			b.ReportAllocs()
			b.SetBytes(int64(len(text)))
			b.ResetTimer()
			for range b.N {
				var lines []typography.TextLine
				var clusters []typography.TextCluster
				if reference {
					lines, clusters = metricsByClusterQueries(native.(*TextLayout))
				} else {
					lines, clusters = native.MeasureMetrics()
				}
				if len(lines) == 0 || len(clusters) == 0 {
					b.Fatal("missing metrics")
				}
			}
		})
	}
}
