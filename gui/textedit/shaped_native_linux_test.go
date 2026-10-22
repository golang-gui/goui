package textedit

import (
	"fmt"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/golang-gui/goui/platform/typography"
	"github.com/golang-gui/goui/platform/typography/pango"
)

// Native automatic wrapping is an independent bidi line-order reference. Use
// its exact line boundaries here to test projection, independently of the
// editor's Unicode break-opportunity choice. No substring is re-itemized.
func TestShapedLineOrderMatchesNativeParagraph(t *testing.T) {
	for index, text := range []string{
		"start \u202e" + strings.Repeat("abc 123  مرحبا עברית  ", 40) + "END\u202c end",
		"עברית \u202d" + strings.Repeat("abc 123  עברית  ", 40) + "END\u202c סוף",
		"start \u2067" + strings.Repeat("עברית 123 مرحبا  ", 40) + "END\u2069 end",
		strings.Repeat("العربية עברית abc \u2066xyz 123\u2069  ", 40),
	} {
		t.Run(fmt.Sprint(index), func(t *testing.T) {
			runtime.LockOSThread()
			defer runtime.UnlockOSThread()
			ctx, err := pango.NewContext()
			if err != nil {
				t.Fatal(err)
			}
			defer ctx.Destroy()
			format := typography.TextFormat{Font: typography.FontInfo{Family: "DejaVu Sans Mono", Size: 12}, WrapMode: typography.WrapWordChar}
			s, err := NewShapedText(ctx.(typography.GlyphContext), text, format)
			if err != nil {
				t.Fatal(err)
			}
			defer s.Release()
			for _, width := range []float32{173, 239} {
				ref, err := ctx.NewTextLayout(text, format, width, 1<<24)
				if err != nil {
					t.Fatal(err)
				}
				lines, _ := ref.MeasureMetrics()
				ref.Destroy()
				w := s.Wrap(width, 0, 20, true)
				w.fragment.data.lines = lines
				checked := 0
				for row, line := range lines {
					// A native line break must be a retained shaping boundary. These
					// fixtures avoid line-end ligature splitting; assert that fact.
					end := line.Start + line.Length
					for _, at := range []int{line.Start, end} {
						i := sort.Search(len(s.metrics.Clusters), func(i int) bool { return s.metrics.Clusters[i].Start >= at })
						if at != len(text) && (i == len(s.metrics.Clusters) || s.metrics.Clusters[i].Start != at) {
							t.Fatal("native line split source cluster", row, at)
						}
					}
					got := w.lineClusters(row)
					// Pango suppresses its final breaking space (and may leave a
					// preceding overridden space inside the RTL run). The editor
					// deliberately retains code whitespace and applies L1. Compare
					// the unaffected interior here; shaped_test.go asserts exact
					// trailing-space positions and reset directions independently.
					interiorEnd := line.Start + len(strings.TrimRight(text[line.Start:end], " "))
					starts := func(cs []typography.TextCluster) []int {
						copyCS := append([]typography.TextCluster(nil), cs...)
						sort.SliceStable(copyCS, func(i, j int) bool { return copyCS[i].X < copyCS[j].X })
						var out []int
						for _, c := range copyCS {
							if c.Width > 0 && c.Start < interiorEnd {
								out = append(out, c.Start)
							}
						}
						return out
					}
					if a, b := starts(got), starts(line.Clusters); !reflect.DeepEqual(a, b) {
						t.Fatalf("line %d [%d,%d) width %v visual bytes=%v native=%v", row, line.Start, end, width, a, b)
					}
					checked++
				}
				if checked < 20 {
					t.Fatal("insufficient native line-order coverage", checked)
				}
			}
		})
	}
}
