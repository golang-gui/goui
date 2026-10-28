package widgets

import (
	"math"

	"github.com/golang-gui/goui/core/geometry"
)

func normalizeInsets(padding geometry.Insets) geometry.Insets {
	return geometry.Insets{
		Left:   normalizeInset(padding.Left),
		Top:    normalizeInset(padding.Top),
		Right:  normalizeInset(padding.Right),
		Bottom: normalizeInset(padding.Bottom),
	}
}

func normalizeInset(value float32) float32 {
	if value < 0 || math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) {
		return 0
	}
	return value
}
