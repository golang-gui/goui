package coretext

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"
	"unsafe"

	. "github.com/golang-gui/goui/platform/darwin/frameworks/core_foundation"
	. "github.com/golang-gui/goui/platform/darwin/frameworks/core_graphics"
	. "github.com/golang-gui/goui/platform/darwin/frameworks/core_text"
	"github.com/golang-gui/goui/platform/typography"
	"github.com/golang-gui/goui/platform/typography/internal/typographytest"
)

func TestMain(m *testing.M) { os.Exit(typographytest.Main(m)) }

func TestInlineDelegateABI(t *testing.T) {
	if unsafe.Sizeof(CTRunDelegateCallbacks{}) != 40 || unsafe.Offsetof(CTRunDelegateCallbacks{}.Deallocate) != 8 || unsafe.Offsetof(CTRunDelegateCallbacks{}.GetWidth) != 32 {
		t.Fatal("CTRunDelegateCallbacks differs from the macOS arm64 SDK layout")
	}
}

func TestFontRegistration(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	ctx, err := NewContext()
	if err != nil {
		t.Fatal(err)
	}
	defer ctx.Destroy()
	path := typographytest.FontFile(t)
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
	font := createCTFont(typography.FontInfo{Family: "Go", Size: 14})
	defer CFRelease(font)
	if family := CTFontCopyFamilyName(font); family != "Go" {
		t.Fatalf("registered font matched %q instead of Go", family)
	}
	other, err := NewContext()
	if err != nil {
		t.Fatal(err)
	}
	if err := other.AddFont(path); err != nil {
		t.Fatal(err)
	}
	other.Destroy() // cannot unregister the original context's process-wide font
	layout, err := ctx.NewTextLayout("Go", typography.TextFormat{Font: typography.FontInfo{Family: "Go", Size: 14}}, 200, 80)
	if err != nil {
		t.Fatal(err)
	}
	defer layout.Destroy()
	if _, err := layout.Rasterize(1, typographytest.RasterRegion(layout, 1), nil); err != nil {
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
	for _, tc := range []struct {
		font typography.FontInfo
		name string
	}{
		{typography.FontInfo{Family: "Go", Size: 20}, "GoRegular"},
		{typography.FontInfo{Family: "Go", Size: 20, Weight: typography.FontWeightBold}, "Go-Bold"},
		{typography.FontInfo{Family: "Go", Size: 20, Style: typography.FontStyleItalic}, "Go-Italic"},
	} {
		font := createCTFont(tc.font)
		name := CTFontCopyPostScriptName(font)
		CFRelease(font)
		if name != tc.name {
			t.Errorf("actual face=%q want %q", name, tc.name)
		}
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
	// CoreText exposes no public strike position metric and applies native
	// placement/grid fitting rather than the OS/2 metric verbatim. Use a bare
	// single-font native frame (without GOUI TextLayout) as the reference.
	typographytest.Strikethrough(t, ctx, 1, 0, func(size, scale float32) float64 {
		font := CTFontCreateWithName("Go", float64(size), nil)
		defer CFRelease(font)
		strike := CFNumberCreateInt32(1)
		defer CFRelease(strike)
		attrs := CFDictionaryCreate([]CFTypeRef{KCTFontAttributeName, KCTStrikethroughStyleAttributeName}, []CFTypeRef{font, strike})
		defer CFRelease(attrs)
		text := CFStringCreateWithString("M M")
		defer CFRelease(text)
		attributed := CFAttributedStringCreate(text, attrs)
		defer CFRelease(attributed)
		framesetter := CTFramesetterCreateWithAttributedString(attributed)
		defer CFRelease(framesetter)
		path := CGPathCreateWithRect(CGRectMake(0, 0, 128, 128), nil)
		defer CFRelease(path)
		frame := CTFramesetterCreateFrame(framesetter, CFRange{}, path, 0)
		defer CFRelease(frame)
		origins := []CGPoint{{}}
		CTFrameGetLineOrigins(frame, CFRangeMake(0, 1), origins)
		line := CFArrayGetValueAtIndex(CTFrameGetLines(frame), 0)
		x1, _ := CTLineGetOffsetForStringIndex(line, 1)
		x2, _ := CTLineGetOffsetForStringIndex(line, 2)
		pixels := make([]byte, 256*256*4)
		space := CGColorSpaceCreateDeviceRGB()
		defer CGColorSpaceRelease(space)
		cr := CGBitmapContextCreate(pixels, 256, 256, 8, 256*4, space, CGImageAlphaPremultipliedLast)
		defer CGContextRelease(cr)
		CGContextScaleCTM(cr, float64(scale), float64(scale))
		CGContextSetRGBFillColor(cr, 0, 0, 0, 1)
		CTFrameDraw(frame, cr)
		x := int((origins[0].X + (x1+x2)/2) * float64(scale))
		var mass, sum float64
		for y := 0; y < 256; y++ {
			alpha := float64(pixels[y*256*4+x*4+3])
			mass += alpha
			sum += (float64(y) + 0.5) * alpha
		}
		if mass == 0 {
			t.Fatal("native reference did not draw a strike")
		}
		return 256 - origins[0].Y*float64(scale) - sum/mass
	})
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

func TestWhitespaceRetainsMetrics(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	ctx, err := NewContext()
	if err != nil {
		t.Fatal(err)
	}
	defer ctx.Destroy()
	layout, err := ctx.NewTextLayout("    ", typography.TextFormat{Font: typography.FontInfo{Family: "Arial", Size: 14}}, 200, 80)
	if err != nil {
		t.Fatal(err)
	}
	defer layout.Destroy()
	lines, clusters := layout.MeasureMetrics()
	if len(lines) != 1 || len(clusters) != 4 || lines[0].Width <= 0 {
		t.Fatalf("whitespace lost native metrics: %+v %+v", lines, clusters)
	}
}

// Run the native lifetime regression in a subprocess so a dangling CF member
// reports a test failure rather than taking down the whole test suite.
func TestTextLayoutFontTraits(t *testing.T) {
	if os.Getenv("GOUI_CORETEXT_TRAITS_TEST") != "1" {
		executable, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command(executable, "-test.run=^TestTextLayoutFontTraits$", "-test.v", "-test.timeout=45s")
		cmd.Env = append(os.Environ(), "GOUI_CORETEXT_TRAITS_TEST=1")
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("native font traits regression: %v\n%s", err, output)
		}
		return
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	c, err := NewContext()
	if err != nil {
		t.Fatal(err)
	}
	defer c.Destroy()
	for i := 0; i < 50; i++ {
		font := typography.FontInfo{Family: "Arial", Size: 14, Weight: typography.FontWeightBold}
		layout, err := c.NewTextLayout("traits 中国", typography.TextFormat{Font: font, TextColor: color.Black}, 200, 80)
		if err != nil {
			t.Fatal(err)
		}
		func() {
			defer layout.Destroy()
			font.Weight = typography.FontWeightLight
			layout.SetTextFont(0, 6, font)
			width, height := layout.MeasureSize()
			if width <= 0 || height <= 0 || math.IsNaN(float64(width+height)) || math.IsInf(float64(width+height), 0) {
				t.Fatalf("invalid layout size: %g x %g", width, height)
			}
			lines, clusters := layout.MeasureMetrics()
			if len(lines) == 0 || len(clusters) == 0 {
				t.Fatal("missing text metrics")
			}
			for _, scale := range []float32{1, 2} {
				bitmap, err := layout.Rasterize(scale, typographytest.RasterRegion(layout, scale), nil)
				if err != nil {
					t.Fatal(err)
				}
				ink := false
				for y := 0; y < bitmap.Height; y++ {
					for x := 0; x < bitmap.Width; x++ {
						ink = ink || bitmap.Pixels[y*bitmap.Stride+x*4+3] != 0
					}
				}
				if !ink {
					t.Fatalf("empty raster at scale %g", scale)
				}
			}
		}()
	}
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
			Family: ".AppleSystemUIFont",
			Size:   32,
		},
		WrapMode:  typography.WrapWordChar,
		TextAlign: typography.TextAlignCenter,
		TextColor: color.RGBA{B: 160, A: 255},
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
			Family: ".AppleSystemUIFont",
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

	// The layout-owned retained CGContext must start every rasterization with an
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

	// Shrinking a retained bitmap must read the same rows as a fresh context,
	// including fractional scales and repeated changes in both directions.
	for _, scale := range []float32{1.25, 1.5, 1.75, 2, 1.5, 1} {
		got, err := layout.Rasterize(scale, typographytest.RasterRegion(layout, scale), nil)
		if err != nil {
			t.Fatal(err)
		}
		fresh, err := c.NewTextLayout("scale", format, 200, 80)
		if err != nil {
			t.Fatal(err)
		}
		want, err := fresh.Rasterize(scale, typographytest.RasterRegion(fresh, scale), nil)
		fresh.Destroy()
		if err != nil {
			t.Fatal(err)
		}
		assertSameBitmap(t, want, got)
	}
}

func TestTextLayoutsOwnRasterResources(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	c, err := NewContext()
	if err != nil {
		t.Fatal(err)
	}
	defer c.Destroy()

	format := typography.TextFormat{
		Font:      typography.FontInfo{Family: ".AppleSystemUIFont", Size: 18},
		WrapMode:  typography.WrapNone,
		TextAlign: typography.TextAlignBegin,
		TextColor: color.Black,
	}
	firstLayout, err := c.NewTextLayout("first", format, 100, 40)
	if err != nil {
		t.Fatal(err)
	}
	defer firstLayout.Destroy()
	secondLayout, err := c.NewTextLayout("a much larger second layout", format, 600, 100)
	if err != nil {
		t.Fatal(err)
	}
	defer secondLayout.Destroy()

	first := firstLayout.(*TextLayout)
	second := secondLayout.(*TextLayout)
	want, err := first.Rasterize(1, typographytest.RasterRegion(first, 1), nil)
	if err != nil {
		t.Fatal(err)
	}
	firstWidth, firstHeight := first.painter.bitmap.Width, first.painter.bitmap.Height
	if _, err = second.Rasterize(2, typographytest.RasterRegion(second, 2), nil); err != nil {
		t.Fatal(err)
	}
	if second.painter.bitmap.Width <= 0 || second.painter.bitmap.Height <= 0 {
		t.Fatal("second layout did not initialize its raster resources")
	}
	got, err := first.Rasterize(1, typographytest.RasterRegion(first, 1), nil)
	if err != nil {
		t.Fatal(err)
	}
	assertSameBitmap(t, want, got)
	if first.painter.bitmap.Width != firstWidth || first.painter.bitmap.Height != firstHeight {
		t.Fatal("rasterizing another layout changed the first layout's raster resources")
	}
}

func TestTextLayoutBitmapRespectsRequestedRegion(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	c, err := NewContext()
	if err != nil {
		t.Fatal(err)
	}
	defer c.Destroy()

	format := typography.TextFormat{
		Font:      typography.FontInfo{Family: ".AppleSystemUIFont", Size: 32},
		WrapMode:  typography.WrapNone,
		TextAlign: typography.TextAlignBegin,
		TextColor: color.Black,
	}
	layout, err := c.NewTextLayout("a long line that exceeds its bounds", format, 40, 20)
	if err != nil {
		t.Fatal(err)
	}
	defer layout.Destroy()

	bitmap, err := layout.Rasterize(1, image.Rect(0, 0, 40, 20), nil)
	if err != nil {
		t.Fatal(err)
	}
	if bitmap.Width > 40 || bitmap.Height > 20 {
		t.Fatalf("bitmap exceeds layout size: got %dx%d, max 40x20", bitmap.Width, bitmap.Height)
	}
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

// 首行位置不应因后面增加几千行而变化；先转 float32 再相减会丢失顶部的小数。
func TestTallLayoutPreservesFirstLinePosition(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	ctx, err := NewContext()
	if err != nil {
		t.Fatal(err)
	}
	defer ctx.Destroy()
	for _, size := range []float32{12.25, 14.25} {
		format := typography.TextFormat{Font: typography.FontInfo{Family: "Arial", Size: size}, WrapMode: typography.WrapWordChar}
		var first []typography.TextLine
		for _, count := range []int{1, 5000} {
			layout, err := ctx.NewTextLayout(strings.Repeat("alpha beta\n", count), format, 280, 1<<24)
			if err != nil {
				t.Fatal(err)
			}
			lines, _ := layout.MeasureMetrics()
			layout.Destroy()
			if len(lines) < count {
				t.Fatalf("字号 %g：只排版了 %d/%d 行", size, len(lines), count)
			}
			if first == nil {
				first = lines
				continue
			}
			if math.Abs(float64(lines[0].Y-first[0].Y)) > .001 || math.Abs(float64(lines[0].Baseline-first[0].Baseline)) > .001 {
				t.Fatalf("字号 %g：短布局首行=%+v，长布局首行=%+v", size, first[0], lines[0])
			}
		}
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

// A retained typesetter can create a finite line from the complete attributed
// paragraph. Unlike an independently allocated substring it retains native
// paragraph analysis. This probe does not establish safe arbitrary cut points.
func TestContextualSubstringShaping(t *testing.T) {
	for _, tc := range []struct{ text, target string }{
		{"start \u202e" + strings.Repeat("abcd 1234 ", 410) + "TARGET text\u202c end", "TARGET"},
		{"אבג \u202d" + strings.Repeat("abcd 1234 ", 410) + "TARGET text\u202c סוף", "TARGET"},
		{"start \u2067" + strings.Repeat("עברית العربية 123 ", 180) + "TARGET text\u2069 end", "TARGET"},
		{strings.Repeat("中文 e\u0301 ffi 👩‍💻 العربية עברית\t ", 130) + "TARGET text", "TARGET"},
		{strings.Repeat("العربية 中文 ", 300) + "العربية", "العربية"},
		{strings.Repeat("office ", 600) + "office", "ffi"},
		{strings.Repeat("text ", 900) + "e\u0301 end", "e\u0301"},
	} {
		t.Run(fmt.Sprintf("%q", tc.text[:min(len(tc.text), 16)]), func(t *testing.T) {
			runtime.LockOSThread()
			defer runtime.UnlockOSThread()
			ctx, err := NewContext()
			if err != nil {
				t.Fatal(err)
			}
			defer ctx.Destroy()
			l, err := ctx.NewTextLayout(tc.text, typography.TextFormat{Font: typography.FontInfo{Family: "Arial", Size: 12}}, 1<<24, 1<<24)
			if err != nil {
				t.Fatal(err)
			}
			defer l.Destroy()
			whole := l.(*TextLayout)
			l.MeasureSize()
			if len(whole.lines) != 1 {
				t.Fatalf("reference lines=%d", len(whole.lines))
			}
			setter := CTTypesetterCreateWithAttributedString(whole.attrString)
			if setter == 0 {
				t.Fatal("create typesetter failed")
			}
			defer CFRelease(setter)
			start := strings.LastIndex(tc.text, tc.target)
			r := CFRangeMake(whole.position.ToUtf16(start), whole.position.ToUtf16(start+len(tc.target))-whole.position.ToUtf16(start))
			line := CTTypesetterCreateLine(setter, r)
			if line == 0 {
				t.Fatal("create range line failed")
			}
			defer CFRelease(line)
			got, want := nativeGlyphFacts(line, r), nativeGlyphFacts(whole.lines[0].line, r)
			if len(got) == 0 || !reflect.DeepEqual(got, want) {
				t.Fatalf("range glyph facts=%+v full paragraph=%+v", got, want)
			}
		})
	}
}

type glyphFact struct {
	glyph   uint16
	index   CFIndex
	advance CGSize
	rtl     bool
}

func nativeGlyphFacts(line CTLineRef, r CFRange) []glyphFact {
	var result []glyphFact
	runs := CTLineGetGlyphRuns(line)
	for i := 0; i < CFArrayGetCount(runs); i++ {
		run := CFArrayGetValueAtIndex(runs, i)
		n := CTRunGetGlyphCount(run)
		glyphs := make([]uint16, n)
		advances := make([]CGSize, n)
		indices := make([]CFIndex, n)
		CTRunGetGlyphs(run, CFRange{}, glyphs)
		CTRunGetAdvances(run, CFRange{}, advances)
		CTRunGetStringIndices(run, CFRange{}, indices)
		for j, index := range indices {
			if int(index) >= r.Location && int(index) < r.Location+r.Length {
				result = append(result, glyphFact{glyphs[j], index, advances[j], CTRunGetStatus(run)&KCTRunStatusRightToLeft != 0})
			}
		}
	}
	return result
}

// 离屏原生排版不创建窗口或显示连接；字节范围和视觉顺序预期不依赖字形光栅化或 hinting。
func TestTextMetricsClusterRanges(t *testing.T) {
	for _, text := range []string{"abc", "中", "a\n中", "e\u0301 office", "abc אבג xyz", "a\u200bb", "a😀👩‍💻z"} {
		t.Run(text, func(t *testing.T) {
			runtime.LockOSThread()
			defer runtime.UnlockOSThread()
			ctx, err := NewContext()
			if err != nil {
				t.Fatal(err)
			}
			defer ctx.Destroy()
			layout, err := ctx.NewTextLayout(text, typography.TextFormat{
				Font: typography.FontInfo{Family: "Arial", Size: 14}, WrapMode: typography.WrapWordChar,
			}, 500, 500)
			if err != nil {
				t.Fatal(err)
			}
			defer layout.Destroy()
			lines, flat := layout.MeasureMetrics()
			if len(lines) == 0 {
				t.Fatal("nonempty text has no lines")
			}
			count := 0
			for li, line := range lines {
				end := line.Start + line.Length
				at := line.Start
				if line.Width < 0 || line.Height <= 0 {
					t.Errorf("invalid line bounds: %+v", line)
				}
				for _, cluster := range line.Clusters {
					if cluster.Start != at || cluster.Length <= 0 || cluster.Start+cluster.Length > end {
						t.Errorf("line %d: cluster %+v, expected next start %d and positive in-line length", li, cluster, at)
					}
					if cluster.Start >= 0 && cluster.Start+cluster.Length <= len(text) && !utf8.ValidString(text[cluster.Start:cluster.Start+cluster.Length]) {
						t.Errorf("cluster splits UTF-8: %+v", cluster)
					}
					if cluster.Direction != typography.TextLeftToRight && cluster.Direction != typography.TextRightToLeft {
						t.Errorf("invalid direction: %+v", cluster)
					}
					if cluster.LineIndex != li || cluster.Width < 0 || cluster.Height <= 0 {
						t.Errorf("invalid cluster geometry: %+v", cluster)
					}
					at = cluster.Start + cluster.Length
					count++
				}
				if at != end {
					t.Errorf("line %d clusters stop at %d, want %d", li, at, end)
				}
			}
			if count != len(flat) {
				t.Fatalf("line clusters=%d, flat=%d", count, len(flat))
			}
			if text == "a\n中" && (len(lines) != 2 || lines[0].Length != 1 || lines[1].Start != 2 || lines[1].Length != 3) {
				t.Fatalf("newline was included in content: %+v", lines)
			}
			if lines[len(lines)-1].Start+lines[len(lines)-1].Length != len(text) {
				t.Fatal("last line does not cover end of text")
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
	layout, err := ctx.NewTextLayout("אבג", typography.TextFormat{
		Font: typography.FontInfo{Family: "Arial", Size: 14},
	}, 200, 100)
	if err != nil {
		t.Fatal(err)
	}
	defer layout.Destroy()
	lines, clusters := layout.MeasureMetrics()
	if len(lines) != 1 || len(clusters) != 3 {
		t.Fatalf("Hebrew: lines=%d clusters=%d", len(lines), len(clusters))
	}
	for i, cluster := range clusters {
		if cluster.Start != i*2 || cluster.Length != 2 || cluster.Direction != typography.TextRightToLeft {
			t.Errorf("Hebrew cluster %d = %+v", i, cluster)
		}
		if i > 0 && cluster.X >= clusters[i-1].X {
			t.Errorf("logical RTL cluster %d must be left of its predecessor", i)
		}
	}
	if lines[0].Width <= 0 {
		t.Fatalf("RTL line width = %v", lines[0].Width)
	}
}

// 原生库对空段落/末尾段落的行数行为不同；不在此合成编辑器光标行。
// 所有报告范围必须有效，非分隔符字节恰好覆盖一次，分隔符不属于正文。
func TestTextMetricsParagraphSeparators(t *testing.T) {
	for _, text := range []string{"", "\n", "\n\n", "a\n", "a\n\n中\n", "a\r\n\r\n中\r\n", "\n😀\n"} {
		t.Run(text, func(t *testing.T) {
			runtime.LockOSThread()
			defer runtime.UnlockOSThread()
			ctx, err := NewContext()
			if err != nil {
				t.Fatal(err)
			}
			defer ctx.Destroy()
			layout, err := ctx.NewTextLayout(text, typography.TextFormat{
				Font:     typography.FontInfo{Family: "Arial", Size: 14},
				WrapMode: typography.WrapWordChar,
			}, 500, 500)
			if err != nil {
				t.Fatal(err)
			}
			defer layout.Destroy()
			lines, clusters := layout.MeasureMetrics()
			covered := make([]bool, len(text))
			for _, line := range lines {
				if line.Start < 0 || line.Length < 0 || line.Start+line.Length > len(text) {
					t.Fatalf("invalid line range: %+v, text length %d", line, len(text))
				}
				content := text[line.Start : line.Start+line.Length]
				if !utf8.ValidString(content) {
					t.Fatalf("line splits UTF-8: %+v", line)
				}
				for _, r := range content {
					if r == '\r' || r == '\n' {
						t.Fatalf("separator included in line content: %+v", line)
					}
				}
			}
			for _, cluster := range clusters {
				end := cluster.Start + cluster.Length
				if cluster.Start < 0 || cluster.Length <= 0 || end > len(text) ||
					cluster.LineIndex < 0 || cluster.LineIndex >= len(lines) {
					t.Fatalf("invalid cluster: %+v", cluster)
				}
				line := lines[cluster.LineIndex]
				if cluster.Start < line.Start || end > line.Start+line.Length ||
					!utf8.ValidString(text[cluster.Start:end]) {
					t.Fatalf("cluster escapes line or splits UTF-8: %+v", cluster)
				}
				for i := cluster.Start; i < end; i++ {
					if covered[i] || text[i] == '\r' || text[i] == '\n' {
						t.Fatalf("byte %d duplicated or separator included: %+v", i, cluster)
					}
					covered[i] = true
				}
			}
			for i := range len(text) {
				if text[i] != '\r' && text[i] != '\n' && !covered[i] {
					t.Fatalf("nonseparator byte %d has no cluster", i)
				}
			}
		})
	}
}

// Preserve the previous native export as a reference. This test compares every
// line/cluster value; sharing scratch storage must not change shaping or output.
func metricsWithPerRunStorage(t *TextLayout) (lines []typography.TextLine, clusters []typography.TextCluster) {
	t.ensureFrame()

	if len(t.lines) == 0 {
		return
	}

	// 坐标先以原生精度转换；这里仍保留逐 run 分配作为 scratch 复用的参考。
	height := t.layoutSize.Height
	lines = make([]typography.TextLine, 0, len(t.lines))
	clusters = make([]typography.TextCluster, 0, t.textLength)

	for lineIndex, li := range t.lines {
		baseline := float32(height - li.origin.Y)
		lineX := float32(li.origin.X)
		lineY := float32(height - (li.origin.Y + li.ascent + li.leading))
		lineWidth := float32(li.width)
		lineHeight := float32(li.ascent + li.descent + li.leading)

		u8Start := t.position.ToUtf8(li.u16Start)
		u8End := t.position.ToUtf8(li.u16Start + li.u16Length)
		if u8End < 0 {
			u8End = len(t.text)
		}
		// CoreText includes the paragraph separator in a line's string range.
		// Keep it out of visible clusters, as in Pango and DirectWrite.
		u8End = u8Start + len(strings.TrimRight(t.text[u8Start:u8End], "\r\n\u2028\u2029"))

		line := typography.TextLine{
			Start:    u8Start,
			Length:   u8End - u8Start,
			X:        lineX,
			Y:        lineY,
			Width:    lineWidth,
			Height:   lineHeight,
			Baseline: baseline,
		}

		clustersBeg := len(clusters)
		clusterByIndex := make(map[int]int)

		// Get clusters from runs
		runs := CTLineGetGlyphRuns(li.line)
		runCount := CFArrayGetCount(runs)
		for ri := 0; ri < runCount; ri++ {
			run := CFArrayGetValueAtIndex(runs, ri)
			status := CTRunGetStatus(run)
			isRTL := (status & KCTRunStatusRightToLeft) != 0

			glyphCount := CTRunGetGlyphCount(run)
			if glyphCount == 0 {
				continue
			}

			positions := make([]CGPoint, glyphCount)
			CTRunGetPositions(run, CFRangeMake(0, 0), positions)
			advances := make([]CGSize, glyphCount)
			CTRunGetAdvances(run, CFRangeMake(0, 0), advances)

			indices := make([]CFIndex, glyphCount)
			CTRunGetStringIndices(run, CFRangeMake(0, 0), indices)

			// A string index may map to several positioned glyphs. Group by
			// that index (not glyph order), preserving native advance geometry.
			for gi := 0; gi < glyphCount; gi++ {
				u16Idx := int(indices[gi])
				u8Pos := t.position.ToUtf8(u16Idx)
				if u8Pos < u8Start || u8Pos >= u8End {
					continue
				}
				x0 := lineX + float32(positions[gi].X)
				x1 := x0 + float32(advances[gi].Width)
				left, right := min(x0, x1), max(x0, x1)
				if ci, ok := clusterByIndex[u8Pos]; ok {
					cluster := &clusters[ci]
					right = max(right, cluster.X+cluster.Width)
					cluster.X = min(left, cluster.X)
					cluster.Width = right - cluster.X
					continue
				}

				cluster := typography.TextCluster{
					Start:     u8Pos,
					X:         left,
					Y:         lineY,
					Width:     right - left,
					Height:    lineHeight,
					LineIndex: lineIndex,
				}
				if placeholder, ok := t.placeholders[u8Pos]; ok {
					cluster.Y, cluster.Height = line.Baseline-placeholder.Baseline, placeholder.Height
				}
				if isRTL {
					cluster.Direction = typography.TextRightToLeft
				}
				clusterByIndex[u8Pos] = len(clusters)
				clusters = append(clusters, cluster)
			}
		}

		line.Clusters = clusters[clustersBeg:]
		slices.SortFunc(line.Clusters, func(a, b typography.TextCluster) int {
			return a.Start - b.Start
		})
		for i := range line.Clusters {
			end := u8End
			if i+1 < len(line.Clusters) {
				end = line.Clusters[i+1].Start
			}
			line.Clusters[i].Length = end - line.Clusters[i].Start
		}
		lines = append(lines, line)
	}
	return
}

func TestMetricsScratchPreservesNativeGeometry(t *testing.T) {
	for _, text := range []string{"", "Aé中", "a\n\nb\n", "a\u2028\u2029b", "abc\tdef\t", "office e\u0301 👩‍👩‍👦 عربى אבג xyz", strings.Repeat("one two אבג e\u0301 \u2067RTL\u2069 word ", 100), "abc \u202eאבג (123)\u202c xyz", "a\ufffcb"} {
		for _, width := range []float32{31, 317, 1 << 20} {
			t.Run(fmt.Sprintf("%q/%g", text[:min(20, len(text))], width), func(t *testing.T) {
				runtime.LockOSThread()
				defer runtime.UnlockOSThread()
				ctx, err := NewContext()
				if err != nil {
					t.Fatal(err)
				}
				defer ctx.Destroy()
				layout, err := ctx.NewTextLayout(text, typography.TextFormat{Font: typography.FontInfo{Family: "Arial", Size: 14}, WrapMode: typography.WrapWordChar, TabWidth: 32}, width, 1<<20)
				if err != nil {
					t.Fatal(err)
				}
				defer layout.Destroy()
				if text == "a\ufffcb" {
					layout.SetPlaceholder(1, &typography.PlaceholderMetrics{Width: 17, Height: 21, Baseline: 16})
				}
				if len(text) > 0 {
					layout.SetTextColor(0, 1, color.RGBA{R: 160, A: 255})
				}
				native := layout.(*TextLayout)
				wantLines, wantClusters := metricsWithPerRunStorage(native)
				gotLines, gotClusters := layout.MeasureMetrics()
				if !reflect.DeepEqual(gotLines, wantLines) || !reflect.DeepEqual(gotClusters, wantClusters) {
					t.Fatal("scratch reuse changed native line or cluster metrics")
				}
				if text == "Aé中" && width > 100 {
					if len(gotClusters) != 3 {
						t.Fatalf("UTF-8 clusters = %d", len(gotClusters))
					}
					for i, start := range []int{0, 1, 3} {
						if gotClusters[i].Start != start || gotClusters[i].Length != i+1 {
							t.Fatalf("cluster %d = %+v", i, gotClusters[i])
						}
					}
				}
				// A subsequent call and changed width may reuse local buffers, but must
				// never mutate ordinary values already returned to an upper-layer cache.
				layout.SetSize(47, 1<<20)
				layout.MeasureMetrics()
				if !reflect.DeepEqual(gotLines, wantLines) || !reflect.DeepEqual(gotClusters, wantClusters) {
					t.Fatal("later measurement mutated returned metrics")
				}
			})
		}
	}
}

func BenchmarkMetricsScratch(b *testing.B) {
	text := strings.Repeat("alpha beta gamma delta ", 8192)
	for _, reuse := range []bool{false, true} {
		b.Run(fmt.Sprint(reuse), func(b *testing.B) {
			runtime.LockOSThread()
			defer runtime.UnlockOSThread()
			ctx, err := NewContext()
			if err != nil {
				b.Fatal(err)
			}
			defer ctx.Destroy()
			layout, err := ctx.NewTextLayout(text, typography.TextFormat{Font: typography.FontInfo{Family: "Arial", Size: 14}, WrapMode: typography.WrapWordChar}, 1000, 1<<24)
			if err != nil {
				b.Fatal(err)
			}
			defer layout.Destroy()
			layout.MeasureSize() // Compare export only, after the same native shaping.
			native := layout.(*TextLayout)
			b.ReportAllocs()
			b.SetBytes(int64(len(text)))
			b.ResetTimer()
			for range b.N {
				if reuse {
					layout.MeasureMetrics()
				} else {
					metricsWithPerRunStorage(native)
				}
			}
		})
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
