package pango

import (
	"bytes"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"

	"github.com/golang-gui/goui/platform/linux/libs/fontconfig"
	native "github.com/golang-gui/goui/platform/linux/libs/pango"
	"github.com/golang-gui/goui/platform/typography"
	"github.com/golang-gui/goui/platform/typography/internal/typographytest"
)

func TestMain(m *testing.M) { os.Exit(typographytest.Main(m)) }

func TestFontRegistration(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	ctx, err := NewContext()
	if err != nil {
		t.Fatal(err)
	}
	defer ctx.Destroy()
	path := typographytest.FontFile(t)
	pattern := fontconfig.FreeTypeQuery(path, 0)
	if pattern == 0 {
		t.Fatal("invalid fixture")
	}
	family, _ := pattern.GetString("family", 0)
	pattern.Destroy()
	if family != "Go" {
		t.Fatalf("fixture family=%q", family)
	}
	// Seed the fallback cache before registration.
	before, err := ctx.NewTextLayout("Go", typography.TextFormat{Font: typography.FontInfo{Family: "Go", Size: 14}}, 200, 80)
	if err != nil {
		t.Fatal(err)
	}
	defer before.Destroy()
	before.MeasureSize()
	for i := 0; i < 2; i++ {
		if err := ctx.AddFont(path); err != nil {
			t.Fatal(err)
		}
	}
	if err := ctx.AddFont(path + ".missing"); err == nil {
		t.Fatal("missing file accepted")
	}
	if err := ctx.AddFont(typographytest.InvalidFontFile(t)); err == nil {
		t.Fatal("invalid font file accepted")
	}
	after, err := ctx.NewTextLayout("Go", typography.TextFormat{Font: typography.FontInfo{Family: "Go", Size: 14}}, 200, 80)
	if err != nil {
		t.Fatal(err)
	}
	defer after.Destroy()
	iter := after.(*TextLayout).layout.GetIter()
	defer iter.Free()
	desc := native.Font(iter.GetRunReadonly().Item.Analysis.Font).Describe()
	defer desc.Free()
	if desc.GetFamily() != "Go" {
		t.Fatalf("registered font matched %q instead of Go", desc.GetFamily())
	}
	if _, err := before.Rasterize(1, typographytest.RasterRegion(before, 1), nil); err != nil {
		t.Fatal(err)
	}
}

func TestFontStyles(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	ctx, err := NewContext()
	if err != nil {
		t.Fatal(err)
	}
	defer ctx.Destroy()
	typographytest.FontStyles(t, ctx)
	for _, font := range []typography.FontInfo{
		{Family: "Go", Size: 20, Weight: typography.FontWeightBold},
		{Family: "Go", Size: 20, Style: typography.FontStyleItalic},
	} {
		layout, err := ctx.NewTextLayout("Go", typography.TextFormat{Font: font}, 200, 80)
		if err != nil {
			t.Fatal(err)
		}
		iter := layout.(*TextLayout).layout.GetIter()
		matched := native.Font(iter.GetRunReadonly().Item.Analysis.Font)
		desc := matched.Describe()
		style := []native.Style{native.StyleNormal, native.StyleItalic, native.StyleOblique}[font.Style]
		wantFace := "Bold"
		if font.Style == typography.FontStyleItalic {
			wantFace = "Italic"
		}
		if desc.GetFamily() != "Go" || matched.GetFace().GetFaceName() != wantFace || desc.GetStyle() != style {
			t.Errorf("actual face differs: family=%q weight=%d style=%d, request=%+v", desc.GetFamily(), desc.GetWeight(), desc.GetStyle(), font)
		}
		desc.Free()
		iter.Free()
		layout.Destroy()
	}
}

func TestStrikethroughMetrics(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	ctx, err := NewContext()
	if err != nil {
		t.Fatal(err)
	}
	defer ctx.Destroy()
	typographytest.Strikethrough(t, ctx, 4.0/3, 0, nil)
}

func TestPlaceholders(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	ctx, err := NewContext()
	if err != nil {
		t.Fatal(err)
	}
	defer ctx.Destroy()
	typographytest.Placeholders(t, ctx)
}

