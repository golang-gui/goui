package pango

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
	"unsafe"

	"github.com/goexlib/cgo"
	"github.com/golang-gui/goui/platform/linux/libs/fontconfig"
	"github.com/golang-gui/goui/platform/linux/libs/pango"
	"github.com/golang-gui/goui/platform/linux/libs/pango_cairo"
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
	desc := pango.Font(iter.GetRunReadonly().Item.Analysis.Font).Describe()
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
		matched := pango.Font(iter.GetRunReadonly().Item.Analysis.Font)
		desc := matched.Describe()
		style := []pango.Style{pango.StyleNormal, pango.StyleItalic, pango.StyleOblique}[font.Style]
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

// Verify the missing fact before designing a cross-platform API: paragraph
// itemization is distinct from shaping a finite substring. Unlike replaying a
// stack of directional controls, native analysis preserves neutral/bracket and
// isolate resolution from the complete paragraph.
func TestContextualSubstringShaping(t *testing.T) {
	for _, tc := range []struct{ text, target string }{
		{"start \u202e" + strings.Repeat("abcd 1234 ", 410) + "TARGET text\u202c end", "TARGET"},
		{"אבג \u202d" + strings.Repeat("abcd 1234 ", 410) + "TARGET text\u202c סוף", "TARGET"},
		{"start \u2067" + strings.Repeat("עברית العربية 123 ", 180) + "TARGET text\u2069 end", "TARGET"},
		{strings.Repeat("中文 e\u0301 ffi 👩‍💻 العربية עברית\t ", 130) + "TARGET text", "TARGET"},
		{strings.Repeat("العربية 中文 ", 300) + "العربية", "عرب"},
		{strings.Repeat("office ", 600) + "office", "ffi"},
		{strings.Repeat("text ", 900) + "e\u0301 end", "e\u0301"},
	} {
		text := tc.text
		t.Run(fmt.Sprintf("%q", text[:min(16, len(text))]), func(t *testing.T) {
			runtime.LockOSThread()
			defer runtime.UnlockOSThread()
			ctx, err := NewContext()
			if err != nil {
				t.Fatal(err)
			}
			defer ctx.Destroy()
			format := typography.TextFormat{Font: typography.FontInfo{Family: "DejaVu Sans", Size: 12}}
			layout, err := ctx.NewTextLayout(text, format, 1<<24, 1<<24)
			if err != nil {
				t.Fatal(err)
			}
			defer layout.Destroy()
			whole := layout.(*TextLayout)
			attrs := pango.AttrListNew()
			defer attrs.Unref()
			desc, err := createFontDescription(format.Font)
			if err != nil {
				t.Fatal(err)
			}
			attr := pango.AttrFontDescNew(desc)
			desc.Free()
			attr.StartIndex, attr.EndIndex = 0, uint32(len(text))
			attrs.Insert(attr)
			items := whole.context.Itemize(text, 0, len(text), attrs)
			defer func() {
				for _, item := range items {
					item.Free()
				}
			}()
			start, end := strings.LastIndex(text, tc.target), strings.LastIndex(text, tc.target)+len(tc.target)
			var item *pango.Item
			for _, candidate := range items {
				if int(candidate.Offset) <= start && end <= int(candidate.Offset+candidate.Length) {
					item = candidate
					break
				}
			}
			if item == nil {
				t.Fatal("target has no native analysis item")
			}
			glyphs := pango.GlyphStringNew()
			defer glyphs.Free()
			buffer := cgo.CString(text)
			defer cgo.Free(buffer)
			var flags pango.ShapeFlags
			if whole.context.GetRoundGlyphPositions() {
				flags = pango.ShapeRoundPositions
			}
			pango.ShapeWithFlags(buffer, len(text), start, end-start, &item.Analysis, glyphs, flags)
			got := glyphs.GetGlyphInfos()
			logs := append([]int32(nil), glyphs.LogClusters()...)
			var want []pango.GlyphInfo
			var wantLogs []int32
			line := whole.layout.GetLineReadonly(0)
			for _, run := range line.GetRuns() {
				indices := run.Glyphs.LogClusters()
				for i, glyph := range run.Glyphs.GetGlyphInfos() {
					at := int(run.Item.Offset + indices[i])
					if at >= start && at < end {
						if run.Item.Analysis.Level != item.Analysis.Level {
							t.Fatal("lost paragraph bidi level")
						}
						want = append(want, glyph)
						wantLogs = append(wantLogs, int32(at-start))
					}
				}
			}
			if len(got) == 0 || !reflect.DeepEqual(logs, wantLogs) {
				t.Fatalf("cluster mapping: %v want %v", logs, wantLogs)
			}
			for i, g := range got {
				if g.Glyph != want[i].Glyph || g.Geometry != want[i].Geometry {
					t.Fatalf("glyph %d: %+v want %+v", i, g, want[i])
				}
			}
		})
	}
}

