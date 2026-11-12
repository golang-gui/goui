package gui

import (
	"image/color"
	"testing"

	"github.com/golang-gui/goui/core/colors"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/style"
)

// Name与Part共同限定文字样式；换Part走正常失效，另一Label的资源不受影响。
func TestLabelExplicitStylePartMetricsAndIsolation(t *testing.T) {
	typo := &styleTypography{}
	setTestApplication(t, typo)
	App.SetStyleSheet(style.Sheet(
		style.Name("label").FontSize(10).ForegroundColor(color.Black),
		style.Name("orders").Part("header-text").FontSize(20).ForegroundColor(color.RGBA{R: 255, A: 255}),
		style.Name("orders").Part("body-text").FontSize(12),
		style.Name("orders").Part("body-blue").FontSize(12).ForegroundColor(color.RGBA{B: 255, A: 255}),
		style.Name("other").Part("header-text").FontSize(30).ForegroundColor(color.RGBA{B: 255, A: 255}),
	))
	a, b := NewLabel("same"), NewLabel("same")
	t.Cleanup(a.releaseLayout)
	t.Cleanup(b.releaseLayout)
	a.SetStyleName("orders")
	b.SetStyleName("other")
	a.SetStylePart("header-text")
	b.SetStylePart("header-text")
	first, other := measureWidget(a, layout.Unbounded()), measureWidget(b, layout.Unbounded())
	if a.cachedLayout.Format().Font.Size != 20 || b.cachedLayout.Format().Font.Size != 30 || first.Height >= other.Height || !colors.Equal(a.cachedLayout.Format().TextColor, color.RGBA{R: 255, A: 255}) {
		t.Fatal("独立Name/Part没有参与文字资源与测量")
	}
	oldA, oldB := a.cachedLayout.(*testTextLayout), b.cachedLayout
	a.SetStylePart("body-text")
	if oldA.destroyed {
		t.Fatal("setter提前释放原生资源")
	}
	second := measureWidget(a, layout.Unbounded())
	if !oldA.destroyed || second.Height >= first.Height || a.cachedLayout.Format().Font.Size != 12 || b.cachedLayout != oldB {
		t.Fatal("Part变化未重新测量或污染其他Label")
	}
	intermediate := a.cachedLayout
	a.SetStylePart("body-blue")
	measureWidget(a, layout.Unbounded())
	if a.cachedLayout != intermediate || !colors.Equal(a.cachedLayout.Format().TextColor, color.RGBA{B: 255, A: 255}) {
		t.Fatal("仅文字颜色不同的Part应更新颜色并复用原生布局")
	}
	a.SetStylePart("")
	measureWidget(a, layout.Unbounded())
	if a.StylePart() != "" || a.cachedLayout.Format().Font.Size != 10 {
		t.Fatal("清除Part未恢复Label基准字体")
	}
}

func TestLabelStylePartScopeAndState(t *testing.T) {
	setTestApplication(t, nil)
	scope := NewStyleScope()
	scope.SetStyleSheet(style.Sheet(style.Name("label").FontSize(10), style.Name("table").Part("header-text").FontSize(18), style.Name("table").Part("header-text").State(style.Disabled).FontSize(16)))
	l := NewLabel("name")
	l.SetStyleName("table")
	l.SetStylePart("header-text")
	scope.SetChild(l)
	if l.resolvedTextFormat().Font.Size != 18 {
		t.Fatal("Part绕过最近StyleScope")
	}
	l.SetEnabled(false)
	if l.resolvedTextFormat().Font.Size != 16 {
		t.Fatal("Part没有使用Label状态")
	}
}
