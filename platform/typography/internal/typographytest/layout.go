package typographytest

import (
	"bytes"
	"encoding/binary"
	"golang.org/x/image/font/gofont/goregular"
	"image/color"
	"math"
	"reflect"
	"strings"
	"testing"

	"github.com/golang-gui/goui/platform/typography"
)

// FontStyles checks base/range agreement, complete replacement and isolation.
// Native face identity is checked separately by each backend's package test.
func FontStyles(t *testing.T, ctx typography.Context) {
	t.Helper()
	RegisterFonts(t, ctx)
	regular := typography.FontInfo{Family: "Go", Size: 24}
	format := typography.TextFormat{Font: regular, TextColor: color.Black}
	base, err := ctx.NewTextLayout("Hello ffi", format, 500, 120)
	if err != nil {
		t.Fatal(err)
	}
	defer base.Destroy()
	peer, err := ctx.NewTextLayout(base.Text(), format, 500, 120)
	if err != nil {
		t.Fatal(err)
	}
	defer peer.Destroy()
	original, err := base.Rasterize(2, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, font := range []typography.FontInfo{
		{Family: "Go", Size: 24, Weight: typography.FontWeightBold},
		{Family: "Go", Size: 24, Style: typography.FontStyleItalic},
	} {
		format.Font = font
		fresh, err := ctx.NewTextLayout(base.Text(), format, 500, 120)
		if err != nil {
			t.Fatal(err)
		}
		want, err := fresh.Rasterize(2, nil)
		fresh.Destroy()
		if err != nil {
			t.Fatal(err)
		}
		base.SetTextFont(0, len(base.Text()), font)
		got, err := base.Rasterize(2, nil)
		if err != nil {
			t.Fatal(err)
		}
		if !sameBitmap(want, got) {
			t.Fatalf("base/range font differ: %+v", font)
		}
		if sameBitmap(original, got) {
			t.Fatalf("font style did not change glyph pixels: %+v", font)
		}
		peerBitmap, err := peer.Rasterize(2, nil)
		if err != nil || !sameBitmap(original, peerBitmap) {
			t.Fatalf("range changes polluted another layout: %v", err)
		}
		base.SetTextFont(0, len(base.Text()), regular)
		restored, err := base.Rasterize(2, nil)
		if err != nil || !sameBitmap(original, restored) {
			t.Fatalf("complete font replacement did not restore Regular: %v", err)
		}
	}
	for _, bad := range []typography.FontInfo{{Size: -1}, {Size: float32(math.Inf(1))}, {Weight: 99}, {Width: 10}, {Style: 3}} {
		if layout, err := ctx.NewTextLayout("bad", typography.TextFormat{Font: bad}, 100, 40); err == nil {
			layout.Destroy()
			t.Fatalf("invalid font accepted: %+v", bad)
		}
	}
	defaultLayout, err := ctx.NewTextLayout("default", typography.TextFormat{}, 300, 80)
	if err != nil {
		t.Fatal(err)
	}
	defer defaultLayout.Destroy()
	if width, height := defaultLayout.MeasureSize(); width <= 0 || height <= 0 {
		t.Fatal("zero font did not resolve platform defaults")
	}
}

func sameBitmap(a, b typography.TextBitmap) bool {
	return a.Width == b.Width && a.Height == b.Height && a.Stride == b.Stride && bytes.Equal(a.Pixels, b.Pixels)
}

// Strikethrough samples whitespace, so no glyph can masquerade as a decoration.
// Expected centers come from the fixture's OpenType OS/2 strike position, not
// another drawing path. Alpha/RGB use premultiplied RGBA; antialiasing may move
// the measured centroid by at most one physical pixel (native grid fitting).
func Strikethrough(t *testing.T, ctx typography.Context, pointsToLogical, thicknessOffset float32, nativeOffset func(size, scale float32) float64) {
	t.Helper()
	RegisterFonts(t, ctx)
	layout, err := ctx.NewTextLayout("M   M   M", typography.TextFormat{Font: typography.FontInfo{Family: "Go", Size: 18}, TextColor: color.NRGBA{R: 255, A: 128}}, 500, 160)
	if err != nil {
		t.Fatal(err)
	}
	defer layout.Destroy()
	layout.SetTextFont(4, 5, typography.FontInfo{Family: "Go", Size: 48})
	layout.SetTextColor(4, 5, color.NRGBA{B: 255, A: 64})
	lines, clusters := layout.MeasureMetrics()
	_, height := layout.MeasureSize()
	position, thickness, em := float32(0), float32(0), float32(0)
	font := goregular.TTF
	for i := 0; i < int(binary.BigEndian.Uint16(font[4:6])); i++ {
		table := font[12+i*16 : 12+(i+1)*16]
		offset := int(binary.BigEndian.Uint32(table[8:12]))
		if string(table[:4]) == "OS/2" {
			position = float32(int16(binary.BigEndian.Uint16(font[offset+28 : offset+30])))
			thickness = float32(int16(binary.BigEndian.Uint16(font[offset+26 : offset+28])))
		}
		if string(table[:4]) == "head" {
			em = float32(binary.BigEndian.Uint16(font[offset+18 : offset+20]))
		}
	}
	if position <= 0 || em <= 0 {
		t.Fatal("fixture lacks independent strikethrough metrics")
	}
	if len(clusters) != 9 || len(lines) != 1 {
		t.Fatalf("unexpected whitespace metrics: %d lines %d clusters", len(lines), len(clusters))
	}
	baselineBitmap, err := layout.Rasterize(1, nil)
	if err != nil {
		t.Fatal(err)
	}
	layout.SetStrikethrough(0, 9, true)
	afterLines, afterClusters := layout.MeasureMetrics()
	if !reflect.DeepEqual(lines, afterLines) || !reflect.DeepEqual(clusters, afterClusters) {
		t.Fatal("strikethrough changed text metrics")
	}
	for _, scale := range []float32{1, 1.5, 2} {
		bitmap, err := layout.Rasterize(scale, nil)
		if err != nil {
			t.Fatal(err)
		}
		for _, sample := range []struct {
			start   int
			size    float32
			alpha   byte
			channel int
		}{{2, 18, 128, 0}, {6, 48, 64, 2}} {
			cluster := clusters[sample.start]
			x := int((cluster.X + cluster.Width/2) * scale)
			var mass, sum float64
			for y := 0; y < bitmap.Height; y++ {
				pixel := bitmap.Pixels[y*bitmap.Stride+x*4 : y*bitmap.Stride+x*4+4]
				a := pixel[3]
				if a > sample.alpha+1 || math.Abs(float64(pixel[sample.channel])-float64(a)) > 1 {
					t.Fatalf("invalid premultiplied strike pixel: %v scale=%g x=%d y=%d cluster=%+v", pixel, scale, x, y, cluster)
				}
				mass += float64(a)
				sum += (float64(y) + 0.5) * float64(a)
			}
			if mass == 0 {
				t.Fatal("strikethrough was not drawn on whitespace")
			}
			want := float64((lines[0].Baseline - sample.size*pointsToLogical*(position-thicknessOffset*thickness)/em) * scale)
			if nativeOffset != nil {
				want = float64(lines[0].Baseline*scale) - nativeOffset(sample.size, scale)
			}
			if math.Abs(sum/mass-want) > 1 {
				t.Fatalf("strike center=%g want %g at scale %g size %g (layout height %g)", sum/mass, want, scale, sample.size, height)
			}
		}
	}
	layout.SetStrikethrough(0, 9, false)
	bitmap, err := layout.Rasterize(1, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !sameBitmap(bitmap, baselineBitmap) {
		t.Fatal("disabling strike did not restore original pixels")
	}
}

// Placeholders validates native advances/line baselines against requested
// logical geometry, and transparent RGBA pixels independently of glyph goldens.
func Placeholders(t *testing.T, ctx typography.Context) {
	t.Helper()
	format := typography.TextFormat{Font: typography.FontInfo{Size: 16}, WrapMode: typography.WrapWordChar}
	for _, text := range []string{"\uFFFC", "\uFFFC\uFFFC", "A\uFFFCB\uFFFCC", "אב\uFFFCגד\uFFFC", "😀a\uFFFC中文\uFFFC"} {
		layout, err := ctx.NewTextLayout(text, format, 80, 300)
		if err != nil {
			t.Fatal(err)
		}
		func() {
			defer layout.Destroy()
			original, err := layout.Rasterize(1, nil)
			if err != nil {
				t.Fatal(err)
			}
			positions := []int{}
			for start := 0; start < len(text); {
				next := strings.Index(text[start:], "\uFFFC")
				if next < 0 {
					break
				}
				positions = append(positions, start+next)
				start += next + 3
			}
			changed := 0
			handle := layout.ConnectChanged(func() { changed++ })
			defer handle.Disconnect()
			for _, start := range positions {
				metrics := typography.PlaceholderMetrics{Width: 37, Height: 42, Baseline: 28}
				layout.SetPlaceholder(start, &metrics)
				metrics.Width, metrics.Height = 999, 999 // caller storage is not retained
			}
			lines, clusters := layout.MeasureMetrics()
			for _, start := range positions {
				found := false
				for _, cluster := range clusters {
					if cluster.Start != start {
						continue
					}
					line := lines[cluster.LineIndex]
					if cluster.Length != 3 || math.Abs(float64(cluster.Width-37)) > 0.01 || cluster.Height != 42 || math.Abs(float64(cluster.Y+28-line.Baseline)) > 0.01 || line.Height < 41.99 {
						t.Fatalf("%q: incorrect inline geometry: cluster=%+v line=%+v", text, cluster, line)
					}
					found = true
				}
				if !found {
					t.Fatalf("%q: no independent cluster at %d: %+v", text, start, clusters)
				}
			}
			if len(positions) > 1 && text == "\uFFFC\uFFFC" && len(clusters) != 2 {
				t.Fatal("adjacent placeholders merged")
			}
			if strings.HasPrefix(text, "אב") {
				for _, cluster := range clusters {
					// The first placeholder is between Hebrew strong characters. A
					// trailing neutral may resolve differently with native paragraph
					// base direction; do not impose another bidi policy here.
					if cluster.Start == positions[0] && cluster.Direction != typography.TextRightToLeft {
						t.Fatal("placeholder lost resolved RTL direction")
					}
				}
			}
			for _, scale := range []float32{1, 1.5, 2} {
				bitmap, err := layout.Rasterize(scale, nil)
				if err != nil {
					t.Fatal(err)
				}
				if text == "\uFFFC" || text == "\uFFFC\uFFFC" {
					for _, pixel := range bitmap.Pixels {
						if pixel != 0 {
							t.Fatalf("%q: placeholder painted replacement glyph at scale %g", text, scale)
						}
					}
				}
			}
			layout.SetSize(45, 300)
			lines, _ = layout.MeasureMetrics()
			if len(positions) > 1 && len(lines) < 2 {
				t.Fatalf("%q: placeholders did not wrap", text)
			}
			layout.SetSize(80, 300)
			updated := typography.PlaceholderMetrics{Width: 23, Height: 60, Baseline: 41}
			layout.SetPlaceholder(positions[0], &updated)
			_, updatedClusters := layout.MeasureMetrics()
			for _, cluster := range updatedClusters {
				if cluster.Start == positions[0] && (math.Abs(float64(cluster.Width-23)) > 0.01 || cluster.Height != 60) {
					t.Fatal("updating placeholder metrics kept the old native dimensions")
				}
			}
			for _, start := range positions {
				layout.SetPlaceholder(start, nil)
			}
			restored, err := layout.Rasterize(1, nil)
			if err != nil || !sameBitmap(original, restored) {
				t.Fatalf("%q: clearing placeholder did not restore native text: %v", text, err)
			}
			if changed < 2*len(positions) {
				t.Fatal("placeholder mutations did not invalidate the layout")
			}
		}()
	}
	// Repeated native object replacement must not accumulate stale delegates or
	// corrupt another layout's independent raster resources.
	first, err := ctx.NewTextLayout("\uFFFC", format, 200, 100)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Destroy()
	second, err := ctx.NewTextLayout("\uFFFC", format, 200, 100)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Destroy()
	second.SetPlaceholder(0, &typography.PlaceholderMetrics{Width: 11, Height: 19, Baseline: 10})
	for i := 0; i < 50; i++ {
		first.SetPlaceholder(0, &typography.PlaceholderMetrics{Width: float32(20 + i), Height: 42, Baseline: 28})
		if _, err := first.Rasterize(1, nil); err != nil {
			t.Fatal(err)
		}
	}
	_, clusters := second.MeasureMetrics()
	if len(clusters) != 1 || math.Abs(float64(clusters[0].Width-11)) > 0.01 || clusters[0].Height != 19 {
		t.Fatal("native placeholder state contaminated another layout")
	}
	second.SetPlaceholder(0, &typography.PlaceholderMetrics{Width: 0, Height: 42, Baseline: 28})
	lines, clusters := second.MeasureMetrics()
	if len(clusters) != 1 || len(lines) != 1 || clusters[0].Width != 0 || clusters[0].Height != 42 || lines[0].Height < 42 {
		t.Fatalf("zero-width placeholder lost metrics: %+v %+v", lines, clusters)
	}
}
