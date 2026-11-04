package geometry

import (
	"math"
	"testing"
)

func TestCornerRadius(t *testing.T) {
	r := CornerRadius{80, 40, 20, 0}.Normalize(Size{100, 100})
	if math.Abs(float64(r.TopLeft)-200.0/3) > 1e-5 || math.Abs(float64(r.TopRight)-100.0/3) > 1e-5 || r.BottomLeft != 0 {
		t.Fatal("没有按相邻角比例整体缩放", r)
	}
	if !UniformCornerRadius(8).Uniform() || (CornerRadius{TopLeft: 8}).Uniform() {
		t.Fatal("统一圆角判断错误")
	}
	invalid := CornerRadius{-1, float32(math.NaN()), float32(math.Inf(1)), 10}
	if got := invalid.Normalize(Size{100, 100}); got != (CornerRadius{BottomLeft: 10}) || invalid.Max() != 10 {
		t.Fatal("无效半径未归零", got)
	}
	if got := UniformCornerRadius(math.MaxFloat32).Normalize(Size{100, 100}); got != UniformCornerRadius(50) {
		t.Fatal("半径和溢出", got)
	}
	if UniformCornerRadius(8).Normalize(Size{}) != (CornerRadius{}) || UniformCornerRadius(8).Inset(10) != (CornerRadius{}) {
		t.Fatal("空尺寸或收缩错误")
	}
	// 左上角超过短边一半，不能按矩形中心象限选角。
	large := CornerRadius{TopLeft: 90}
	rect := Rect(10, 20, 100, 100)
	if large.Contains(rect, Point{11, 50}) || !large.Contains(rect, Point{100, 21}) || large.Contains(rect, Point{110, 50}) {
		t.Fatal("独立圆角命中错误")
	}
	if (CornerRadius{TopLeft: 90, BottomRight: 90}).Contains(Rect(0, 0, 100, 100), Point{85, 85}) {
		t.Fatal("重叠角包围区只检查了第一个圆角")
	}
}
