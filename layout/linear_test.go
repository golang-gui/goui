package layout

import (
	"testing"

	"github.com/golang-gui/goui/core/geometry"
)

func TestLinearLayoutMeasureHorizontal(t *testing.T) {
	layout := &LinearLayout{
		Direction: DirectionHorizontal,
		Spacing:   3,
	}
	children := []Child{
		&testChild{size: geometry.Size{Width: 10, Height: 20}},
		&testChild{size: geometry.Size{Width: 30, Height: 15}},
		&testChild{size: geometry.Size{Width: 5, Height: 25}},
	}

	size := layout.Measure(children, Loose(geometry.Size{Width: 100, Height: 50}))

	if size.Size != (geometry.Size{Width: 51, Height: 25}) {
		t.Fatalf("unexpected measured size: %+v", size)
	}
}

func TestLinearLayoutMeasureSkipsNilChildren(t *testing.T) {
	layout := &LinearLayout{
		Direction: DirectionHorizontal,
		Spacing:   3,
	}
	children := []Child{
		&testChild{size: geometry.Size{Width: 10, Height: 20}},
		nil,
		&testChild{size: geometry.Size{Width: 30, Height: 15}},
	}

	size := layout.Measure(children, Loose(geometry.Size{Width: 100, Height: 50}))

	if size.Size != (geometry.Size{Width: 43, Height: 20}) {
		t.Fatalf("unexpected measured size: %+v", size)
	}
}

func TestLinearLayoutMeasureVertical(t *testing.T) {
	layout := &LinearLayout{
		Direction: DirectionVertical,
		Spacing:   4,
	}
	children := []Child{
		&testChild{size: geometry.Size{Width: 10, Height: 20}},
		&testChild{size: geometry.Size{Width: 30, Height: 15}},
	}

	size := layout.Measure(children, Loose(geometry.Size{Width: 100, Height: 50}))

	if size.Size != (geometry.Size{Width: 30, Height: 39}) {
		t.Fatalf("unexpected measured size: %+v", size)
	}
}

func TestLinearLayoutArrangeHorizontal(t *testing.T) {
	layout := &LinearLayout{
		Direction: DirectionHorizontal,
		Spacing:   2,
	}
	first := &testChild{size: geometry.Size{Width: 10, Height: 20}}
	second := &testChild{size: geometry.Size{Width: 30, Height: 15}}
	children := []Child{first, second}

	layout.Arrange(children, geometry.Rect(100, 200, 80, 40))

	// CrossDefault centers a horizontal row while preserving natural heights.
	// Main axis still packs from the start with spacing.
	if first.rect != geometry.Rect(100, 210, 10, 20) {
		t.Fatalf("unexpected first rect: %+v", first.rect)
	}
	if second.rect != geometry.Rect(112, 212.5, 30, 15) {
		t.Fatalf("unexpected second rect: %+v", second.rect)
	}
	// Both children are measured against the full available extent (weight pass
	// measures all before distributing), so available is the full rect size.
	if first.available != (geometry.Size{Width: 80, Height: 40}) {
		t.Fatalf("unexpected first available size: %+v", first.available)
	}
	if second.available != (geometry.Size{Width: 80, Height: 40}) {
		t.Fatalf("unexpected second available size: %+v", second.available)
	}
}

func TestLinearLayoutArrangeVertical(t *testing.T) {
	layout := &LinearLayout{
		Direction: DirectionVertical,
		Spacing:   2,
	}
	first := &testChild{size: geometry.Size{Width: 10, Height: 20}}
	second := &testChild{size: geometry.Size{Width: 30, Height: 15}}
	children := []Child{first, second}

	layout.Arrange(children, geometry.Rect(100, 200, 80, 40))

	// CrossDefault starts a vertical column and preserves natural widths.
	if first.rect != geometry.Rect(100, 200, 10, 20) {
		t.Fatalf("unexpected first rect: %+v", first.rect)
	}
	if second.rect != geometry.Rect(100, 222, 30, 15) {
		t.Fatalf("unexpected second rect: %+v", second.rect)
	}
	if first.available != (geometry.Size{Width: 80, Height: 40}) {
		t.Fatalf("unexpected first available size: %+v", first.available)
	}
	if second.available != (geometry.Size{Width: 80, Height: 40}) {
		t.Fatalf("unexpected second available size: %+v", second.available)
	}
}

func TestLinearLayoutMainAlign(t *testing.T) {
	// Two children (10 + 30) in a 100-wide row leave 60 free on the main axis.
	cases := []struct {
		align   MainAlign
		firstX  float32
		secondX float32
	}{
		{MainStart, 0, 10},
		{MainCenter, 30, 40},
		{MainEnd, 60, 70},
		{MainSpaceBetween, 0, 70},
	}
	for _, tc := range cases {
		layout := &LinearLayout{Direction: DirectionHorizontal, MainAlign: tc.align}
		first := &testChild{size: geometry.Size{Width: 10, Height: 20}}
		second := &testChild{size: geometry.Size{Width: 30, Height: 20}}
		layout.Arrange([]Child{first, second}, geometry.Rect(0, 0, 100, 40))
		if first.rect.X != tc.firstX || second.rect.X != tc.secondX {
			t.Fatalf("%v: got firstX=%v secondX=%v, want %v/%v", tc.align, first.rect.X, second.rect.X, tc.firstX, tc.secondX)
		}
	}
}

