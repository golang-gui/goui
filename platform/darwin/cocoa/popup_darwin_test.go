package cocoa

import (
	"math"
	"testing"

	. "github.com/golang-gui/goui/platform/darwin/frameworks/core_graphics"
	. "github.com/golang-gui/goui/platform/darwin/frameworks/foundation"
)

// Pure size conversion: no AppKit calls or desktop connection.
func TestPopupSizeInPoints(t *testing.T) {
	for _, tt := range []struct {
		name           string
		width, height  float32
		backing, scale CGFloat
		want           NSSize
	}{
		{"fractional-1x", 290, 238.05859375, 1, 0, NSSize{Width: 290, Height: 239}},
		{"fractional-retina", 290, 238.05859375, 2, 0, NSSize{Width: 290, Height: 238.5}},
		{"override-1x-retina", 290, 238.05859375, 2, 1, NSSize{Width: 145, Height: 119.5}},
		{"override-2x", 290, 238.05859375, 1, 2, NSSize{Width: 580, Height: 477}},
		{"override-fractional", 100.1, 10.1, 2, 1.25, NSSize{Width: 63, Height: 6.5}},
		{"integral", 290, 170, 1, 1, NSSize{Width: 290, Height: 170}},
		{"half-point-retina", 123.5, 170.5, 2, 0, NSSize{Width: 123.5, Height: 170.5}},
		{"fractional-width", 123.25, 170, 1, 0, NSSize{Width: 124, Height: 170}},
		{"minimum", 0, -1, 2, 1, NSSize{Width: .5, Height: .5}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := popupSizeInPoints(tt.width, tt.height, tt.backing, tt.scale)
			if got != tt.want {
				t.Fatalf("got %+v points, want %+v", got, tt.want)
			}
			scale := tt.scale
			if scale <= 0 {
				scale = tt.backing
			}
			for _, extent := range []struct {
				logical float32
				points  CGFloat
			}{{tt.width, got.Width}, {tt.height, got.Height}} {
				pixels := float64(extent.points * tt.backing)
				requested := float64(max(1, extent.logical)) * float64(scale)
				if pixels != math.Trunc(pixels) || pixels < requested || pixels-requested >= 1 {
					t.Fatalf("%g backing pixels must minimally contain %g requested pixels", pixels, requested)
				}
			}
		})
	}
}
