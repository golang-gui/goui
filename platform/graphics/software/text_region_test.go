package software

import (
	"image"
	"image/color"
	"math"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/platform/typography"
)

type patternedTextLayout struct {
	testTextLayout
	requests []image.Rectangle
}

func (*patternedTextLayout) MeasureSize() (float32, float32) { return 20000, 20 }
func (l *patternedTextLayout) Rasterize(_ float32, r image.Rectangle, _ []byte) (typography.TextBitmap, error) {
	l.requests = append(l.requests, r)
	b := typography.TextBitmap{Width: r.Dx(), Height: r.Dy(), Stride: r.Dx() * 4}
	b.Pixels = make([]byte, b.Stride*b.Height)
	for y := 0; y < b.Height; y++ {
		for x := 0; x < b.Width; x++ {
			i := b.PixOffset(x, y)
			b.Pixels[i], b.Pixels[i+3] = byte((x+r.Min.X)&127), 255
		}
	}
	return b, nil
}

// Expected values come from physical pixel coordinates, independently of the
// region planner. This catches misplaced regions and repeated per-region snaps.
func TestLongTextRegionPlacementAtFractionalScale(t *testing.T) {
	for _, scale := range []float32{1, 1.25, 2} {
		l := &patternedTextLayout{}
		var drawer testDrawer
		base, err := NewPainter(&drawer)
		if err != nil {
			t.Fatal(err)
		}
		func() {
			defer base.Destroy()
			base.Begin(100, 40, scale)
			base.Clear(graphics.Color{})
			base.SetClipRect(geometry.Rect(8/scale, 0, 84/scale, 40/scale))
			origin := geometry.Point{X: -14999.75}
			base.DrawTextLayout(origin, l)
			base.End()
			start := int(math.Round(float64(origin.X * scale)))
			for x := 0; x < 100; x++ {
				want := color.RGBA{}
				if x >= 8 && x < 92 {
					want = color.RGBA{R: byte((x - start) & 127), A: 255}
				}
				got := color.RGBAModel.Convert(drawer.result.At(x, 5)).(color.RGBA)
				if got != want {
					t.Fatalf("scale=%g x=%d: %v want %v", scale, x, got, want)
				}
			}
			if len(l.requests) == 0 {
				t.Fatal("no raster request")
			}
			for _, r := range l.requests {
				if r.Min.X < 10000 || r.Dx() > 1024 || r.Dy() > 1024 {
					t.Fatal("unbounded or misplaced native request", r)
				}
			}
		}()
	}
}
