package textedit

import (
	"reflect"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform/typography"
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