func TestLinearLayoutCrossAlign(t *testing.T) {
	// A 10-wide child in an 80-wide column: hug (Start/Center/End) vs Stretch.
	cases := []struct {
		align CrossAlign
		x     float32
		width float32
	}{
		{CrossStart, 0, 10},
		{CrossCenter, 35, 10},
		{CrossEnd, 70, 10},
		{CrossStretch, 0, 80},
	}
	for _, tc := range cases {
		layout := &LinearLayout{Direction: DirectionVertical, CrossAlign: tc.align}
		child := &testChild{size: geometry.Size{Width: 10, Height: 20}}
		layout.Arrange([]Child{child}, geometry.Rect(0, 0, 80, 40))
		if child.rect.X != tc.x || child.rect.Width != tc.width {
			t.Fatalf("%v: got x=%v width=%v, want %v/%v", tc.align, child.rect.X, child.rect.Width, tc.x, tc.width)
		}
	}
}

func TestLinearLayoutCrossStretchUsesCrossAxisSize(t *testing.T) {
	layout := &LinearLayout{Direction: DirectionHorizontal, CrossAlign: CrossStretch}
	child := &testChild{
		size:   geometry.Size{},
		weight: 1,
	}
	layout.Arrange([]Child{child}, geometry.Rect(0, 0, 80, 40))

	if child.rect != geometry.Rect(0, 0, 80, 40) {
		t.Fatalf("child CrossStretch should fill the cross axis, got %+v", child.rect)
	}
}

func TestLinearLayoutCrossStretchUsesFiniteMeasureCrossSize(t *testing.T) {
	layout := &LinearLayout{Direction: DirectionHorizontal, CrossAlign: CrossStretch}
	child := &testChild{
		size: geometry.Size{Width: 10},
	}

	if got := layout.Measure([]Child{child}, Loose(geometry.Size{Width: 80, Height: 40})); got.Size != (geometry.Size{Width: 10, Height: 40}) {
		t.Fatalf("child CrossStretch should contribute the finite cross size, got %v", got)
	}
}

func TestLinearLayoutCrossStretchUsesFiniteMinimumWhenMaximumIsUnbounded(t *testing.T) {
	layout := &LinearLayout{Direction: DirectionVertical, CrossAlign: CrossStretch}
	child := &wrappingChild{}

	measured := layout.Measure([]Child{child}, Constraint{
		Min: geometry.Size{Width: 60},
		Max: geometry.Size{Width: Inf, Height: 100},
	})
	if measured.Size != (geometry.Size{Width: 60, Height: 20}) {
		t.Fatalf("cross stretch did not measure at finite minimum width: %+v", measured)
	}
	if len(child.constraints) != 1 || child.constraints[0].Min.Width != 60 || child.constraints[0].Max.Width != 60 {
		t.Fatalf("cross-stretched child did not receive tight width: %+v", child.constraints)
	}
}

func TestLinearLayoutMainWeight(t *testing.T) {
	// Two 10-wide children in a 100-wide row: 80 free split 1:3 by weight.
	layout := &LinearLayout{Direction: DirectionHorizontal}
	first := &testChild{size: geometry.Size{Width: 10, Height: 20}, weight: 1}
	second := &testChild{size: geometry.Size{Width: 10, Height: 20}, weight: 3}
	layout.Arrange([]Child{first, second}, geometry.Rect(0, 0, 100, 40))

	if first.rect != geometry.Rect(0, 10, 30, 20) { // 10 + 80*1/4
		t.Fatalf("unexpected first rect: %+v", first.rect)
	}
	if second.rect != geometry.Rect(30, 10, 70, 20) { // 10 + 80*3/4
		t.Fatalf("unexpected second rect: %+v", second.rect)
	}
}

