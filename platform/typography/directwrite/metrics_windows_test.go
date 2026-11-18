package directwrite

import (
	"runtime"
	"testing"
	"unicode/utf8"

	"github.com/golang-gui/goui/platform/typography"
)

// 离屏原生排版不创建窗口或显示连接；字节范围和视觉顺序预期不依赖字形光栅化或 hinting。
func TestTextMetricsClusterRanges(t *testing.T) {
	for _, text := range []string{"abc", "中", "a\n中", "e\u0301 office", "abc אבג xyz", "a\u200bb", "a😀👩‍💻z"} {
		t.Run(text, func(t *testing.T) {
			runtime.LockOSThread()
			defer runtime.UnlockOSThread()
			ctx, err := NewContext()
			if err != nil {
				t.Fatal(err)
			}
			defer ctx.Destroy()
			layout, err := ctx.NewTextLayout(text, typography.TextFormat{
				Font: typography.FontInfo{Family: "Arial", Size: 14}, WrapMode: typography.WrapWordChar,
			}, 500, 500)
			if err != nil {
				t.Fatal(err)
			}
			defer layout.Destroy()
			lines, flat := layout.MeasureMetrics()
			if len(lines) == 0 {
				t.Fatal("nonempty text has no lines")
			}
			count := 0
			for li, line := range lines {
				end := line.Start + line.Length
				at := line.Start
				if line.Width < 0 || line.Height <= 0 {
					t.Errorf("invalid line bounds: %+v", line)
				}
				for _, cluster := range line.Clusters {
					if cluster.Start != at || cluster.Length <= 0 || cluster.Start+cluster.Length > end {
						t.Errorf("line %d: cluster %+v, expected next start %d and positive in-line length", li, cluster, at)
					}
					if cluster.Start >= 0 && cluster.Start+cluster.Length <= len(text) && !utf8.ValidString(text[cluster.Start:cluster.Start+cluster.Length]) {
						t.Errorf("cluster splits UTF-8: %+v", cluster)
					}
					if cluster.Direction != typography.TextLeftToRight && cluster.Direction != typography.TextRightToLeft {
						t.Errorf("invalid direction: %+v", cluster)
					}
					if cluster.LineIndex != li || cluster.Width < 0 || cluster.Height <= 0 {
						t.Errorf("invalid cluster geometry: %+v", cluster)
					}
					at = cluster.Start + cluster.Length
					count++
				}
				if at != end {
					t.Errorf("line %d clusters stop at %d, want %d", li, at, end)
				}
			}
			if count != len(flat) {
				t.Fatalf("line clusters=%d, flat=%d", count, len(flat))
			}
			if text == "a\n中" && (len(lines) != 2 || lines[0].Length != 1 || lines[1].Start != 2 || lines[1].Length != 3) {
				t.Fatalf("newline was included in content: %+v", lines)
			}
			if lines[len(lines)-1].Start+lines[len(lines)-1].Length != len(text) {
				t.Fatal("last line does not cover end of text")
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
	layout, err := ctx.NewTextLayout("אבג", typography.TextFormat{
		Font: typography.FontInfo{Family: "Arial", Size: 14},
	}, 200, 100)
	if err != nil {
		t.Fatal(err)
	}
	defer layout.Destroy()
	lines, clusters := layout.MeasureMetrics()
	if len(lines) != 1 || len(clusters) != 3 {
		t.Fatalf("Hebrew: lines=%d clusters=%d", len(lines), len(clusters))
	}
	for i, cluster := range clusters {
		if cluster.Start != i*2 || cluster.Length != 2 || cluster.Direction != typography.TextRightToLeft {
			t.Errorf("Hebrew cluster %d = %+v", i, cluster)
		}
		if i > 0 && cluster.X >= clusters[i-1].X {
			t.Errorf("logical RTL cluster %d must be left of its predecessor", i)
		}
	}
	if lines[0].Width <= 0 {
		t.Fatalf("RTL line width = %v", lines[0].Width)
	}
}

// 原生库对空段落/末尾段落的行数行为不同；不在此合成编辑器光标行。
// 所有报告范围必须有效，非分隔符字节恰好覆盖一次，分隔符不属于正文。
func TestTextMetricsParagraphSeparators(t *testing.T) {
	for _, text := range []string{"", "\n", "\n\n", "a\n", "a\n\n中\n", "a\r\n\r\n中\r\n", "\n😀\n"} {
		t.Run(text, func(t *testing.T) {
			runtime.LockOSThread()
			defer runtime.UnlockOSThread()
			ctx, err := NewContext()
			if err != nil {
				t.Fatal(err)
			}
			defer ctx.Destroy()
			layout, err := ctx.NewTextLayout(text, typography.TextFormat{
				Font:     typography.FontInfo{Family: "Arial", Size: 14},
				WrapMode: typography.WrapWordChar,
			}, 500, 500)
			if err != nil {
				t.Fatal(err)
			}
			defer layout.Destroy()
			lines, clusters := layout.MeasureMetrics()
			covered := make([]bool, len(text))
			for _, line := range lines {
				if line.Start < 0 || line.Length < 0 || line.Start+line.Length > len(text) {
					t.Fatalf("invalid line range: %+v, text length %d", line, len(text))
				}
				content := text[line.Start : line.Start+line.Length]
				if !utf8.ValidString(content) {
					t.Fatalf("line splits UTF-8: %+v", line)
				}
				for _, r := range content {
					if r == '\r' || r == '\n' {
						t.Fatalf("separator included in line content: %+v", line)
					}
				}
			}
			for _, cluster := range clusters {
				end := cluster.Start + cluster.Length
				if cluster.Start < 0 || cluster.Length <= 0 || end > len(text) ||
					cluster.LineIndex < 0 || cluster.LineIndex >= len(lines) {
					t.Fatalf("invalid cluster: %+v", cluster)
				}
				line := lines[cluster.LineIndex]
				if cluster.Start < line.Start || end > line.Start+line.Length ||
					!utf8.ValidString(text[cluster.Start:end]) {
					t.Fatalf("cluster escapes line or splits UTF-8: %+v", cluster)
				}
				for i := cluster.Start; i < end; i++ {
					if covered[i] || text[i] == '\r' || text[i] == '\n' {
						t.Fatalf("byte %d duplicated or separator included: %+v", i, cluster)
					}
					covered[i] = true
				}
			}
			for i := range len(text) {
				if text[i] != '\r' && text[i] != '\n' && !covered[i] {
					t.Fatalf("nonseparator byte %d has no cluster", i)
				}
			}
		})
	}
}
