package textedit

import (
	"github.com/golang-gui/goui/platform/typography"
	"math"
	"testing"
)

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
