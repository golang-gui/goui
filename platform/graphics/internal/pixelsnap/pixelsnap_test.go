package pixelsnap

import (
	"image"
	"math"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
)

func TestClipConservativePixelEnvelope(t *testing.T) {
	for _, tc := range []struct {
		name  string
		rect  geometry.Rectangle
		scale float32
		want  image.Rectangle
	}{
		{"integer", geometry.Rect(2, 3, 4, 5), 1, image.Rect(2, 3, 6, 8)},
		{"fraction", geometry.Rect(10.5, 20.25, 3.5, 1.25), 1, image.Rect(10, 20, 14, 22)},
		{"negative", geometry.Rect(-1.5, -.5, 1.5, 1.5), 1, image.Rect(-2, -1, 0, 1)},
		{"fractional-scale", geometry.Rect(.2, .4, .8, .4), 1.25, image.Rect(0, 0, 2, 1)},
		{"decimal-endpoint", geometry.Rect(.2, .2, .8, .8), 1, image.Rect(0, 0, 1, 1)},
		{"tiny", geometry.Rect(1, 1, math.SmallestNonzeroFloat32, math.SmallestNonzeroFloat32), 1, image.Rect(1, 1, 2, 2)},
		{"empty-fraction", geometry.Rect(.5, .5, 0, 2), 1, image.Rectangle{}},
		{"inverted", geometry.Rect(2, 3, -1, 2), 1, image.Rectangle{}},
		{"invalid", geometry.Rect(float32(math.NaN()), 0, 1, 2), 1, image.Rectangle{}},
		{"invalid-scale", geometry.Rect(0, 0, 1, 2), float32(math.Inf(1)), image.Rectangle{}},
		{"huge", geometry.Rect(-1e20, 0, 2e20, 1), 1, image.Rect(-math.MaxInt32, 0, math.MaxInt32, 1)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := Clip(tc.rect, tc.scale); got != tc.want {
				t.Fatalf("像素包络=%v，预期%v", got, tc.want)
			}
		})
	}
}

func TestFillAndStrokeShareDeviceEdges(t *testing.T) {
	for _, scale := range []float32{1, 1.25, 1.5, 1.75, 2} {
		for _, offset := range []geometry.Point{{}, {X: 6.2, Y: -5.3}, {X: 1000.37, Y: 700.25}} {
			transform := geometry.Translate(offset.X, offset.Y)
			outer := geometry.Rect(0, 0, 23.4, 17.7)
			fill := Rect(outer, transform, scale)
			for _, pixels := range []float32{1, 2, 3, 1.25, 1.5, 1.75} {
				width := pixels / scale
				stroke := StrokeRect(outer.Inset(width/2), width, transform, scale).Inset(-width / 2)
				for _, pair := range [][2]float32{
					{stroke.X, fill.X}, {stroke.Y, fill.Y},
					{stroke.X + stroke.Width, fill.X + fill.Width},
					{stroke.Y + stroke.Height, fill.Y + fill.Height},
				} {
					if math.Abs(float64(pair[0]-pair[1])) > 0.001 {
						t.Fatalf("scale=%g offset=%v width=%g: stroke=%v fill=%v", scale, offset, width, stroke, fill)
					}
				}
			}
			for _, value := range []float32{
				(fill.X + offset.X) * scale, (fill.X + fill.Width + offset.X) * scale,
				(fill.Y + offset.Y) * scale, (fill.Y + fill.Height + offset.Y) * scale,
			} {
				if math.Abs(float64(value)-math.Round(float64(value))) > 0.001 {
					t.Fatalf("edge %g is not on the device grid", value)
				}
			}
		}
	}
}

func TestUserTransformsAndDiagonalsAreNotQuantized(t *testing.T) {
	rect := geometry.Rect(0.2, 1.3, 23.4, 17.7)
	p0, p1 := geometry.Point{X: 1.2, Y: 2.3}, geometry.Point{X: 13.2, Y: 2.3}
	for _, transform := range []geometry.Transform{
		geometry.Scale(1.2, 1.2), geometry.Rotate(20),
		{A11: 1, A12: 0.2, A22: 1}, geometry.Scale(-1, 1),
	} {
		if got := Rect(rect, transform, 1.5); got != rect {
			t.Fatalf("transformed fill changed: %v", got)
		}
		if got := StrokeRect(rect, 1, transform, 1.5); got != rect {
			t.Fatalf("transformed stroke changed: %v", got)
		}
		if a, b := Line(p0, p1, 1, transform, 1.5); a != p0 || b != p1 {
			t.Fatalf("transformed line changed: %v %v", a, b)
		}
	}
	p1.Y = 13.7
	if a, b := Line(p0, p1, 1, geometry.Translate(6.2, 5.3), 1.5); a != p0 || b != p1 {
		t.Fatalf("diagonal changed: %v %v", a, b)
	}
}

func TestEdgeHalfPixelTiesAndDegenerateGeometry(t *testing.T) {
	for _, test := range []struct{ x, want float32 }{{0.5, 1}, {-0.5, 0}, {1.5, 2}, {-1.5, -1}} {
		if got := Edge(test.x, 0, 1); got != test.want {
			t.Fatalf("Edge(%g) = %g, want %g", test.x, got, test.want)
		}
	}
	rect := geometry.Rect(0, 0, 0.1, 0.1)
	if got := StrokeRect(rect, 2.4, geometry.Identity(), 1); got != rect {
		t.Fatalf("too-small stroke changed: %v", got)
	}
	if got := Rect(rect, geometry.Identity(), 0); got != rect {
		t.Fatalf("zero-scale rect changed: %v", got)
	}
}
