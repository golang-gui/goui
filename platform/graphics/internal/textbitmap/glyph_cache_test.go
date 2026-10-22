package textbitmap

import (
	"github.com/golang-gui/goui/core/geometry"
	"image"
	"image/color"
	"testing"

	"github.com/golang-gui/goui/platform/typography"
)

type glyphCacheLayout struct{ cacheTestLayout }

func (*glyphCacheLayout) GlyphMetrics() (typography.GlyphMetrics, error) {
	return typography.GlyphMetrics{}, nil
}
func (*glyphCacheLayout) RasterizeGlyphs(int, int, color.Color, geometry.Point, float32, image.Rectangle, []byte) (typography.TextBitmap, error) {
	panic("cache should not rasterize")
}

func TestGlyphImageKeysAndNativeLifetime(t *testing.T) {
	l := new(glyphCacheLayout)
	released := 0
	c := NewImageCache(4, func(int) { released++ })
	region := image.Rect(-1, -1, 100, 20)
	c.Store(l, 1, region, 100)
	c.StoreGlyphs(l, 0, 3, nil, geometry.Point{}, 1, region, 101)
	c.StoreGlyphs(l, 0, 3, color.Black, geometry.Point{}, 1, region, 102)
	c.StoreGlyphs(l, 3, 3, color.Black, geometry.Point{}, 1, region, 103)
	for _, test := range []struct {
		start int
		color color.Color
		want  int
	}{{0, nil, 101}, {0, color.NRGBA{A: 255}, 102}, {3, color.Black, 103}} {
		got, ok := c.LookupGlyphs(l, test.start, 3, test.color, geometry.Point{}, 1, region)
		if !ok || got != test.want {
			t.Fatal("range/color key collision", test, got, ok)
		}
	}
	if got, ok := c.Lookup(l, 1, region); !ok || got != 100 {
		t.Fatal("glyph key collided with whole layout")
	}
	if _, ok := c.LookupGlyphs(l, 0, 3, color.Black, geometry.Point{}, 2, region); ok {
		t.Fatal("ignored scale")
	}
	if _, ok := c.LookupGlyphs(l, 0, 3, color.Black, geometry.Point{}, 1, region.Add(image.Pt(1, 0))); ok {
		t.Fatal("ignored region")
	}
	l.changed.Emit()
	if released != 4 {
		t.Fatal("change did not release images", released)
	}
	c.StoreGlyphs(l, 0, 3, nil, geometry.Point{}, 1, region, 200)
	l.Destroy()
	if released != 5 || len(c.records) != 0 {
		t.Fatal("source destruction retained native images")
	}
}

func TestGlyphCacheSeparatesPixelPhases(t *testing.T) {
	l := new(glyphCacheLayout)
	c := NewImageCache(2, func(int) {})
	defer c.Destroy()
	r := image.Rect(0, 0, 10, 20)
	c.StoreGlyphs(l, 0, 3, nil, geometry.Point{}, 1.5, r, 1)
	c.StoreGlyphs(l, 0, 3, nil, geometry.Point{X: -.5}, 1.5, r, 2)
	for _, test := range []struct {
		phase float32
		want  int
	}{{0, 1}, {-.5, 2}} {
		got, ok := c.LookupGlyphs(l, 0, 3, nil, geometry.Point{X: test.phase}, 1.5, r)
		if !ok || got != test.want {
			t.Fatal("cache ignored fractional glyph phase", test, got, ok)
		}
	}
}
