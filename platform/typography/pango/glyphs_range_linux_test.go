package pango

import (
	"bytes"
	"image"
	"image/color"
	"math"
	"runtime"
	"strings"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform/typography"
)

func TestGlyphRangesRetainNativePixels(t *testing.T) {
	for _, text := range []string{"office ffi e\u0301 中文 👩‍💻", "العربية مرحبا", "abc \u202eRTL 123 العربية\u202c end", "abc \u2067עברית 123\u2069 xyz"} {
		t.Run(text, func(t *testing.T) {
			runtime.LockOSThread()
			defer runtime.UnlockOSThread()
			ctx, err := NewContext()
			if err != nil {
				t.Fatal(err)
			}
			defer ctx.Destroy()
			l, err := ctx.(*Context).NewGlyphLayout(text, typography.TextFormat{Font: typography.FontInfo{Family: "DejaVu Sans", Size: 13}, TextColor: color.NRGBA{R: 190, G: 50, B: 80, A: 180}})
			if err != nil {
				t.Fatal(err)
			}
			defer l.Destroy()
			m, err := l.GlyphMetrics()
			if err != nil {
				t.Fatal(err)
			}
			for _, scale := range []float32{1, 1.25, 1.5, 2} {
				region := image.Rect(-3, -3, int(math.Ceil(float64(m.Line.Width*scale)))+4, int(math.Ceil(float64(m.Line.Height*scale)))+4)
				a, err := l.Rasterize(scale, region, nil)
				if err != nil {
					t.Fatal(err)
				}
				b, err := l.RasterizeGlyphs(0, len(text), nil, geometry.Point{}, scale, region, nil)
				if err != nil {
					t.Fatal(err)
				}
				// Exact premultiplied RGBA comparison of every pixel, including
				// transparent margins and antialiasing, on the same font/backend.
				if !bytes.Equal(a.Pixels, b.Pixels) {
					t.Fatalf("whole glyph rendering differs from native layout at scale %v", scale)
				}
			}
			// Every single cluster remains identical to its native source ink at
			// an integer placement. Crop away neighboring glyphs by using ASCII
			// padding in the separate exact pixel test below.
			for _, c := range m.Clusters {
				if _, err := l.RasterizeGlyphs(c.Start, c.Length, color.Black, geometry.Point{}, 1, image.Rect(-2, -2, 64, 64), nil); err != nil {
					t.Fatal(err)
				}
			}
			if strings.Contains(text, "ffi") {
				at := strings.Index(text, "ffi")
				for _, c := range m.Clusters {
					if c.Start <= at && c.Start+c.Length > at+1 {
						if _, err := l.RasterizeGlyphs(c.Start, 1, nil, geometry.Point{}, 1, image.Rect(0, 0, 1, 1), nil); err == nil {
							t.Fatal("accepted split ligature")
						}
					}
				}
			}
		})
	}
}

func TestGlyphRangePixelsAndInvalidation(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	ctx, err := NewContext()
	if err != nil {
		t.Fatal(err)
	}
	defer ctx.Destroy()
	text := "    colored glyphs    "
	l, err := ctx.(*Context).NewGlyphLayout(text, typography.TextFormat{Font: typography.FontInfo{Family: "DejaVu Sans Mono", Size: 12}})
	if err != nil {
		t.Fatal(err)
	}
	defer l.Destroy()
	region := image.Rect(-2, -2, 180, 40)
	rangeText := strings.TrimSpace(text)
	start := strings.Index(text, rangeText)
	for _, col := range []color.Color{color.Black, color.NRGBA{G: 180, B: 50, A: 120}} {
		l.SetTextColor(0, len(text), col)
		m, err := l.GlyphMetrics()
		if err != nil {
			t.Fatal(err)
		}
		x := 0
		for _, c := range m.Clusters {
			if c.Start < start {
				x += int(c.Width)
			}
		}
		full, err := l.Rasterize(1, region.Add(image.Pt(x, 0)), nil)
		if err != nil {
			t.Fatal(err)
		}
		part, err := l.RasterizeGlyphs(start, len(rangeText), nil, geometry.Point{}, 1, region, nil)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(full.Pixels, part.Pixels) {
			t.Fatal("range-local glyph pixels differ from native crop")
		}
	}
	if _, err := l.RasterizeGlyphs(-1, 2, nil, geometry.Point{}, 1, region, nil); err == nil {
		t.Fatal("accepted negative range")
	}
	if _, err := l.RasterizeGlyphs(0, len(text), nil, geometry.Point{}, 0, region, nil); err == nil {
		t.Fatal("accepted invalid scale")
	}
	before, _ := l.GlyphMetrics()
	l.SetTextFont(0, len(text), typography.FontInfo{Family: "DejaVu Sans Mono", Size: 24})
	after, _ := l.GlyphMetrics()
	if after.Line.Width <= before.Line.Width {
		t.Fatal("font mutation retained stale glyphs")
	}
	if before.Clusters[1].Width == after.Clusters[1].Width {
		t.Fatal("metrics snapshot was mutated")
	}
}

func TestHugeGlyphSourceKeepsIntegerAdvances(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	ctx, err := NewContext()
	if err != nil {
		t.Fatal(err)
	}
	defer ctx.Destroy()
	text := strings.Repeat("M", 300000)
	l, err := ctx.(*Context).NewGlyphLayout(text, typography.TextFormat{Font: typography.FontInfo{Family: "DejaVu Sans Mono", Size: 12}})
	if err != nil {
		t.Fatal(err)
	}
	defer l.Destroy()
	m, err := l.GlyphMetrics()
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Clusters) != len(text) || m.Line.Width < 2100000 {
		t.Fatal("fixture did not exceed native int32 Pango coordinates", len(m.Clusters), m.Line.Width)
	}
	b, err := l.RasterizeGlyphs(len(text)-3, 3, color.Black, geometry.Point{}, 1, image.Rect(0, 0, 40, 30), nil)
	if err != nil {
		t.Fatal(err)
	}
	ink := false
	for i := 3; i < len(b.Pixels); i += 4 {
		ink = ink || b.Pixels[i] != 0
	}
	if !ink {
		t.Fatal("far range lost glyph ink after native coordinate overflow")
	}
}

func TestGlyphDefaultTabsFollowFirstEffectiveFont(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	ctx, err := NewContext()
	if err != nil {
		t.Fatal(err)
	}
	defer ctx.Destroy()
	format := typography.TextFormat{Font: typography.FontInfo{Family: "DejaVu Sans Mono", Size: 17}}
	l, err := ctx.(*Context).NewGlyphLayout("abc\tdef\t", format)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Destroy()
	for _, size := range []float32{17, 29} {
		if size != 17 {
			l.SetTextFont(0, 1, typography.FontInfo{Family: "DejaVu Sans Mono", Size: size})
		}
		m, err := l.GlyphMetrics()
		if err != nil {
			t.Fatal(err)
		}
		ref, err := ctx.NewTextLayout("\t", typography.TextFormat{Font: typography.FontInfo{Family: "DejaVu Sans Mono", Size: size}}, 0, 0)
		if err != nil {
			t.Fatal(err)
		}
		want, _ := ref.MeasureSize()
		ref.Destroy()
		if m.TabWidth != want || want <= 0 {
			t.Fatal("default tab interval differs from independent native reference", m.TabWidth, want)
		}
	}
}
