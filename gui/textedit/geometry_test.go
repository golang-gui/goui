package textedit

import (
	"fmt"
	"math"
	"math/rand"
	"reflect"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform/typography"
)

func TestTextGeometryAtomicClusters(t *testing.T) {
	// e + combining accent (3 bytes), one emoji cluster (8), and an fi
	// ligature (2). Widths are independent fixture metrics, not rune counts.
	g := NewGeometry([]typography.TextLine{{Length: 13, Height: 20}}, []typography.TextCluster{
		{Start: 0, Length: 3, X: 0, Width: 10},
		{Start: 3, Length: 8, X: 10, Width: 20},
		{Start: 11, Length: 2, X: 30, Width: 12},
	}, 13, 20)
	for _, tc := range []struct {
		x    float32
		want int
	}{{1, 0}, {6, 3}, {14, 3}, {25, 11}, {40, 13}} {
		if got := g.Hit(geometry.Point{X: tc.x}).Offset; got != tc.want {
			t.Fatalf("Hit %g: got %d want %d", tc.x, got, tc.want)
		}
	}
	if g.Snap(5, false) != 3 || g.Snap(5, true) != 11 || g.Adjacent(11, false) != 3 || g.Adjacent(3, true) != 11 {
		t.Fatal("Snap/delete split an atomic cluster")
	}
	pos := Position{}
	for _, want := range []int{3, 11, 13} {
		var ok bool
		pos, ok = g.Visual(pos, true)
		if !ok || pos.Offset != want {
			t.Fatalf("Visual right=%+v want %d", pos, want)
		}
	}
}

func TestTextGeometryBidiAndDisjointSelection(t *testing.T) {
	// Logical a, alef, bet, c; Visual a, bet, alef, c.
	g := NewGeometry([]typography.TextLine{{Length: 6, Height: 20}}, []typography.TextCluster{
		{Start: 0, Length: 1, X: 0, Width: 10},
		{Start: 1, Length: 2, X: 20, Width: 10, Direction: typography.TextRightToLeft},
		{Start: 3, Length: 2, X: 10, Width: 10, Direction: typography.TextRightToLeft},
		{Start: 5, Length: 1, X: 30, Width: 10},
	}, 6, 20)
	if got := g.Hit(geometry.Point{X: 21}); got != (Position{3, true}) {
		t.Fatalf("RTL left half=%+v", got)
	}
	if got := g.Hit(geometry.Point{X: 29}); got != (Position{1, false}) {
		t.Fatalf("RTL right half=%+v", got)
	}
	if g.Caret(Position{1, true}).X != 10 || g.Caret(Position{1, false}).X != 30 {
		t.Fatal("bidi boundary affinity lost")
	}
	want := []geometry.Rectangle{geometry.Rect(0, 0, 10, 20), geometry.Rect(20, 0, 10, 20)}
	if got := g.Selection(Range{0, 3}); !reflect.DeepEqual(got, want) {
		t.Fatalf("Selection bridged unselected bidi cluster: %v", got)
	}
	if got := g.Selection(Range{1, 5}); !reflect.DeepEqual(got, []geometry.Rectangle{geometry.Rect(10, 0, 20, 20)}) {
		t.Fatalf("Adjacent Selection not merged: %v", got)
	}
}

func TestTextGeometrySoftWrapAndEmptyLine(t *testing.T) {
	g := NewGeometry([]typography.TextLine{
		{Length: 2, Height: 20}, {Start: 2, Length: 1, Y: 20, Height: 20},
	}, []typography.TextCluster{
		{Length: 2, Width: 20}, {Start: 2, Length: 1, Width: 10, Y: 20, LineIndex: 1},
	}, 3, 20)
	if got := g.Caret(Position{2, true}); got != geometry.Rect(20, 0, 1, 20) {
		t.Fatalf("Upstream Caret %v", got)
	}
	if got := g.Caret(Position{2, false}); got != geometry.Rect(0, 20, 1, 20) {
		t.Fatalf("downstream Caret %v", got)
	}
	if got, ok := g.Visual(Position{2, true}, true); !ok || got != (Position{2, false}) {
		t.Fatalf("cross wrap=%+v,%v", got, ok)
	}
	empty := NewGeometry(nil, nil, 0, 23)
	if got := empty.Caret(Position{}); got != geometry.Rect(0, 0, 1, 23) {
		t.Fatalf("empty Caret %v", got)
	}
	if got := empty.Hit(geometry.Point{X: 100, Y: 100}); got.Offset != 0 {
		t.Fatal("empty line returned nonzero byte Offset")
	}
}

func TestTextGeometryZeroWidthCluster(t *testing.T) {
	g := NewGeometry([]typography.TextLine{{Length: 5, Height: 20}}, []typography.TextCluster{
		{Length: 1, Width: 10}, {Start: 1, Length: 3, X: 10}, {Start: 4, Length: 1, X: 10, Width: 10},
	}, 5, 20)
	if g.Adjacent(4, false) != 1 || g.Adjacent(1, true) != 4 {
		t.Fatal("zero width cluster is not logically deletable")
	}
	if got, ok := g.Visual(Position{1, false}, true); !ok || got.Offset != 5 {
		t.Fatal("Visual movement stuck on zero width content")
	}
}