func TestAnalysisBindingPrefixes(t *testing.T) {
	// These prefix layouts are dereferenced by the bindings; native allocations
	// remain native-owned and are never allocated using these Go struct sizes.
	ptr := unsafe.Sizeof(uintptr(0))
	align := func(n uintptr) uintptr { return (n + ptr - 1) &^ (ptr - 1) }
	if unsafe.Offsetof(pango.Item{}.Analysis) != align(12) || unsafe.Sizeof(pango.Analysis{}) != align(5*ptr+4) || unsafe.Sizeof(pango.Item{}) != align(12)+align(5*ptr+4) {
		t.Fatal("unexpected PangoItem/PangoAnalysis ABI")
	}
}

func BenchmarkContextualShaping(b *testing.B) {
	for _, length := range []int{0, 512, 2048} {
		name := fmt.Sprintf("ShapeRange/%d", length)
		if length == 0 {
			name = "AnalyzeParagraph"
		}
		b.Run(name, func(b *testing.B) {
			runtime.LockOSThread()
			defer runtime.UnlockOSThread()
			ctx, err := NewContext()
			if err != nil {
				b.Fatal(err)
			}
			defer ctx.Destroy()
			text := strings.Repeat(`{"title":"hello world","value":1234},`, 4500)
			l, err := ctx.NewTextLayout(text, typography.TextFormat{Font: typography.FontInfo{Family: "monospace", Size: 12}}, 1<<24, 1<<24)
			if err != nil {
				b.Fatal(err)
			}
			defer l.Destroy()
			whole := l.(*TextLayout)
			attrs := pango.AttrListNew()
			defer attrs.Unref()
			desc, err := createFontDescription(l.Format().Font)
			if err != nil {
				b.Fatal(err)
			}
			attr := pango.AttrFontDescNew(desc)
			desc.Free()
			attr.StartIndex, attr.EndIndex = 0, uint32(len(text))
			attrs.Insert(attr)
			items := whole.context.Itemize(text, 0, len(text), attrs)
			defer func() {
				for _, item := range items {
					item.Free()
				}
			}()
			buffer := cgo.CString(text)
			defer cgo.Free(buffer)
			glyphs := pango.GlyphStringNew()
			defer glyphs.Free()
			var flags pango.ShapeFlags
			if whole.context.GetRoundGlyphPositions() {
				flags = pango.ShapeRoundPositions
			}
			b.ReportAllocs()
			if length == 0 {
				b.SetBytes(int64(len(text)))
			} else {
				b.SetBytes(int64(length))
			}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if length == 0 {
					next := whole.context.Itemize(text, 0, len(text), attrs)
					for _, item := range next {
						item.Free()
					}
				} else {
					pango.ShapeWithFlags(buffer, len(text), len(text)/2, length, &items[0].Analysis, glyphs, flags)
				}
			}
		})
	}
}

