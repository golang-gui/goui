package pango

import (
	"image"
	"image/color"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	native "github.com/golang-gui/goui/platform/linux/libs/pango"
	"github.com/golang-gui/goui/platform/typography"
)

type retainedGlyph struct {
	offset              int
	level               uint8
	glyph               uint32
	geometry            native.GlyphGeometry
	clusterStart, color bool
}

func glyphSignature(runs []native.GlyphItem) (out []retainedGlyph) {
	for _, run := range runs {
		logs := run.Glyphs.LogClusters()
		for i, g := range run.Glyphs.GetGlyphInfos() {
			// Only defined PangoGlyphVisAttr bits are meaningful; its reserved
			// bits are not initialized by the native shaper.
			out = append(out, retainedGlyph{offset: int(run.Item.Offset) + int(logs[i]), level: run.Item.Analysis.Level, glyph: g.Glyph, geometry: g.Geometry, clusterStart: g.Attr.IsClusterStart(), color: g.Attr.IsColor()})
		}
	}
	return
}

func (r *retainedReflow) lineGlyphs(index int) []retainedGlyph {
	runs := r.lineRuns(index)
	nativeRuns := make([]native.GlyphItem, len(runs))
	for i := range runs {
		nativeRuns[i] = native.GlyphItem{Item: &runs[i].item, Glyphs: runs[i].glyphs}
	}
	return glyphSignature(nativeRuns)
}

