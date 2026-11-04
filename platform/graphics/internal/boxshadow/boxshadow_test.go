package boxshadow

import (
	"math"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
)

func TestIndependentCornersAndNarrowGaussian(t *testing.T) {
	for _, r := range []geometry.CornerRadius{{TopLeft: 20, TopRight: 0, BottomRight: 4, BottomLeft: 12}, {TopLeft: 160, TopRight: 0, BottomRight: 160, BottomLeft: 0}, {TopLeft: 150, TopRight: 20, BottomRight: 0, BottomLeft: 5}} {
		s, ok := Normalize(geometry.Rect(0, 0, 180, 170), r, geometry.Point{}, 2, 0)
		if !ok {
			t.Fatal("有效四角被拒绝")
		}
		for _, point := range []geometry.Point{{X: .5, Y: .5}, {X: 179.5, Y: .5}, {X: 179.5, Y: 169.5}, {X: .5, Y: 169.5}, {X: 12, Y: 12}, {X: 45, Y: 45}, {X: 90, Y: 85}, {X: 7, Y: 130}, {X: 130, Y: 7}, {X: -1, Y: 85}} {
			want := referenceCoverage(s, point)
			if got := float64(s.Alpha(point)); math.Abs(got-want) > .008 {
				t.Fatalf("圆角%+v 位置%+v：高斯覆盖%g，独立积分%g", r, point, got, want)
			}
		}
	}
	r := geometry.CornerRadius{TopLeft: 20, TopRight: 6, BottomRight: 2}
	s, ok := Normalize(geometry.Rect(0, 0, 100, 80), r, geometry.Point{X: 3, Y: 4}, 4, -2)
	if !ok || s.Rect != geometry.Rect(5, 6, 96, 76) || s.Radius != (geometry.CornerRadius{TopLeft: 18, TopRight: 4}) {
		t.Fatal("Spread／Offset没有逐角生效", s)
	}
}

// 独立参考：按竖向截面求圆弧交区，使用标准库 Erf 和4096点积分；
// 不调用角缺口、SignedDistance 或生产近似误差函数。
func referenceCoverage(s Shape, p geometry.Point) float64 {
	sigma := float64(s.Sigma())
	width, height := float64(s.Rect.Width), float64(s.Rect.Height)
	px, py := float64(p.X-s.Rect.X), float64(p.Y-s.Rect.Y)
	lo, hi := math.Max(0, px-6*sigma), math.Min(width, px+6*sigma)
	if hi <= lo {
		return 0
	}
	dx := (hi - lo) / 4096
	sum := 0.0
	for i := 0; i < 4096; i++ {
		x := lo + (float64(i)+.5)*dx
		top, bottom := 0.0, height
		for j, radius := range []float32{s.Radius.TopLeft, s.Radius.TopRight, s.Radius.BottomRight, s.Radius.BottomLeft} {
			r := float64(radius)
			if r <= 0 {
				continue
			}
			cx := r
			if j == 1 || j == 2 {
				cx = width - r
			}
			outside := (j == 0 || j == 3) && x < r || (j == 1 || j == 2) && x > width-r
			if !outside {
				continue
			}
			arc := math.Sqrt(math.Max(0, r*r-(x-cx)*(x-cx)))
			if j < 2 {
				top = math.Max(top, r-arc)
			} else {
				bottom = math.Min(bottom, height-r+arc)
			}
		}
		if bottom <= top {
			continue
		}
		z := (x - px) / sigma
		y := .5 * (math.Erf((bottom-py)/(sigma*math.Sqrt2)) - math.Erf((top-py)/(sigma*math.Sqrt2)))
		sum += math.Exp(-.5*z*z) / (sigma * math.Sqrt(2*math.Pi)) * y * dx
	}
	return sum
}

func TestNormalizeAppliesOffsetSpreadAndRadius(t *testing.T) {
	shape, ok := Normalize(
		geometry.Rect(10, 20, 30, 20), geometry.UniformCornerRadius(8),
		geometry.Point{X: 3, Y: -2}, 0.1, 4,
	)
	if !ok {
		t.Fatal("Normalize rejected valid shadow")
	}
	if want := geometry.Rect(9, 14, 38, 28); shape.Rect != want {
		t.Fatalf("rect = %+v, want %+v", shape.Rect, want)
	}
	if shape.Radius != geometry.UniformCornerRadius(12) {
		t.Fatalf("radius = %v, want 12", shape.Radius)
	}
	if shape.BlurRadius != 0.5 {
		t.Fatalf("blur = %v, want 0.5", shape.BlurRadius)
	}
}

func TestNormalizeNegativeSpreadAndRadiusClamp(t *testing.T) {
	shape, ok := Normalize(geometry.Rect(0, 0, 20, 10), geometry.UniformCornerRadius(20), geometry.Point{}, 2, -3)
	if !ok {
		t.Fatal("Normalize rejected non-empty inset shadow")
	}
	if want := geometry.Rect(3, 3, 14, 4); shape.Rect != want {
		t.Fatalf("rect = %+v, want %+v", shape.Rect, want)
	}
	if shape.Radius != geometry.UniformCornerRadius(2) {
		t.Fatalf("radius = %v, want height/2 = 2", shape.Radius)
	}

	if _, ok := Normalize(geometry.Rect(0, 0, 4, 4), geometry.UniformCornerRadius(2), geometry.Point{}, 1, -2); ok {
		t.Fatal("Normalize accepted a contour contracted to empty")
	}
	if _, ok := Normalize(geometry.Rectangle{}, geometry.UniformCornerRadius(0), geometry.Point{}, 1, 0); ok {
		t.Fatal("Normalize accepted an empty source rectangle")
	}
}

