package graphics

import (
	"image/color"
	"testing"
)

func TestColorPremultipliedContract(t *testing.T) {
	for _, alpha := range []byte{0, 1, 64, 128, 192, 255} {
		c := RGBA(255, 100, 60, alpha)
		if c.R > c.A || c.G > c.A || c.B > c.A {
			t.Fatalf("RGBA returned non-premultiplied channels: %+v", c)
		}
		want := color.NRGBA{R: 255, G: 100, B: 60, A: alpha}
		wr, wg, wb, wa := want.RGBA()
		r, g, b, a := c.RGBA()
		for i, pair := range [][2]uint32{{r, wr}, {g, wg}, {b, wb}, {a, wa}} {
			if delta := int(pair[0]) - int(pair[1]); delta < -1 || delta > 1 {
				t.Fatalf("alpha=%d channel=%d: %d want %d", alpha, i, pair[0], pair[1])
			}
		}
		converted := ColorOf(want)
		r, g, b, a = converted.RGBA()
		if r != wr || g != wg || b != wb || a != wa {
			t.Fatalf("ColorOf lost premultiplied precision: %v", converted)
		}
		if ColorOf(c) != c && alpha == 255 {
			t.Fatal("opaque conversion changed color")
		}
	}
	if RGBA(255, 100, 60, 0) != (Color{}) {
		t.Fatal("zero-alpha constructor retained hidden RGB")
	}
}

func TestColorOfPreserves16BitPrecision(t *testing.T) {
	src := color.RGBA64{R: 1, G: 128, B: 1000, A: 2000}
	r, g, b, a := ColorOf(src).RGBA()
	if r != 1 || g != 128 || b != 1000 || a != 2000 {
		t.Fatalf("16-bit values quantized to bytes: %d %d %d %d", r, g, b, a)
	}
	r8, g8, b8, a8 := (Color{R: .5, G: .25, A: .5}).RGBA8()
	if r8 != 128 || g8 != 64 || b8 != 0 || a8 != 128 {
		t.Fatalf("incorrect rounded premultiplied bytes: %d %d %d %d", r8, g8, b8, a8)
	}
}
