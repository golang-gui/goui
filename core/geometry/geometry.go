package geometry

type Point struct {
	X float32 `json:"x"`
	Y float32 `json:"y"`
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
	Width  float32 `json:"width"`
	Height float32 `json:"height"`
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
