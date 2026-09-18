package icon

import (
	"image"
	"image/color"
	"testing"

	"github.com/golang-gui/goui/gui"
)

func iconPixels(size int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			img.SetRGBA(x, y, color.RGBA{G: 128, A: 128})
		}
	}
	return img
}

func TestImageResolutionAndTint(t *testing.T) {
	images := []image.Image{iconPixels(64), iconPixels(16), iconPixels(32)}
	s, err := NewImage(images...)
	if err != nil {
		t.Fatal(err)
	}
	original, err := NewImage(images...)
	if err != nil {
		t.Fatal(err)
	}
	original = original.WithRawColors()
	for _, tc := range []struct{ request, want int }{{8, 16}, {16, 16}, {24, 32}, {32, 32}, {40, 64}, {100, 64}} {
		img := original.Image(tc.request, tc.request, gui.Color{})
		if img == nil || img.Bounds().Dx() != tc.want {
			t.Fatalf("request %d: %v", tc.request, img)
		}
	}
	red := gui.Color{R: .5, A: .5}
	a := s.Image(24, 24, red)
	b := s.Image(25, 25, red)
	if a != b {
		t.Fatal("same selected version was regenerated")
	}
	if got := color.RGBAModel.Convert(a.At(10, 10)); got != (color.RGBA{R: 64, A: 64}) {
		t.Fatal(got)
	}
	c := s.Image(25, 25, gui.Color{B: 1, A: 1})
	if a == c || color.RGBAModel.Convert(a.At(10, 10)) != (color.RGBA{R: 64, A: 64}) {
		t.Fatal("recolor mutated retained pixels")
	}
	img := original.Image(25, 25, red)
	if color.RGBAModel.Convert(img.At(10, 10)) != (color.RGBA{G: 128, A: 128}) {
		t.Fatal("original-color source was tinted")
	}
	if (&Image{}).Image(16, 16, red) != nil {
		t.Fatal("zero-value source must be missing")
	}
}

func TestImageNonSquareResolution(t *testing.T) {
	a, b := image.NewRGBA(image.Rect(0, 0, 32, 16)), image.NewRGBA(image.Rect(0, 0, 64, 32))
	s, err := NewImage(a, b)
	if err != nil {
		t.Fatal(err)
	}
	s = s.WithRawColors()
	img := s.Image(24, 24, gui.Color{})
	if img != a {
		t.Fatal("contain only needs 24x12 pixels", err)
	}
}

func TestNewImageValidatesAndSnapshotsList(t *testing.T) {
	for _, images := range [][]image.Image{nil, {nil}, {image.NewRGBA(image.Rectangle{})}, {iconPixels(16), nil}} {
		if got, err := NewImage(images...); err == nil || got != nil {
			t.Fatal("invalid input must fail at construction", images)
		}
	}
	pixels := iconPixels(16)
	images := []image.Image{pixels}
	s, err := NewImage(images...)
	if err != nil {
		t.Fatal(err)
	}
	s = s.WithRawColors()
	images[0] = iconPixels(32)
	if got := s.Image(16, 16, gui.Color{}); got != pixels {
		t.Fatal("constructor must copy the list but share immutable pixels")
	}
	if got := s.Image(0, 16, gui.Color{}); got != nil {
		t.Fatal("invalid size accepted")
	}
}
