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
