package textedit

import (
	"image/color"
	"reflect"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform/typography"
)

type shapedTestContext struct{ source *shapedTestLayout }

func (c shapedTestContext) NewGlyphLayout(string, typography.TextFormat) (typography.GlyphLayout, error) {
	return c.source, nil
}

type shapedTestLayout struct {
	typography.GlyphLayout
	text                string
	metrics             typography.GlyphMetrics
	destroyed, measured int
}

func (l *shapedTestLayout) Text() string {
	if l.destroyed > 0 {
		panic("access after destroy")
	}
	return l.text
}
func (l *shapedTestLayout) GlyphMetrics() (typography.GlyphMetrics, error) {
	l.measured++
	return l.metrics, nil
}
func (l *shapedTestLayout) Destroy() { l.destroyed++ }

func shapedASCII(text string, visual []int) *shapedTestLayout {
	l := &shapedTestLayout{text: text, metrics: typography.GlyphMetrics{Line: typography.TextLine{Height: 20, Baseline: 15}}}
	for i := range text {
		l.metrics.Clusters = append(l.metrics.Clusters, typography.TextCluster{Start: i, Length: 1, Width: 10, Height: 20})
	}
	if visual == nil {
		for i := range text {
			visual = append(visual, i)
		}
	}
	l.metrics.VisualOrder = visual
	return l
}

func TestShapedWidthsShareFactsAndFragmentsSurviveSource(t *testing.T) {
	l := shapedASCII("abcdefghijklmnop", nil)
	s, err := NewShapedText(shapedTestContext{l}, l.text, typography.TextFormat{})
	if err != nil {
		t.Fatal(err)
	}
	a, b := s.Wrap(30, 0, 20, false), s.Retain().Wrap(50, 0, 20, false)
	if a.Fragment().LineCount() != 6 || b.Fragment().LineCount() != 4 || l.measured != 1 {
		t.Fatal("width changes performed measurement or incorrect breaks")
	}
	ga, gb := a.Fragment().Geometry(20), b.Fragment().Geometry(20)
	if r := ga.Caret(Position{Offset: 7}); r.X != 10 || r.Y != 40 {
		t.Fatal("incorrect narrow caret", r)
	}
	if r := gb.Caret(Position{Offset: 7}); r.X != 20 || r.Y != 20 {
		t.Fatal("incorrect wide caret", r)
	}
	s.Release()
	if l.destroyed != 0 {
		t.Fatal("released source still owned by second width")
	}
	s.Release()
	if l.destroyed != 1 {
		t.Fatal("source release count", l.destroyed)
	}
	// No native methods may be called by immutable snapshots after release.
	_, clusters := a.Fragment().Metrics()
	if len(clusters) != len(l.text) || ga.Caret(Position{Offset: 7}).Y != 40 {
		t.Fatal("released native source invalidated ordinary snapshot")
	}
}

func TestShapedLineResetsTrailingWhitespace(t *testing.T) {
	// Synthetic native facts isolate L1 from shaping: an embedded RTL span
	// has its two spaces on its visual left in the full paragraph. Breaking
	// after those spaces must reset/move them to the LTR paragraph's tail.
	l := shapedASCII("aBC  d", []int{0, 4, 3, 2, 1, 5})
	for i := 1; i <= 4; i++ {
		l.metrics.Clusters[i].Direction = typography.TextRightToLeft
	}
	s, err := NewShapedText(shapedTestContext{l}, l.text, typography.TextFormat{})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Release()
	w := s.Wrap(50, 0, 20, false)
	cs := w.lineClusters(0)
	for i, want := range []float32{0, 20, 10, 30, 40} {
		if cs[i].X != want {
			t.Fatalf("cluster %d x=%v want %v", i, cs[i].X, want)
		}
	}
	if cs[3].Direction != typography.TextLeftToRight || cs[4].Direction != typography.TextLeftToRight {
		t.Fatal("L1 did not reset trailing levels")
	}
}

type glyphDraw struct {
	origin        geometry.Point
	start, length int
	col           color.Color
}
type glyphDrawRecorder struct{ draws []glyphDraw }

func (p *glyphDrawRecorder) DrawTextGlyphs(o geometry.Point, _ typography.GlyphLayout, start, length int, c color.Color) {
	p.draws = append(p.draws, glyphDraw{o, start, length, c})
}

func TestShapedPaintUsesVisibleLinesAndClusterColors(t *testing.T) {
	l := shapedASCII("abcdefghijklmnop", nil)
	s, err := NewShapedText(shapedTestContext{l}, l.text, typography.TextFormat{})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Release()
	w := s.Wrap(40, 0, 20, false)
	p := new(glyphDrawRecorder)
	w.Paint(p, geometry.Point{X: 5, Y: 10}, geometry.Rect(0, 30, 100, 20), color.Black, []GlyphColor{{Range: Range{Start: 5, End: 7}, Color: color.White}})
	want := []glyphDraw{{geometry.Point{X: 5, Y: 30}, 4, 1, color.Black}, {geometry.Point{X: 15, Y: 30}, 5, 2, color.White}, {geometry.Point{X: 35, Y: 30}, 7, 1, color.Black}}
	if !reflect.DeepEqual(p.draws, want) {
		t.Fatal("wrong visible range/color grouping", p.draws)
	}
}

func TestShapedTabsRecomputeAtDisplayLineOrigin(t *testing.T) {
	l := shapedASCII("aa\tbbbb\tc", nil)
	l.metrics.TabWidth = 40
	s, err := NewShapedText(shapedTestContext{l}, l.text, typography.TextFormat{})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Release()
	w := s.Wrap(50, 0, 20, false) // Use the retained native default after wrapping.
	lines, cs := w.Fragment().Metrics()
	if len(lines) != 2 {
		t.Fatal("unexpected tab lines", lines)
	}
	for _, c := range cs {
		if l.text[c.Start] == '\t' && c.X+c.Width != 40 {
			t.Fatal("tab failed to use new line origin", c)
		}
	}
}
