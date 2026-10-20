package textedit

import (
	"math"
	"math/rand"
	"reflect"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform/typography"
)

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
