package main

import (
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform/typography"
)

func TestInteractionVerifier(t *testing.T) {
	// 独立给定两字母的原生参考；故意移动第二个单元，验证比较器能发现实际错误。
	c := interactionCase{Name: "fixture", Text: "AB", Boundaries: []int{0, 1, 2},
		Lines:      []typography.TextLine{{Length: 2, Width: 20, Height: 20, Baseline: 15}},
		Clusters:   []typography.TextCluster{{Length: 1, Width: 10, Height: 20}, {Start: 1, Length: 1, X: 10, Width: 10, Height: 20}},
		Carets:     []nativeCaret{{0, 0, 0, 0}, {1, 0, 10, 10}, {2, 0, 20, 20}},
		Selections: []nativeSelection{{Start: 1, End: 2, Rects: []geometry.Rectangle{geometry.Rect(10, 0, 10, 20)}}},
	}
	if result := verifyInteractions([]interactionCase{c}); result.Failures != 0 {
		t.Fatal(result)
	}
	c.Clusters[1].X = 15
	if result := verifyInteractions([]interactionCase{c}); result.Failures == 0 {
		t.Fatal("错误几何未被原生参考比较器发现")
	}
}

func TestInteractionGraphemeOracle(t *testing.T) {
	// 模拟字号边界切进组合字符：保留原生内部命中，契约参考显式吸附至字素边界。
	c := interactionCase{Name: "combining", Text: "a\u0301", Boundaries: []int{0, 3},
		Lines:    []typography.TextLine{{Length: 3, Width: 10, Height: 20}},
		Clusters: []typography.TextCluster{{Length: 3, Width: 10, Height: 20}},
		Hits: []nativeHit{{nativeCaret: nativeCaret{Offset: 1, Primary: 10, Secondary: 10}, X: 8, Y: 10,
			ExpectedOffset: 3, ExpectedX: 10, Reason: "grapheme-snap"}},
	}
	result := verifyInteractions([]interactionCase{c})
	if result.Failures != 0 || result.GraphemeSnaps != 1 || result.RawHitDifferences != 1 {
		t.Fatal(result)
	}
	c.Hits[0].ExpectedOffset = 1
	if result := verifyInteractions([]interactionCase{c}); result.Failures == 0 {
		t.Fatal("不能把字素内部位置当成合法参考而掩盖失败")
	}
}
