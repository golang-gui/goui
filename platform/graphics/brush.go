package graphics

import "image/color"

type Brush interface {
	isBrush()
}

// Color stores premultiplied RGBA channels in [0, 1]. RGB must not exceed A;
// transparent black is the only valid color with zero alpha. Struct literals
// and RGBA/RGBA8 results are premultiplied; RGBA constructs from straight bytes.
type Color struct {
	R, G, B, A float32
}

// ColorOf preserves the premultiplied components of a standard Go color.
func ColorOf(c color.Color) Color {
	r, g, b, a := c.RGBA()
	return Color{
		R: float32(r) / 65535,
		G: float32(g) / 65535,
		B: float32(b) / 65535,
		A: float32(a) / 65535,
	}
}

func RGB(r, g, b byte) Color {
	return Color{
		R: float32(r) / 255,
		G: float32(g) / 255,
		B: float32(b) / 255,
		A: 1.0,
	}
}

// RGBA constructs a premultiplied Color from straight (non-premultiplied) bytes.
func RGBA(r, g, b, a byte) Color {
	alpha := float32(a) / 255
	return Color{
		R: float32(r) / 255 * alpha,
		G: float32(g) / 255 * alpha,
		B: float32(b) / 255 * alpha,
		A: alpha,
	}
}

func (c Color) isBrush() {}

// LinearGradient interpolates from StartColor at Start to EndColor at End.
// Its points use the same local logical coordinate space as the geometry being
// drawn and are transformed together with that geometry. Colors are clamped at
// either end of the gradient line. Endpoints and their interpolation are
// premultiplied, including when the endpoints have different alpha values.
type LinearGradient struct {
	Start      Point
	End        Point
	StartColor Color
	EndColor   Color
}

func (LinearGradient) isBrush() {}

func (c Color) RGBA() (r, g, b, a uint32) {
	r = uint32(c.R*65535 + .5)
	g = uint32(c.G*65535 + .5)
	b = uint32(c.B*65535 + .5)
	a = uint32(c.A*65535 + .5)
	return
}

func (c Color) RGBA8() (r, g, b, a uint8) {
	r = uint8(c.R*255 + .5)
	g = uint8(c.G*255 + .5)
	b = uint8(c.B*255 + .5)
	a = uint8(c.A*255 + .5)
	return
}
