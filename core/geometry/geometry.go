package geometry

type Point struct {
	X float32
	Y float32
}

type Pos = Point

func (p Point) Add(q Point) Point {
	return Point{
		X: p.X + q.X,
		Y: p.Y + q.Y,
	}
}

func (p Point) Scale(factor float32) Point {
	return Point{
		X: p.X * factor,
		Y: p.Y * factor,
	}
}

type Size struct {
	Width  float32
	Height float32
}

func (s Size) Add(q Size) Size {
	return Size{
		Width:  s.Width + q.Width,
		Height: s.Height + q.Height,
	}
}

func (s Size) Scale(factor float32) Size {
	return Size{
		Width:  s.Width * factor,
		Height: s.Height * factor,
	}
}
