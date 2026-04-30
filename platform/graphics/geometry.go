package graphics

type Point struct {
	X, Y float32
}

type Pos = Point

func (p Point) Add(q Point) Point {
	return Point{p.X + q.X, p.Y + q.Y}
}

type Size struct {
	Width, Height float32
}

type Rectangle struct {
	Pos
	Size
}

func Rect(x, y, w, h float32) Rectangle {
	return Rectangle{
		Pos: Point{
			X: x,
			Y: y,
		},
		Size: Size{
			Width:  w,
			Height: h,
		},
	}
}

func (r Rectangle) LeftTop() Point {
	return r.Pos
}

func (r Rectangle) RightTop() Point {
	return Point{
		X: r.X + r.Width,
		Y: r.Y,
	}
}

func (r Rectangle) RightBottom() Point {
	return Point{
		X: r.X + r.Width,
		Y: r.Y + r.Height,
	}
}

func (r Rectangle) LeftBottom() Point {
	return Point{
		X: r.X,
		Y: r.Y + r.Height,
	}
}

func (r Rectangle) Center() Point {
	return Point{
		X: r.X + r.Width/2,
		Y: r.Y + r.Height/2,
	}
}

func (r Rectangle) Contains(r2 Rectangle) bool {
	return r.Pos.X <= r2.Pos.X && r.Pos.Y <= r2.Pos.Y &&
		r.Width >= r2.Width && r.Height >= r2.Height
}
