package typographytest

import (
	"bytes"
	"image"
	"image/color"
	"math"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/golang-gui/goui/platform/typography"
)

// RasterRegions checks placement against native line metrics and compares
// integer pixel windows of the same positioned layout. Comparisons use exact
// premultiplied RGBA bytes within a backend, not cross-platform glyph goldens.
func RasterRegions(t *testing.T, ctx typography.Context) {
	t.Helper()
	RegisterFonts(t, ctx)
	for _, align := range []typography.TextAlignment{typography.TextAlignBegin, typography.TextAlignCenter, typography.TextAlignEnd} {
		layout, err := ctx.NewTextLayout("M  ffi e\u0301\nM  word", typography.TextFormat{Font: typography.FontInfo{Family: "Go", Size: 16}, TextAlign: align, WrapMode: typography.WrapWordChar, TextColor: color.NRGBA{R: 255, A: 128}}, 180, 200)
		if err != nil {
			t.Fatal(err)
		}
		func() {
			defer layout.Destroy()
			lines, clusters := layout.MeasureMetrics()
			lines = slices.Clone(lines)
			for i := range lines {
				lines[i].Clusters = slices.Clone(lines[i].Clusters)
			}
			clusters = slices.Clone(clusters)
			for _, scale := range []float32{1, 1.25, 1.5, 2} {
				full := image.Rect(-16, -16, 400, 220)
				bitmap, err := layout.Rasterize(scale, full, nil)
				if err != nil {
					t.Fatal(err)
				}
				if bitmap.Width != full.Dx() || bitmap.Height != full.Dy() || bitmap.Stride != full.Dx()*4 {
					t.Fatal("raster request was resized")
				}
				if bitmap.Pixels[3] != 0 {
					t.Fatal("negative margin is not transparent")
				}
				ink := false
				// The first M must lie inside the first native line. Rebasing the
				// occupied extents to X=0 loses center/end alignment and fails here.
				line := lines[0]
				for y := int(math.Floor(float64(line.Y * scale))); y < int(math.Ceil(float64((line.Y+line.Height)*scale))); y++ {
					for x := int(math.Floor(float64(line.X * scale))); x < int(math.Ceil(float64((line.X+line.Width)*scale))); x++ {
						i := bitmap.PixOffset(x-full.Min.X, y-full.Min.Y)
						ink = ink || bitmap.Pixels[i+3] != 0
					}
				}
				if !ink {
					t.Fatalf("aligned text is missing at native metrics: align=%v scale=%g line=%+v", align, scale, line)
				}
				for _, region := range []image.Rectangle{image.Rect(-8, -8, 140, 80), image.Rect(7, 11, 180, 100), image.Rect(125, 0, 240, 200), image.Rect(800, 800, 840, 820)} {
					reuse := bytes.Repeat([]byte{0xff}, region.Dx()*region.Dy()*4)
					part, err := layout.Rasterize(scale, region, reuse[:0])
					if err != nil {
						t.Fatal(err)
					}
					if part.Width != region.Dx() || part.Height != region.Dy() || len(part.Pixels) != len(reuse) || &part.Pixels[0] != &reuse[0] {
						t.Fatal("region dimensions or caller storage changed")
					}
					for y := 0; y < part.Height; y++ {
						for x := 0; x < part.Width; x++ {
							var want [4]byte
							global := image.Pt(x+region.Min.X, y+region.Min.Y)
							if global.In(full) {
								i := bitmap.PixOffset(global.X-full.Min.X, global.Y-full.Min.Y)
								copy(want[:], bitmap.Pixels[i:i+4])
							}
							i := part.PixOffset(x, y)
							if !bytes.Equal(part.Pixels[i:i+4], want[:]) {
								t.Fatalf("region moved pixels: align=%v scale=%g region=%v at %v: got %v want %v", align, scale, region, global, part.Pixels[i:i+4], want)
							}
						}
					}
				}
			}
			gotLines, gotClusters := layout.MeasureMetrics()
			if !reflect.DeepEqual(lines, gotLines) || !reflect.DeepEqual(clusters, gotClusters) {
				t.Fatal("raster regions changed shaping metrics")
			}
		}()
	}
	for _, text := range []string{"", "M"} {
		layout, err := ctx.NewTextLayout(text, typography.TextFormat{}, 100, 40)
		if err != nil {
			t.Fatal(err)
		}
		for _, region := range []image.Rectangle{{}, {Min: image.Pt(5, 5), Max: image.Pt(2, 2)}} {
			b, err := layout.Rasterize(1, region, nil)
			if err != nil || len(b.Pixels) != 0 || b.Width != 0 || b.Height != 0 {
				t.Fatal("empty region did not return an empty result", err)
			}
		}
		b, err := layout.Rasterize(1, image.Rect(100, 100, 140, 140), nil)
		if err != nil {
			t.Fatal(err)
		}
		for _, v := range b.Pixels {
			if v != 0 {
				t.Fatal("uncovered pixels are not transparent")
			}
		}
		for _, scale := range []float32{0, -1, float32(math.NaN()), float32(math.Inf(1))} {
			if _, err := layout.Rasterize(scale, image.Rectangle{}, nil); err == nil {
				t.Fatal("invalid scale accepted")
			}
		}
		if _, err := layout.Rasterize(1, image.Rect(0, 0, math.MaxInt, 1), nil); err == nil {
			t.Fatal("oversized request accepted")
		}
		layout.Destroy()
	}
	// Long/wrapped layouts must retain their full shaping coverage even though
	// only a small window near the end is rasterized.
	for _, text := range []string{strings.Repeat("W ", 12000) + "END", strings.Repeat("W\n", 1600) + "END"} {
		layout, err := ctx.NewTextLayout(text, typography.TextFormat{Font: typography.FontInfo{Family: "Go", Size: 18}}, 1<<20, 1<<24)
		if err != nil {
			t.Fatal(err)
		}
		func() {
			defer layout.Destroy()
			layout.SetTextColor(len(text)-3, 3, color.NRGBA{G: 255, A: 255})
			_, clusters := layout.MeasureMetrics()
			var end typography.TextCluster
			for _, c := range clusters {
				if c.Start == len(text)-3 {
					end = c
					break
				}
			}
			if end.Length == 0 || max(end.X, end.Y) < 20000 {
				t.Fatalf("layout lost its long tail: %+v", end)
			}
			region := image.Rect(int(math.Floor(float64(end.X)))-2, int(math.Floor(float64(end.Y)))-2, int(math.Ceil(float64(end.X)))+100, int(math.Ceil(float64(end.Y+end.Height)))+2)
			b, err := layout.Rasterize(1, region, nil)
			if err != nil {
				t.Fatal(err)
			}
			green := false
			for i := 0; i < len(b.Pixels); i += 4 {
				green = green || b.Pixels[i+1] > b.Pixels[i] && b.Pixels[i+3] > 0
			}
			if !green {
				t.Fatalf("long layout tail is blank at %+v", end)
			}
		}()
	}
}
