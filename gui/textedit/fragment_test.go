package textedit

import (
	"math"
	"math/rand"
	"reflect"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform/typography"
)

func TestFragmentOwnsMetricsAndRebasesLineViews(t *testing.T) {
	lines := []typography.TextLine{
		{Length: 3, Width: 12, Height: 20, Baseline: 15},
		{Start: 3, Length: 5, X: 2, Y: 20, Width: 30, Height: 24, Baseline: 38},
		{Start: 8, Length: 1, Y: 48, Width: 9, Height: 20, Baseline: 63},
	}
	clusters := []typography.TextCluster{
		{Length: 3, Width: 12, Height: 20},
		{Start: 3, Length: 2, X: 22, Y: 20, Width: 10, Height: 24, Direction: typography.TextRightToLeft, LineIndex: 1},
		{Start: 5, Length: 2, X: 12, Y: 20, Width: 10, Height: 24, Direction: typography.TextRightToLeft, LineIndex: 1},
		{Start: 7, Length: 1, X: 2, Y: 20, Width: 10, Height: 24, LineIndex: 1},
		{Start: 8, Length: 1, Y: 48, Width: 9, Height: 20, LineIndex: 2},
	}
	f, err := NewFragment(Range{100, 109}, geometry.Size{Width: 32, Height: 68}, lines, clusters)
	if err != nil {
		t.Fatal(err)
	}
	// Simulate a backend reusing or clearing both of its result buffers.
	lines[1].Baseline, clusters[1].X = -999, -999
	view, err := f.LineRange(1, 3)
	if err != nil {
		t.Fatal(err)
	}
	if view.Range() != (Range{103, 109}) || view.Origin() != (geometry.Point{Y: 20}) || view.Size() != (geometry.Size{Width: 32, Height: 48}) {
		t.Fatalf("view mapping: range=%+v origin=%+v size=%+v", view.Range(), view.Origin(), view.Size())
	}
	ls, cs := view.Metrics()
	if ls[0].Start != 0 || ls[0].Y != 0 || ls[0].Baseline != 18 || cs[0].Start != 0 || cs[0].X != 22 || cs[0].LineIndex != 0 {
		t.Fatalf("incorrect local metrics: %+v %+v", ls[0], cs[0])
	}
	g := view.Geometry(20)
	if got := g.Hit(geometry.Point{X: 23, Y: 2}); got != (Position{2, true}) {
		t.Fatalf("independent RTL hit = %+v", got)
	}
	if got := g.Caret(Position{0, false}); got != geometry.Rect(32, 0, 1, 24) {
		t.Fatalf("independent RTL caret = %v", got)
	}
	ls[0].Height, cs[0].X = 1000, 1000
	if again, _ := view.Metrics(); again[0].Height != 24 {
		t.Fatal("mutating exported metrics changed the snapshot")
	}
	nested, _ := view.LineRange(1, 2)
	if nested.Range() != (Range{108, 109}) || nested.Origin().Y != 48 {
		t.Fatal("nested view lost original parent coordinates", nested.Range(), nested.Origin())
	}
	empty, _ := f.LineRange(3, 3)
	if empty.Range() != (Range{109, 109}) || empty.Size() != (geometry.Size{}) || empty.Origin().Y != 68 {
		t.Fatal("empty end view", empty.Range(), empty.Size(), empty.Origin())
	}
}

func TestFragmentCopiesNestedMetricsAndRejectsUnmeasuredInput(t *testing.T) {
	empty, err := NewFragment(Range{}, geometry.Size{Height: 23}, nil, nil)
	if err != nil || empty.Size().Height != 23 || empty.Range() != (Range{}) {
		t.Fatal("empty measured input lost its occupied size", empty.Size(), err)
	}
	zeroGeometry := empty.Geometry(23)
	if zeroGeometry.Caret(Position{}) != geometry.Rect(0, 0, 1, 23) {
		t.Fatal("empty fragment lost caret geometry")
	}
	lines := []typography.TextLine{{Length: 3, Width: 10, Height: 20, Clusters: []typography.TextCluster{{Length: 3, Width: 10}}}}
	f, err := NewFragment(Range{0, 3}, geometry.Size{Width: 10, Height: 20}, lines, nil)
	if err != nil {
		t.Fatal(err)
	}
	lines[0].Clusters[0].Length = 1
	g := f.Geometry(20)
	if g.Adjacent(0, true) != 3 {
		t.Fatal("nested cluster storage was borrowed")
	}
	for _, tc := range []struct {
		name     string
		rng      Range
		size     geometry.Size
		lines    []typography.TextLine
		clusters []typography.TextCluster
	}{
		{name: "unmeasured", rng: Range{0, 3}},
		{name: "range", rng: Range{3, 1}},
		{name: "NaN", size: geometry.Size{Width: float32(math.NaN())}},
		{name: "out of range", rng: Range{0, 3}, lines: []typography.TextLine{{Length: 4, Height: 20}}},
		{name: "cross-line cluster", rng: Range{0, 4}, lines: []typography.TextLine{{Length: 2, Height: 20}, {Start: 2, Length: 2, Y: 20, Height: 20}}, clusters: []typography.TextCluster{{Length: 3, Width: 10}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := NewFragment(tc.rng, tc.size, tc.lines, tc.clusters); err == nil {
				t.Fatal("invalid/unmeasured input accepted")
			}
		})
	}
	if _, err := f.LineRange(-1, 1); err == nil {
		t.Fatal("invalid line view accepted")
	}
}

