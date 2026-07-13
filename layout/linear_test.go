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

	if size != (geometry.Size{Width: 51, Height: 25}) {
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

	if size != (geometry.Size{Width: 43, Height: 20}) {
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

	if size != (geometry.Size{Width: 30, Height: 39}) {
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

	// Default CrossStart: children hug their cross size (Height), no longer
	// stretched to the full 40. Main axis packs from the start with spacing.
	if first.rect != geometry.Rect(100, 200, 10, 20) {
		t.Fatalf("unexpected first rect: %+v", first.rect)
	}
	if second.rect != geometry.Rect(112, 200, 30, 15) {
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

	// Default CrossStart: children hug their cross size (Width), no longer
	// stretched to the full 80.
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

func TestLinearLayoutMainWeight(t *testing.T) {
	// Two 10-wide children in a 100-wide row: 80 free split 1:3 by weight.
	layout := &LinearLayout{Direction: DirectionHorizontal}
	first := &testChild{size: geometry.Size{Width: 10, Height: 20}, weight: 1}
	second := &testChild{size: geometry.Size{Width: 10, Height: 20}, weight: 3}
	layout.Arrange([]Child{first, second}, geometry.Rect(0, 0, 100, 40))

	if first.rect != geometry.Rect(0, 0, 30, 20) { // 10 + 80*1/4
		t.Fatalf("unexpected first rect: %+v", first.rect)
	}
	if second.rect != geometry.Rect(30, 0, 70, 20) { // 10 + 80*3/4
		t.Fatalf("unexpected second rect: %+v", second.rect)
	}
}

type testChild struct {
	size      geometry.Size
	weight    float32
	rect      geometry.Rectangle
	available geometry.Size
}

func (c *testChild) Measure(cs Constraint) geometry.Size {
	c.available = cs.Max
	return c.size
}

func (c *testChild) Arrange(rect geometry.Rectangle) {
	c.rect = rect
}

func (c *testChild) MainWeight() float32 {
	return c.weight
}
