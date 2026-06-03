package geometry

type Rectangle struct {
	Pos
	Size
}

func Rect(x, y, w, h float32) Rectangle {
	return Rectangle{
		Pos:  Pos{X: x, Y: y},
		Size: Size{Width: w, Height: h},
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

func (r Rectangle) Inset(n float32) Rectangle {
	if r.Width < 2*n {
		r.X = r.X + r.Width/2
		r.Width = 0
	} else {
		r.X += n
		r.Width -= 2 * n
	}
	if r.Height < 2*n {
		r.Y = r.Y + r.Height/2
		r.Height = 0
	} else {
		r.Y += n
		r.Height -= 2 * n
	}
	return r
}

func (r Rectangle) Insets(left, top, right, bottom float32) Rectangle {
	horizontalInset := left + right
	if r.Width <= horizontalInset {
		r.X = r.X + r.Width/2
		r.Width = 0
	} else {
		r.X += left
		r.Width -= horizontalInset
	}

	verticalInset := top + bottom
	if r.Height <= verticalInset {
		r.Y = r.Y + r.Height/2
		r.Height = 0
	} else {
		r.Y += top
		r.Height -= verticalInset
	}

	return r
}

func (r Rectangle) Intersect(r2 Rectangle) Rectangle {
	x0 := max(r.X, r2.X)
	y0 := max(r.Y, r2.Y)
	x1 := min(r.X+r.Width, r2.X+r2.Width)
	y1 := min(r.Y+r.Height, r2.Y+r2.Height)
	if x0 >= x1 || y0 >= y1 {
		return Rectangle{}
	}
	return Rect(x0, y0, x1-x0, y1-y0)
}

func (r Rectangle) Scale(factor float32) Rectangle {
	return Rect(r.X*factor, r.Y*factor, r.Width*factor, r.Height*factor)
}
