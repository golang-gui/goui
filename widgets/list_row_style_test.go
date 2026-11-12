package widgets

import (
	"image/color"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/style"
	"github.com/golang-gui/goui/theme/basic"
	"github.com/golang-gui/goui/theme/modern"
)

func TestListRowThemeSurfacePixels(t *testing.T) {
	for _, tc := range []struct {
		name                           string
		sheet                          style.StyleSheet
		normal, hover, pressed, border color.RGBA
	}{
		{"Basic", basic.Sheet(), color.RGBA{255, 255, 255, 255}, color.RGBA{230, 230, 230, 255}, color.RGBA{180, 180, 180, 255}, color.RGBA{140, 140, 140, 255}},
		{"ModernLight", modern.Sheet(modern.Options{}), color.RGBA{255, 255, 255, 255}, color.RGBA{246, 246, 246, 255}, color.RGBA{237, 237, 237, 255}, color.RGBA{228, 228, 229, 255}},
		{"ModernDark", modern.Sheet(modern.Options{Dark: true}), color.RGBA{50, 50, 56, 255}, color.RGBA{57, 57, 63, 255}, color.RGBA{65, 65, 71, 255}, color.RGBA{72, 72, 78, 255}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			useProgressApplication(t, &progressTestApplication{sheet: tc.sheet})
			for _, scale := range []float32{1, 2} {
				for _, state := range []style.State{style.Normal, style.Hovered, style.Pressed, style.Disabled} {
					r := NewListRow("")
					r.SetActivatable(true)
					r.Arrange(geometry.Rect(0, 0, 80, 48))
					want := tc.normal
					switch state {
					case style.Hovered:
						testWidgetHover(t, r, true)
						want = tc.hover
					case style.Pressed:
						r.SetState(style.Pressed, true)
						want = tc.pressed
					case style.Disabled:
						r.SetEnabled(false)
					}
					// 使用已有完整的矩形/圆角描边适配器，不绕过 ListRow.Paint。
					probe := &progressPaintWidget{paint: func(p gui.Painter) { r.Paint(checkPixelPainter{Painter: p, native: p.(*progressPixelPainter).native}) }}
					probe.Arrange(r.Rect())
					img := renderProgressTest(t, probe, scale)
					// 预乘 sRGB RGBA8；内部点远离 AA，边框取直边中线，允许 1 字节量化。
					for _, sample := range []struct {
						x, y  int
						color color.RGBA
					}{{20, 20, want}, {0, 24, tc.border}, {40, 0, tc.border}} {
						got := color.RGBAModel.Convert(img.At(sample.x*int(scale), sample.y*int(scale))).(color.RGBA)
						for channel, v := range []uint8{got.R, got.G, got.B, got.A} {
							w := []uint8{sample.color.R, sample.color.G, sample.color.B, sample.color.A}[channel]
							if delta := int(v) - int(w); delta < -1 || delta > 1 {
								t.Fatalf("state=%v %gx (%d,%d)=%v，预期 %v", state, scale, sample.x, sample.y, got, sample.color)
							}
						}
					}
				}
			}
		})
	}
}

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