// The native cluster iterator is an independent geometry oracle. The optimized
// exporter walks native glyph arrays without NextCluster/GetClusterExtents per
// cluster; compare its exact integer-unit origins and advances with those APIs.
func TestBulkMetricsMatchNativeClusters(t *testing.T) {
	for _, text := range []string{"", "\n\n", "abc\n\n中\n", strings.Repeat("AV ffi e\u0301 👩‍💻 中文\t", 40), strings.Repeat("עברית (123) العربية \u2067abc\u2069 ", 30), "\u202e" + strings.Repeat("ab e\u0301 (123) ", 300) + "\u202c end", "a\ufffcb"} {
		for _, align := range []typography.TextAlignment{typography.TextAlignBegin, typography.TextAlignCenter, typography.TextAlignEnd, typography.TextAlignFill} {
			t.Run(fmt.Sprintf("%q/%d", text[:min(len(text), 10)], align), func(t *testing.T) {
				runtime.LockOSThread()
				defer runtime.UnlockOSThread()
				ctx, err := NewContext()
				if err != nil {
					t.Fatal(err)
				}
				defer ctx.Destroy()
				l, err := ctx.NewTextLayout(text, typography.TextFormat{Font: typography.FontInfo{Family: "DejaVu Sans", Size: 12}, WrapMode: typography.WrapWordChar, TextAlign: align, TabWidth: 40}, 173, 1<<24)
				if err != nil {
					t.Fatal(err)
				}
				defer l.Destroy()
				if len(text) > 20 {
					_, first := utf8.DecodeRuneInString(text)
					_, second := utf8.DecodeRuneInString(text[first:])
					l.SetTextFont(0, first, typography.FontInfo{Family: "serif", Size: 22})
					l.SetTextColor(first, second, color.NRGBA{R: 200, A: 255})
					l.SetUnderline(0, first+second, true)
				}
				if at := strings.IndexRune(text, '\ufffc'); at >= 0 {
					l.SetPlaceholder(at, &typography.PlaceholderMetrics{Width: 31, Height: 47, Baseline: 30})
				}
				lines, clusters := l.MeasureMetrics()
				whole := l.(*TextLayout)
				byStart := make(map[int]typography.TextCluster, len(clusters))
				for _, c := range clusters {
					byStart[c.Start] = c
				}
				iter := whole.layout.GetIter()
				defer iter.Free()
				visited := 0
				for {
					run := iter.GetRunReadonly()
					if run != nil && run.Item != nil {
						// Native indices for RTL clusters point at the final code point. Find
						// the exported logical range that contains that byte index.
						index := iter.GetIndex()
						var got typography.TextCluster
						found := false
						for _, c := range lines[nativeClusterLine(lines, index)].Clusters {
							if c.Start <= index && index < c.Start+c.Length {
								got = c
								found = true
								break
							}
						}
						if !found {
							t.Fatalf("native cluster at %d missing", index)
						}
						var rect pango.Rectangle
						iter.GetClusterExtents(nil, &rect)
						if got.X != float32(rect.X)/pango.Scale || got.Width != float32(rect.Width)/pango.Scale {
							t.Fatalf("cluster %d geometry=(%v,%v), native=(%v,%v)", got.Start, got.X, got.Width, float32(rect.X)/pango.Scale, float32(rect.Width)/pango.Scale)
						}
						delete(byStart, got.Start)
						visited++
					}
					if !iter.NextCluster() {
						break
					}
				}
				if visited != len(clusters) || len(byStart) != 0 {
					t.Fatalf("visited=%d exported=%d missing=%v", visited, len(clusters), byStart)
				}
			})
		}
	}
}

func nativeClusterLine(lines []typography.TextLine, index int) int {
	for i, l := range lines {
		if l.Start <= index && index < l.Start+l.Length {
			return i
		}
	}
	return 0
}

