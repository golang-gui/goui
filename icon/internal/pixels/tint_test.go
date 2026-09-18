package pixels

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"testing"

	"github.com/golang-gui/goui/gui"
)

func TestTintPremultipliedMask(t *testing.T) {
	src := image.NewNRGBA(image.Rect(7, 9, 9, 10))
	src.SetNRGBA(7, 9, color.NRGBA{G: 255, A: 128})
	dst := Tint(src, gui.Color{R: .5, A: .5})
	if dst.Bounds() != image.Rect(0, 0, 2, 1) {
		t.Fatal("origin was not normalized")
	}
	// 128/255 coverage times 0.5 premultiplied foreground rounds to 64/255.
	if dst.RGBAAt(0, 0) != (color.RGBA{R: 64, A: 64}) || dst.RGBAAt(1, 0) != (color.RGBA{}) {
		t.Fatal("incorrect mask/alpha")
	}
	if src.NRGBAAt(7, 9) != (color.NRGBA{G: 255, A: 128}) {
		t.Fatal("input pixels were modified")
	}
}

// Keep the old generic implementation as an explicit benchmark baseline and
// byte-for-byte regression reference, independent of the new fast path.
func tintReference(src image.Image, foreground gui.Color) *image.RGBA {
	bounds := src.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	for y := 0; y < bounds.Dy(); y++ {
		for x := 0; x < bounds.Dx(); x++ {
			_, _, _, a := src.At(bounds.Min.X+x, bounds.Min.Y+y).RGBA()
			coverage := float32(a) / 65535
			o := dst.PixOffset(x, y)
			dst.Pix[o] = byte(foreground.R*coverage*255 + .5)
			dst.Pix[o+1] = byte(foreground.G*coverage*255 + .5)
			dst.Pix[o+2] = byte(foreground.B*coverage*255 + .5)
			dst.Pix[o+3] = byte(foreground.A*coverage*255 + .5)
		}
	}
	return dst
}

// Mask the concrete type to exercise the image.Image fallback.
type genericImage struct{ image.Image }

