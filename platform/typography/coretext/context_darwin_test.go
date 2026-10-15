package coretext

import (
	"bytes"
	"image/color"
	"image/png"
	"math"
	"os"
	"os/exec"
	"runtime"
	"testing"
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
	if _, err := layout.Rasterize(1, nil); err != nil {
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
				bitmap, err := layout.Rasterize(scale, nil)
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

	bitmap, err := layout.Rasterize(2.0, nil)
	if err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	buf.Grow(bitmap.Width * bitmap.Height)
	err = png.Encode(&buf, bitmap)
	if err != nil {
		t.Fatal(err)
	}

	os.WriteFile("text.png", buf.Bytes(), 0666)
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

	first2x, err := layout.Rasterize(2, nil)
	if err != nil {
		t.Fatal(err)
	}

	// The layout-owned retained CGContext must start every rasterization with an
	// identity CTM, even when drawing the same layout again.
	layout.SetTextAlignment(format.TextAlign)
	second2x, err := layout.Rasterize(2, nil)
	if err != nil {
		t.Fatal(err)
	}
	assertSameBitmap(t, first2x, second2x)

	actual1x, err := layout.Rasterize(1, nil)
	if err != nil {
		t.Fatal(err)
	}
	fresh, err := c.NewTextLayout("scale", format, 200, 80)
	if err != nil {
		t.Fatal(err)
	}
	defer fresh.Destroy()
	want1x, err := fresh.Rasterize(1, nil)
	if err != nil {
		t.Fatal(err)
	}
	assertSameBitmap(t, want1x, actual1x)

	// Shrinking a retained bitmap must read the same rows as a fresh context,
	// including fractional scales and repeated changes in both directions.
	for _, scale := range []float32{1.25, 1.5, 1.75, 2, 1.5, 1} {
		got, err := layout.Rasterize(scale, nil)
		if err != nil {
			t.Fatal(err)
		}
		fresh, err := c.NewTextLayout("scale", format, 200, 80)
		if err != nil {
			t.Fatal(err)
		}
		want, err := fresh.Rasterize(scale, nil)
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
	want, err := first.Rasterize(1, nil)
	if err != nil {
		t.Fatal(err)
	}
	firstWidth, firstHeight := first.painter.width, first.painter.height
	if _, err = second.Rasterize(2, nil); err != nil {
		t.Fatal(err)
	}
	if second.painter.width <= 0 || second.painter.height <= 0 {
		t.Fatal("second layout did not initialize its raster resources")
	}
	got, err := first.Rasterize(1, nil)
	if err != nil {
		t.Fatal(err)
	}
	assertSameBitmap(t, want, got)
	if first.painter.width != firstWidth || first.painter.height != firstHeight {
		t.Fatal("rasterizing another layout changed the first layout's raster resources")
	}
}

func TestTextLayoutBitmapRespectsSize(t *testing.T) {
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

	bitmap, err := layout.Rasterize(1, nil)
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
