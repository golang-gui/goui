package colors

import (
	"image/color"
	"testing"
)

func TestEqual(t *testing.T) {
	tests := []struct {
		name string
		a, b color.Color
		want bool
	}{
		{name: "both nil", a: nil, b: nil, want: true},
		{name: "nil and non-nil", a: nil, b: color.Black, want: false},
		{name: "same value", a: color.RGBA{R: 10, G: 20, B: 30, A: 255}, b: color.RGBA{R: 10, G: 20, B: 30, A: 255}, want: true},
		{name: "different value", a: color.RGBA{R: 10, A: 255}, b: color.RGBA{R: 11, A: 255}, want: false},
		{
			name: "same color across types",
			a:    color.RGBA{R: 255, G: 255, B: 255, A: 255},
			b:    color.Gray{Y: 255},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Equal(tt.a, tt.b); got != tt.want {
				t.Fatalf("Equal(%v, %v) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}
