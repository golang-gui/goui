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

// 内容高度被 padding 耗尽时，矩形内缩会把原点放到轴中点。
// 基线必须使用这个实际原点；子内容即使被裁剪，也保留自身的排版基线。
func TestLayoutBaselineUsesClampedPaddingOrigin(t *testing.T) {
	for _, manager := range []struct {
		name string
		new  func(Direction, CrossAlign, geometry.Insets) LayoutManager
	}{
		{"linear", func(d Direction, a CrossAlign, p geometry.Insets) LayoutManager {
			return &LinearLayout{Direction: d, CrossAlign: a, Padding: p}
		}},
		{"wrap", func(d Direction, a CrossAlign, p geometry.Insets) LayoutManager {
			return &WrapLayout{Direction: d, CrossAlign: a, Padding: p}
		}},
	} {
		for _, orientation := range []struct {
			name      string
			direction Direction
			align     CrossAlign
		}{
			{"row-start", DirectionHorizontal, CrossStart},
			{"row-baseline", DirectionHorizontal, CrossBaseline},
			{"column-start", DirectionVertical, CrossStart},
		} {
			for _, tc := range []struct {
				name               string
				height, contentTop float32
			}{
				{"less-than-padding", 12, 6},
				{"equal-to-padding", 20, 10},
				{"positive-content-height", 24, 4},
			} {
				t.Run(manager.name+"/"+orientation.name+"/"+tc.name, func(t *testing.T) {
					padding := geometry.Insets{Left: 10, Top: 4, Right: 6, Bottom: 16}
					l := manager.new(orientation.direction, orientation.align, padding)
					child := &wrapTestChild{size: geometry.Size{Width: 20, Height: 10}, baseline: 7, hasBaseline: true}
					children := []Child{child}
					size := geometry.Size{Width: 100, Height: tc.height}
					measured := l.Measure(children, Tight(size))
					want := tc.contentTop + 7
					if measured.Size != size || !measured.HasBaseline || measured.Baseline != want {
						t.Fatalf("测量=%+v，期望尺寸=%+v、基线=%v", measured, size, want)
					}
					l.Arrange(children, geometry.Rectangle{Pos: geometry.Point{X: 13, Y: 17}, Size: size})
					if got := child.rect.Y - 17 + 7; got != want {
						t.Fatalf("布局后的子基线=%v，期望=%v；子矩形=%+v", got, want, child.rect)
					}
				})
			}
		}
	}
}
