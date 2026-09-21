package textedit

import (
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
