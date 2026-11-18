package main

import (
	"fmt"

	"github.com/golang-gui/goui/platform"
)

func checkExternalOpenInvalidInput(plat platform.Platform) {
	p := plat
	var err error
	for _, value := range []string{"", "example.com", "bad:escape%", "test:a\x00b"} {
		err = p.OpenURL(value)
		if err == nil {
			panic(fmt.Errorf("OpenURL accepted %q", value))
		}
	}
	for _, value := range []string{"", "a\x00b"} {
		err = p.OpenPath(value)
		if err == nil {
			panic(fmt.Errorf("OpenPath accepted %q", value))
		}
	}
}
