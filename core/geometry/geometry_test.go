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