func Test_TextLayout(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	c, err := NewContext()
	if err != nil {
		t.Fatal(err)
	}
	defer c.Destroy()

	format := typography.TextFormat{
		Font: typography.FontInfo{
			Family: "Microsoft YaHei",
			Size:   32,
		},
		WrapMode:  typography.WrapWordChar,
		TextAlign: typography.TextAlignCenter,
	}

	text := "abc中国中文👨‍👩‍👧‍👦 مشروع "
	layout, err := c.NewTextLayout(text, format, 200, 100)
	if err != nil {
		t.Fatal(err)
	}
	defer layout.Destroy()

	layout.SetTextFont(0, 3, typography.FontInfo{
		Family: "mono sans",
		Size:   24,
	})
	layout.SetTextColor(0, 3, color.RGBA{R: 160, A: 255})
	layout.SetUnderline(0, 1, true)
	layout.SetStrikethrough(1, 2, true)

	width, height := layout.MeasureSize()
	t.Logf("size=%fx%f", width, height)

	lines, clusters := layout.MeasureMetrics()
	t.Logf("lines=%d clusters=%d", len(lines), len(clusters))

	bitmap, err := layout.Rasterize(2.0, typographytest.RasterRegion(layout, 2.0), nil)
	if err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	buf.Grow(bitmap.Width * bitmap.Height)
	err = png.Encode(&buf, bitmap)
	if err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(t.TempDir(), "text.png"), buf.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestTextLayoutEmptyMeasureMetrics(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	c, err := NewContext()
	if err != nil {
		t.Fatal(err)
	}
	defer c.Destroy()

	format := typography.TextFormat{
		Font: typography.FontInfo{
			Family: "Microsoft YaHei",
			Size:   32,
		},
		WrapMode: typography.WrapWordChar,
	}

	layout, err := c.NewTextLayout("", format, 200, 100)
	if err != nil {
		t.Fatal(err)
	}
	defer layout.Destroy()

	_, clusters := layout.MeasureMetrics()
	if len(clusters) != 0 {
		t.Fatalf("empty text should not produce clusters, got %d", len(clusters))
	}
	bitmap, err := layout.Rasterize(1, typographytest.RasterRegion(layout, 1), nil)
	if err != nil {
		t.Fatal(err)
	}
	if bitmap.Width != 0 || bitmap.Height != 0 || len(bitmap.Pixels) != 0 {
		t.Fatalf("empty text raster = %dx%d/%d bytes", bitmap.Width, bitmap.Height, len(bitmap.Pixels))
	}
}

func TestTextLayoutsOwnRasterResourcesAndDoNotLeakPixels(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	ctx, err := NewContext()
	if err != nil {
		t.Fatal(err)
	}
	defer ctx.Destroy()
	small, err := ctx.NewTextLayout("small", typography.TextFormat{
		Font: typography.FontInfo{Family: "sans", Size: 12},
	}, 100, 40)
	if err != nil {
		t.Fatal(err)
	}
	defer small.Destroy()
	smallNative := small.(*TextLayout)
	first, err := small.Rasterize(1, typographytest.RasterRegion(small, 1), nil)
	if err != nil {
		t.Fatal(err)
	}

	large, err := ctx.NewTextLayout("a much larger layout used to grow the shared raster surface", typography.TextFormat{
		Font: typography.FontInfo{Family: "sans", Size: 48},
	}, 1200, 200)
	if err != nil {
		t.Fatal(err)
	}
	defer large.Destroy()
	largeNative := large.(*TextLayout)
	if smallNative.context.GObject == largeNative.context.GObject {
		t.Fatal("text layouts unexpectedly share a PangoContext")
	}
	smallWidth, smallHeight := smallNative.painter.bitmap.Width, smallNative.painter.bitmap.Height
	if _, err = large.Rasterize(2, typographytest.RasterRegion(large, 2), nil); err != nil {
		t.Fatal(err)
	}
	if largeNative.painter.bitmap.Width <= 0 || largeNative.painter.bitmap.Height <= 0 {
		t.Fatal("large layout did not initialize its raster resources")
	}

	second, err := small.Rasterize(1, typographytest.RasterRegion(small, 1), nil)
	if err != nil {
		t.Fatal(err)
	}
	assertSameBitmap(t, first, second)
	if smallNative.painter.bitmap.Width != smallWidth || smallNative.painter.bitmap.Height != smallHeight {
		t.Fatal("rasterizing another layout changed the first layout's raster resources")
	}
}

func TestTextLayoutRasterScaleDoesNotAccumulate(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	c, err := NewContext()
	if err != nil {
		t.Fatal(err)
	}
	defer c.Destroy()

	format := typography.TextFormat{
		Font: typography.FontInfo{
			Family: "sans",
			Size:   18,
		},
		WrapMode:  typography.WrapNone,
		TextAlign: typography.TextAlignBegin,
		TextColor: color.Black,
	}

	layout, err := c.NewTextLayout("scale", format, 200, 80)
	if err != nil {
		t.Fatal(err)
	}
	defer layout.Destroy()

	first2x, err := layout.Rasterize(2, typographytest.RasterRegion(layout, 2), nil)
	if err != nil {
		t.Fatal(err)
	}

	// The layout-owned retained cairo context must start every rasterization with an
	// identity CTM, even when drawing the same layout again.
	layout.SetTextAlignment(format.TextAlign)
	second2x, err := layout.Rasterize(2, typographytest.RasterRegion(layout, 2), nil)
	if err != nil {
		t.Fatal(err)
	}
	assertSameBitmap(t, first2x, second2x)

	actual1x, err := layout.Rasterize(1, typographytest.RasterRegion(layout, 1), nil)
	if err != nil {
		t.Fatal(err)
	}
	fresh, err := c.NewTextLayout("scale", format, 200, 80)
	if err != nil {
		t.Fatal(err)
	}
	defer fresh.Destroy()
	want1x, err := fresh.Rasterize(1, typographytest.RasterRegion(fresh, 1), nil)
	if err != nil {
		t.Fatal(err)
	}
	assertSameBitmap(t, want1x, actual1x)
}

func assertSameBitmap(t *testing.T, want, got typography.TextBitmap) {
	t.Helper()
	if want.Width != got.Width || want.Height != got.Height || want.Stride != got.Stride {
		t.Fatalf("bitmap dimensions differ: want=%dx%d/%d got=%dx%d/%d",
			want.Width, want.Height, want.Stride, got.Width, got.Height, got.Stride)
	}
	if !bytes.Equal(want.Pixels, got.Pixels) {
		t.Fatal("bitmap pixels differ")
	}
}

// 原生资源统计不能改变创建、重排、栅格化和幂等销毁的契约。
func TestDiagnosticsResources(t *testing.T) { typographytest.Diagnostics(t, NewContext) }

// 离屏文字栅格化只依赖原生库；以通道/alpha 不变量为参考，不使用平台字形金图。
// 保留 8 位覆盖率/颜色量化容差：R 与 A 相差最多 1/255，最大 alpha 相差最多 2/255。
func TestTextPremultipliedAlpha(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	ctx, err := NewContext()
	if err != nil {
		t.Fatal(err)
	}
	defer ctx.Destroy()
	for _, scale := range []float32{1, 2} {
		for _, ranged := range []bool{false, true} {
			ink := color.NRGBA{R: 255, A: 128}
			format := typography.TextFormat{Font: typography.FontInfo{Family: "sans", Size: 48}, TextColor: ink}
			if ranged {
				format.TextColor = color.NRGBA{B: 255, A: 64}
			}
			layout, err := ctx.NewTextLayout("MMMM", format, 400, 100)
			if err != nil {
				t.Fatal(err)
			}
			func() {
				defer layout.Destroy()
				check := func(alpha byte) {
					t.Helper()
					bmp, err := layout.Rasterize(scale, typographytest.RasterRegion(layout, scale), nil)
					if err != nil {
						t.Fatal(err)
					}
					maximum := byte(0)
					for y := 0; y < bmp.Height; y++ {
						for x := 0; x < bmp.Width; x++ {
							i := y*bmp.Stride + x*4
							r, g, b, a := bmp.Pixels[i], bmp.Pixels[i+1], bmp.Pixels[i+2], bmp.Pixels[i+3]
							if int(r)-int(a) < -1 || int(r)-int(a) > 1 || g != 0 || b != 0 || a > alpha+1 && alpha < 255 {
								t.Fatalf("scale=%g ranged=%v alpha=%d: invalid red pixel %v", scale, ranged, alpha, bmp.Pixels[i:i+4])
							}
							maximum = max(maximum, a)
						}
					}
					if int(maximum)-int(alpha) < -2 || int(maximum)-int(alpha) > 2 {
						t.Fatalf("scale=%g ranged=%v max alpha=%d want %d", scale, ranged, maximum, alpha)
					}
				}
				if ranged {
					// 重复范围更新同时替换 RGB 和 alpha，覆盖全透明后恢复不透明的路径。
					for _, alpha := range []byte{128, 0, 255, 64} {
						layout.SetTextColor(0, 4, color.NRGBA{R: 255, A: alpha})
						check(alpha)
					}
				} else {
					check(128)
				}
			}()
		}
	}
}

// 新范围替换旧范围，即使旧范围起点更晚；以独立构造的最终样式为参考，字体和缩放相同。
func TestTextAttributesReplaceNestedRanges(t *testing.T) {
	for _, property := range []string{"color", "font", "underline", "strikethrough"} {
		t.Run(property, func(t *testing.T) {
			runtime.LockOSThread()
			defer runtime.UnlockOSThread()
			ctx, err := NewContext()
			if err != nil {
				t.Fatal(err)
			}
			defer ctx.Destroy()
			font := typography.FontInfo{Family: "sans", Size: 32}
			format := typography.TextFormat{Font: font, TextColor: color.NRGBA{R: 255, A: 128}}
			actual, err := ctx.NewTextLayout("MMMM", format, 500, 200)
			if err != nil {
				t.Fatal(err)
			}
			defer actual.Destroy()
			want, err := ctx.NewTextLayout("MMMM", format, 500, 200)
			if err != nil {
				t.Fatal(err)
			}
			defer want.Destroy()
			switch property {
			case "color":
				actual.SetTextColor(1, 2, color.NRGBA{B: 255, A: 255})
				actual.SetTextColor(0, 4, format.TextColor)
			case "font":
				actual.SetTextFont(1, 2, typography.FontInfo{Family: "monospace", Size: 48})
				actual.SetTextFont(0, 4, font)
			case "underline":
				actual.SetUnderline(1, 2, true)
				actual.SetUnderline(0, 4, false)
			case "strikethrough":
				actual.SetStrikethrough(1, 2, true)
				actual.SetStrikethrough(0, 4, false)
			}
			gotLines, gotClusters := actual.MeasureMetrics()
			wantLines, wantClusters := want.MeasureMetrics()
			if !reflect.DeepEqual(gotLines, wantLines) || !reflect.DeepEqual(gotClusters, wantClusters) {
				t.Fatal("replaced range retains old font metrics")
			}
			for _, scale := range []float32{1, 2} {
				gotBitmap, err := actual.Rasterize(scale, typographytest.RasterRegion(actual, scale), nil)
				if err != nil {
					t.Fatal(err)
				}
				wantBitmap, err := want.Rasterize(scale, typographytest.RasterRegion(want, scale), nil)
				if err != nil {
					t.Fatal(err)
				}
				assertSameBitmap(t, wantBitmap, gotBitmap)
			}
		})
	}
}

func TestTextColorClearOverlappingRanges(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	ctx, err := NewContext()
	if err != nil {
		t.Fatal(err)
	}
	defer ctx.Destroy()
	l, err := ctx.NewTextLayout("MMMM", typography.TextFormat{
		Font: typography.FontInfo{Family: "sans", Size: 32}, TextColor: color.Transparent,
	}, 500, 200)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Destroy()
	for _, scale := range []float32{1, 2} {
		l.SetTextColor(1, 2, color.NRGBA{B: 255, A: 255})
		l.SetTextColor(2, 2, color.NRGBA{G: 255, A: 64})
		l.SetTextColor(0, 4, color.Transparent)
		bmp, err := l.Rasterize(scale, typographytest.RasterRegion(l, scale), nil)
		if err != nil {
			t.Fatal(err)
		}
		for i, channel := range bmp.Pixels {
			if channel != 0 {
				t.Fatalf("scale=%g: clearing all text left pixel channel %d=%d", scale, i, channel)
			}
		}
	}
}

func TestTextColorOverlapPreservesOutsideRange(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	ctx, err := NewContext()
	if err != nil {
		t.Fatal(err)
	}
	defer ctx.Destroy()
	red := color.NRGBA{R: 255, A: 128}
	blue := color.NRGBA{B: 255, A: 255}
	green := color.NRGBA{G: 255, A: 64}
	format := typography.TextFormat{Font: typography.FontInfo{Family: "sans", Size: 32}, TextColor: red}
	actual, err := ctx.NewTextLayout("MMMM", format, 500, 200)
	if err != nil {
		t.Fatal(err)
	}
	defer actual.Destroy()
	actual.SetTextColor(1, 2, blue)
	actual.SetTextColor(2, 2, green)
	// 用互不重叠的范围独立构造最终预期样式。
	want, err := ctx.NewTextLayout("MMMM", format, 500, 200)
	if err != nil {
		t.Fatal(err)
	}
	defer want.Destroy()
	want.SetTextColor(1, 1, blue)
	want.SetTextColor(2, 2, green)
	for _, scale := range []float32{1, 2} {
		gotBitmap, err := actual.Rasterize(scale, typographytest.RasterRegion(actual, scale), nil)
		if err != nil {
			t.Fatal(err)
		}
		wantBitmap, err := want.Rasterize(scale, typographytest.RasterRegion(want, scale), nil)
		if err != nil {
			t.Fatal(err)
		}
		assertSameBitmap(t, wantBitmap, gotBitmap)
	}
}