// 原生排版依赖 Pango/字体库，无需显示连接或窗口；字节范围与方向预期不依赖字体像素。
func TestTextMetricsClusterRanges(t *testing.T) {
	for _, text := range []string{"abc", "中", "a\n\n中\n", "e\u0301 office 👨‍👩‍👧‍👦", "abc אבג xyz", "\u202babc אבג\u202c"} {
		t.Run(text, func(t *testing.T) {
			runtime.LockOSThread()
			defer runtime.UnlockOSThread()
			ctx, err := NewContext()
			if err != nil {
				t.Fatal(err)
			}
			defer ctx.Destroy()
			layout, err := ctx.NewTextLayout(text, typography.TextFormat{
				Font: typography.FontInfo{Family: "sans", Size: 14}, WrapMode: typography.WrapWordChar,
			}, 100, 1000)
			if err != nil {
				t.Fatal(err)
			}
			defer layout.Destroy()
			lines, flat := layout.MeasureMetrics()
			if text == "a\n\n中\n" {
				if len(lines) != 4 || lines[1].Start != 2 || lines[1].Length != 0 || lines[3].Start != 7 || lines[3].Length != 0 {
					t.Fatalf("empty and trailing line metrics lost: %+v", lines)
				}
			}
			count := 0
			for li, line := range lines {
				end := line.Start + line.Length
				at := line.Start
				for _, cluster := range line.Clusters {
					if cluster.Start != at || cluster.Length <= 0 || cluster.Start+cluster.Length > end {
						t.Errorf("line %d range [%d,%d): cluster %+v, expected next start %d and positive in-line length", li, line.Start, end, cluster, at)
					}
					if cluster.Start >= 0 && cluster.Start+cluster.Length <= len(text) && !utf8.ValidString(text[cluster.Start:cluster.Start+cluster.Length]) {
						t.Errorf("cluster splits UTF-8: %+v", cluster)
					}
					if cluster.Direction != typography.TextLeftToRight && cluster.Direction != typography.TextRightToLeft {
						t.Errorf("embedding level exposed as direction: %+v", cluster)
					}
					if cluster.LineIndex != li || cluster.Width < 0 || cluster.Height <= 0 {
						t.Errorf("invalid cluster geometry: %+v", cluster)
					}
					at = cluster.Start + cluster.Length
					count++
				}
				if at != end {
					t.Errorf("line %d clusters stop at byte %d, want %d", li, at, end)
				}
			}
			if count != len(flat) {
				t.Fatalf("line clusters=%d flat clusters=%d", count, len(flat))
			}
		})
	}
}

func TestTextMetricsRTLVisualOrder(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	ctx, err := NewContext()
	if err != nil {
		t.Fatal(err)
	}
	defer ctx.Destroy()
	layout, err := ctx.NewTextLayout("אבג", typography.TextFormat{Font: typography.FontInfo{Family: "sans", Size: 14}}, 200, 100)
	if err != nil {
		t.Fatal(err)
	}
	defer layout.Destroy()
	_, clusters := layout.MeasureMetrics()
	if len(clusters) != 3 {
		t.Fatalf("Hebrew letters: got %d clusters, want 3", len(clusters))
	}
	for i, cluster := range clusters {
		if cluster.Start != i*2 || cluster.Length != 2 || cluster.Direction != typography.TextRightToLeft {
			t.Errorf("Hebrew cluster %d = %+v", i, cluster)
		}
		if i > 0 && cluster.X >= clusters[i-1].X {
			t.Errorf("logical RTL cluster %d must be left of its predecessor", i)
		}
	}
}