func fragmentGeometryFixture(count int) ([]typography.TextLine, []typography.TextCluster, int, float32) {
	var lines []typography.TextLine
	var clusters []typography.TextCluster
	at, y := 0, float32(0)
	for i := 0; i < count; i++ {
		height := float32(20 + i%3)
		lines = append(lines, typography.TextLine{Start: at, Length: 6, Width: 40, Y: y, Height: height, Baseline: y + 15})
		clusters = append(clusters,
			typography.TextCluster{Start: at, Length: 1, Width: 10, Y: y, Height: height, LineIndex: i},
			typography.TextCluster{Start: at + 1, Length: 2, X: 20, Width: 10, Y: y, Height: height, LineIndex: i, Direction: typography.TextRightToLeft},
			typography.TextCluster{Start: at + 3, Length: 2, X: 10, Width: 10, Y: y, Height: height, LineIndex: i, Direction: typography.TextRightToLeft},
			typography.TextCluster{Start: at + 5, Length: 1, X: 30, Width: 10, Y: y, Height: height, LineIndex: i})
		at += 6
		y += height + float32(i%2) // gaps are legal; native heights are preserved
	}
	return lines, clusters, at, y
}

func TestFragmentGeometryPagesPreserveAllQueries(t *testing.T) {
	lines, clusters, length, height := fragmentGeometryFixture(145)
	f, err := NewFragment(Range{0, length}, geometry.Size{Width: 40, Height: height}, lines, clusters)
	if err != nil {
		t.Fatal(err)
	}
	got, want := f.Geometry(20), NewGeometry(lines, clusters, length, 20)
	if got.pages == nil || len(got.pages.pages) != 0 || len(got.lines) != 0 {
		t.Fatal("detailed geometry was eagerly built")
	}
	for at := -1; at <= length+1; at++ {
		for _, flag := range []bool{false, true} {
			pos := Position{at, flag}
			if got.Caret(pos) != want.Caret(pos) || got.LineIndex(pos) != want.LineIndex(pos) {
				t.Fatalf("caret/line differs at %+v: %v/%d want %v/%d", pos, got.Caret(pos), got.LineIndex(pos), want.Caret(pos), want.LineIndex(pos))
			}
			if got.Snap(at, flag) != want.Snap(at, flag) || got.Adjacent(at, flag) != want.Adjacent(at, flag) {
				t.Fatalf("logical boundary differs at %d/%v", at, flag)
			}
			for _, right := range []bool{false, true} {
				a, aok := got.Visual(pos, right)
				b, bok := want.Visual(pos, right)
				if a != b || aok != bok {
					t.Fatalf("visual differs at %+v right=%v: %+v/%v want %+v/%v", pos, right, a, aok, b, bok)
				}
			}
		}
	}
	for y := float32(-1); y <= height+1; y += 7 {
		if got.LineAtY(y) != want.LineAtY(y) {
			t.Fatalf("Y query differs at %g", y)
		}
		for x := float32(-3); x <= 43; x += 3 {
			point := geometry.Point{X: x, Y: y}
			if got.Hit(point) != want.Hit(point) {
				t.Fatalf("hit differs at %v", point)
			}
		}
	}
	for _, span := range []Range{{0, length}, {31*6 + 1, 34*6 + 3}, {60*6 + 1, 130*6 + 3}, {length - 1, length}} {
		if !reflect.DeepEqual(got.Selection(span), want.Selection(span)) {
			t.Fatalf("selection differs at %+v", span)
		}
		if !reflect.DeepEqual(got.SelectionLines(span, 30, 66), want.SelectionLines(span, 30, 66)) {
			t.Fatalf("visible selection differs at %+v", span)
		}
	}
	if len(got.pages.pages) > geometryPageLimit {
		t.Fatal("geometry working set grew without bound")
	}
	// Independent expectations across the first page boundary.
	if got.Caret(Position{32 * 6, true}).Y != lines[31].Y || got.Caret(Position{32 * 6, false}).Y != lines[32].Y {
		t.Fatal("page boundary lost soft-wrap affinity")
	}
	if got.Adjacent(32*6+2, false) != 32*6+1 || got.Adjacent(32*6+2, true) != 32*6+3 {
		t.Fatal("paged deletion split a two-byte cluster")
	}
	line := got.LineMetrics(40)
	if line.Start != 40*6 || len(line.Clusters) != 4 || line.Clusters[1].Start != 40*6+1 || line.Clusters[1].LineIndex != 40 {
		t.Fatal("line metrics leaked page-local coordinates", line)
	}
}

