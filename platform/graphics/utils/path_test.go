package utils

import (
	"math"
	"slices"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform/graphics"
)

func TestRoundRectInsetPaths(t *testing.T) {
	rect := geometry.Rect(10, 20, 80, 60)
	radius := geometry.UniformCornerRadius(12)
	for _, width := range []float32{1, 6, 12, 30, 90} {
		got := RoundRectBorderInsetsPath(rect, radius, geometry.UniformInsets(width))
		want := RoundRectBorderPath(rect, radius, width)
		if !slices.Equal(pathData(got), pathData(want)) {
			t.Fatal("统一内缩改变了原有环带", width)
		}
	}
	// 只内缩顶部4：TL内角中心仍为(22,32)，X半径12，Y半径8。
	p := RoundRectBorderInsetsPath(rect, radius, geometry.Insets{Top: 4})
	var inner []float32
	p.Range(func(op graphics.PathOperation, args []float32) bool {
		if op == graphics.PathHole {
			t.Fatal("缺边轮廓不能生成接触外轮廓的孔洞")
		} else if op == graphics.PathBezierTo && args[4] == 10 && args[5] == 32 {
			inner = slices.Clone(args)
		}
		return false
	})
	const k float32 = .5522847498307936
	want := []float32{22 - 12*k, 24, 10, 32 - 8*k, 10, 32}
	if len(inner) != len(want) {
		t.Fatal("缺少反向内轮廓", inner)
	}
	for i := range want {
		if math.Abs(float64(inner[i]-want[i])) > 1e-5 {
			t.Fatalf("内角控制点%d=%g，期望%g", i, inner[i], want[i])
		}
	}
	if !slices.Equal(pathData(RoundRectBorderInsetsPath(rect, radius, geometry.Insets{Top: 60})), pathData(RoundRectPath(rect, radius))) {
		t.Fatal("单边消耗完整高度后未填满外轮廓")
	}
	for _, insets := range []geometry.Insets{{Left: -1}, {Top: float32(math.NaN())}, {Bottom: float32(math.Inf(1))}, {Top: 3, Left: 4}} {
		if len(pathData(RoundRectBorderInsetsPath(rect, radius, insets))) != 0 || len(pathData(RoundRectInsetPath(rect, radius, insets))) != 0 {
			t.Fatal("无效内缩生成路径", insets)
		}
	}
	if len(pathData(RoundRectBorderInsetsPath(rect, radius, geometry.Insets{}))) != 0 || len(pathData(RoundRectInsetPath(rect, radius, geometry.Insets{Top: 60}))) != 0 {
		t.Fatal("空边集合或消失的内轮廓生成路径")
	}
}

func pathData(p graphics.Path) []float32 {
	var data []float32
	p.Range(func(op graphics.PathOperation, args []float32) bool {
		data = append(data, float32(op))
		data = append(data, args...)
		return false
	})
	return data
}

func TestRoundRectPaths(t *testing.T) {
	rect := geometry.Rect(10, 20, 80, 60)
	radius := geometry.CornerRadius{TopLeft: 12, BottomRight: 4, BottomLeft: 8}
	var starts []geometry.Point
	closes, holes, curves := 0, 0, 0
	p := RoundRectBorderPath(rect, radius, 3)
	p.Range(func(op graphics.PathOperation, args []float32) bool {
		switch op {
		case graphics.PathMoveTo:
			starts = append(starts, geometry.Point{X: args[0], Y: args[1]})
		case graphics.PathClose:
			closes++
		case graphics.PathHole:
			holes++
		case graphics.PathBezierTo:
			curves++
		}
		return false
	})
	if !slices.Equal(starts, []geometry.Point{{X: 22, Y: 20}, {X: 22, Y: 23}}) || closes != 2 || holes != 1 || curves != 6 {
		t.Fatalf("环带轮廓错误：起点%v 闭合%d 孔洞%d 曲线%d", starts, closes, holes, curves)
	}
	// 内轮廓消失时只留下外轮廓，不生成负尺寸轮廓。
	closes, holes = 0, 0
	RoundRectBorderPath(rect, radius, 30).Range(func(op graphics.PathOperation, _ []float32) bool {
		if op == graphics.PathClose {
			closes++
		}
		if op == graphics.PathHole {
			holes++
		}
		return false
	})
	if closes != 1 || holes != 0 {
		t.Fatal("内轮廓消失后仍生成孔洞", closes, holes)
	}
	for _, p := range []graphics.Path{
		RoundRectPath(geometry.Rectangle{}, radius),
		RoundRectPath(geometry.Rect(0, 0, float32(math.Inf(1)), 20), radius),
		RoundRectBorderPath(rect, radius, -1),
		RoundRectBorderPath(rect, radius, float32(math.NaN())),
	} {
		p.Range(func(graphics.PathOperation, []float32) bool {
			t.Fatal("无效输入生成了绘制指令")
			return true
		})
	}
}

func TestQuadBezierTo(t *testing.T) {
	path := graphics.MoveTo(1, 2).LineTo(4, 5).Close().MoveTo(12, 15).Hole()
	path = QuadBezierTo(path, 18, 21, 24, 12)
	var cubic []float32
	path.Range(func(op graphics.PathOperation, args []float32) bool {
		if op == graphics.PathBezierTo {
			cubic = args
		}
		return false
	})
	if len(cubic) != 6 {
		t.Fatal("没有生成三次贝塞尔指令")
	}
	// 独立比较二次和三次曲线的参数方程，误差只允许浮点运算舍入。
	for _, t0 := range []float64{0, .25, .5, .75, 1} {
		u := 1 - t0
		for axis, points := range [][3]float64{{12, 18, 24}, {15, 21, 12}} {
			want := u*u*points[0] + 2*u*t0*points[1] + t0*t0*points[2]
			got := u*u*u*points[0] + 3*u*u*t0*float64(cubic[axis]) + 3*u*t0*t0*float64(cubic[axis+2]) + t0*t0*t0*float64(cubic[axis+4])
			if math.Abs(got-want) > 1e-5 {
				t.Fatalf("t=%g 轴%d：曲线=%g，二次参考=%g", t0, axis, got, want)
			}
		}
	}
}
