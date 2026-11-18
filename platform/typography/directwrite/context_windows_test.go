package directwrite

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/goexlib/cgo"
	"github.com/golang-gui/goui/platform/typography"
	"github.com/golang-gui/goui/platform/typography/internal/typographytest"
	"github.com/golang-gui/goui/platform/typography/utils"
	"github.com/golang-gui/goui/platform/windows/sdk/com"
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
	want, err := first.Rasterize(1, typographytest.RasterRegion(first, 1), nil)
	if err != nil {
		t.Fatal(err)
	}
	firstWidth, firstHeight := first.painter.width, first.painter.height
	if _, err = second.Rasterize(2, typographytest.RasterRegion(second, 2), nil); err != nil {
		t.Fatal(err)
	}
	if second.painter.width <= 0 || second.painter.height <= 0 {
		t.Fatal("second layout did not initialize its raster resources")
	}
	got, err := first.Rasterize(1, typographytest.RasterRegion(first, 1), nil)
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

func TestFullColorReplacementDropsObsoleteRanges(t *testing.T) {
	l := &TextLayout{text: "A中🙂Z", position: utils.CalcStringPosition("A中🙂Z")}
	for i := 0; i < 1000; i++ {
		l.SetTextColor(0, len(l.text), color.Black)
		l.SetTextColor(1, 3, color.White)
		if len(l.colors) != 2 {
			t.Fatalf("full overwrite retained obsolete ranges at update %d: %d", i, len(l.colors))
		}
		if l.colors[1].Range.StartPosition != 1 || l.colors[1].Range.Length != 1 {
			t.Fatal("UTF-8 range was not converted to UTF-16", l.colors[1].Range)
		}
	}
}

// Bulk native range boxes plus native cluster advances must agree with
// independent HitTestTextPosition queries, including resolved bidi and tabs.
func TestBulkMetricsMatchNativePositions(t *testing.T) {
	for _, text := range []string{"", "\n\n", "abc\n\n中\n", strings.Repeat("AV ffi e\u0301 👩‍💻 中文\t", 40), strings.Repeat("עברית (123) العربية \u2067abc\u2069 ", 30), "\u202e" + strings.Repeat("ab e\u0301 (123) ", 300) + "\u202c end", "a\ufffcb", strings.Repeat("alpha 12345 ", 12000)} {
		for _, align := range []typography.TextAlignment{typography.TextAlignBegin, typography.TextAlignCenter, typography.TextAlignEnd, typography.TextAlignFill} {
			t.Run(fmt.Sprintf("%q/%d", text[:min(len(text), 10)], align), func(t *testing.T) {
				runtime.LockOSThread()
				defer runtime.UnlockOSThread()
				ctx, err := NewContext()
				if err != nil {
					t.Fatal(err)
				}
				defer ctx.Destroy()
				l, err := ctx.NewTextLayout(text, typography.TextFormat{Font: typography.FontInfo{Family: "Arial", Size: 12}, WrapMode: typography.WrapWordChar, TextAlign: align, TabWidth: 40}, 173, 1<<24)
				if err != nil {
					t.Fatal(err)
				}
				defer l.Destroy()
				if len(text) > 20 {
					_, first := utf8.DecodeRuneInString(text)
					l.SetTextFont(0, first, typography.FontInfo{Family: "serif", Size: 22})
				}
				if at := strings.IndexRune(text, '\ufffc'); at >= 0 {
					l.SetPlaceholder(at, &typography.PlaceholderMetrics{Width: 31, Height: 47, Baseline: 30})
				}
				_, clusters := l.MeasureMetrics()
				whole := l.(*TextLayout)
				for _, c := range clusters {
					_, _, want, hr := whole.layout.HitTestTextPosition(whole.position.ToUtf16(c.Start), false)
					if hr.Failed() {
						t.Fatal(hr)
					}
					// Both values are float32 native logical units. Allow at most two ULPs
					// for a different summation order, never a font- or text-sized tolerance.
					tolerance := float64(math.Nextafter32(want.Left, float32(math.Inf(1)))-want.Left) * 2
					if math.Abs(float64(c.X)-float64(want.Left)) > tolerance || uint32(c.Direction) != want.BidiLevel&1 {
						t.Fatalf("cluster %d: x=%v dir=%v, native x=%v level=%v tolerance=%v", c.Start, c.X, c.Direction, want.Left, want.BidiLevel, tolerance)
					}
				}
				if len(text) > 100000 && align != typography.TextAlignFill {
					cms, hr := whole.layout.GetClusterMetrics()
					if hr.Failed() {
						t.Fatal(hr)
					}
					runs := whole.clusterMetricRuns(cms)
					valid := 0
					for _, r := range runs {
						if r.valid {
							valid++
						}
					}
					if valid < len(runs)/2 || valid == 0 {
						t.Fatalf("fixture did not exercise bulk positions: %d/%d", valid, len(runs))
					}
				}
			})
		}
	}
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
