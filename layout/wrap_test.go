package layout

import (
	"github.com/golang-gui/goui/core/geometry"
	"testing"
)

// These children obey the Constraint contract and record both allocations and
// remeasurement. Expected rectangles below are independently calculated DIP.
type wrapTestChild struct {
	size        geometry.Size
	baseline    float32
	hasBaseline bool
	weight      float32
	rect        geometry.Rectangle
	constraints []Constraint
}

func (c *wrapTestChild) Measure(cs Constraint) Measurement {
	c.constraints = append(c.constraints, cs)
	return Measurement{Size: cs.Clamp(c.size), Baseline: c.baseline, HasBaseline: c.hasBaseline}
}
func (c *wrapTestChild) Arrange(r geometry.Rectangle) { c.rect = r }
func (c *wrapTestChild) MainWeight() float32          { return c.weight }

func TestWrapReflowsAndTransposes(t *testing.T) {
	for _, direction := range []Direction{DirectionHorizontal, DirectionVertical} {
		t.Run(map[Direction]string{DirectionHorizontal: "horizontal", DirectionVertical: "vertical"}[direction], func(t *testing.T) {
			l := NewWrapLayout(direction)
			l.Spacing, l.LineSpacing, l.Padding = 8, 6, 2
			l.CrossAlign = CrossStart
			size := geometry.Size{Width: 90, Height: 20}
			if direction == DirectionVertical {
				size = geometry.Size{Width: 20, Height: 90}
			}
			a, b, c := &wrapTestChild{size: size}, &wrapTestChild{size: size, weight: 100}, &wrapTestChild{size: size}
			children := []Child{a, b, c}
			for _, tc := range []struct{ limit, main, cross, thirdMain, thirdCross float32 }{
				{300, 290, 24, 198, 2}, {284, 192, 50, 2, 28}, {290, 290, 24, 198, 2},
			} {
				maxSize := geometry.Size{Width: tc.limit, Height: 1000}
				wantSize := geometry.Size{Width: tc.main, Height: tc.cross}
				wantRect := geometry.Rect(tc.thirdMain, tc.thirdCross, 90, 20)
				if direction == DirectionVertical {
					maxSize = geometry.Size{Width: 1000, Height: tc.limit}
					wantSize = geometry.Size{Width: tc.cross, Height: tc.main}
					wantRect = geometry.Rect(tc.thirdCross, tc.thirdMain, 20, 90)
				}
				got := l.Measure(children, Loose(maxSize))
				if got.Size != wantSize {
					t.Fatalf("limit=%v size=%v want=%v", tc.limit, got.Size, wantSize)
				}
				l.Arrange(children, geometry.Rectangle{Size: got.Size})
				if c.rect != wantRect {
					t.Fatalf("limit=%v third=%v want=%v", tc.limit, c.rect, wantRect)
				}
			}
		})
	}
}

func TestWrapEmptyUnboundedOversizeAndZero(t *testing.T) {
	l := &WrapLayout{Padding: 2, Spacing: 5, LineSpacing: 3}
	if m := l.Measure([]Child{nil}, Unbounded()); m.Size != (geometry.Size{Width: 4, Height: 4}) || m.HasBaseline {
		t.Fatal(m)
	}
	a, b := &wrapTestChild{size: geometry.Size{Width: 200, Height: 20}}, &wrapTestChild{}
	children := []Child{nil, a, b}
	if m := l.Measure(children, Unbounded()); m.Size != (geometry.Size{Width: 209, Height: 24}) {
		t.Fatal(m)
	}
	// The oversized item is clamped to the entire 50 DIP inner width; the
	// visible zero-width item still needs a gap, hence occupies a second line.
	m := l.Measure(children, Loose(geometry.Size{Width: 54, Height: 100}))
	if m.Size != (geometry.Size{Width: 54, Height: 27}) {
		t.Fatal(m)
	}
	l.Arrange(children, geometry.Rectangle{Size: m.Size})
	if a.rect.Width != 50 || b.rect.X != 2 || b.rect.Y != 25 {
		t.Fatalf("%v %v", a.rect, b.rect)
	}
	m = l.Measure(children, Tight(geometry.Size{}))
	if m.Size != (geometry.Size{}) {
		t.Fatal(m)
	}
}

