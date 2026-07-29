package common

import "os"

func GetPreferPainter() string {
	return os.Getenv("GOUI_PLAT_PAINTER")
}