func TestShapeBoundsDistanceAndAlpha(t *testing.T) {
	shape, ok := Normalize(geometry.Rect(10, 10, 20, 10), geometry.UniformCornerRadius(3), geometry.Point{}, 2, 0)
	if !ok {
		t.Fatal("Normalize rejected valid shadow")
	}
	if shape.Sigma() != 1 || shape.Extent() != 3 {
		t.Fatalf("sigma/extent = %v/%v, want 1/3", shape.Sigma(), shape.Extent())
	}
	if want := geometry.Rect(7, 7, 26, 16); shape.Bounds() != want {
		t.Fatalf("bounds = %+v, want %+v", shape.Bounds(), want)
	}

	tests := []struct {
		name  string
		point geometry.Point
		alpha float32
		tol   float32
	}{
		{name: "deep inside", point: geometry.Point{X: 20, Y: 15}, alpha: 1, tol: 0.001},
		{name: "one blur inside", point: geometry.Point{X: 20, Y: 12}, alpha: 0.97725, tol: 0.002},
		{name: "straight contour", point: geometry.Point{X: 20, Y: 10}, alpha: 0.5, tol: 0.002},
		{name: "three sigma outside", point: geometry.Point{X: 20, Y: 7}, alpha: 0.00135, tol: 0.001},
		{name: "outside finite support", point: geometry.Point{X: 20, Y: 6.99}, alpha: 0, tol: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shape.Alpha(tt.point); abs(got-tt.alpha) > tt.tol {
				t.Fatalf("alpha = %v, want %v (distance %v)", got, tt.alpha, shape.SignedDistance(tt.point))
			}
		})
	}
}

func TestAlphaAtDistanceUsesCSSGaussianMapping(t *testing.T) {
	tests := []struct {
		distance float32
		want     float32
	}{
		{-4, 0.97725},
		{0, 0.5},
		{4, 0.02275},
	}
	for _, tt := range tests {
		if got := AlphaAtDistance(tt.distance, 4); abs(got-tt.want) > 0.001 {
			t.Fatalf("AlphaAtDistance(%v, 4) = %v, want %v", tt.distance, got, tt.want)
		}
	}
}

func TestRoundedConvolutionIsSymmetricMonotonicAndAccurate(t *testing.T) {
	shape, ok := Normalize(geometry.Rect(0, 0, 18, 12), geometry.UniformCornerRadius(5), geometry.Point{}, 6, 0)
	if !ok {
		t.Fatal("Normalize rejected valid shadow")
	}
	center := shape.Rect.Center()
	previous := float32(2)
	for x := center.X; x <= shape.Bounds().X+shape.Bounds().Width; x += 0.5 {
		got := shape.Alpha(geometry.Point{X: x, Y: center.Y})
		mirror := shape.Alpha(geometry.Point{X: 2*center.X - x, Y: center.Y})
		if abs(got-mirror) > 1e-5 {
			t.Fatalf("asymmetric alpha at %v: %v != %v", x, got, mirror)
		}
		if got > previous+1e-5 {
			t.Fatalf("alpha increased away from center at %v: %v > %v", x, got, previous)
		}
		previous = got
	}

	// Compare the fixed 16-sample corner integration with a dense reference.
	px, py := float64(8), float64(5)
	ex, ey, radius, sigma := float64(9), float64(6), float64(5), float64(3)
	got := cornerCutout(px, py, ex, ey, radius, sigma, 1, 1)
	want := denseCornerCutout(px, py, ex, ey, radius, sigma, 1, 1, 65536)
	if math.Abs(got-want) > 0.01 {
		t.Fatalf("corner quadrature error = %v, got %v want %v", math.Abs(got-want), got, want)
	}
}

func denseCornerCutout(px, py, ex, ey, radius, sigma, sx, sy float64, samples int) float64 {
	cx := sx * (ex - radius)
	cy := sy * (ey - radius)
	dx := radius / float64(samples)
	sum := 0.0
	for i := 0; i < samples; i++ {
		u := (float64(i) + 0.5) * dx
		x := cx + sx*u
		arc := math.Sqrt(math.Max(radius*radius-u*u, 0))
		y0, y1 := cy+sy*arc, sy*ey
		if y0 > y1 {
			y0, y1 = y1, y0
		}
		sum += gaussianDensity(x-px, sigma) * gaussianRange(y0, y1, py, sigma)
	}
	return sum * dx
}

func TestClearShapeAlpha(t *testing.T) {
	shape, ok := Normalize(geometry.Rect(0, 0, 10, 10), geometry.UniformCornerRadius(0), geometry.Point{}, -4, 0)
	if !ok {
		t.Fatal("Normalize rejected valid clear shadow")
	}
	if got := shape.Alpha(geometry.Point{X: 10, Y: 5}); got != 1 {
		t.Fatalf("contour alpha = %v, want 1", got)
	}
	if got := shape.Alpha(geometry.Point{X: 10.01, Y: 5}); got != 0 {
		t.Fatalf("outside alpha = %v, want 0", got)
	}
}