func TestLinearLayoutBaselineMeasureAndArrange(t *testing.T) {
	layout := &LinearLayout{
		Direction:  DirectionHorizontal,
		CrossAlign: CrossBaseline,
	}
	first := &testChild{
		size:        geometry.Size{Width: 20, Height: 20},
		baseline:    15,
		hasBaseline: true,
	}
	second := &testChild{
		size:        geometry.Size{Width: 30, Height: 30},
		baseline:    20,
		hasBaseline: true,
	}
	withoutBaseline := &testChild{size: geometry.Size{Width: 10, Height: 40}}
	children := []Child{first, second, withoutBaseline}

	measured := layout.Measure(children, Loose(geometry.Size{Width: 100, Height: 100}))
	if measured.Size != (geometry.Size{Width: 60, Height: 40}) {
		t.Fatalf("unexpected baseline row size: %+v", measured.Size)
	}
	if !measured.HasBaseline || measured.Baseline != 20 {
		t.Fatalf("unexpected row baseline: %+v", measured)
	}

	layout.Arrange(children, geometry.Rect(0, 0, 100, 50))
	if first.rect != geometry.Rect(0, 5, 20, 20) {
		t.Fatalf("first child did not align to baseline 20: %+v", first.rect)
	}
	if second.rect != geometry.Rect(20, 0, 30, 30) {
		t.Fatalf("second child did not align to baseline 20: %+v", second.rect)
	}
	if withoutBaseline.rect != geometry.Rect(50, 0, 10, 40) {
		t.Fatalf("child without baseline should align to cross start: %+v", withoutBaseline.rect)
	}
}

func TestLinearLayoutBaselineInVerticalDirectionFallsBackToStart(t *testing.T) {
	layout := &LinearLayout{Direction: DirectionVertical, CrossAlign: CrossBaseline}
	child := &testChild{
		size:        geometry.Size{Width: 20, Height: 10},
		baseline:    7,
		hasBaseline: true,
	}

	layout.Arrange([]Child{child}, geometry.Rect(0, 0, 50, 30))
	if child.rect != geometry.Rect(0, 0, 20, 10) {
		t.Fatalf("vertical CrossBaseline should behave like CrossStart: %+v", child.rect)
	}
}

func TestLinearLayoutRemeasuresWeightedChildAtFinalMainSize(t *testing.T) {
	layout := &LinearLayout{Direction: DirectionHorizontal}
	fixed := &testChild{size: geometry.Size{Width: 60, Height: 10}}
	flexible := &wrappingChild{weight: 1}

	layout.Arrange([]Child{fixed, flexible}, geometry.Rect(0, 0, 100, 40))

	if len(flexible.constraints) != 2 {
		t.Fatalf("weighted child measure calls: want natural + final, got %d", len(flexible.constraints))
	}
	final := flexible.constraints[1]
	if final.Min.Width != 40 || final.Max.Width != 40 {
		t.Fatalf("weighted child final width should be tight 40, got %+v", final)
	}
	if flexible.rect != geometry.Rect(60, 10, 40, 20) {
		t.Fatalf("weighted wrapping child should use final height: %+v", flexible.rect)
	}
}

func TestLinearLayoutWeightedChildrenUseUnboundedMainAxisBasis(t *testing.T) {
	layout := &LinearLayout{Direction: DirectionVertical}
	first := &viewportChild{weight: 1}
	second := &viewportChild{weight: 1}

	layout.Arrange([]Child{first, second}, geometry.Rect(0, 0, 100, 100))

	if first.rect != geometry.Rect(0, 0, 0, 50) {
		t.Fatalf("first viewport should receive half the main axis, got %+v", first.rect)
	}
	if second.rect != geometry.Rect(0, 50, 0, 50) {
		t.Fatalf("second viewport should receive half the main axis, got %+v", second.rect)
	}
}

type testChild struct {
	size        geometry.Size
	weight      float32
	baseline    float32
	hasBaseline bool
	rect        geometry.Rectangle
	available   geometry.Size
}

func (c *testChild) Measure(cs Constraint) Measurement {
	c.available = cs.Max
	if c.hasBaseline {
		return MeasuredWithBaseline(c.size, c.baseline)
	}
	return Measured(c.size)
}

func (c *testChild) Arrange(rect geometry.Rectangle) {
	c.rect = rect
}

func (c *testChild) MainWeight() float32 {
	return c.weight
}

type wrappingChild struct {
	weight      float32
	rect        geometry.Rectangle
	constraints []Constraint
}

func (c *wrappingChild) Measure(cs Constraint) Measurement {
	c.constraints = append(c.constraints, cs)
	if cs.Min.Width == cs.Max.Width && cs.Max.Width < 80 {
		return MeasuredWithBaseline(geometry.Size{Width: cs.Max.Width, Height: 20}, 8)
	}
	return MeasuredWithBaseline(geometry.Size{Width: 80, Height: 10}, 8)
}

func (c *wrappingChild) Arrange(rect geometry.Rectangle) { c.rect = rect }
func (c *wrappingChild) MainWeight() float32             { return c.weight }

type viewportChild struct {
	weight float32
	rect   geometry.Rectangle
}

func (c *viewportChild) Measure(cs Constraint) Measurement {
	if cs.Max.Height >= Inf {
		return Measurement{}
	}
	return Measured(geometry.Size{Height: cs.Max.Height})
}

func (c *viewportChild) Arrange(rect geometry.Rectangle) {
	c.rect = rect
}

func (c *viewportChild) MainWeight() float32 {
	return c.weight
}
