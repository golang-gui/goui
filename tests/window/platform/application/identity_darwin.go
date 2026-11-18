package main

import (
	"fmt"

	"github.com/golang-gui/goui/platform"
)

func checkIdentity(plat platform.Platform) {
	fmt.Println("NOT APPLICABLE: repeated native Platform creation on macOS")
}
