package layout

import (
	"testing"

	"github.com/golang-gui/goui/core/geometry"
)

func TestMeasurementConstrainPreservesBaseline(t *testing.T) {
	measured := MeasuredWithBaseline(geometry.Size{Width: 80, Height: 30}, 18)
	got := measured.Constrain(Tight(geometry.Size{Width: 40, Height: 20}))

	if got.Size != (geometry.Size{Width: 40, Height: 20}) {
		t.Fatalf("unexpected constrained size: %+v", got.Size)
	}
	if !got.HasBaseline || got.Baseline != 18 {
		t.Fatalf("constraining size changed the content baseline: %+v", got)
	}
}

func TestConstraintClampLetsMaxWinWhenOverConstrained(t *testing.T) {
	c := Constraint{
		Min: geometry.Size{Width: 80, Height: 60},
		Max: geometry.Size{Width: 40, Height: 30},
	}
	if got := c.Clamp(geometry.Size{Width: 50, Height: 50}); got != c.Max {
		t.Fatalf("over-constrained axis should resolve to max: got %+v want %+v", got, c.Max)
	}
}

func TestConstraintInsetPreservesUnboundedAxis(t *testing.T) {
	c := Constraint{
		Min: geometry.Size{Width: 60, Height: 20},
		Max: geometry.Size{Width: Inf, Height: 100},
	}
	got := c.Inset(10)
	want := Constraint{
		Min: geometry.Size{Width: 40},
		Max: geometry.Size{Width: Inf, Height: 80},
	}
	if got != want {
		t.Fatalf("unexpected inset constraint: got %+v want %+v", got, want)
	}
}

func TestConstraintInsetsPreservesUnboundedAxis(t *testing.T) {
	c := Constraint{
		Min: geometry.Size{Width: 60, Height: 20},
		Max: geometry.Size{Width: Inf, Height: 100},
	}
	got := c.Insets(geometry.Insets{Left: 5, Top: 2, Right: 3, Bottom: 4})
	want := Constraint{
		Min: geometry.Size{Width: 52, Height: 14},
		Max: geometry.Size{Width: Inf, Height: 94},
	}
	if got != want {
		t.Fatalf("unexpected asymmetric inset constraint: got %+v want %+v", got, want)
	}
}
