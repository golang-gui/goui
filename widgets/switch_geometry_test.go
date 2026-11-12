package widgets

import (
	"fmt"
	"image/color"
	"math"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/platform/graphics/software"
	"github.com/golang-gui/goui/style"
)

func TestSwitchThumbUsesPaintedTrack(t *testing.T) {
	// 红色轨道不含蓝通道；透明底上的预乘 RGBA8 蓝通道单独表示滑块覆盖率。
	// 用像素面积重心验证圆心，允许 0.03 物理像素的路径/定点量化误差，
	// 不用取整后的包围盒代替重心，否则会漏掉半像素偏移。
	rules := []style.Rule{}
	for _, suffix := range []string{"", "-checked"} {
		rules = append(rules,
			style.Name("switch").Part("track"+suffix).BackgroundColor(color.RGBA{R: 255, A: 255}).BorderWidth(0).Radius(10),
			style.Name("switch").Part("thumb"+suffix).BackgroundColor(color.RGBA{B: 255, A: 255}).BorderWidth(0),
		)
	}
	useProgressApplication(t, &progressTestApplication{sheet: style.Sheet(rules...)})
	for _, scale := range []float32{1, 1.25, 1.5, 2} {
		for _, height := range []float32{32, 33, 29.5} {
			for _, offset := range []float32{0, .25, .5, .75} {
				t.Run(fmt.Sprintf("scale=%g/height=%g/offset=%g", scale, height, offset), func(t *testing.T) {
					out := new(progressPixelSurface)
					p, err := software.NewPainter(out)
					if err != nil {
						t.Fatal(err)
					}
					defer p.Destroy()
					b := NewSwitch()
					b.SetAnimated(false)
					b.Arrange(geometry.Rect(0, 0, 48, height))
					// 独立公式：两端在窗口物理坐标取最近整数，再按最终轨道求圆心。
					factor := min(float64(1), float64(height-12)/20)
					edge := func(v float64) float64 { return math.Floor((v+float64(offset))*float64(scale) + .5) }
					left, right := edge(6), edge(6+36*factor)
					top := edge(6 + (float64(height)-12-20*factor)/2)
					bottom := edge(6 + (float64(height)-12+20*factor)/2)
					cy, diameter := (top+bottom)/2, (bottom-top)*.8
					for _, position := range []float32{0, .25, .5, .75, 1} {
						b.SetChecked(position >= .5)
						b.position = position // 包内绘制回归覆盖过渡/拖动的中间位置。
						p.Begin(64*scale, 48*scale, scale)
						p.Clear(graphics.Color{})
						p.SetTransform(geometry.Translate(offset, offset))
						p.SetClipRect(geometry.Rect(0, 0, 64, 48))
						b.Paint(checkPixelPainter{native: p})
						p.End()
						var mass, mx, my, xx, yy float64
						for y := 0; y < out.pixels.Bounds().Dy(); y++ {
							for x := 0; x < out.pixels.Bounds().Dx(); x++ {
								w := float64(out.pixels.RGBAAt(x, y).B) / 255
								px, py := float64(x)+.5, float64(y)+.5
								mass += w
								mx, my = mx+px*w, my+py*w
								xx, yy = xx+px*px*w, yy+py*py*w
							}
						}
						if mass == 0 {
							t.Fatal("滑块没有绘制")
						}
						cx := left + (bottom-top)/2 + float64(position)*(right-left-(bottom-top))
						gx, gy := mx/mass, my/mass
						if math.Abs(gx-cx) > .03 || math.Abs(gy-cy) > .03 {
							t.Fatalf("position=%g 滑块重心=(%.4f,%.4f)，最终轨道要求=(%.4f,%.4f)", position, gx, gy, cx, cy)
						}
						// 圆盘每轴方差为 r²/4；像素面积积分允许 0.2 px² 量化差异。
						wantVariance := diameter * diameter / 16
						for _, variance := range []float64{xx/mass - gx*gx, yy/mass - gy*gy} {
							if math.Abs(variance-wantVariance) > .2 {
								t.Fatalf("滑块圆形/大小改变：variance=%.4f，预期 %.4f", variance, wantVariance)
							}
						}
					}
				})
			}
		}
	}
}
