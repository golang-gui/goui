package textbitmap

import (
	"math"

	"github.com/golang-gui/goui/core/geometry"
)

// RasterScale returns the bitmap scale needed to avoid magnifying
// pre-rasterized text after an affine transform is applied.
func RasterScale(deviceScale float32, transform geometry.Transform) float32 {
	if deviceScale <= 0 || !finite(float64(deviceScale)) {
		return 0
	}

	stretch := float64(transform.MaxScale())
	rasterScale := float64(deviceScale) * stretch
	if rasterScale <= 0 || rasterScale > math.MaxFloat32 || !finite(rasterScale) {
		return 0
	}
	return float32(rasterScale)
}

// SnapOrigin aligns a text bitmap's transformed origin to physical pixels.
func SnapOrigin(origin geometry.Point, transform geometry.Transform, deviceScale float32) geometry.Point {
	if deviceScale <= 0 || !finite(float64(deviceScale)) {
		return origin
	}
	deviceTransform := geometry.Scale(deviceScale, deviceScale).Multiply(transform)
	det := float64(deviceTransform.A11)*float64(deviceTransform.A22) -
		float64(deviceTransform.A12)*float64(deviceTransform.A21)
	if math.Abs(det) < 1e-12 || !finite(det) {
		return origin
	}

	deviceOrigin := deviceTransform.TransformPoint(origin)
	if !finitePoint(deviceOrigin) {
		return origin
	}
	deviceOrigin.X = float32(math.Round(float64(deviceOrigin.X)))
	deviceOrigin.Y = float32(math.Round(float64(deviceOrigin.Y)))
	snapped := deviceTransform.Inverse().TransformPoint(deviceOrigin)
	if !finitePoint(snapped) {
		return origin
	}
	return snapped
}

func finitePoint(point geometry.Point) bool {
	return finite(float64(point.X)) && finite(float64(point.Y))
}

func finite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}
