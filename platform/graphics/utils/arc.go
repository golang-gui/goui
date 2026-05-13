package utils

import (
	"github.com/goexlib/mathx"
	"math"
)

type LineTo func(x, y float32)

type BezierTo func(x0, y0, x1, y1, x2, y2 float32)

func ArcTo(lineTo LineTo, bezierTo BezierTo, sx, sy, rx, ry, angle, large, sweep, ex, ey float32) {
	rx = mathx.Abs(rx)
	ry = mathx.Abs(ry)
	if rx < 0.1 || ry < 0.1 {
		lineTo(ex, ey)
		return
	}

	const PI = float32(math.Pi)

	phi := angle * (PI / 180)
	dx := (sx - ex) * 0.5
	dy := (sy - ey) * 0.5
	x1p := mathx.Cos(phi)*dx + mathx.Sin(phi)*dy
	y1p := -mathx.Sin(phi)*dx + mathx.Cos(phi)*dy

	lambda := (x1p*x1p)/(rx*rx) + (y1p*y1p)/(ry*ry)
	if lambda > 1 {
		scale := mathx.Sqrt(lambda)
		rx *= scale
		ry *= scale
	}

	sign := float32(1)
	if large == sweep {
		sign = -1
	}
	sq := (rx*rx*ry*ry - rx*rx*y1p*y1p - ry*ry*x1p*x1p) / (rx*rx*y1p*y1p + ry*ry*x1p*x1p)
	if sq < 0 {
		sq = 0
	}
	coef := sign * mathx.Sqrt(sq)
	cxp := coef * (rx * y1p / ry)
	cyp := coef * -(ry * x1p / rx)

	mx := (sx + ex) * 0.5
	my := (sy + ey) * 0.5
	cx := mathx.Cos(phi)*cxp - mathx.Sin(phi)*cyp + mx
	cy := mathx.Sin(phi)*cxp + mathx.Cos(phi)*cyp + my

	ux := (x1p - cxp) / rx
	uy := (y1p - cyp) / ry
	vx := (-x1p - cxp) / rx
	vy := (-y1p - cyp) / ry
	theta1 := mathx.Atan2(uy, ux)
	dtheta := mathx.Atan2(vy, vx) - theta1

	if sweep != 0 && dtheta < 0 {
		dtheta += 2.0 * PI
	} else if sweep == 0 && dtheta > 0 {
		dtheta -= 2.0 * PI
	}

	segs := mathx.Ceil(mathx.Abs(dtheta) / (PI / 2))
	if segs < 1 {
		segs = 1
	}
	d := dtheta / segs
	t := theta1

	for i := 0; i < int(segs); i++ {
		t2 := t + d

		cosT, sinT := mathx.Cos(t), mathx.Sin(t)
		cosT2, sinT2 := mathx.Cos(t2), mathx.Sin(t2)
		p0x, p0y := rx*cosT, ry*sinT
		p3x, p3y := rx*cosT2, ry*sinT2

		k := (4.0 / 3.0) * mathx.Tan((t2-t)/4.0)
		p1x := p0x + k*(-rx*sinT)
		p1y := p0y + k*(ry*cosT)
		p2x := p3x - k*(-rx*sinT2)
		p2y := p3y - k*(ry*cosT2)

		cp1x := mathx.Cos(phi)*p1x - mathx.Sin(phi)*p1y + cx
		cp1y := mathx.Sin(phi)*p1x + mathx.Cos(phi)*p1y + cy
		cp2x := mathx.Cos(phi)*p2x - mathx.Sin(phi)*p2y + cx
		cp2y := mathx.Sin(phi)*p2x + mathx.Cos(phi)*p2y + cy
		px := mathx.Cos(phi)*p3x - mathx.Sin(phi)*p3y + cx
		py := mathx.Sin(phi)*p3x + mathx.Cos(phi)*p3y + cy

		bezierTo(cp1x, cp1y, cp2x, cp2y, px, py)
		t = t2
	}
}