func TestGeometryWrappedQueriesKeepAffinity(t *testing.T) {
	g := NewGeometry([]typography.TextLine{
		{Length: 3, Height: 20},
		{Start: 3, Length: 6, Y: 20, Height: 24},
		{Start: 9, Length: 2, Y: 48, Height: 20},
	}, []typography.TextCluster{
		{Start: 0, Length: 3, Width: 12},
		{Start: 3, Length: 1, Width: 8, LineIndex: 1},
		{Start: 4, Length: 2, X: 16, Width: 8, Direction: typography.TextRightToLeft, LineIndex: 1},
		{Start: 6, Length: 2, X: 8, Width: 8, Direction: typography.TextRightToLeft, LineIndex: 1},
		{Start: 8, Length: 1, X: 24, Width: 8, LineIndex: 1},
		{Start: 9, Length: 2, Width: 16, LineIndex: 2},
	}, 11, 20)
	for _, tc := range []struct {
		pos  Position
		rect geometry.Rectangle
	}{
		{Position{3, true}, geometry.Rect(12, 0, 1, 20)},
		{Position{3, false}, geometry.Rect(0, 20, 1, 24)},
		{Position{4, true}, geometry.Rect(8, 20, 1, 24)},
		{Position{4, false}, geometry.Rect(24, 20, 1, 24)},
		{Position{9, true}, geometry.Rect(32, 20, 1, 24)},
		{Position{9, false}, geometry.Rect(0, 48, 1, 20)},
		// No stop at the middle of a multi-byte cluster. Equal distances
		// retain the first visual stop, regardless of requested affinity.
		{Position{10, true}, geometry.Rect(32, 20, 1, 24)},
	} {
		if got := g.Caret(tc.pos); got != tc.rect {
			t.Fatalf("Caret(%+v) = %v, want %v", tc.pos, got, tc.rect)
		}
	}
	for _, tc := range []struct {
		y    float32
		line int
	}{{-10, 0}, {0, 0}, {19.9, 0}, {20, 1}, {43.9, 1}, {44, 2}, {47, 2}, {68, 2}, {1000, 2}, {float32(math.NaN()), 0}} {
		if got := g.LineAtY(tc.y); got != tc.line {
			t.Fatalf("LineAtY(%v) = %d, want %d", tc.y, got, tc.line)
		}
	}
	want := []geometry.Rectangle{geometry.Rect(0, 20, 8, 24), geometry.Rect(16, 20, 8, 24)}
	if got := g.SelectionLines(Range{0, 6}, 1, 2); !reflect.DeepEqual(got, want) {
		t.Fatalf("visible bidi selection = %v, want %v", got, want)
	}
	if got := g.SelectionLines(Range{0, 11}, 2, 2); len(got) != 0 {
		t.Fatalf("empty visual range = %v", got)
	}
}

// The reference intentionally scans every stop, as before indexing. Cover
// arbitrary supplied geometry as well as normal native line ordering, so the
// optimization cannot assume that every caller's offsets or Y are monotonic.
func TestGeometryIndexedQueriesMatchExhaustiveSearch(t *testing.T) {
	random := rand.New(rand.NewSource(42))
	for trial := 0; trial < 60; trial++ {
		var lines []typography.TextLine
		var clusters []typography.TextCluster
		offset, y := 0, float32(0)
		for i := 0; i < 30; i++ {
			line := typography.TextLine{Start: offset, Y: y, Height: float32(10 + random.Intn(20))}
			count := random.Intn(8)
			if trial%3 == 0 {
				count++ // also exercise ordinary non-empty line partitions
			}
			for j := 0; j < count; j++ {
				length := 1 + random.Intn(4)
				x := j * 10
				if trial%2 != 0 {
					x = (count - j - 1) * 10
				}
				clusters = append(clusters, typography.TextCluster{
					Start: offset, Length: length, LineIndex: i, X: float32(x),
					Width: float32(random.Intn(11)), Direction: typography.TextDirection(trial % 2),
				})
				offset += length
			}
			line.Length = offset - line.Start
			lines = append(lines, line)
			offset += random.Intn(3) // hard breaks / gaps without cluster stops
			y += line.Height
		}
		if trial%3 == 1 {
			// Non-monotonic Y, while keeping the logical text intact.
			lines[10].Y = lines[15].Y
		}
		if trial%3 == 2 {
			// Overlapping logical ranges in different visual lines.
			clusters = append(clusters, typography.TextCluster{Start: 0, Length: 1, LineIndex: 20, Width: 5})
		}
		g := NewGeometry(lines, clusters, offset, 20)
		for pos := -1; pos <= offset+1; pos++ {
			for _, upstream := range []bool{false, true} {
				query := Position{pos, upstream}
				wantLine, wantStop := exhaustiveLocation(&g, query)
				gotLine, gotStop := g.locate(query)
				if gotLine != wantLine || gotStop != wantStop {
					t.Fatalf("trial %d position %+v: %d/%d, want %d/%d", trial, query, gotLine, gotStop, wantLine, wantStop)
				}
			}
		}
		for point := float32(-1); point < y+1; point += 3 {
			want := 0
			for want+1 < len(lines) && point >= lines[want].Y+lines[want].Height {
				want++
			}
			if got := g.LineAtY(point); got != want {
				t.Fatalf("trial %d y=%g: line %d, want %d", trial, point, got, want)
			}
		}
	}
}

