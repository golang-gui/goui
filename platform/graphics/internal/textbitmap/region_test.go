package textbitmap

import (
	"image"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform/typography"
)

type regionLayout struct {
	cacheTestLayout
	width, height float32
	requests      []image.Rectangle
}

func (l *regionLayout) Size() (float32, float32)        { return l.width, l.height }
func (l *regionLayout) MeasureSize() (float32, float32) { return l.width, l.height }
func (l *regionLayout) Rasterize(_ float32, r image.Rectangle, buf []byte) (typography.TextBitmap, error) {
	l.requests = append(l.requests, r)
	n := r.Dx() * r.Dy() * 4
	if cap(buf) < n {
		buf = make([]byte, n)
	} else {
		buf = buf[:n]
	}
	for y := 0; y < r.Dy(); y++ {
		for x := 0; x < r.Dx(); x++ {
			i := (y*r.Dx() + x) * 4
			buf[i], buf[i+1], buf[i+2], buf[i+3] = byte((x+r.Min.X)&127), byte((y+r.Min.Y)&127), 0, 255
		}
	}
	return typography.TextBitmap{Width: r.Dx(), Height: r.Dy(), Stride: r.Dx() * 4, Pixels: buf}, nil
}

func TestRegionRestrictsLongLayoutsToVisiblePixels(t *testing.T) {
	l := &regionLayout{width: 1_500_000, height: 24}
	clip := image.Rect(0, 0, 800, 300)
	for _, scale := range []float32{1, 1.25, 2} {
		origin := SnapOrigin(geometry.Point{X: -1_200_000}, geometry.Identity(), scale)
		r := Region(l, origin, geometry.Identity(), scale, scale, clip)
		if r.Empty() || r.Dx() > 800+2*TileSize+4 || r.Dy() > int(24*scale)+4 {
			t.Fatalf("unbounded region %v", r)
		}
		if !image.Pt(int(1_200_000*scale), 0).In(r) {
			t.Fatal("horizontal offset was lost", r)
		}
		if got := Region(l, geometry.Point{X: 1000}, geometry.Identity(), scale, scale, clip); !got.Empty() {
			t.Fatal("invisible layout was rasterized", got)
		}
	}
}

func TestRegionContainsInverseTransformedClip(t *testing.T) {
	l := &regionLayout{width: 10000, height: 10000}
	for _, transform := range []geometry.Transform{geometry.Translate(30, 40).Rotate(27).Scale(1.2, .75), geometry.Translate(900, 20).Scale(-1, 2), {A11: 1, A12: .8, A22: 1}} {
		scale := float32(1.25)
		origin := SnapOrigin(geometry.Point{X: -40, Y: -60}, transform, scale)
		rasterScale := RasterScale(scale, transform)
		clip := image.Rect(100, 100, 400, 300)
		r := Region(l, origin, transform, scale, rasterScale, clip)
		inv := geometry.Scale(scale, scale).Multiply(transform).Inverse()
		for y := 100; y < 300; y += 10 {
			for x := 100; x < 400; x += 10 {
				local := inv.TransformPoint(geometry.Point{X: float32(x), Y: float32(y)})
				px, py := (local.X-origin.X)*rasterScale, (local.Y-origin.Y)*rasterScale
				if px >= 0 && py >= 0 && !image.Pt(int(px), int(py)).In(r) {
					t.Fatalf("inverse clip point %g,%g missing from %v", px, py, r)
				}
			}
		}
	}
	if r := Region(l, geometry.Point{}, geometry.Scale(0, 1), 1, 1, image.Rect(0, 0, 100, 100)); !r.Empty() {
		t.Fatal("singular transform accepted")
	}
}

func TestRasterAssemblesNativeBlocksOnOnePixelGrid(t *testing.T) {
	l := &regionLayout{}
	r := image.Rect(-7, 20001, TileSize*2+29, 20001+TileSize+31)
	reuse := make([]byte, r.Dx()*r.Dy()*4)
	for pass := 0; pass < 2; pass++ {
		b, err := Rasterize(l, 1.25, r, reuse[:0])
		if err != nil {
			t.Fatal(err)
		}
		if b.Width != r.Dx() || b.Height != r.Dy() || &b.Pixels[0] != &reuse[0] {
			t.Fatal("assembled dimensions or caller storage changed")
		}
		for y := 0; y < b.Height; y++ {
			for x := 0; x < b.Width; x++ {
				i := b.PixOffset(x, y)
				if b.Pixels[i] != byte((x+r.Min.X)&127) || b.Pixels[i+1] != byte((y+r.Min.Y)&127) || b.Pixels[i+3] != 255 {
					t.Fatalf("incorrect pixel at %d,%d", x, y)
				}
			}
		}
	}
	if len(l.requests) != 12 {
		t.Fatal("unexpected native block count", len(l.requests))
	}
	for _, part := range l.requests {
		if part.Dx() > TileSize || part.Dy() > TileSize {
			t.Fatal("unbounded native target", part)
		}
	}
}

func TestImageCacheDistinguishesRegions(t *testing.T) {
	cache := NewImageCache[int](2, nil)
	defer cache.Destroy()
	l := &cacheTestLayout{}
	a, b := image.Rect(0, 0, 100, 20), image.Rect(50000, 0, 50100, 20)
	cache.Store(l, 1, a, 11)
	cache.Store(l, 1, b, 22)
	if got, ok := cache.Lookup(l, 1, a); !ok || got != 11 {
		t.Fatal("first region was replaced", got, ok)
	}
	if got, ok := cache.Lookup(l, 1, b); !ok || got != 22 {
		t.Fatal("second region was lost", got, ok)
	}
	if _, ok := cache.Lookup(l, 2, a); ok {
		t.Fatal("raster scale was ignored")
	}
	l.changed.Emit()
	if _, ok := cache.Lookup(l, 1, b); ok {
		t.Fatal("region survived layout invalidation")
	}
}
