package pango

import (
	"fmt"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/golang-gui/goui/platform/typography"
)

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
