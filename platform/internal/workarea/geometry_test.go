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