// The previous native cursor path is retained as a reference for reducing FFI
// queries. Every cluster position still comes from Pango; this does not shape
// independent substrings or infer bidi positions from glyph advances.
func metricsByClusterQueries(t *TextLayout) (lines []typography.TextLine, clusters []typography.TextCluster) {
	lineCount := t.layout.GetLineCount()
	if lineCount == 0 {
		return
	}
	lines = make([]typography.TextLine, 0, lineCount)
	clusters = make([]typography.TextCluster, 0, t.chars)
	iter := t.layout.GetIter()
	defer iter.Free()
	// Keep independent line/cluster cursors. NextCluster skips empty lines,
	// while copying a line cursor on Pango 1.50.6 loses end_x_offset and can
	// produce uninitialized X positions when advancing to the next font run.
	clusterIter := t.layout.GetIter()
	defer clusterIter.Free()
	var currentRun *pango.GlyphItem
	var clusterStarts []int
	clusterOrdinal := 0
	for {
		nativeLine := iter.GetLineReadonly()
		if nativeLine == nil {
			break
		}
		var ink, rect pango.Rectangle
		iter.GetLineExtents(&ink, &rect)
		line := typography.TextLine{
			Start: int(nativeLine.StartIndex), Length: int(nativeLine.Length),
			X:     float32(rect.X) / pango.Scale,
			Y:     float32(rect.Y) / pango.Scale,
			Width: float32(rect.Width) / pango.Scale, Height: float32(rect.Height) / pango.Scale,
			Baseline: float32(iter.GetBaseline()) / pango.Scale,
		}
		beg := len(clusters)
		// NextCluster skips empty lines. Keep a separate line iterator so empty
		// and trailing lines retain their native metrics and stable LineIndex.
		for clusterIter.GetLineReadonly() == nativeLine {
			run := clusterIter.GetRunReadonly()
			if run != nil && run.Item != nil {
				if run != currentRun {
					currentRun, clusterOrdinal = run, 0
					clusterStarts = clusterStarts[:0]
					logs := run.Glyphs.LogClusters()
					for i, offset := range logs {
						if i == 0 || offset != logs[i-1] {
							clusterStarts = append(clusterStarts, int(run.Item.Offset+offset))
						}
					}
				}
				// Pango's RTL iterator index identifies the last character of a
				// cluster, not its logical start. Take byte starts from the shaped
				// glyph log instead, in the iterator's same visual cluster order.
				start := clusterStarts[clusterOrdinal]
				clusterOrdinal++
				var ink, clusterRect pango.Rectangle
				clusterIter.GetClusterExtents(&ink, &clusterRect)
				clusters = append(clusters, typography.TextCluster{
					Start: start,
					X:     float32(clusterRect.X) / pango.Scale, Y: line.Y,
					Width: float32(clusterRect.Width) / pango.Scale, Height: line.Height,
					LineIndex: len(lines),
					Direction: typography.TextDirection(run.Item.Analysis.Level & 1),
				})
				if placeholder, ok := t.placeholders[start]; ok {
					cluster := &clusters[len(clusters)-1]
					cluster.Y, cluster.Height = line.Baseline-placeholder.Baseline, placeholder.Height
				}
			}
			if !clusterIter.NextCluster() {
				break
			}
		}
		line.Clusters = clusters[beg:]
		slices.SortFunc(line.Clusters, func(a, b typography.TextCluster) int { return a.Start - b.Start })
		for i := range line.Clusters {
			end := line.Start + line.Length
			if i+1 < len(line.Clusters) {
				end = line.Clusters[i+1].Start
			}
			line.Clusters[i].Length = end - line.Clusters[i].Start
		}
		lines = append(lines, line)
		if !iter.NextLine() {
			break
		}
	}
	return
}

func TestMetricsByRunsPreserveNativeClusterExtents(t *testing.T) {
	for _, text := range []string{"", "Aé中", "a\n\nb\n", "a\u2028\u2029b", "abc\tdef\t", "office e\u0301 👩‍👩‍👦 عربى אבג xyz", strings.Repeat("one two אבג e\u0301 \u2067RTL\u2069 word ", 100), "abc \u202eאבג (123)\u202c xyz"} {
		for _, width := range []float32{31, 317, 1 << 20} {
			t.Run(text[:min(20, len(text))]+"/"+fmt.Sprint(width), func(t *testing.T) {
				runtime.LockOSThread()
				defer runtime.UnlockOSThread()
				c, err := NewContext()
				if err != nil {
					t.Fatal(err)
				}
				defer c.Destroy()
				native, err := c.NewTextLayout(text, typography.TextFormat{Font: typography.FontInfo{Family: "Go", Size: 14}, WrapMode: typography.WrapWordChar}, width, 1<<24)
				if err != nil {
					t.Fatal(err)
				}
				defer native.Destroy()
				layout := native.(*TextLayout)
				// Split native attribute/font runs as well as fallback/script runs.
				if len(text) > 3 {
					native.SetTextColor(1, 2, color.NRGBA{R: 255, A: 255})
				}
				wantLines, wantClusters := metricsByClusterQueries(layout)
				lines, clusters := native.MeasureMetrics()
				if !reflect.DeepEqual(lines, wantLines) || !reflect.DeepEqual(clusters, wantClusters) {
					t.Fatal("run traversal changed native lines or cluster extents")
				}
				if text == "Aé中" && width == 1<<20 {
					if len(clusters) != 3 {
						t.Fatal("missing independent rune clusters", clusters)
					}
					for i, r := range [][2]int{{0, 1}, {1, 2}, {3, 3}} {
						if clusters[i].Start != r[0] || clusters[i].Length != r[1] || clusters[i].Direction != typography.TextLeftToRight {
							t.Fatal("wrong independent UTF-8 boundary", clusters)
						}
					}
				}
			})
		}
	}
}

