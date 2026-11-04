package software

import (
	"image/color"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform/graphics"
)

func TestRoundRectBorderCoverage(t *testing.T) {
	for _, scale := range []float32{1, 1.25, 1.5, 2} {
		for _, radius := range []geometry.CornerRadius{{TopLeft: 1, TopRight: 18, BottomRight: 5, BottomLeft: 0}, {TopLeft: 36, TopRight: 0, BottomRight: 36, BottomLeft: 0}, geometry.UniformCornerRadius(1)} {
			d := new(testDrawer)
			p, err := NewPainter(d)
			if err != nil {
				t.Fatal(err)
			}
			rect := geometry.Rect(4, 4, 48, 40)
			img, err := p.RenderImage(int(64*scale), int(54*scale), scale, func() {
				p.Clear(graphics.Color{})
				p.FillPath(graphics.RoundRectBorderPath(rect, radius, 6), graphics.RGBA(200, 80, 40, 128))
			})
			if err != nil {
				t.Fatal(err)
			}
			outer := radius.Normalize(rect.Size)
			inner := rect.Inset(6)
			inside := outer.Inset(6).Normalize(inner.Size)
			mask := func(x, y float32) bool {
				pt := geometry.Point{X: x, Y: y}
				return outer.Contains(rect, pt) && !inside.Contains(inner, pt)
			}
			// 只对离真实几何边界至少0.75物理像素的位置断言满覆盖／零覆盖；
			// AA带内仍检查预乘及不重复合成，不把后端AA曲线压成金图。
			for y := 0; y < img.Bounds().Dy(); y++ {
				for x := 0; x < img.Bounds().Dx(); x++ {
					px, py := (float32(x)+.5)/scale, (float32(y)+.5)/scale
					value := mask(px, py)
					stable := true
					for _, dx := range []float32{-.75 / scale, 0, .75 / scale} {
						for _, dy := range []float32{-.75 / scale, 0, .75 / scale} {
							stable = stable && mask(px+dx, py+dy) == value
						}
					}
					pixel := color.RGBAModel.Convert(img.At(x, y)).(color.RGBA)
					if pixel.A > 129 || pixel.R > pixel.A || pixel.G > pixel.A || pixel.B > pixel.A {
						t.Fatal("重复合成或非预乘像素", pixel)
					}
					if stable && ((value && pixel.A < 126) || (!value && pixel.A > 2)) {
						t.Fatalf("比例%g 圆角%+v 像素%d,%d：覆盖错误 %v 预期%v", scale, radius, x, y, pixel, value)
					}
				}
			}
			p.Destroy()
		}
	}
}
