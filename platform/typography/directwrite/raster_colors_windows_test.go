package directwrite

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"runtime"
	"strings"
	"testing"

	"github.com/golang-gui/goui/platform/typography"
)

func TestRasterColorsSurviveTilesAndTargetChanges(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	context, err := NewContext()
	if err != nil {
		t.Fatal(err)
	}
	defer context.Destroy()
	c := context.(*Context)
	format := typography.TextFormat{Font: typography.FontInfo{Family: "Arial", Size: 16}, WrapMode: typography.WrapWordChar, TextColor: color.Black}
	text := strings.Repeat("Aé中 e\u0301 👩‍💻 אבג العربية 123\n", 20)
	create := func() *TextLayout {
		l, err := c.NewTextLayout(text, format, 240, 1<<20)
		if err != nil {
			t.Fatal(err)
		}
		return l.(*TextLayout)
	}
	l := create()
	defer l.Destroy()
	ink := color.NRGBA{R: 190, B: 70, A: 180}
	l.SetTextColor(0, len(text), ink)
	for _, scale := range []float32{1, 2} {
		for _, region := range []image.Rectangle{image.Rect(0, 0, 180, 100), image.Rect(0, 32, 180, 132), image.Rect(0, 0, 480, 240)} {
			check := func() {
				t.Helper()
				got, err := l.Rasterize(scale, region, nil)
				if err != nil {
					t.Fatal(err)
				}
				// A fresh native layout always applies effects, independently of
				// the cache and the resource history of the layout under test.
				ref := create()
				defer ref.Destroy()
				ref.SetTextColor(0, len(text), ink)
				want, err := ref.Rasterize(scale, region, nil)
				if err != nil {
					t.Fatal(err)
				}
				// Same backend, font, scale and integer origin: compare exact
				// premultiplied RGBA bytes, without a cross-platform golden.
				if got.Width != want.Width || got.Height != want.Height || got.Stride != want.Stride || !bytes.Equal(got.Pixels, want.Pixels) {
					t.Fatalf("scale %g region %v: stale colors or mixed target resources", scale, region)
				}
				visible, expectedInk := false, false
				for i := 0; i < len(got.Pixels); i += 4 {
					r, g, b, a := got.Pixels[i], got.Pixels[i+1], got.Pixels[i+2], got.Pixels[i+3]
					if r > a || g > a || b > a {
						t.Fatal("non-premultiplied pixel")
					}
					if a > 20 {
						visible = true
						// Color-font emoji may retain their own palette. Ordinary
						// glyphs must also produce the requested foreground.
						if g == 0 && (ink.R > ink.B && r > b || ink.B > ink.R && b > r) {
							expectedInk = true
						}
					}
				}
				if !visible || !expectedInk || !l.rasterColorsApplied {
					t.Fatal("missing colored text or unapplied owned effects")
				}
			}
			check()
			check() // Same target and colors, including reuse across tile origins.
			var external textPainter
			if err := external.Init(c.d2dFactory, c.imgFactory, region.Dx(), region.Dy()); err != nil {
				t.Fatal(err)
			}
			err := external.DrawTextLayout(l, region, scale)
			external.Destroy()
			if err != nil {
				t.Fatal(err)
			}
			if l.rasterColorsApplied {
				t.Fatal("external draw retained owned-target cache validity")
			}
			check()
			ink.R, ink.B = ink.B, ink.R
			l.SetTextColor(0, len(text), ink)
			check()
			l.painter.Destroy() // Recreate even if COM reuses the target address.
			check()
		}
	}
}

func BenchmarkRasterColorEffects(b *testing.B) {
	for _, reuse := range []bool{false, true} {
		b.Run(fmt.Sprint(reuse), func(b *testing.B) {
			runtime.LockOSThread()
			defer runtime.UnlockOSThread()
			ctx, err := NewContext()
			if err != nil {
				b.Fatal(err)
			}
			defer ctx.Destroy()
			text := strings.Repeat("alpha beta gamma ", 8192)
			native, err := ctx.NewTextLayout(text, typography.TextFormat{Font: typography.FontInfo{Size: 14}, WrapMode: typography.WrapWordChar}, 1000, 1<<24)
			if err != nil {
				b.Fatal(err)
			}
			defer native.Destroy()
			l := native.(*TextLayout)
			l.SetTextColor(0, len(text), color.NRGBA{R: 180, A: 255})
			region := image.Rect(0, 1000, 1000, 1256)
			bitmap, err := l.Rasterize(1, region, nil)
			if err != nil {
				b.Fatal(err)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				if !reuse {
					l.rasterColorsApplied = false // Prior repeated-application behavior.
				}
				if _, err := l.Rasterize(1, region, bitmap.Pixels); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