func TestTintFormatsAndSubimages(t *testing.T) {
	parentBounds := image.Rect(-8, 5, 13, 25)
	subBounds := image.Rect(-5, 7, 11, 23)
	rgba := image.NewRGBA(parentBounds)
	nrgba := image.NewNRGBA(parentBounds)
	rgba64 := image.NewRGBA64(subBounds)
	alpha := image.NewAlpha(subBounds)
	paletted := image.NewPaletted(subBounds, color.Palette{color.Transparent, color.NRGBA{R: 255, A: 63}, color.White})
	for y := subBounds.Min.Y; y < subBounds.Max.Y; y++ {
		for x := subBounds.Min.X; x < subBounds.Max.X; x++ {
			a := uint8((y-subBounds.Min.Y)*16 + x - subBounds.Min.X)
			rgba.SetRGBA(x, y, color.RGBA{R: a / 2, G: a, A: a})
			nrgba.SetNRGBA(x, y, color.NRGBA{R: 217, G: 53, B: 99, A: a})
			// Include 16-bit alpha values not representable by an 8-bit mask.
			rgba64.SetRGBA64(x, y, color.RGBA64{A: uint16(a)*251 + 37})
			alpha.SetAlpha(x, y, color.Alpha{A: a})
			paletted.SetColorIndex(x, y, a%3)
		}
	}
	beforeRGBA, beforeNRGBA := bytes.Clone(rgba.Pix), bytes.Clone(nrgba.Pix)
	colors := []gui.Color{{}, {R: 1, A: 1}, {R: .5, G: .125, B: .25, A: .5}, {R: .13, G: .23, B: .07, A: .31}}
	for _, tc := range []struct {
		name string
		src  image.Image
	}{
		{"rgba-subimage", rgba.SubImage(subBounds)},
		{"nrgba-subimage", nrgba.SubImage(subBounds)},
		{"generic", genericImage{rgba.SubImage(subBounds)}},
		{"rgba64", rgba64}, {"alpha", alpha}, {"paletted", paletted},
		{"empty-rgba", image.NewRGBA(image.Rect(3, 5, 3, 5))},
		{"empty-nrgba", image.NewNRGBA(image.Rect(3, 5, 3, 5))},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, fg := range colors {
				got := Tint(tc.src, fg)
				want := tintReference(tc.src, fg)
				if got.Bounds() != want.Bounds() || !bytes.Equal(got.Pix, want.Pix) {
					t.Fatal("optimized result differs from the original algorithm", fg)
				}
				// Independent premultiplied-color × coverage expectation, evaluated
				// in float64. Allow one byte for float32 rounding at half boundaries.
				for y := 0; y < got.Rect.Dy(); y++ {
					for x := 0; x < got.Rect.Dx(); x++ {
						_, _, _, a := tc.src.At(tc.src.Bounds().Min.X+x, tc.src.Bounds().Min.Y+y).RGBA()
						p := got.RGBAAt(x, y)
						for j, component := range []float32{fg.R, fg.G, fg.B, fg.A} {
							expected := int(float64(component)*float64(a)/65535*255 + .5)
							v := int([]uint8{p.R, p.G, p.B, p.A}[j])
							if delta := v - expected; delta < -1 || delta > 1 {
								t.Fatalf("(%d,%d) channel %d: got %d want %d", x, y, j, v, expected)
							}
						}
						if p.R > p.A || p.G > p.A || p.B > p.A {
							t.Fatal("output is not premultiplied", p)
						}
					}
				}
				saved := bytes.Clone(got.Pix)
				next := Tint(tc.src, gui.Color{G: 1, A: 1})
				clear(next.Pix)
				if !bytes.Equal(saved, got.Pix) {
					t.Fatal("later output aliases an earlier result")
				}
			}
		})
	}
	if !bytes.Equal(beforeRGBA, rgba.Pix) || !bytes.Equal(beforeNRGBA, nrgba.Pix) {
		t.Fatal("source pixels (including pixels outside the subimage) changed")
	}
}

var tintSink *image.RGBA

func TestTintAllocations(t *testing.T) {
	for _, size := range []int{16, 48, 96} {
		for _, src := range []image.Image{image.NewRGBA(image.Rect(0, 0, size, size)), image.NewNRGBA(image.Rect(0, 0, size, size))} {
			allocs := testing.AllocsPerRun(20, func() { tintSink = Tint(src, gui.Color{R: .5, A: .5}) })
			// One RGBA header and one pixel buffer; never per-pixel allocations.
			if allocs > 2 {
				t.Fatalf("%T %dx%d: %g allocations, want <= 2", src, size, size, allocs)
			}
		}
	}
}

func BenchmarkTint(b *testing.B) {
	for _, size := range []int{24, 48, 96} {
		for _, format := range []string{"RGBA", "NRGBA"} {
			var src image.Image
			if format == "RGBA" {
				src = image.NewRGBA(image.Rect(0, 0, size, size))
			} else {
				src = image.NewNRGBA(image.Rect(0, 0, size, size))
			}
			for y := 0; y < size; y++ {
				for x := 0; x < size; x++ {
					c := color.NRGBA{R: 255, A: uint8((x + y*size) % 256)}
					switch s := src.(type) {
					case *image.RGBA:
						s.Set(x, y, c)
					case *image.NRGBA:
						s.SetNRGBA(x, y, c)
					}
				}
			}
			for _, tc := range []struct {
				name string
				fn   func(image.Image, gui.Color) *image.RGBA
			}{{"Reference", tintReference}, {"Optimized", Tint}} {
				b.Run(fmt.Sprintf("%s/%d/%s", format, size, tc.name), func(b *testing.B) {
					b.ReportAllocs()
					for i := 0; i < b.N; i++ {
						tintSink = tc.fn(src, gui.Color{R: .5, G: .125, B: .25, A: .5})
					}
				})
			}
		}
	}
}
