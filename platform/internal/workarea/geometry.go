// Package workarea contains geometry shared by native desktop observations.
package workarea

import (
	"fmt"
	"math"

	"github.com/golang-gui/goui/core/geometry"
)

func Finite(v float32) bool { return !math.IsNaN(float64(v)) && !math.IsInf(float64(v), 0) }

func ValidatePoint(p geometry.Point) error {
	if !Finite(p.X) || !Finite(p.Y) {
		return fmt.Errorf("invalid work area point: %v", p)
	}
	return nil
}

func Valid(r geometry.Rectangle) bool {
	return r.Width > 0 && r.Height > 0 && Finite(r.X) && Finite(r.Y) &&
		Finite(r.Width) && Finite(r.Height) && Finite(r.X+r.Width) && Finite(r.Y+r.Height)
}

// Nearest uses display bounds, not workarea bounds: panels are on the display.
// Invalid entries are ignored; -1 means that no valid display was observed.
func Nearest(displays []geometry.Rectangle, p geometry.Point) int {
	best, distance := -1, math.Inf(1)
	for i, r := range displays {
		if !Valid(r) {
			continue
		}
		if p.X >= r.X && p.X < r.X+r.Width && p.Y >= r.Y && p.Y < r.Y+r.Height {
			return i
		}
		dx := float64(p.X) - min(max(float64(p.X), float64(r.X)), float64(r.X)+float64(r.Width))
		dy := float64(p.Y) - min(max(float64(p.Y), float64(r.Y)), float64(r.Y)+float64(r.Height))
		if d := dx*dx + dy*dy; d < distance {
			best, distance = i, d
		}
	}
	return best
}