func TestWrapMainAlignmentPerLine(t *testing.T) {
	for _, tc := range []struct {
		align        MainAlign
		x1, x2, last float32
	}{
		{MainStart, 0, 35, 0}, {MainCenter, 15, 50, 32.5}, {MainEnd, 30, 65, 65}, {MainSpaceBetween, 0, 65, 0},
	} {
		l := &WrapLayout{Spacing: 5, LineSpacing: 4, MainAlign: tc.align}
		a := &wrapTestChild{size: geometry.Size{Width: 30, Height: 10}}
		b := &wrapTestChild{size: geometry.Size{Width: 35, Height: 20}}
		c := &wrapTestChild{size: geometry.Size{Width: 35, Height: 10}}
		l.Arrange([]Child{a, b, c}, geometry.Rect(0, 0, 100, 100))
		if a.rect != geometry.Rect(tc.x1, 5, 30, 10) || b.rect != geometry.Rect(tc.x2, 0, 35, 20) || c.rect != geometry.Rect(tc.last, 24, 35, 10) {
			t.Fatalf("%v: %v %v %v", tc.align, a.rect, b.rect, c.rect)
		}
	}
}

func TestWrapCrossAlignmentAndStretch(t *testing.T) {
	for _, tc := range []struct {
		align CrossAlign
		y, h  float32
	}{
		{CrossDefault, 5, 10}, {CrossStart, 0, 10}, {CrossCenter, 5, 10}, {CrossEnd, 10, 10}, {CrossStretch, 0, 20},
	} {
		l := &WrapLayout{CrossAlign: tc.align}
		a := &wrapTestChild{size: geometry.Size{Width: 20, Height: 10}}
		b := &wrapTestChild{size: geometry.Size{Width: 20, Height: 20}}
		l.Arrange([]Child{a, b}, geometry.Rect(7, 9, 100, 100))
		if a.rect != geometry.Rect(7, 9+tc.y, 20, tc.h) {
			t.Fatalf("%v: %v", tc.align, a.rect)
		}
		if tc.align == CrossStretch {
			if len(a.constraints) != 2 || a.constraints[1] != Tight(geometry.Size{Width: 20, Height: 20}) {
				t.Fatal(a.constraints)
			}
		}
	}
}

func TestWrapBaselinePerLineAndPropagation(t *testing.T) {
	l := &WrapLayout{CrossAlign: CrossBaseline, LineSpacing: 4, Padding: 2}
	a := &wrapTestChild{size: geometry.Size{Width: 20, Height: 20}, hasBaseline: true, baseline: 15}
	b := &wrapTestChild{size: geometry.Size{Width: 20, Height: 30}, hasBaseline: true, baseline: 10}
	c := &wrapTestChild{size: geometry.Size{Width: 40, Height: 10}, hasBaseline: true, baseline: 7}
	children := []Child{a, b, c}
	m := l.Measure(children, Loose(geometry.Size{Width: 44, Height: 100}))
	if m.Size != (geometry.Size{Width: 44, Height: 53}) || !m.HasBaseline || m.Baseline != 17 {
		t.Fatal(m)
	}
	l.Arrange(children, geometry.Rectangle{Size: m.Size})
	if a.rect.Y != 2 || b.rect.Y != 7 || c.rect.Y != 41 {
		t.Fatalf("%v %v %v", a.rect, b.rect, c.rect)
	}
	l.Direction = DirectionVertical
	l.Arrange(children, geometry.Rect(0, 0, 100, 100))
	if a.rect.X != 2 || b.rect.X != 2 {
		t.Fatal("vertical baseline must fall back to start")
	}
}

type wrapTextChild struct{ rect geometry.Rectangle }

func (c *wrapTextChild) MainWeight() float32          { return 0 }
func (c *wrapTextChild) Arrange(r geometry.Rectangle) { c.rect = r }
func (c *wrapTextChild) Measure(cs Constraint) Measurement {
	width := min(float32(100), cs.Max.Width)
	height := float32(10)
	if width < 100 {
		height = 20
	}
	return Measured(cs.Clamp(geometry.Size{Width: width, Height: height}))
}
func TestWrapRemeasuresAtActualWidth(t *testing.T) {
	l := &WrapLayout{LineSpacing: 3}
	a, b := &wrapTextChild{}, &wrapTextChild{}
	children := []Child{a, b}
	if m := l.Measure(children, Loose(geometry.Size{Width: 200, Height: 100})); m.Height != 10 {
		t.Fatal(m)
	}
	l.Arrange(children, geometry.Rect(0, 0, 60, 100))
	if a.rect != geometry.Rect(0, 0, 60, 20) || b.rect != geometry.Rect(0, 23, 60, 20) {
		t.Fatalf("%v %v", a.rect, b.rect)
	}
}
