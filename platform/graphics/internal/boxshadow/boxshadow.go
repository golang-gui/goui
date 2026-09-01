// Package boxshadow contains the backend-independent geometry and opacity
// semantics for graphics.BoxShadow. It is internal so implementation details
// such as influence bounds do not become public API.
package boxshadow

import (
	"math"

	"github.com/golang-gui/goui/core/geometry"
)

const (
	minimumBlurRadius = float32(0.5)
	gaussianCutoff    = float32(3)
	cornerSamples     = 16
)

// Shape is a normalized rounded-rectangle shadow contour.
type Shape struct {
	Rect       geometry.Rectangle
	Radius     float32
	BlurRadius float32
}

// Normalize applies offset and spread, clamps the resulting corner radius and
// normalizes every positive blur to at least half a logical unit.
func Normalize(rect geometry.Rectangle, radius float32, offset geometry.Point, blurRadius, spreadRadius float32) (Shape, bool) {
	if !finite(rect.X) || !finite(rect.Y) || !finite(rect.Width) || !finite(rect.Height) ||
		!finite(radius) || !finite(offset.X) || !finite(offset.Y) ||
		!finite(blurRadius) || !finite(spreadRadius) || rect.Width <= 0 || rect.Height <= 0 {
		return Shape{}, false
	}

	rect.X += offset.X - spreadRadius
	rect.Y += offset.Y - spreadRadius
	rect.Width += 2 * spreadRadius
	rect.Height += 2 * spreadRadius
	if !finite(rect.X) || !finite(rect.Y) || !finite(rect.Width) || !finite(rect.Height) || rect.Width <= 0 || rect.Height <= 0 {
		return Shape{}, false
	}

	radius += spreadRadius
	if !finite(radius) {
		return Shape{}, false
	}
	if radius < 0 {
		radius = 0
	}
	if limit := min(rect.Width, rect.Height) / 2; radius > limit {
		radius = limit
	}
	if blurRadius > 0 && blurRadius < minimumBlurRadius {
		blurRadius = minimumBlurRadius
	} else if blurRadius < 0 {
		blurRadius = 0
	}

	return Shape{Rect: rect, Radius: radius, BlurRadius: blurRadius}, true
}

// Sigma maps the public CSS-like blur radius to the Gaussian standard
// deviation used by the renderers.
func (s Shape) Sigma() float32 { return s.BlurRadius / 2 }

// Extent is the finite support used for rendering. A Gaussian is effectively
// transparent beyond three standard deviations.
func (s Shape) Extent() float32 { return gaussianCutoff * s.Sigma() }

// Bounds returns the finite region in which the shadow is evaluated.
func (s Shape) Bounds() geometry.Rectangle {
	b := s.Extent()
	return geometry.Rect(s.Rect.X-b, s.Rect.Y-b, s.Rect.Width+2*b, s.Rect.Height+2*b)
}

// SignedDistance returns the distance to the normalized rounded-rectangle
// contour. It is retained for clear shadows and geometry diagnostics; blurred
// coverage is computed by convolving the complete finite shape instead of by
// fading this distance.
func (s Shape) SignedDistance(point geometry.Point) float32 {
	halfWidth := s.Rect.Width / 2
	halfHeight := s.Rect.Height / 2
	center := s.Rect.Center()
	qx := abs(point.X-center.X) - halfWidth + s.Radius
	qy := abs(point.Y-center.Y) - halfHeight + s.Radius
	outside := float32(math.Hypot(float64(max(qx, 0)), float64(max(qy, 0))))
	inside := min(max(qx, qy), 0)
	return outside + inside - s.Radius
}

// Alpha returns the Gaussian convolution of the rounded-rectangle mask at a
// point. Rectangle coverage is integrated analytically; the four rounded
// corner cut-outs use fixed midpoint quadrature. This is also the reference
// implementation for the OpenGL shader.
func (s Shape) Alpha(point geometry.Point) float32 {
	if s.BlurRadius <= 0 {
		if s.SignedDistance(point) <= 0 {
			return 1
		}
		return 0
	}
	bounds := s.Bounds()
	if point.X < bounds.X || point.X > bounds.X+bounds.Width || point.Y < bounds.Y || point.Y > bounds.Y+bounds.Height {
		return 0
	}

	sigma := float64(s.Sigma())
	center := s.Rect.Center()
	px := float64(point.X - center.X)
	py := float64(point.Y - center.Y)
	ex := float64(s.Rect.Width / 2)
	ey := float64(s.Rect.Height / 2)
	radius := float64(s.Radius)

	coverage := gaussianRange(-ex, ex, px, sigma) * gaussianRange(-ey, ey, py, sigma)
	if radius > 0 {
		for _, sx := range [...]float64{-1, 1} {
			for _, sy := range [...]float64{-1, 1} {
				coverage -= cornerCutout(px, py, ex, ey, radius, sigma, sx, sy)
			}
		}
	}
	return float32(clamp64(coverage, 0, 1))
}

// AlphaAtDistance is the exact one-dimensional Gaussian half-plane response.
// Positive distance is outside the shape.
func AlphaAtDistance(distance, blurRadius float32) float32 {
	if blurRadius <= 0 {
		if distance <= 0 {
			return 1
		}
		return 0
	}
	sigma := float64(blurRadius / 2)
	return float32(normalCDF(-float64(distance) / sigma))
}

func cornerCutout(px, py, ex, ey, radius, sigma, sx, sy float64) float64 {
	cx := sx * (ex - radius)
	cy := sy * (ey - radius)
	x0, x1 := cx, sx*ex
	y0, y1 := cy, sy*ey
	if x0 > x1 {
		x0, x1 = x1, x0
	}
	if y0 > y1 {
		y0, y1 = y1, y0
	}
	cutoff := float64(gaussianCutoff) * sigma
	if px < x0-cutoff || px > x1+cutoff || py < y0-cutoff || py > y1+cutoff {
		return 0
	}
	dx := radius / cornerSamples
	sum := 0.0
	for i := 0; i < cornerSamples; i++ {
		u := (float64(i) + 0.5) * dx
		x := cx + sx*u
		arc := math.Sqrt(math.Max(radius*radius-u*u, 0))
		y0 := cy + sy*arc
		y1 := sy * ey
		if y0 > y1 {
			y0, y1 = y1, y0
		}
		sum += gaussianDensity(x-px, sigma) * gaussianRange(y0, y1, py, sigma)
	}
	return sum * dx
}

func gaussianRange(lo, hi, center, sigma float64) float64 {
	return normalCDF((hi-center)/sigma) - normalCDF((lo-center)/sigma)
}

func gaussianDensity(distance, sigma float64) float64 {
	z := distance / sigma
	return math.Exp(-0.5*z*z) / (math.Sqrt(2*math.Pi) * sigma)
}

func normalCDF(v float64) float64 { return 0.5 * (1 + erfApprox(v/math.Sqrt2)) }

// erfApprox matches the compact approximation used by the NanoVG shader.
func erfApprox(v float64) float64 {
	sign := 1.0
	if v < 0 {
		sign = -1
		v = -v
	}
	q := 1 + (0.278393+(0.230389+0.078108*v*v)*v)*v
	q *= q
	return sign * (1 - 1/(q*q))
}

func clamp64(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func abs(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}

func finite(v float32) bool       { return !float32IsNaN(v) && !float32IsInf(v) }
func float32IsNaN(v float32) bool { return v != v }
func float32IsInf(v float32) bool { return v > math.MaxFloat32 || v < -math.MaxFloat32 }
