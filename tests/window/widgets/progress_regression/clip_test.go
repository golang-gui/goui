package main

import (
	"math"
	"testing"
)

// 面积参考自身使用手算的整格、四分之一格、凹口和三角形校验。
func TestPolygonPixelAreaReference(t *testing.T) {
	polygon := [6]edgePoint{{0, 0}, {2, 0}, {2, 2}, {1, 2}, {1, 1}, {0, 1}}
	for _, tc := range []struct {
		x, y int
		want float64
	}{{0, 0, 1}, {1, 0, 1}, {1, 1, 1}, {0, 1, 0}, {-1, 0, 0}, {2, 2, 0}} {
		if got := polygonPixelArea(polygon, tc.x, tc.y); math.Abs(got-tc.want) > 1e-9 {
			t.Fatal("整格凹多边形面积错误", tc, got)
		}
	}
	for i := range polygon {
		polygon[i].x += .5
		polygon[i].y += .5
	}
	for _, tc := range []struct {
		x, y int
		want float64
	}{{0, 0, .25}, {1, 0, .5}, {1, 1, .75}, {0, 1, .25}, {2, 2, .25}} {
		if got := polygonPixelArea(polygon, tc.x, tc.y); math.Abs(got-tc.want) > 1e-9 {
			t.Fatal("小数位置面积错误", tc, got)
		}
	}
	triangle := [6]edgePoint{{1000, 1000}, {1001, 1000}, {1000, 1001}, {1000, 1001}, {1000, 1001}, {1000, 1000}}
	if got := polygonPixelArea(triangle, 1000, 1000); math.Abs(got-.5) > 1e-9 {
		t.Fatal("平移后三角形面积错误", got)
	}
}
