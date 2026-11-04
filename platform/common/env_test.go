package common

import "testing"

func TestGetPreferScale(t *testing.T) {
	for _, s := range []string{"", "invalid", "0", "-1", "NaN", "Inf", "+Inf", "1e1000"} {
		t.Run(s, func(t *testing.T) {
			t.Setenv("GOUI_PLAT_SCALE", s)
			if got := GetPreferScale(); got != 0 {
				t.Fatalf("无效比例 %q 得到 %v", s, got)
			}
		})
	}
	t.Setenv("GOUI_PLAT_SCALE", "1.5")
	if got := GetPreferScale(); got != 1.5 {
		t.Fatal(got)
	}
}
