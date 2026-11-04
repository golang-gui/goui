package utils

import "github.com/golang-gui/goui/platform/graphics"

// QuadBezierTo appends a quadratic Bezier segment converted to a cubic segment.
// The segment starts at path.CurrentPos and ends at x,y, with control point cx,cy.
func QuadBezierTo(path graphics.Path, cx, cy, x, y float32) graphics.Path {
	x0, y0 := path.CurrentPos()
	return path.BezierTo(
		x0+2.0/3.0*(cx-x0), y0+2.0/3.0*(cy-y0),
		x+2.0/3.0*(cx-x), y+2.0/3.0*(cy-y),
		x, y,
	)
}
