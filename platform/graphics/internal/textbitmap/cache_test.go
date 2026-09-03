package textbitmap

import (
	"image/color"
	"slices"
	"testing"

	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/platform/typography"
)

type cacheTestLayout struct {
	changed   signal.Signal0
	destroyed signal.Signal0
}

func (l *cacheTestLayout) Destroy() { l.destroyed.Emit() }
func (*cacheTestLayout) Rasterize(float32, []byte) (typography.TextBitmap, error) {
	return typography.TextBitmap{}, nil
}
func (l *cacheTestLayout) ConnectChanged(fn func()) signal.Handle { return l.changed.Connect(fn) }
func (l *cacheTestLayout) ConnectDestroy(fn func()) signal.Handle { return l.destroyed.Connect(fn) }
func (*cacheTestLayout) Text() string                             { return "" }
func (*cacheTestLayout) Format() typography.TextFormat            { return typography.TextFormat{} }
func (*cacheTestLayout) Size() (float32, float32)                 { return 0, 0 }
func (l *cacheTestLayout) SetSize(float32, float32)               { l.changed.Emit() }
func (l *cacheTestLayout) SetTextAlignment(typography.TextAlignment) {
	l.changed.Emit()
}
func (l *cacheTestLayout) SetWrapMode(typography.WrapMode) { l.changed.Emit() }
func (l *cacheTestLayout) SetTextFont(int, int, typography.FontInfo) {
	l.changed.Emit()
}
func (l *cacheTestLayout) SetTextColor(int, int, color.Color) { l.changed.Emit() }
func (l *cacheTestLayout) SetUnderline(int, int, bool)        { l.changed.Emit() }
func (l *cacheTestLayout) SetStrikethrough(int, int, bool)    { l.changed.Emit() }
func (*cacheTestLayout) MeasureSize() (float32, float32)      { return 0, 0 }
func (*cacheTestLayout) MeasureMetrics() ([]typography.TextLine, []typography.TextCluster) {
	return nil, nil
}

func TestImageCacheLRUInvalidationAndDestroy(t *testing.T) {
	var released []int
	cache := NewImageCache(2, func(image int) {
		released = append(released, image)
	})
	layout := new(cacheTestLayout)

	cache.Store(layout, 1, 10)
	cache.Store(layout, 2, 20)
	if image, ok := cache.Lookup(layout, 1); !ok || image != 10 {
		t.Fatalf("lookup scale 1 = %v, %v", image, ok)
	}
	cache.Store(layout, 3, 30)
	if !slices.Equal(released, []int{20}) {
		t.Fatalf("LRU releases = %v, want [20]", released)
	}

	layout.changed.Emit()
	if _, ok := cache.Lookup(layout, 1); ok {
		t.Fatal("changed layout retained a cached image")
	}
	if !slices.Equal(released, []int{20, 10, 30}) {
		t.Fatalf("invalidation releases = %v", released)
	}

	cache.Store(layout, 4, 40)
	layout.Destroy()
	if _, ok := cache.Lookup(layout, 4); ok {
		t.Fatal("destroyed layout retained a cached image")
	}
	if !slices.Equal(released, []int{20, 10, 30, 40}) {
		t.Fatalf("destroy releases = %v", released)
	}
}

func TestImageCacheLayoutDestroyFansOutAndCacheDestroyDisconnects(t *testing.T) {
	layout := new(cacheTestLayout)
	firstReleased, secondReleased := 0, 0
	first := NewImageCache(1, func(int) { firstReleased++ })
	second := NewImageCache(1, func(int) { secondReleased++ })
	first.Store(layout, 1, 1)
	second.Store(layout, 1, 2)

	layout.Destroy()
	if firstReleased != 1 || secondReleased != 1 {
		t.Fatalf("destroy releases = %d, %d, want 1, 1", firstReleased, secondReleased)
	}

	thirdReleased := 0
	third := NewImageCache(1, func(int) { thirdReleased++ })
	other := new(cacheTestLayout)
	third.Store(other, 1, 3)
	third.Destroy()
	other.changed.Emit()
	other.destroyed.Emit()
	if thirdReleased != 1 {
		t.Fatalf("cache destroy released %d images, want 1", thirdReleased)
	}
}
