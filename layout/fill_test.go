package layout

import (
	"testing"

	"github.com/golang-gui/goui/core/geometry"
)

func TestFillLayoutMeasure(t *testing.T) {
	layout := NewFillLayout()
	children := []Child{
		&testChild{size: geometry.Size{Width: 10, Height: 20}},
		nil,
		&testChild{size: geometry.Size{Width: 30, Height: 15}},
	}

	size := layout.Measure(children, Loose(geometry.Size{Width: 100, Height: 50}))

	if size.Size != (geometry.Size{Width: 30, Height: 20}) {
		t.Fatalf("unexpected measured size: %+v", size)
	}
}

func TestFillLayoutArrange(t *testing.T) {
	layout := NewFillLayout()
	first := &testChild{size: geometry.Size{Width: 10, Height: 20}}
	second := &testChild{size: geometry.Size{Width: 30, Height: 15}}
	children := []Child{first, nil, second}

	layout.Arrange(children, geometry.Rect(5, 6, 70, 80))

	if first.rect != geometry.Rect(5, 6, 70, 80) {
		t.Fatalf("unexpected first rect: %+v", first.rect)
	}
	if second.rect != geometry.Rect(5, 6, 70, 80) {
		t.Fatalf("unexpected second rect: %+v", second.rect)
	}
}

func TestFillLayoutPropagatesFirstBaseline(t *testing.T) {
	layout := NewFillLayout()
	child := &testChild{
		size:        geometry.Size{Width: 30, Height: 20},
		baseline:    14,
		hasBaseline: true,
	}

	measured := layout.Measure([]Child{child}, Loose(geometry.Size{Width: 100, Height: 100}))
	if !measured.HasBaseline || measured.Baseline != 14 {
		t.Fatalf("fill layout did not propagate child baseline: %+v", measured)
	}
}