func BenchmarkMetricsRunQueries(b *testing.B) {
	for _, reference := range []bool{true, false} {
		name := "runs"
		if reference {
			name = "per-cluster"
		}
		b.Run(name, func(b *testing.B) {
			runtime.LockOSThread()
			defer runtime.UnlockOSThread()
			c, err := NewContext()
			if err != nil {
				b.Fatal(err)
			}
			defer c.Destroy()
			text := strings.Repeat("one two three four five six seven ", 4096)
			native, err := c.NewTextLayout(text, typography.TextFormat{Font: typography.FontInfo{Family: "Go", Size: 14}, WrapMode: typography.WrapWordChar}, 1000, 1<<24)
			if err != nil {
				b.Fatal(err)
			}
			defer native.Destroy()
			native.MeasureSize()
			b.ReportAllocs()
			b.SetBytes(int64(len(text)))
			b.ResetTimer()
			for range b.N {
				var lines []typography.TextLine
				var clusters []typography.TextCluster
				if reference {
					lines, clusters = metricsByClusterQueries(native.(*TextLayout))
				} else {
					lines, clusters = native.MeasureMetrics()
				}
				if len(lines) == 0 || len(clusters) == 0 {
					b.Fatal("missing metrics")
				}
			}
		})
	}
}

// Regression fixture for future incremental layout: a measurement window
// starting after the RLO must retain the override from the paragraph prefix.
// UTF-8-safe cuts, word boundaries and a fixed overlap do not establish that.
func TestWrappedParagraphKeepsOverrideAcrossLongPrefix(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	ctx, err := NewContext()
	if err != nil {
		t.Fatal(err)
	}
	defer ctx.Destroy()
	prefix := "start \u202e" + strings.Repeat("abcd 1234 ", 400)
	text := prefix + "XYZ\u202c end"
	layout, err := ctx.NewTextLayout(text, typography.TextFormat{
		Font:     typography.FontInfo{Family: "DejaVu Sans", Size: 12},
		WrapMode: typography.WrapWordChar,
	}, 520, 1<<24)
	if err != nil {
		t.Fatal(err)
	}
	defer layout.Destroy()
	lines, clusters := layout.MeasureMetrics()
	if len(lines) < 20 {
		t.Fatalf("fixture should wrap over many lines; got %d", len(lines))
	}
	found := 0
	for _, c := range clusters {
		if c.Start >= len(prefix) && c.Start < len(prefix)+3 {
			if c.Direction != typography.TextRightToLeft {
				t.Fatalf("override lost after long prefix: %+v", c)
			}
			found += c.Length
		}
	}
	if found != 3 {
		t.Fatalf("XYZ coverage = %d bytes, want 3", found)
	}
}

