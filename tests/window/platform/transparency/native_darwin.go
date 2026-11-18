package main

import (
	"fmt"

	"github.com/golang-gui/goui/platform"
)

func checkNativeAlpha(plat platform.Platform) {
	fmt.Println("INFO macOS: no additional native alpha assertion")
}
