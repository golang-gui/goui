package textedit

import (
	"reflect"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform/typography"
	"github.com/rivo/uniseg"
)

func TestObscuredTextMapsSourceWithoutChangingNativeMetrics(t *testing.T) {
	for _, char := range []rune{0, '*', '●'} {
		m := NewObscuredText("é中👨‍👩‍👧‍👦", char)
		mask := char
		if mask == 0 {
			mask = '•'
		}
		if m.Text() != string([]rune{mask, mask, mask}) || m.SourceLength() != len("é中👨‍👩‍👧‍👦") {
			t.Fatal("掩码单元错误", m.Text())
		}
		n := len(string(mask))
		clusters := []typography.TextCluster{{Start: 0, Length: n, X: 0, Width: 8, Height: 20}, {Start: n, Length: n, X: 8, Width: 8, Height: 20}, {Start: 2 * n, Length: n, X: 16, Width: 8, Height: 20}}
		lines := []typography.TextLine{{Length: 3 * n, Width: 24, Height: 20, Baseline: 15, Clusters: clusters}}
		before := append([]typography.TextCluster(nil), clusters...)
		mappedLines, mapped := m.Metrics(lines, clusters)
		if !reflect.DeepEqual(before, clusters) || lines[0].Length != 3*n {
			t.Fatal("改写了原生度量")
		}
		if mapped[0].Length != 3 || mapped[1].Start != 3 || mapped[1].Length != 3 || mapped[2].Start != 6 || mappedLines[0].Length != m.SourceLength() {
			t.Fatal("源字节映射错误", mappedLines, mapped)
		}
		g := NewGeometry(mappedLines, mapped, m.SourceLength(), 20)
		if p := g.Hit(geometry.Point{X: 12, Y: 10}); p.Offset != 3 && p.Offset != 6 {
			t.Fatal("命中进入原文字素内部", p)
		}
		if got := g.Adjacent(3, true); got != 6 {
			t.Fatal("无法按原文边界前进", got)
		}
	}
}

func TestObscuringCharacterValidation(t *testing.T) {
	for _, char := range []rune{'\n', ' ', '\u0301', '\u200d', -1, 0x110000} {
		if ValidObscureChar(char) {
			t.Fatalf("接受无效掩码 %U", char)
		}
	}
}

// 显露的是完整源字素，混合显示的 UTF-8 长度不能当成源偏移。
func TestRevealedTextSourceMapping(t *testing.T) {
	for _, tc := range []struct {
		name, source, want string
		reveal             Range
	}{
		{"middle", "é中👨‍👩‍👧‍👦", "*中*", Range{3, 6}},
		{"combining_join", "é中", "é*", Range{1, 3}},
		{"emoji", "中👨‍👩‍👧‍👦x", "*👨‍👩‍👧‍👦*", Range{3, 28}},
		{"many", "abc", "***", Range{0, 2}},
		{"empty", "abc", "***", Range{}},
		{"invalid", "abc", "***", Range{-1, 1}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := NewRevealedText(tc.source, '*', tc.reveal)
			if m.Text() != tc.want || m.SourceLength() != len(tc.source) {
				t.Fatal("显露范围或源长度错误", m.Text(), m.SourceLength())
			}
			var clusters []typography.TextCluster
			display := uniseg.NewGraphemes(m.Text())
			source := uniseg.NewGraphemes(tc.source)
			for display.Next() {
				start, end := display.Positions()
				clusters = append(clusters, typography.TextCluster{Start: start, Length: end - start, Width: 10, Height: 20})
			}
			_, mapped := m.Metrics(nil, clusters)
			for _, c := range mapped {
				if !source.Next() {
					t.Fatal("源字素数不足")
				}
				start, end := source.Positions()
				if c.Start != start || c.Length != end-start {
					t.Fatal("混合排版没有恢复源坐标", c, start, end)
				}
			}
		})
	}
}

func TestRevealedTextMergesNativeFallbackClusters(t *testing.T) {
	m := NewRevealedText("éx", '*', Range{1, 3})
	clusters := []typography.TextCluster{
		{Start: 0, Length: 1, X: 0, Width: 8, Height: 20},
		{Start: 1, Length: 2, X: 6, Width: 4, Height: 20},
		{Start: 3, Length: 1, X: 10, Width: 8, Height: 20},
	}
	lines, mapped := m.Metrics([]typography.TextLine{{Length: 4, Width: 18, Height: 20, Clusters: clusters}}, clusters)
	if len(mapped) != 2 || mapped[0].Start != 0 || mapped[0].Length != 3 || mapped[0].X != 0 || mapped[0].Width != 10 || mapped[1].Start != 3 {
		t.Fatal("原生分裂的组合字素没有合并", mapped)
	}
	g := NewGeometry(lines, mapped, 4, 20)
	if g.Adjacent(0, true) != 3 || g.Adjacent(3, false) != 0 || clusters[0].Length != 1 {
		t.Fatal("光标进入显露字素内部，或改写了原生度量")
	}
}

// 显露 ASCII 比三字节圆点更短，后续掩码仍须准确映射到源字节。
func TestRevealedTextShorterThanMask(t *testing.T) {
	m := NewRevealedText("a中b", '•', Range{0, 1})
	_, mapped := m.Metrics(nil, []typography.TextCluster{
		{Start: 0, Length: 1}, {Start: 1, Length: 3}, {Start: 4, Length: 3},
	})
	if m.Text() != "a••" || len(mapped) != 3 || mapped[0].Length != 1 || mapped[1].Start != 1 || mapped[1].Length != 3 || mapped[2].Start != 4 || mapped[2].Length != 1 {
		t.Fatal("显露字符比掩码短时的源坐标错误", mapped)
	}
}
