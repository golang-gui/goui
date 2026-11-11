package widgets

import (
	"image/color"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/style"
	"github.com/golang-gui/goui/theme/basic"
	"github.com/golang-gui/goui/theme/modern"
)

func TestListRowThemeAndInactivePixels(t *testing.T) {
	for _, sheet := range []style.StyleSheet{basic.Sheet(), modern.Sheet(modern.Options{}), modern.Sheet(modern.Options{Dark: true, FontSize: 24})} {
		for _, name := range []string{"list-row-title", "list-row-subtitle"} {
			s := sheet.Resolve(style.Sel{Name: name})
			if font, ok := s.FontSize(); !ok || font <= 0 {
				t.Fatal("文字样式缺少独立字号", name)
			}
			if fg, ok := s.ForegroundColor(); !ok || fg == color.Transparent {
				t.Fatal("文字样式缺少前景", name)
			}
			if _, ok := sheet.Resolve(style.Sel{Name: name, State: style.Disabled}).ForegroundColor(); !ok {
				t.Fatal("缺少禁用文字样式")
			}
		}
		if width, _ := sheet.Resolve(style.Sel{Name: "list-row", Part: "focus", State: style.FocusVisible}).BorderWidth(); width != 2 {
			t.Fatal("键盘焦点框缺失")
		}
	}
	red, green := color.RGBA{R: 255, A: 255}, color.RGBA{G: 255, A: 255}
	useProgressApplication(t, &progressTestApplication{sheet: style.Sheet(style.Name("list-row").BackgroundColor(green), style.Name("list-row").State(style.Hovered).BackgroundColor(red))})
	for _, scale := range []float32{1, 2} {
		for _, active := range []bool{false, true} {
			r := NewListRow("")
			r.SetActivatable(active)
			r.Arrange(geometry.Rect(0, 0, 80, 48))
			testWidgetHover(t, r, true)
			img := renderProgressTest(t, r, scale)
			want := green
			if active {
				want = red
			}
			// 远离圆角/焦点边缘的内部像素；预乘 sRGB RGBA8，不透明色无量化误差。
			if got := color.RGBAModel.Convert(img.At(int(20*scale), int(20*scale))).(color.RGBA); got != want {
				t.Fatalf("active=%v %gx: got=%v want=%v", active, scale, got, want)
			}
		}
	}
}
