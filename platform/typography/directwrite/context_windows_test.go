package directwrite

import (
	"bytes"
	"github.com/goexlib/cgo"
	"github.com/golang-gui/goui/platform/windows/sdk/com"
	"image/color"
	"image/png"
	"os"
	"runtime"
	"testing"

	"github.com/golang-gui/goui/platform/typography"
	"github.com/golang-gui/goui/platform/typography/internal/typographytest"
	"github.com/golang-gui/goui/platform/windows/sdk/dwrite"
)

func TestMain(m *testing.M) { os.Exit(typographytest.Main(m)) }

var growingInlineMetrics = cgo.Symbol(cgo.NewCallback(func(object *inlineObject, result *dwrite.InlineObjectMetrics) com.HRESULT {
	growInlineCallbackStack(80)
	*result = object.metrics
	return com.S_OK
}))

func growInlineCallbackStack(depth int) byte {
	var frame [1024]byte
	frame[depth] = byte(depth)
	if depth > 0 {
		frame[0] = growInlineCallbackStack(depth - 1)
	}
	runtime.KeepAlive(&frame)
	return frame[depth]
}

func TestInlineMeasurementCallbackStackGrowth(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	original := inlineClass.GetMetrics
	inlineClass.GetMetrics = growingInlineMetrics
	defer func() { inlineClass.GetMetrics = original }()
	ctx, err := NewContext()
	if err != nil {
		t.Fatal(err)
	}
	defer ctx.Destroy()
	for i := 0; i < 40; i++ {
		layout, err := ctx.NewTextLayout("A\uFFFCB\uFFFCC", typography.TextFormat{Font: typography.FontInfo{Size: 16}, WrapMode: typography.WrapWordChar}, 200, 300)
		if err != nil {
			t.Fatal(err)
		}
		layout.SetPlaceholder(1, &typography.PlaceholderMetrics{Width: 37, Height: 42, Baseline: 28})
		layout.SetPlaceholder(5, &typography.PlaceholderMetrics{Width: 37, Height: 42, Baseline: 28})
		layout.SetSize(45, 300)
		lines, clusters := layout.MeasureMetrics()
		layout.Destroy()
		if len(lines) < 2 || len(clusters) != 5 {
			t.Fatalf("native callback invalidated output addresses: %d lines %d clusters", len(lines), len(clusters))
		}
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
	if err := ctx.AddFont(path); err != nil {
		t.Fatal(err)
	}
	c := ctx.(*Context)
	_, exists, hr := c.fonts().FindFamilyName("Go")
	if hr.Failed() || !exists {
		t.Fatalf("registered font missing from DirectWrite collection: %v", hr)
	}
	layout, err := ctx.NewTextLayout("Go", typography.TextFormat{Font: typography.FontInfo{Family: "Go", Size: 14}}, 200, 80)
	if err != nil {
		t.Fatal(err)
	}
	defer layout.Destroy()
	collection := c.fontCollection
	if err := ctx.AddFont(path); err != nil {
		t.Fatal(err)
	}
	if c.fontCollection != collection {
		t.Fatal("duplicate registration replaced collection")
	}
	if err := ctx.AddFont(path + ".missing"); err == nil {
		t.Fatal("missing file accepted")
	}
	if err := ctx.AddFont(typographytest.InvalidFontFile(t)); err == nil {
		t.Fatal("invalid font file accepted")
	}
	if c.fontCollection != collection {
		t.Fatal("failed registration replaced collection")
	}
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
	index, exists, hr := ctx.(*Context).fonts().FindFamilyName("Go")
	if hr.Failed() || !exists {
		t.Fatal("registered TTC family is absent")
	}
	family, hr := ctx.(*Context).fonts().GetFontFamily(int(index))
	if hr.Failed() {
		t.Fatal(hr)
	}
	defer family.Release()
	for _, tc := range []struct {
		weight dwrite.FontWeight
		style  dwrite.FontStyle
		name   string
	}{
		{dwrite.DWRITE_FONT_WEIGHT_BOLD, dwrite.DWRITE_FONT_STYLE_NORMAL, "Bold"},
		{dwrite.DWRITE_FONT_WEIGHT_NORMAL, dwrite.DWRITE_FONT_STYLE_ITALIC, "Italic"},
	} {
		font, hr := family.GetFirstMatchingFont(tc.weight, dwrite.DWRITE_FONT_STRETCH_NORMAL, tc.style)
		if hr.Failed() {
			t.Fatal(hr)
		}
		names, hr := font.GetFaceNames()
		if hr.Failed() {
			font.Release()
			t.Fatal(hr)
		}
		name, simulations := names.GetString(0), font.GetSimulations()
		names.Release()
		font.Release()
		if name != tc.name || simulations != 0 {
			t.Fatalf("actual TTC face=%q simulations=%d, want %q without simulation", name, simulations, tc.name)
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
	// DirectWrite DWRITE_STRIKETHROUGH.offset is the top edge, not its center.
	typographytest.Strikethrough(t, ctx, 4.0/3, 0.5, nil)
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
		TextColor: color.White,
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
	t.Logf("%fx%f", width, height)

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

func TestTextLayoutsOwnRasterResources(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	c, err := NewContext()
	if err != nil {
		t.Fatal(err)
	}
	defer c.Destroy()

	format := typography.TextFormat{
		Font:      typography.FontInfo{Family: "Microsoft YaHei", Size: 18},
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
	if want.Width != got.Width || want.Height != got.Height || want.Stride != got.Stride || !bytes.Equal(want.Pixels, got.Pixels) {
		t.Fatal("rasterizing another layout changed the first layout's bitmap")
	}
	if first.painter.width != firstWidth || first.painter.height != firstHeight {
		t.Fatal("rasterizing another layout changed the first layout's raster resources")
	}
}
