package gui

import (
	"fmt"
	"math"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform/graphics/software"
)

func TestPainterSnapRectTracksWidgetScope(t *testing.T) {
	// 使用真实 Software Painter，验证 GUI 局部坐标到后端像素网格的完整边界。
	for _, scale := range []float32{1, 1.25, 2} {
		for _, transform := range []geometry.Transform{
			geometry.Identity(), geometry.Translate(.25, -.5),
			geometry.Rotate(90), geometry.Scale(1.2, .8),
			{A11: 1, A12: .2, A22: 1},
		} {
			t.Run(fmt.Sprintf("scale=%g/transform=%v", scale, transform), func(t *testing.T) {
				backend, err := software.NewPainter(&widgetRenderSurface{})
				if err != nil {
					t.Fatal(err)
				}
				defer backend.Destroy()
				backend.Begin(120*scale, 100*scale, scale)
				defer backend.End()
				r := geometry.Rect(.3, .7, 36.2, 20.4)
				want := r
				if transform.A11 == 1 && transform.A22 == 1 && transform.A12 == 0 && transform.A21 == 0 {
					// 父 (4.25,8.5) + 子 (10.25,4.5) + 子自身的用户平移。
					edge := func(v, offset float32) float32 {
						return float32(math.Floor(float64((v+offset)*scale)+.5))/scale - offset
					}
					x, y := float32(14.5)+transform.TX, float32(13)+transform.TY
					left, top := edge(r.X, x), edge(r.Y, y)
					want = geometry.Rect(left, top, edge(r.X+r.Width, x)-left, edge(r.Y+r.Height, y)-top)
				}
				painted := false
				child := newPainterTestWidget(func(p Painter) {
					painted = true
					p.SetTransform(transform)
					check := func() {
						t.Helper()
						if got := p.SnapRect(r); got != want || p.SnapRect(got) != got {
							t.Fatalf("局部对齐/幂等错误：got=%v want=%v", got, want)
						}
					}
					check()
					p.Save()
					p.SetClipRect(geometry.Rect(1000, 1000, 1, 1))
					check() // 空 clip 不应改变纯几何查询或变换。
					p.SetTransform(geometry.Rotate(15))
					if p.SnapRect(r) != r {
						t.Fatal("用户旋转被量化")
					}
					p.Restore()
					check()
				})
				parent := newPainterTestWidget(nil)
				parent.Arrange(geometry.Rect(4.25, 8.5, 100, 80))
				child.Arrange(geometry.Rect(10.25, 4.5, 64, 48))
				parent.AddChild(child)
				paintWidget(parent, newPainter(backend, geometry.Rect(0, 0, 120, 100), scale))
				if !painted {
					t.Fatal("没有经过实际控件绘制作用域")
				}
			})
		}
	}
}
