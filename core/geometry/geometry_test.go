package geometry

import "testing"

func TestSizeInset(t *testing.T) {
	tests := []struct {
		name string
		size Size
		n    float32
		want Size
	}{
		{name: "shrink", size: Size{Width: 100, Height: 40}, n: 8, want: Size{Width: 84, Height: 24}},
		{name: "outset via negative", size: Size{Width: 100, Height: 40}, n: -8, want: Size{Width: 116, Height: 56}},
		{name: "clamp to zero", size: Size{Width: 10, Height: 6}, n: 8, want: Size{Width: 0, Height: 0}},
		{name: "zero inset is identity", size: Size{Width: 30, Height: 20}, n: 0, want: Size{Width: 30, Height: 20}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.size.Inset(tt.n); got != tt.want {
				t.Fatalf("Size%+v.Inset(%v) = %+v, want %+v", tt.size, tt.n, got, tt.want)
			}
		})
	}
}

func TestSizeInsets(t *testing.T) {
	tests := []struct {
		name                     string
		size                     Size
		left, top, right, bottom float32
		want                     Size
	}{
		{name: "asymmetric shrink", size: Size{Width: 100, Height: 40}, left: 10, top: 2, right: 6, bottom: 4, want: Size{Width: 84, Height: 34}},
		{name: "outset via negative values", size: Size{Width: 100, Height: 40}, left: -10, top: -2, right: 6, bottom: 4, want: Size{Width: 104, Height: 38}},
		{name: "clamp width only", size: Size{Width: 10, Height: 40}, left: 6, top: 2, right: 6, bottom: 4, want: Size{Width: 0, Height: 34}},
		{name: "zero insets are identity", size: Size{Width: 30, Height: 20}, want: Size{Width: 30, Height: 20}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.size.Insets(tt.left, tt.top, tt.right, tt.bottom); got != tt.want {
				t.Fatalf("Size%+v.Insets(%v,%v,%v,%v) = %+v, want %+v", tt.size, tt.left, tt.top, tt.right, tt.bottom, got, tt.want)
			}
		})
	}
}

func TestInsetsHelpers(t *testing.T) {
	if got := UniformInsets(6); got != (Insets{Left: 6, Top: 6, Right: 6, Bottom: 6}) {
		t.Fatalf("UniformInsets = %+v", got)
	}
	p := Insets{Left: 8, Top: 4, Right: 6, Bottom: 2}
	if p.Horizontal() != 14 || p.Vertical() != 6 {
		t.Fatalf("Insets axis totals = %v/%v", p.Horizontal(), p.Vertical())
	}
}
