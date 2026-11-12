package widgets

import (
	"fmt"
	"image/color"
	"math"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/graphics/software"
	"github.com/golang-gui/goui/style"
	"github.com/golang-gui/goui/theme/modern"
)

func TestSwitchOpaqueTrackCoverage(t *testing.T) {
	for _, theme := range []struct {
		name  string
		sheet style.StyleSheet
	}{{"Basic", nil}, {"ModernLight", modern.Sheet(modern.Options{})}, {"ModernDark", modern.Sheet(modern.Options{Dark: true})}} {
		for _, checked := range []bool{false, true} {
			for _, state := range []style.State{style.Normal, style.Hovered, style.Pressed, style.Disabled} {
				for _, scale := range []float32{1, 1.25, 1.5, 2} {
					t.Run(fmt.Sprintf("%s/checked=%v/state=%v/scale=%g", theme.name, checked, state, scale), func(t *testing.T) {
						useProgressApplication(t, &progressTestApplication{sheet: theme.sheet})
						b, host, dispatcher := switchInput()
						b.SetAnimated(false)
						b.Arrange(geometry.Rect(0, 0, 48, 33))
						b.SetChecked(checked)
						switch state {
						case style.Hovered:
							dispatchTabPointer(t, dispatcher, host, events.PointerMove, 16, 16)
						case style.Pressed:
							dispatchTabPointer(t, dispatcher, host, events.PointerDown, 16, 16)
						case style.Disabled:
							b.SetEnabled(false)
						}
						p, err := software.NewPainter(new(progressPixelSurface))
						if err != nil {
							t.Fatal(err)
						}
						defer p.Destroy()
						img, err := p.RenderImage(int(48*scale), int(math.Ceil(float64(33*scale))), scale, func() { b.Paint(checkPixelPainter{native: p}) })
						if err != nil {
							t.Fatal(err)
						}
						// 独立胶囊方程：1x 半 DIP 顶边先对齐，半径保持10 DIP。
						// 预乘 sRGB RGBA8，整颗像素连同0.05 px余量在形内时alpha必须255。
						left, top := math.Floor(float64(6*scale)+.5), math.Floor(float64(6.5*scale)+.5)
						radius := float64(10 * scale)
						right := math.Floor(float64(42*scale) + .5)
						inside := 0
						for y := 0; y < img.Bounds().Dy(); y++ {
							for x := 0; x < img.Bounds().Dx(); x++ {
								covered := true
								for _, d := range [][2]float64{{-.05, -.05}, {1.05, -.05}, {-.05, 1.05}, {1.05, 1.05}} {
									px, py := float64(x)+d[0], float64(y)+d[1]
									cx := max(left+radius, min(right-radius, px))
									if math.Hypot(px-cx, py-top-radius) >= radius {
										covered = false
									}
								}
								if covered {
									inside++
									if c := color.RGBAModel.Convert(img.At(x, y)).(color.RGBA); c.A != 255 {
										t.Fatalf("Switch形内漏底 (%d,%d)=%v，alpha须为255", x, y, c)
									}
								}
							}
						}
						if inside < 100 || b.Snapshot().Attributes[SwitchInfoKey].(SwitchInfo).Checked != checked {
							t.Fatal("采样不足或绘制改变了语义状态")
						}
					})
				}
			}
		}
	}
}
