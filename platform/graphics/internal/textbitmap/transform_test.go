package textbitmap

import (
	"math"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
)

func TestRasterScaleFollowsMaximumTransformStretch(t *testing.T) {
	tests := []struct {
		name        string
		deviceScale float32
		transform   geometry.Transform
		want        float32
	}{
		{name: "identity", deviceScale: 2, transform: geometry.Identity(), want: 2},
		{name: "translation", deviceScale: 1.25, transform: geometry.Translate(30, 40), want: 1.25},
		{name: "uniform scale", deviceScale: 2, transform: geometry.Scale(1.5, 1.5), want: 3},
		{name: "rotation", deviceScale: 1.5, transform: geometry.Rotate(37), want: 1.5},
		{name: "rotated non-uniform scale", deviceScale: 1, transform: geometry.Rotate(23).Scale(0.5, 2), want: 2},
		{name: "reflection", deviceScale: 1, transform: geometry.Scale(-3, 2), want: 3},
		{name: "shear", deviceScale: 1, transform: geometry.Transform{A11: 1, A12: 1, A22: 1}, want: float32((math.Sqrt(5) + 1) / 2)},
		{name: "collapsed", deviceScale: 1, transform: geometry.Transform{}, want: 0},
		{name: "invalid device scale", deviceScale: -1, transform: geometry.Identity(), want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RasterScale(tt.deviceScale, tt.transform)
			if math.Abs(float64(got-tt.want)) > 1e-5 {
				t.Fatalf("RasterScale(%v, %+v) = %v, want %v", tt.deviceScale, tt.transform, got, tt.want)
			}
		})
	}
}

func TestRasterScaleRejectsNonFiniteValues(t *testing.T) {
	nan := float32(math.NaN())
	inf := float32(math.Inf(1))
	if got := RasterScale(nan, geometry.Identity()); got != 0 {
		t.Fatalf("RasterScale(NaN, identity) = %v, want 0", got)
	}
	if got := RasterScale(1, geometry.Transform{A11: inf, A22: 1}); got != 0 {
		t.Fatalf("RasterScale(1, infinite transform) = %v, want 0", got)
	}
}

func TestSnapOriginUsesFinalDeviceSpace(t *testing.T) {
	deviceScale := float32(2)
	transform := geometry.Translate(10.25, 20.375).Rotate(27).Scale(1.5, 0.75)
	origin := geometry.Point{X: 2.3, Y: 4.7}

	snapped := SnapOrigin(origin, transform, deviceScale)
	deviceTransform := geometry.Scale(deviceScale, deviceScale).Multiply(transform)
	before := deviceTransform.TransformPoint(origin)
	after := deviceTransform.TransformPoint(snapped)

	if math.Abs(float64(after.X)-math.Round(float64(after.X))) > 1e-4 ||
		math.Abs(float64(after.Y)-math.Round(float64(after.Y))) > 1e-4 {
		t.Fatalf("snapped device origin = %v, want integer coordinates", after)
	}
	if math.Abs(float64(after.X-before.X)) > 0.5001 || math.Abs(float64(after.Y-before.Y)) > 0.5001 {
		t.Fatalf("snap moved device origin too far: before %v, after %v", before, after)
	}
}

func TestSnapOriginLeavesNonInvertibleTransformAlone(t *testing.T) {
	origin := geometry.Point{X: 2.3, Y: 4.7}
	got := SnapOrigin(origin, geometry.Scale(0, 1), 1)
	if got != origin {
		t.Fatalf("SnapOrigin with non-invertible transform = %v, want %v", got, origin)
	}
}