func TestRetainedParagraphReflow(t *testing.T) {
	for _, sample := range []struct{ name, text string }{
		{"empty", ""},
		{"word", strings.Repeat("a", 2048)},
		{"json", strings.Repeat(`{"title":"hello world","value":1234},`, 30)},
		{"ligature", strings.Repeat("office affine difficult AVATAR ", 30)},
		{"cjk", strings.Repeat("中文排版测试（不能随意截断）", 20)},
		{"emoji", strings.Repeat("a👨‍👩‍👧‍👦e\u0301 ", 20)},
		{"arabic", strings.Repeat("مرحبا بالعالم مع العربية ", 30)},
		{"bidi", strings.Repeat("hello עברית 123 العربية world ", 30)},
		{"override", "start \u202e" + strings.Repeat("abc 123 العربية ", 30) + "\u202c end"},
		{"tabs", strings.Repeat("abc\tdef ghi\tjkl ", 30)},
		{"hyphens", strings.Repeat("co\u00adoperate zero\u200bwidth non\u00a0breaking ", 20)},
		{"isolates", strings.Repeat("L \u2067עברית 123 العربية\u2069 R \u2066abc\u2069 ", 20)},
		{"spaces", strings.Repeat("     \t \t   ", 20)},
		{"combining", "x" + strings.Repeat("\u0301", 500) + " y " + strings.Repeat("e\u0301fi ", 20)},
		{"separators", "hello\u2028world\u2029עברית\u2028abc"},
		{"empty_paragraphs", "\u2029\u2029hello\u2029"},
		{"palette", strings.Repeat("plain العربية colored ", 20)},
		{"palette_ligature", strings.Repeat("office affine ", 20)},
		{"fonts", strings.Repeat("plain العربية office ", 20)},
		{"linefeeds", "left\n\nabc\nالعربية\n"},
		{"default_tabs", strings.Repeat("abc\tdef ghi\tjkl ", 30)},
	} {
		t.Run(sample.name, func(t *testing.T) {
			runtime.LockOSThread()
			defer runtime.UnlockOSThread()
			ctx, err := NewContext()
			if err != nil {
				t.Fatal(err)
			}
			defer ctx.Destroy()
			f := typography.TextFormat{Font: typography.FontInfo{Family: "DejaVu Sans", Size: 12}, WrapMode: typography.WrapNone, TabWidth: 40}
			if sample.name == "default_tabs" {
				f.TabWidth = 0
			}
			style := func(l typography.TextLayout) {
				if sample.name == "palette_ligature" {
					l.SetTextColor(2, 1, color.NRGBA{R: 230, A: 153})
				}
				if sample.name == "fonts" {
					l.SetTextFont(0, 5, typography.FontInfo{Family: "DejaVu Sans", Size: 18})
					at := strings.Index(sample.text, "العربية")
					l.SetTextFont(at, len("العربية"), typography.FontInfo{Family: "DejaVu Serif", Size: 10})
				}
				if sample.name == "palette" {
					l.SetTextColor(0, len("plain"), color.NRGBA{R: 230, A: 153})
					at := strings.Index(sample.text, "العربية")
					l.SetTextColor(at, len("العربية"), color.NRGBA{G: 160, A: 255})
				}
			}
			base, e := ctx.NewTextLayout(sample.text, f, 520, 1<<24)
			if e != nil {
				t.Fatal(e)
			}
			style(base)
			base.MeasureSize()
			m, e := retainParagraph(base.(*TextLayout))
			base.Destroy()
			if e != nil {
				t.Fatal(e)
			}
			defer m.destroy()
			for _, wrap := range []typography.WrapMode{typography.WrapChar, typography.WrapWordChar} {
				for _, width := range []float32{1, 17, 80, 480, 520, 700} {
					f.WrapMode = wrap
					ref, e := ctx.NewTextLayout(sample.text, f, width, 1<<24)
					if e != nil {
						t.Fatal(e)
					}
					style(ref)
					want, _ := ref.MeasureMetrics()
					wantGlyphs := make([][]retainedGlyph, len(want))
					for i := range want {
						wantGlyphs[i] = glyphSignature(ref.(*TextLayout).layout.GetLineReadonly(i).GetRuns())
					}
					r := retainedReflow{paragraph: m, width: int64(width * native.Scale), wrap: wrap}
					for !r.advance(4) {
					}
					w, h := ref.MeasureSize()
					if r.sizeWidth != w || r.sizeHeight != h {
						t.Errorf("wrap=%d width=%g extent=%g/%g want%g/%g", wrap, width, r.sizeWidth, r.sizeHeight, w, h)
					}
					if len(r.lines) != len(want) {
						t.Errorf("wrap=%d width=%g line count %d want %d", wrap, width, len(r.lines), len(want))
					}
					for i, a := range r.lines[:min(len(r.lines), len(want))] {
						b := want[i]
						if a.Start != b.Start || a.Length != b.Length || a.Width != b.Width || a.X != b.X || a.Y != b.Y || a.Height != b.Height || a.Baseline != b.Baseline {
							t.Errorf("wrap=%d width=%g line=%d got=(%d,%d,%g) want=(%d,%d,%g)", wrap, width, i, a.Start, a.Length, a.Width, b.Start, b.Length, b.Width)
							t.Logf("geometry got x/y/h/b=%g/%g/%g/%g want=%g/%g/%g/%g", a.X, a.Y, a.Height, a.Baseline, b.X, b.Y, b.Height, b.Baseline)
							t.Logf("got %q want %q", sample.text[a.Start:a.Start+a.Length], sample.text[b.Start:b.Start+b.Length])
							break
						}
						if got := r.lineGlyphs(i); !slices.Equal(got, wantGlyphs[i]) {
							t.Errorf("glyph mismatch wrap=%d width=%g line=%d glyphs=%d want=%d", wrap, width, i, len(got), len(wantGlyphs[i]))
							break
						}
						if !slices.Equal(a.Clusters, b.Clusters) {
							t.Errorf("cluster mismatch wrap=%d width=%g line=%d got=%v want=%v", wrap, width, i, a.Clusters, b.Clusters)
							break
						}
					}
					for _, scale := range []float32{1, 2} {
						region := image.Rect(-2, 0, int((width+2)*scale), int(min(float32(200), want[len(want)-1].Y+want[len(want)-1].Height)*scale))
						actual, e := r.rasterize(scale, region)
						if e != nil {
							t.Fatal(e)
						}
						expected, e := ref.Rasterize(scale, region, nil)
						if e != nil {
							t.Fatal(e)
						}
						// Same native font/glyphs, premultiplied RGBA and phase. No
						// golden image, no cross-platform antialiasing tolerance.
						if !slices.Equal(actual.Pixels, expected.Pixels) {
							t.Errorf("prepared pixel mismatch wrap=%d width=%g scale=%g", wrap, width, scale)
						}
					}
					r.destroy()
					ref.Destroy()
				}
			}
		})
	}
}

func BenchmarkRetainedReflow(b *testing.B) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	ctx, e := NewContext()
	if e != nil {
		b.Fatal(e)
	}
	defer ctx.Destroy()
	f := typography.TextFormat{Font: typography.FontInfo{Family: "DejaVu Sans", Size: 12}}
	l, e := ctx.NewTextLayout(strings.Repeat("a", 32768), f, 520, 1<<24)
	if e != nil {
		b.Fatal(e)
	}
	l.MeasureSize()
	m, e := retainParagraph(l.(*TextLayout))
	l.Destroy()
	if e != nil {
		b.Fatal(e)
	}
	defer m.destroy()
	b.ResetTimer()
	var longest time.Duration
	for range b.N {
		r := retainedReflow{paragraph: m, width: 480 * native.Scale, wrap: typography.WrapWordChar}
		for {
			start := time.Now()
			done := r.advance(4)
			longest = max(longest, time.Since(start))
			if done {
				break
			}
		}
		r.destroy()
	}
	b.ReportMetric(float64(longest)/float64(time.Millisecond), "max-step-ms")
}
