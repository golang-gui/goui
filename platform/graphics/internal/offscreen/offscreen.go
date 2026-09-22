// Package offscreen holds the common allocation contract for image rendering.
package offscreen

import (
	"fmt"
	"math"
)

func Validate(width, height int, scale float32, draw func()) error {
	if width <= 0 || height <= 0 || width > 16384 || height > 16384 || int64(width)*int64(height) > 64*1024*1024 {
		return fmt.Errorf("graphics: invalid offscreen size %dx%d", width, height)
	}
	if scale <= 0 || math.IsNaN(float64(scale)) || math.IsInf(float64(scale), 0) {
		return fmt.Errorf("graphics: invalid offscreen scale %g", scale)
	}
	if draw == nil {
		return fmt.Errorf("graphics: nil offscreen draw function")
	}
	return nil
}