func TestFragmentGeometryDoesNotIndexOtherPagesForVisibleSelection(t *testing.T) {
	lines, clusters, length, height := fragmentGeometryFixture(1000)
	f, err := NewFragment(Range{0, length}, geometry.Size{Width: 40, Height: height}, lines, clusters)
	if err != nil {
		t.Fatal(err)
	}
	g := f.Geometry(20)
	if len(g.SelectionLines(Range{0, length}, 450, 460)) != 10 || len(g.pages.pages) != 1 {
		t.Fatal("visible selection indexed remote geometry", len(g.pages.pages))
	}
	peer := f.Geometry(20)
	peer.Caret(Position{})
	if len(peer.pages.pages) != 1 || len(g.pages.pages) != 1 || g.pages == peer.pages {
		t.Fatal("independent views share mutable geometry caches")
	}
	short := f.Geometry(20)
	if len(short.Selection(Range{Start: 450*6 + 1, End: 450*6 + 3})) != 1 || len(short.pages.pages) != 1 {
		t.Fatal("short selection indexed remote pages", len(short.pages.pages))
	}
}

func TestFragmentGeometrySparseLinesAndSeparators(t *testing.T) {
	random := rand.New(rand.NewSource(93))
	for trial := 0; trial < 8; trial++ {
		var lines []typography.TextLine
		var clusters []typography.TextCluster
		at, y := 0, float32(0)
		for i := 0; i < 100; i++ {
			start := at
			for j := 0; j < random.Intn(5); j++ {
				length := 1 + random.Intn(4)
				clusters = append(clusters, typography.TextCluster{Start: at, Length: length, LineIndex: i, X: float32(j * 10), Y: y, Width: float32(random.Intn(11)), Height: 20, Direction: typography.TextDirection(trial % 2)})
				at += length
			}
			lines = append(lines, typography.TextLine{Start: start, Length: at - start, Y: y, Height: 20})
			at += random.Intn(3)
			y += 20
		}
		f, err := NewFragment(Range{End: at}, geometry.Size{Width: 100, Height: y}, lines, clusters)
		if err != nil {
			t.Fatal(err)
		}
		got, want := f.Geometry(20), NewGeometry(lines, clusters, at, 20)
		for n := 0; n <= at; n++ {
			for _, flag := range []bool{false, true} {
				pos := Position{n, flag}
				if got.Caret(pos) != want.Caret(pos) || got.Snap(n, flag) != want.Snap(n, flag) || got.Adjacent(n, flag) != want.Adjacent(n, flag) {
					t.Fatalf("trial %d pos=%+v caret=%v want=%v snap=%d want=%d adjacent=%d want=%d", trial, pos, got.Caret(pos), want.Caret(pos), got.Snap(n, flag), want.Snap(n, flag), got.Adjacent(n, flag), want.Adjacent(n, flag))
				}
			}
		}
	}
}

func TestFragmentGeometryPagesKeepParentMappingForLineViews(t *testing.T) {
	lines, clusters, length, height := fragmentGeometryFixture(150)
	f, err := NewFragment(Range{Start: 100, End: 100 + length}, geometry.Size{Width: 40, Height: height}, lines, clusters)
	if err != nil {
		t.Fatal(err)
	}
	view, err := f.LineRange(17, 130)
	if err != nil {
		t.Fatal(err)
	}
	g := view.Geometry(20)
	if g.pages == nil {
		t.Fatal("large view did not use pages")
	}
	pos := Position{Offset: 32 * 6}
	wantY := lines[49].Y - lines[17].Y
	if got := g.Caret(pos); got.Y != wantY || g.LineIndex(pos) != 32 {
		t.Fatal("line view lost its origin", got, wantY)
	}
	if got := g.Caret(Position{Offset: pos.Offset, Upstream: true}); got.Y != lines[48].Y-lines[17].Y {
		t.Fatal("line view page affinity", got)
	}
	if got := g.Hit(geometry.Point{X: 23, Y: wantY + 1}); got != (Position{Offset: 32*6 + 3, Upstream: true}) {
		t.Fatal("line view bidi hit", got)
	}
	if got := g.LineMetrics(32); got.Start != pos.Offset || got.Y != wantY || got.Clusters[1].LineIndex != 32 {
		t.Fatal("line view metrics", got)
	}
}
