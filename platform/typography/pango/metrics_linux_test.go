package pango

import (
	"runtime"
	"testing"
	"unicode/utf8"

	"github.com/golang-gui/goui/platform/typography"
)

// 原生排版依赖 Pango/字体库，无需显示连接或窗口；字节范围与方向预期不依赖字体像素。
func TestTextMetricsClusterRanges(t *testing.T) {
	for _, text := range []string{"abc", "中", "a\n\n中\n", "e\u0301 office 👨‍👩‍👧‍👦", "abc אבג xyz", "\u202babc אבג\u202c"} {
		t.Run(text, func(t *testing.T) {
			runtime.LockOSThread()
			defer runtime.UnlockOSThread()
			ctx, err := NewContext()
			if err != nil {
				t.Fatal(err)
			}
			defer ctx.Destroy()
			layout, err := ctx.NewTextLayout(text, typography.TextFormat{
				Font: typography.FontInfo{Family: "sans", Size: 14}, WrapMode: typography.WrapWordChar,
			}, 100, 1000)
			if err != nil {
				t.Fatal(err)
			}
			defer layout.Destroy()
			lines, flat := layout.MeasureMetrics()
			if text == "a\n\n中\n" {
				if len(lines) != 4 || lines[1].Start != 2 || lines[1].Length != 0 || lines[3].Start != 7 || lines[3].Length != 0 {
					t.Fatalf("empty and trailing line metrics lost: %+v", lines)
				}
			}
			count := 0
			for li, line := range lines {
				end := line.Start + line.Length
				at := line.Start
				for _, cluster := range line.Clusters {
					if cluster.Start != at || cluster.Length <= 0 || cluster.Start+cluster.Length > end {
						t.Errorf("line %d range [%d,%d): cluster %+v, expected next start %d and positive in-line length", li, line.Start, end, cluster, at)
					}
					if cluster.Start >= 0 && cluster.Start+cluster.Length <= len(text) && !utf8.ValidString(text[cluster.Start:cluster.Start+cluster.Length]) {
						t.Errorf("cluster splits UTF-8: %+v", cluster)
					}
					if cluster.Direction != typography.TextLeftToRight && cluster.Direction != typography.TextRightToLeft {
						t.Errorf("embedding level exposed as direction: %+v", cluster)
					}
					if cluster.LineIndex != li || cluster.Width < 0 || cluster.Height <= 0 {
						t.Errorf("invalid cluster geometry: %+v", cluster)
					}
					at = cluster.Start + cluster.Length
					count++
				}
				if at != end {
					t.Errorf("line %d clusters stop at byte %d, want %d", li, at, end)
				}
			}
			if count != len(flat) {
				t.Fatalf("line clusters=%d flat clusters=%d", count, len(flat))
			}
		})
	}
}

func TestTextMetricsRTLVisualOrder(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	ctx, err := NewContext()
	if err != nil {
		t.Fatal(err)
	}
	defer ctx.Destroy()
	layout, err := ctx.NewTextLayout("אבג", typography.TextFormat{Font: typography.FontInfo{Family: "sans", Size: 14}}, 200, 100)
	if err != nil {
		t.Fatal(err)
	}
	defer layout.Destroy()
	_, clusters := layout.MeasureMetrics()
	if len(clusters) != 3 {
		t.Fatalf("Hebrew letters: got %d clusters, want 3", len(clusters))
	}
	for i, cluster := range clusters {
		if cluster.Start != i*2 || cluster.Length != 2 || cluster.Direction != typography.TextRightToLeft {
			t.Errorf("Hebrew cluster %d = %+v", i, cluster)
		}
		if i > 0 && cluster.X >= clusters[i-1].X {
			t.Errorf("logical RTL cluster %d must be left of its predecessor", i)
		}
	}
}
