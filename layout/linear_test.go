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

	size := layout.Measure(children, geometry.Size{Width: 100, Height: 50})

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

	size := layout.Measure(children, geometry.Size{Width: 100, Height: 50})

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

	size := layout.Measure(children, geometry.Size{Width: 100, Height: 50})

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

	if first.rect != geometry.Rect(100, 200, 10, 40) {
		t.Fatalf("unexpected first rect: %+v", first.rect)
	}
	if second.rect != geometry.Rect(112, 200, 30, 40) {
		t.Fatalf("unexpected second rect: %+v", second.rect)
	}
	if first.available != (geometry.Size{Width: 80, Height: 40}) {
		t.Fatalf("unexpected first available size: %+v", first.available)
	}
	if second.available != (geometry.Size{Width: 68, Height: 40}) {
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

	if first.rect != geometry.Rect(100, 200, 80, 20) {
		t.Fatalf("unexpected first rect: %+v", first.rect)
	}
	if second.rect != geometry.Rect(100, 222, 80, 15) {
		t.Fatalf("unexpected second rect: %+v", second.rect)
	}
	if first.available != (geometry.Size{Width: 80, Height: 40}) {
		t.Fatalf("unexpected first available size: %+v", first.available)
	}
	if second.available != (geometry.Size{Width: 80, Height: 18}) {
		t.Fatalf("unexpected second available size: %+v", second.available)
	}
}

type testChild struct {
	size      geometry.Size
	rect      geometry.Rectangle
	available geometry.Size
}

func (c *testChild) Measure(available geometry.Size) geometry.Size {
	c.available = available
	return c.size
}

func (c *testChild) Arrange(rect geometry.Rectangle) {
	c.rect = rect
}
