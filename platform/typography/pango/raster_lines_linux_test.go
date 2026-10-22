package pango

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"runtime"
	"strings"
	"testing"

	"github.com/goexlib/cgo"
	"github.com/golang-gui/goui/platform/linux/libs/pango_cairo"
	"github.com/golang-gui/goui/platform/typography"
)

// Compare region pixels with the original native whole-layout renderer. Both
// render to premultiplied RGBA with identical grayscale font options and glyph
// phase. Exact equality is expected on the same machine; no cross-font golden.
func TestRasterVisibleLinesMatchWholeLayout(t *testing.T) {
	for _, align := range []typography.TextAlignment{typography.TextAlignBegin, typography.TextAlignCenter, typography.TextAlignEnd, typography.TextAlignFill} {
		t.Run(fmt.Sprint(align), func(t *testing.T) {
			runtime.LockOSThread()
			defer runtime.UnlockOSThread()
			ctx, err := NewContext()
			if err != nil {
				t.Fatal(err)
			}
			defer ctx.Destroy()
			text := strings.Repeat("AV ffi e\u0301 👩‍💻 中文\t עברית (123) العربية \u2067abc\u2069\n\n", 15) + "a\ufffcb"
			l, err := ctx.NewTextLayout(text, typography.TextFormat{Font: typography.FontInfo{Family: "DejaVu Sans", Size: 12, Style: typography.FontStyleItalic}, WrapMode: typography.WrapWordChar, TextAlign: align, TabWidth: 40, TextColor: color.NRGBA{R: 21, G: 89, B: 133, A: 190}}, 173, 1<<24)
			if err != nil {
				t.Fatal(err)
			}
			defer l.Destroy()
			whole := l.(*TextLayout)
			for mutation := 0; mutation < 4; mutation++ {
				switch mutation {
				case 1:
					l.SetSize(219, 1<<24)
				case 2:
					l.SetTextColor(0, 6, color.NRGBA{R: 230, A: 120})
					l.SetUnderline(0, 6, true)
					l.SetStrikethrough(0, 6, true)
				case 3:
					l.SetTextFont(0, 2, typography.FontInfo{Family: "serif", Size: 37})
					l.SetPlaceholder(len(text)-4, &typography.PlaceholderMetrics{Width: 31, Height: 47, Baseline: 30})
				}
				if whole.rasterLinesValid {
					t.Fatal("mutation retained borrowed line pointers")
				}
				for _, scale := range []float32{1, 1.5, 2} {
					_, height := l.MeasureSize()
					for _, region := range []image.Rectangle{image.Rect(-9, -7, 240, 135), image.Rect(17, int(height*scale)/2, 235, int(height*scale)/2+107), image.Rect(-3, int(height*scale)-85, 240, int(height*scale)+9)} {
						got, err := l.Rasterize(scale, region, nil)
						if err != nil {
							t.Fatal(err)
						}
						want := rasterWholeNative(t, whole, scale, region)
						if !bytes.Equal(got.Pixels, want.Pixels) {
							for i := range got.Pixels {
								if got.Pixels[i] != want.Pixels[i] {
									t.Fatalf("mutation=%d scale=%v region=%v byte=%d got=%d native=%d", mutation, scale, region, i, got.Pixels[i], want.Pixels[i])
								}
							}
						}
					}
				}
			}
		})
	}
}
func rasterWholeNative(t *testing.T, l *TextLayout, scale float32, region image.Rectangle) typography.TextBitmap {
	t.Helper()
	var p textPainter
	if err := p.Init(region.Dx(), region.Dy()); err != nil {
		t.Fatal(err)
	}
	defer p.Destroy()
	cgo.Memset(cgo.CSlice(p.bitmap.Pixels), 0, cgo.Sizet(len(p.bitmap.Pixels)))
	p.context.Translate(-float64(region.Min.X), -float64(region.Min.Y))
	p.context.Scale(float64(scale), float64(scale))
	r, g, b, a := toColor(l.format.TextColor)
	p.context.SetSourceRGBA(r, g, b, a)
	p.context.MoveTo(0, 0)
	pango_cairo.ShowLayout(p.context, l.layout)
	l.drawStrikethrough(p.context, 0, 0)
	return p.GetBitmap(region.Dx(), region.Dy(), nil)
}

func BenchmarkRasterLineTraversal(b *testing.B) {
	for _, mode := range []string{"whole", "visible-cold", "visible-hot"} {
		b.Run(mode, func(b *testing.B) {
			runtime.LockOSThread()
			defer runtime.UnlockOSThread()
			ctx, err := NewContext()
			if err != nil {
				b.Fatal(err)
			}
			defer ctx.Destroy()
			text := strings.Repeat(`{"title":"hello world","value":1234},`, 4500)
			l, err := ctx.NewTextLayout(text, typography.TextFormat{Font: typography.FontInfo{Family: "monospace", Size: 12}, WrapMode: typography.WrapWordChar}, 900, 1<<24)
			if err != nil {
				b.Fatal(err)
			}
			defer l.Destroy()
			whole := l.(*TextLayout)
			_, h := l.MeasureSize()
			region := image.Rect(0, int(h/2), 900, int(h/2)+700)
			var p textPainter
			if err := p.Init(region.Dx(), region.Dy()); err != nil {
				b.Fatal(err)
			}
			defer p.Destroy()
			whole.prepareRasterLines()
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if mode == "visible-cold" {
					whole.rasterLines = nil
					whole.rasterLinesValid = false
				}
				if mode != "whole" {
					if err := p.DrawTextLayout(whole, region, 1); err != nil {
						b.Fatal(err)
					}
					continue
				}
				cgo.Memset(cgo.CSlice(p.bitmap.Pixels), 0, cgo.Sizet(len(p.bitmap.Pixels)))
				p.context.Save()
				p.context.Translate(-float64(region.Min.X), -float64(region.Min.Y))
				r, g, bl, a := toColor(l.Format().TextColor)
				p.context.SetSourceRGBA(r, g, bl, a)
				p.context.MoveTo(0, 0)
				pango_cairo.ShowLayout(p.context, whole.layout)
				p.context.Restore()
			}
		})
	}
}

func TestRasterLinesFollowNativeContextSerial(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	ctx, err := NewContext()
	if err != nil {
		t.Fatal(err)
	}
	defer ctx.Destroy()
	l, err := ctx.NewTextLayout("native context generations", typography.TextFormat{WrapMode: typography.WrapWordChar}, 100, 1000)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Destroy()
	l.MeasureMetrics()
	whole := l.(*TextLayout)
	serial := whole.rasterSerial
	// Native font/context changes can precede a wrapper setter notification.
	whole.layout.ContextChanged()
	if whole.layout.GetSerial() == serial {
		t.Fatal("native context did not change serial")
	}
	region := image.Rect(0, 0, 110, 130)
	got, err := l.Rasterize(1, region, nil)
	if err != nil {
		t.Fatal(err)
	}
	if whole.rasterSerial == serial || whole.rasterSerial != whole.layout.GetSerial() {
		t.Fatal("borrowed line pointers survived native invalidation")
	}
	want := rasterWholeNative(t, whole, 1, region)
	if !bytes.Equal(got.Pixels, want.Pixels) {
		t.Fatal("native context invalidation changed raster output")
	}
}
