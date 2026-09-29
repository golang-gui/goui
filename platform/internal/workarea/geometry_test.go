package workarea

import (
	"github.com/golang-gui/goui/core/geometry"
	"math"
	"testing"
)

func TestNearest(t *testing.T) {
	displays := []geometry.Rectangle{geometry.Rect(-200, 40, 100, 100), geometry.Rect(0, 0, 200, 100)}
	for _, tc := range []struct {
		p    geometry.Point
		want int
	}{
		{geometry.Point{-150, 80}, 0}, {geometry.Point{10, 10}, 1},
		{geometry.Point{-90, 80}, 0}, {geometry.Point{-10, 0}, 1}, {geometry.Point{500, 200}, 1},
	} {
		if got := Nearest(displays, tc.p); got != tc.want {
			t.Fatalf("%v: %d, want %d", tc.p, got, tc.want)
		}
	}
	if Nearest(nil, geometry.Point{}) != -1 || Nearest([]geometry.Rectangle{{}}, geometry.Point{}) != -1 {
		t.Fatal("accepted absent displays")
	}
	if ValidatePoint(geometry.Point{X: float32(math.NaN())}) == nil {
		t.Fatal("accepted NaN")
	}
}

func TestNativePosition(t *testing.T) {
	for _, tc := range []struct {
		x, y         float64
		wantX, wantY int32
	}{
		{0, 0, 0, 0}, {-120.5, 90.5, -121, 91},
		{100 + 12.25*2, -80 + 30.75*2, 125, -19},
		{math.MinInt32, math.MaxInt32, math.MinInt32, math.MaxInt32},
	} {
		x, y, err := NativePosition(tc.x, tc.y)
		if err != nil || x != tc.wantX || y != tc.wantY {
			t.Fatalf("(%v,%v): got (%d,%d), %v", tc.x, tc.y, x, y, err)
		}
	}
	for _, invalid := range []float64{math.NaN(), math.Inf(1), math.Inf(-1), math.MaxInt32 + 1, math.MinInt32 - 1} {
		if _, _, err := NativePosition(invalid, 0); err == nil {
			t.Fatalf("accepted x=%v", invalid)
		}
		if _, _, err := NativePosition(0, invalid); err == nil {
			t.Fatalf("accepted y=%v", invalid)
		}
	}
}
