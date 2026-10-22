package directwrite

import (
	"fmt"
	"math"
	"runtime"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/golang-gui/goui/platform/typography"
)

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