// Explicit performance probe, excluded from normal go test. Context creation
// and input generation are outside timing; font caches are warm. Layout is
// creation plus first size measurement, Metrics is export after size measurement,
// and Reflow changes the width of a measured layout. No desktop is required.
// max-call-ms reports the largest synchronous call, not an event-loop latency.
func BenchmarkParagraphPhases(b *testing.B) {
	for _, size := range []int{8192, 32768} {
		for _, sample := range []struct{ name, unit string }{
			{"word", "a"},
			{"json", `{"title":"hello world","value":1234},`},
			{"bidi", "hello עברית 123 العربية world "},
		} {
			text := strings.Repeat(sample.unit, (size+len(sample.unit)-1)/len(sample.unit))
			for _, phase := range []string{"Layout", "Metrics", "Reflow"} {
				b.Run(fmt.Sprintf("%s/%d/%s", sample.name, size, phase), func(b *testing.B) {
					runtime.LockOSThread()
					defer runtime.UnlockOSThread()
					ctx, err := NewContext()
					if err != nil {
						b.Fatal(err)
					}
					defer ctx.Destroy()
					format := typography.TextFormat{
						Font:     typography.FontInfo{Family: "DejaVu Sans", Size: 12},
						WrapMode: typography.WrapWordChar, TabWidth: 40,
					}
					native, err := ctx.NewTextLayout(text, format, 520, 1<<24)
					if err != nil {
						b.Fatal(err)
					}
					native.MeasureSize()
					// Capture the variable so newly created layouts are also released.
					defer func() { native.Destroy() }()
					b.ReportAllocs()
					b.SetBytes(int64(len(text)))
					b.ResetTimer()
					var longest time.Duration
					for i := 0; i < b.N; i++ {
						if phase == "Layout" {
							b.StopTimer()
							native.Destroy()
							b.StartTimer()
						}
						start := time.Now()
						switch phase {
						case "Layout":
							next, createErr := ctx.NewTextLayout(text, format, 520, 1<<24)
							if createErr != nil {
								b.Fatal(createErr)
							}
							native = next
							native.MeasureSize()
						case "Metrics":
							native.MeasureMetrics()
						case "Reflow":
							native.SetSize(float32(480+40*(i%2)), 1<<24)
							native.MeasureSize()
						}
						longest = max(longest, time.Since(start))
					}
					b.StopTimer()
					b.ReportMetric(float64(longest)/float64(time.Millisecond), "max-call-ms")
				})
			}
		}
	}
}

// Pango's visual RTL iterator starts a cluster at its final character. Public
// cluster ranges must instead start at the logical UTF-8 boundary of the glyph
// cluster. e + combining acute must remain one range, even under RLO.
func TestRTLClusterLogicalStarts(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	ctx, err := NewContext()
	if err != nil {
		t.Fatal(err)
	}
	defer ctx.Destroy()
	text := "\u202ee\u0301 \u202c"
	l, err := ctx.NewTextLayout(text, typography.TextFormat{Font: typography.FontInfo{Family: "DejaVu Sans", Size: 12}}, 520, 200)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Destroy()
	_, clusters := l.MeasureMetrics()
	found := false
	for _, c := range clusters {
		if c.Start <= 3 && c.Start+c.Length > 3 {
			if c.Start != 3 || c.Length != 3 || c.Direction != typography.TextRightToLeft {
				t.Fatalf("combining cluster=%+v want logical range[3,6) RTL", c)
			}
			found = true
		}
	}
	if !found {
		t.Fatal("combining cluster missing")
	}
}

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