func exhaustiveLocation(g *Geometry, pos Position) (int, int) {
	lineIndex, stopIndex, best := 0, 0, int(^uint(0)>>1)
	for i, line := range g.lines {
		for j, stop := range line.stops {
			if stop.Position == pos {
				return i, j
			}
			distance := stop.Offset - pos.Offset
			if distance < 0 {
				distance = -distance
			}
			if distance < best {
				lineIndex, stopIndex, best = i, j, distance
			}
		}
	}
	return lineIndex, stopIndex
}

func TestGeometrySelectionLinesBounds(t *testing.T) {
	g := NewGeometry(nil, nil, 0, 20)
	for _, bounds := range [][2]int{{-1, 0}, {1, 0}, {0, 2}} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("SelectionLines accepted %v", bounds)
				}
			}()
			g.SelectionLines(Range{}, bounds[0], bounds[1])
		}()
	}
}

func TestVisualNavigationCollapsesRoundedEdges(t *testing.T) {
	for _, origin := range []float32{0, 100000, -100000} {
		width := float32(3.2)
		edge := origin + width
		next := math.Nextafter32(edge, float32(math.Inf(1)))
		g := NewGeometry([]typography.TextLine{{Length: 2, Height: 20}}, []typography.TextCluster{{Length: 1, X: origin, Width: width}, {Start: 1, Length: 1, X: next, Width: width}}, 2, 20)
		if p, ok := g.Visual(Position{Offset: 1}, false); !ok || p.Offset != 0 {
			t.Fatalf("origin=%v: left stayed at rounded edge: %+v/%v", origin, p, ok)
		}
		if p, ok := g.Visual(Position{Offset: 1, Upstream: true}, true); !ok || p.Offset != 2 {
			t.Fatalf("origin=%v: right stayed at rounded edge: %+v/%v", origin, p, ok)
		}
	}
}

func TestVisualNavigationRetainsDistinctAffinityPositions(t *testing.T) {
	// A real gap at the same byte is outside representation uncertainty.
	origin := float32(100000)
	width := float32(3.2)
	next := origin + width + .25
	g := NewGeometry([]typography.TextLine{{Length: 2, Height: 20}}, []typography.TextCluster{{Length: 1, X: origin, Width: width}, {Start: 1, Length: 1, X: next, Width: width}}, 2, 20)
	if p, ok := g.Visual(Position{Offset: 1}, false); !ok || p != (Position{Offset: 1, Upstream: true}) {
		t.Fatal(p, ok)
	}
	// Distinct byte positions at a bidi boundary are never collapsed.
	g = NewGeometry([]typography.TextLine{{Length: 2, Height: 20}}, []typography.TextCluster{{Length: 1, X: 0, Width: 10}, {Start: 1, Length: 1, X: 10, Width: 10, Direction: typography.TextRightToLeft}}, 2, 20)
	if p, ok := g.Visual(Position{Offset: 1}, false); !ok || p.Offset != 2 {
		t.Fatal(p, ok)
	}
}

func wrappedGeometryFixture(count int) Geometry {
	const columns = 80
	lines := make([]typography.TextLine, count)
	clusters := make([]typography.TextCluster, 0, count*columns)
	for i := range lines {
		lines[i] = typography.TextLine{Start: i * columns, Length: columns, Y: float32(i * 20), Height: 20}
		for j := 0; j < columns; j++ {
			clusters = append(clusters, typography.TextCluster{
				Start: i*columns + j, Length: 1, LineIndex: i,
				X: float32(j * 8), Y: float32(i * 20), Width: 8, Height: 20,
			})
		}
	}
	return NewGeometry(lines, clusters, count*columns, 20)
}

// These benchmarks measure queries on already supplied native metrics. They
// deliberately exclude shaping and index construction, and cannot establish
// first-paint latency or event-loop responsiveness.
func BenchmarkGeometryWrappedQueries(b *testing.B) {
	for _, count := range []int{100, 10000} {
		b.Run(fmt.Sprint(count), func(b *testing.B) {
			g := wrappedGeometryFixture(count)
			b.Run("Caret", func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					_ = g.Caret(Position{Offset: (count/2+i%20)*80 + 40})
				}
			})
			b.Run("Hit", func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					_ = g.Hit(geometry.Point{X: 320, Y: float32((count/2+i%20)*20 + 10)})
				}
			})
			b.Run("ShortSelection", func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					start := (count/2 + i%20) * 80
					_ = g.Selection(Range{start + 20, start + 40})
				}
			})
		})
	}
}
