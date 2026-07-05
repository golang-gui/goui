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

// Inset shrinks the size by n on every edge (2n per axis), clamping each axis
// at 0. Pass a negative n to outset (grow); growth is not clamped.
func (s Size) Inset(n float32) Size {
	s.Width -= 2 * n
	if s.Width < 0 {
		s.Width = 0
	}
	s.Height -= 2 * n
	if s.Height < 0 {
		s.Height = 0
	}
	return s
}
