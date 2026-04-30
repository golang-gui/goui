package graphics

type Path struct {
	data []float32
}

type PathOperation float32

const (
	PathMoveTo   PathOperation = iota // (x, y)
	PathLineTo                        // (x, y)
	PathArcTo                         // (rx, ry, xRotate, large, sweep, x, y)
	PathBezierTo                      // (c1x, c1y, c2x, c2y, x, y)
	PathClose                         // (0, 0)
)

func (op PathOperation) String() string {
	switch op {
	case PathMoveTo:
		return "MoveTo"
	case PathLineTo:
		return "LineTo"
	case PathArcTo:
		return "ArcTo"
	case PathClose:
		return "Close"
	default:
		return ""
	}
}

func MoveTo(x, y float32) (p Path) {
	p.data = append(p.data, float32(PathMoveTo), x, y)
	return
}

func (p Path) LineTo(x, y float32) Path {
	p.data = append(p.data, float32(PathLineTo), x, y)
	return p
}

func (p Path) ArcTo(rx, ry, xRotate, large, sweep, x, y float32) Path {
	p.data = append(p.data, float32(PathArcTo), rx, ry, xRotate, large, sweep, x, y)
	return p
}

func (p Path) BezierTo(c1x, c1y, c2x, c2y, x, y float32) Path {
	p.data = append(p.data, float32(PathBezierTo), c1x, c1y, c2x, c2y, x, y)
	return p
}

func (p Path) QuadBezierTo(cx, cy, x, y float32) Path {
	x0, y0 := p.CurrentPos()
	return p.BezierTo(
		x0+2.0/3.0*(cx-x0), y0+2.0/3.0*(cy-y0),
		x+2.0/3.0*(cx-x), y+2.0/3.0*(cy-y),
		x, y,
	)
}

func (p Path) Close() Path {
	p.data = append(p.data, float32(PathClose), 0, 0)
	return p
}

func (p Path) Range(f func(op PathOperation, args []float32) (stop bool)) {
	var args []float32
	for i := 0; i < len(p.data); i++ {
		op := PathOperation(p.data[i])
		switch op {
		case PathMoveTo, PathLineTo, PathClose:
			args = p.data[i+1 : i+3]
		case PathArcTo:
			args = p.data[i+1 : i+8]
		case PathBezierTo:
			args = p.data[i+1 : i+7]
		}
		if f(op, args) {
			return
		}
		i += len(args)
	}
}

func (p Path) CurrentPos() (x, y float32) {
	if length := len(p.data); length != 0 {
		return p.data[length-2], p.data[length-1]
	}
	return
}
