package x11

import "testing"

func TestPopupPhysicalExtent(t *testing.T) {
	for _, tc := range []struct {
		name           string
		logical, scale float32
		want           int
	}{
		{"menu width at 150 percent", 227, 1.5, 341},
		{"menu height at 150 percent", 544, 1.5, 816},
		{"fractional logical width at 125 percent", 151.25, 1.25, 190},
		{"fractional logical height at 175 percent", 160.25, 1.75, 281},
		{"fractional height at 1x", 40.25, 1, 41},
		{"integral pixels at 1x", 227, 1, 227},
		{"integral pixels at 2x", 227, 2, 454},
		{"small positive extent", .25, 1.5, 1},
		{"zero extent", 0, 1.5, 1},
		{"negative extent", -10, 1.5, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := popupPhysicalExtent(tc.logical, tc.scale)
			if got != tc.want {
				t.Fatalf("logical=%g scale=%g: got %d pixels, want %d", tc.logical, tc.scale, got, tc.want)
			}
			// 请求范围有效时，结果必须容纳全文，且不能多占一个完整像素。
			requested := float64(tc.logical) * float64(tc.scale)
			if float64(got) < requested || got > 1 && float64(got-1) >= requested {
				t.Fatal("not the minimum containing physical extent")
			}
		})
	}
}
