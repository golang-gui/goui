package utils

import (
	"image"
	"math"
	"testing"
)

func TestRasterDimensions(t *testing.T) {
	for _, region := range []image.Rectangle{image.Rect(-10, -20, 30, 40), image.Rect(0, 0, 4096, 4096)} {
		w, h, err := RasterDimensions(region)
		if err != nil || w != region.Dx() || h != region.Dy() {
			t.Fatalf("%v: %dx%d, %v", region, w, h, err)
		}
	}
	for _, region := range []image.Rectangle{image.Rect(0, 0, 4097, 4096), {Min: image.Pt(math.MinInt, 0), Max: image.Pt(math.MaxInt, 1)}, image.Rect(0, 0, math.MaxInt, math.MaxInt)} {
		if _, _, err := RasterDimensions(region); err == nil {
			t.Fatalf("oversized region accepted: %v", region)
		}
	}
	for _, region := range []image.Rectangle{{}, {Min: image.Pt(20, 30), Max: image.Pt(10, 10)}, {Min: image.Pt(math.MinInt, 1), Max: image.Pt(math.MaxInt, 1)}} {
		if w, h, err := RasterDimensions(region); err != nil || w != 0 || h != 0 {
			t.Fatalf("empty region: %dx%d, %v", w, h, err)
		}
	}
}
