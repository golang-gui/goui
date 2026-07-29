package common

import (
	"os"
	"strconv"
)

func GetPreferPainter() string {
	return os.Getenv("GOUI_PLAT_PAINTER")
}

// GetPreferScale returns the GOUI_PLAT_SCALE env override. Returns 0 if unset
// or invalid, in which case the platform's auto-detected scale is used.
func GetPreferScale() float32 {
	if v, err := strconv.ParseFloat(os.Getenv("GOUI_PLAT_SCALE"), 32); err == nil && v > 0 {
		return float32(v)
	}
	return 0
}
