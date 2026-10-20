package pango

import (
	"image"
	"image/color"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/golang-gui/goui/platform/typography"
)

func TestDeferredReflowPublicationAndCancellation(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	ctx, err := NewContext()
	if err != nil {
		t.Fatal(err)
	}
	defer ctx.Destroy()
	text := "start \u202e" + strings.Repeat("office עברית 123 العربية e\u0301 👨‍👩‍👧‍👦\t ", 150) + "\u202c end"
	f := typography.TextFormat{Font: typography.FontInfo{Family: "DejaVu Sans", Size: 12}, WrapMode: typography.WrapWordChar, TabWidth: 40}
	l, err := ctx.NewTextLayout(text, f, 520, 1<<24)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Destroy()
	native := l.(*TextLayout)
	old, _ := l.MeasureMetrics()
	oldWidth, oldHeight := l.MeasureSize()
	changes := 0
	l.ConnectChanged(func() { changes++ })
	if !native.Reflow(480, 1<<24) {
		t.Fatal("reflow declined")
	}
	done, err := native.AdvanceReflow(time.Now().Add(-time.Second))
	if err != nil || done || native.retained != nil {
		t.Fatal("expired budget did analysis")
	}
	native.AdvanceReflow(time.Now().Add(100 * time.Microsecond))
	if native.pending == nil {
		t.Fatal("fixture completed in first slice")
	}
	if w, h := l.MeasureSize(); w != oldWidth || h != oldHeight {
		t.Fatal("pending layout changed old extent")
	}
	current, _ := l.MeasureMetrics()
	if !slices.EqualFunc(old, current, sameTextLine) {
		t.Fatal("pending request changed input geometry")
	}
	measurement := native.retained
	if !native.Reflow(700, 1<<24) || native.retained != measurement {
		t.Fatal("new width discarded measurement")
	}
	for slices := 0; ; slices++ {
		done, err := native.AdvanceReflow(time.Now().Add(time.Millisecond))
		if err != nil {
			t.Fatal(err)
		}
		if done {
			break
		}
		if slices > 10000 {
			t.Fatal("reflow failed to converge")
		}
	}
	if changes != 1 {
		t.Fatalf("publication changes=%d want1", changes)
	}
	if w, _ := l.Size(); w != 700 {
		t.Fatalf("obsolete width published: %g", w)
	}
	reference, err := ctx.NewTextLayout(text, f, 700, 1<<24)
	if err != nil {
		t.Fatal(err)
	}
	defer reference.Destroy()
	assertDeferredMatches(t, l, reference)
	if len(native.prepared.resources) > 80 {
		t.Fatal("glyph cache retained more than three raster screens")
	}
	for _, y := range []int{1000, 0, 3000, 1000} {
		_, err = l.Rasterize(1, image.Rect(0, y, 700, y+180), nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(native.prepared.resources) > 32 {
			t.Fatalf("unbounded glyph cache at y=%d: %d", y, len(native.prepared.resources))
		}
	}
	l.SetTextColor(0, len(text), color.NRGBA{R: 100, B: 240, A: 153})
	reference.SetTextColor(0, len(text), color.NRGBA{R: 100, B: 240, A: 153})
	if native.retained != measurement || native.prepared == nil {
		t.Fatal("color invalidated measured text")
	}
	assertDeferredMatches(t, l, reference)
	native.Reflow(17, 1<<24)
	native.AdvanceReflow(time.Now().Add(time.Millisecond))
	native.CancelReflow()
	if w, _ := l.Size(); w != 700 || native.pending != nil {
		t.Fatal("cancel changed published layout")
	}
	l.SetSize(700, 1<<24)
	if native.retained != measurement || native.prepared == nil {
		t.Fatal("no-op box change lost prepared layout")
	}
	l.SetTextFont(0, len("start"), typography.FontInfo{Family: "DejaVu Sans", Size: 18})
	if native.retained != nil || native.prepared != nil {
		t.Fatal("font change reused stale analysis")
	}
	native.Reflow(480, 1<<24)
	l.SetSize(600, 1<<24)
	if native.pending != nil {
		t.Fatal("synchronous box change retained obsolete request")
	}
	l.SetUnderline(0, 1, true)
	if native.Reflow(480, 1<<24) {
		t.Fatal("unsupported native decoration accepted")
	}
}

func sameTextLine(a, b typography.TextLine) bool {
	return a.Start == b.Start && a.Length == b.Length && a.X == b.X && a.Y == b.Y && a.Width == b.Width && a.Height == b.Height && a.Baseline == b.Baseline && slices.Equal(a.Clusters, b.Clusters)
}

func assertDeferredMatches(t *testing.T, got, want typography.TextLayout) {
	t.Helper()
	if g, ok := got.(*TextLayout); ok && g.prepared != nil {
		for i := range g.prepared.lines {
			a := g.prepared.lineGlyphs(i)
			b := glyphSignature(want.(*TextLayout).layout.GetLineReadonly(i).GetRuns())
			if !slices.Equal(a, b) {
				t.Fatalf("native glyph mismatch line%d", i)
			}
		}
	}
	gl, gc := got.MeasureMetrics()
	wl, wc := want.MeasureMetrics()
	if !slices.EqualFunc(gl, wl, sameTextLine) || !slices.Equal(gc, wc) {
		t.Logf("line counts %d/%d clusters%d/%d", len(gl), len(wl), len(gc), len(wc))
		for i := 0; i < min(len(gl), len(wl)); i++ {
			if !sameTextLine(gl[i], wl[i]) {
				t.Logf("first line mismatch %d got range=%d/%d want=%d/%d", i, gl[i].Start, gl[i].Length, wl[i].Start, wl[i].Length)
				break
			}
		}
		t.Fatal("published geometry differs from complete native reference")
	}
	gw, gh := got.MeasureSize()
	ww, wh := want.MeasureSize()
	if gw != ww || gh != wh {
		t.Fatalf("extent %g/%g want%g/%g", gw, gh, ww, wh)
	}
	for _, scale := range []float32{1, 2} {
		region := image.Rect(0, 0, int(700*scale), int(180*scale))
		g, e := got.Rasterize(scale, region, nil)
		if e != nil {
			t.Fatal(e)
		}
		w, e := want.Rasterize(scale, region, nil)
		if e != nil {
			t.Fatal(e)
		}
		if !slices.Equal(g.Pixels, w.Pixels) {
			for j := 0; j < len(g.Pixels); j++ {
				if g.Pixels[j] != w.Pixels[j] {
					t.Logf("pixel diff at x=%d y=%d channel=%d got%d want%d", j/4%g.Width, j/4/g.Width, j%4, g.Pixels[j], w.Pixels[j])
					break
				}
			}
			t.Fatalf("published pixels differ at scale%g", scale)
		}
	}
}

// Both cases include the complete line/cluster export and exclude initial
// paragraph analysis, drawing and GUI Geometry construction. Advance's largest
// call is reported separately from total CPU time (there are no frame waits).
func BenchmarkDeferredResize(b *testing.B) {
	for _, mode := range []string{"native", "deferred"} {
		b.Run(mode, func(b *testing.B) {
			runtime.LockOSThread()
			defer runtime.UnlockOSThread()
			ctx, err := NewContext()
			if err != nil {
				b.Fatal(err)
			}
			defer ctx.Destroy()
			f := typography.TextFormat{Font: typography.FontInfo{Family: "DejaVu Sans", Size: 12}, WrapMode: typography.WrapWordChar, TabWidth: 40}
			l, err := ctx.NewTextLayout(strings.Repeat("a", 32768), f, 520, 1<<24)
			if err != nil {
				b.Fatal(err)
			}
			defer l.Destroy()
			l.MeasureMetrics()
			p := l.(*TextLayout)
			if mode == "deferred" {
				p.retained, err = retainParagraph(p)
				if err != nil {
					b.Fatal(err)
				}
			}
			b.ResetTimer()
			var longest time.Duration
			for i := 0; i < b.N; i++ {
				width := float32(480)
				if i%2 != 0 {
					width = 520
				}
				if mode == "native" {
					start := time.Now()
					l.SetSize(width, 1<<24)
					l.MeasureSize()
					l.MeasureMetrics()
					longest = max(longest, time.Since(start))
				} else {
					p.Reflow(width, 1<<24)
					for {
						start := time.Now()
						done, e := p.AdvanceReflow(start.Add(2 * time.Millisecond))
						longest = max(longest, time.Since(start))
						if e != nil {
							b.Fatal(e)
						}
						if done {
							break
						}
					}
					l.MeasureSize()
					l.MeasureMetrics()
				}
			}
			b.ReportMetric(float64(longest)/float64(time.Millisecond), "max-call-ms")
		})
	}
}

func TestDeferredReflowResourcesBelongToEachLayout(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	ctx, err := NewContext()
	if err != nil {
		t.Fatal(err)
	}
	defer ctx.Destroy()
	text := strings.Repeat("office العربية 👨‍👩‍👧‍👦 ", 200)
	f := typography.TextFormat{Font: typography.FontInfo{Family: "DejaVu Sans", Size: 12}, WrapMode: typography.WrapWordChar}
	a, e := ctx.NewTextLayout(text, f, 520, 1<<24)
	if e != nil {
		t.Fatal(e)
	}
	defer a.Destroy()
	b, e := ctx.NewTextLayout(text, f, 520, 1<<24)
	if e != nil {
		t.Fatal(e)
	}
	defer b.Destroy()
	a.MeasureMetrics()
	b.MeasureMetrics()
	first, second := a.(*TextLayout), b.(*TextLayout)
	first.Reflow(480, 1<<24)
	second.Reflow(700, 1<<24)
	first.AdvanceReflow(time.Now().Add(time.Millisecond))
	second.AdvanceReflow(time.Now().Add(time.Millisecond))
	if first.retained == nil || second.retained == nil || first.retained == second.retained || first.retained.runs[0].Glyphs == second.retained.runs[0].Glyphs {
		t.Fatal("layouts share mutable retained resources")
	}
	a.SetTextColor(0, len(text), color.NRGBA{R: 240, A: 153})
	// Exercise a translucent ordinary native layout before the other layout's
	// prepared glyph draw. A shared native renderer must not leak its alpha.
	if _, e := a.Rasterize(1, image.Rect(0, 0, 520, 80), nil); e != nil {
		t.Fatal(e)
	}
	a.Destroy()
	for {
		done, e := second.AdvanceReflow(time.Now().Add(time.Millisecond))
		if e != nil {
			t.Fatal(e)
		}
		if done {
			break
		}
	}
	reference, e := ctx.NewTextLayout(text, f, 700, 1<<24)
	if e != nil {
		t.Fatal(e)
	}
	defer reference.Destroy()
	assertDeferredMatches(t, b, reference)
}