// 光栅化应绘制已测量的显示行，不按设备缩放重新换行；逐行设红色，其余文字透明，
// 检查所有墨迹落在该行垂直带内。依赖原生库但不依赖桌面，RGBA 字节为预乘值；
// 1 个物理像素容差仅覆盖逻辑行盒的光栅取整，不能包含第二行文字。
func TestTextLayoutRasterPreservesMeasuredLines(t *testing.T) {
	for _, scale := range []float32{1, 1.5, 2} {
		for width := float32(430); width <= 620; width += 5 {
			t.Run(fmt.Sprintf("scale=%g/width=%g", scale, width), func(t *testing.T) {
				runtime.LockOSThread()
				defer runtime.UnlockOSThread()
				ctx, err := NewContext()
				if err != nil {
					t.Fatal(err)
				}
				defer ctx.Destroy()
				text := "[000005:start] text editing, viewport layout; [000005:middle] selection and undo; [000005:end] 中文测试。"
				l, err := ctx.NewTextLayout(text, typography.TextFormat{
					Font:     typography.FontInfo{Family: "sans", Size: 18},
					WrapMode: typography.WrapWordChar, TextColor: color.Transparent,
				}, width, 1000)
				if err != nil {
					t.Fatal(err)
				}
				defer l.Destroy()
				lines, _ := l.MeasureMetrics()
				if len(lines) < 2 {
					t.Fatal("fixture did not wrap")
				}
				for i, line := range lines {
					colored, err := ctx.NewTextLayout(text, l.Format(), width, 1000)
					if err != nil {
						t.Fatal(err)
					}
					defer colored.Destroy()
					colored.SetTextColor(line.Start, line.Length, color.RGBA{R: 255, A: 255})
					beforeLines, beforeClusters := colored.MeasureMetrics()
					bitmap, err := colored.Rasterize(scale, typographytest.RasterRegion(colored, scale), nil)
					if err != nil {
						t.Fatal(err)
					}
					afterLines, afterClusters := colored.MeasureMetrics()
					if !reflect.DeepEqual(beforeLines, afterLines) || !reflect.DeepEqual(beforeClusters, afterClusters) {
						for j := range min(len(beforeClusters), len(afterClusters)) {
							if beforeClusters[j] != afterClusters[j] {
								t.Fatalf("rasterization changed cluster %d: before=%+v after=%+v", j, beforeClusters[j], afterClusters[j])
							}
						}
						t.Fatalf("rasterization changed lines: before=%+v after=%+v", beforeLines, afterLines)
					}
					minY := int(math.Floor(float64(line.Y*scale))) - 1
					maxY := int(math.Ceil(float64((line.Y+line.Height)*scale))) + 1
					ink := 0
					for y := 0; y < bitmap.Height; y++ {
						for x := 0; x < bitmap.Width; x++ {
							o := y*bitmap.Stride + 4*x
							if bitmap.Pixels[o+3] == 0 {
								continue
							}
							ink++
							if y < minY || y >= maxY {
								t.Fatalf("line %d [%d,%d) painted at y=%d outside measured band [%d,%d)", i, line.Start, line.Start+line.Length, y, minY, maxY)
							}
						}
					}
					if ink == 0 {
						t.Fatalf("line %d was not painted", i)
					}
				}
			})
		}
	}
}

func TestRasterRegions(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	ctx, err := NewContext()
	if err != nil {
		t.Fatal(err)
	}
	defer ctx.Destroy()
	typographytest.RasterRegions(t, ctx)
}

func TestRasterNativeFailureCanRetry(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	ctx, err := NewContext()
	if err != nil {
		t.Fatal(err)
	}
	defer ctx.Destroy()
	layout, err := ctx.NewTextLayout("retry", typography.TextFormat{}, 100, 40)
	if err != nil {
		t.Fatal(err)
	}
	defer layout.Destroy()
	native := layout.(*TextLayout)
	// Cairo image surfaces reject this width (including Cairo 1.16.0).
	// Repeat both before and after a successful rasterization: the old code
	// published failed capacity and dereferenced a null context on the retry.
	bad, good := image.Rect(0, 0, 40000, 1), image.Rect(0, 0, 100, 40)
	for pass := 0; pass < 2; pass++ {
		before := native.painter.context
		for i := 0; i < 2; i++ {
			if _, err := layout.Rasterize(1, bad, nil); err == nil {
				t.Fatal("Cairo accepted oversized surface")
			}
			if native.painter.context != before {
				t.Fatal("failed allocation changed the valid painter")
			}
			if before == 0 && (native.painter.bitmap.Width != 0 || native.painter.bitmap.Pixels != nil) {
				t.Fatal("failed initialization retained capacity")
			}
		}
		if _, err := layout.Rasterize(1, good, nil); err != nil {
			t.Fatal(err)
		}
	}
	// Sticky drawing errors must also discard the context and recover.
	native.painter.context.Scale(0, 0)
	if _, err := layout.Rasterize(1, good, nil); err == nil {
		t.Fatal("invalid Cairo matrix was ignored")
	}
	if native.painter.context != 0 {
		t.Fatal("sticky error context retained")
	}
	if _, err := layout.Rasterize(1, good, nil); err != nil {
		t.Fatal(err)
	}
}

func TestTabWidth(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	ctx, err := NewContext()
	if err != nil {
		t.Fatal(err)
	}
	defer ctx.Destroy()
	typographytest.Tabs(t, ctx)
}
