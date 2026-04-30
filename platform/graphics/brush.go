package graphics

type Brush interface {
	isBrush()
}

type Color struct {
	R, G, B, A float32
}

func RGBA(r, g, b, a byte) Color {
	return Color{
		R: float32(r) / 255,
		G: float32(g) / 255,
		B: float32(b) / 255,
		A: float32(a) / 255,
	}
}

func (c Color) isBrush() {}

func (c Color) RGBA() (r, g, b, a uint32) {
	r = uint32(c.R * 255)
	r |= r << 8
	g = uint32(c.G * 255)
	g |= g << 8
	b = uint32(c.B * 255)
	b |= b << 8
	a = uint32(c.A * 255)
	a |= a << 8
	return
}

func (c Color) RGBA8() (r, g, b, a uint8) {
	r = uint8(c.R * 255)
	g = uint8(c.G * 255)
	b = uint8(c.B * 255)
	a = uint8(c.A * 255)
	return
}
