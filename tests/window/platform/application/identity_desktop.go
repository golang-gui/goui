//go:build !darwin

package main

import (
	"fmt"

	"github.com/golang-gui/goui/platform"
)

func checkApplicationIdentity(plat platform.Platform) {
	var err error
	func() {
		got, createErr := platform.NewPlatform(platform.DefaultName(), testApplicationID)
		if createErr != nil || got != plat {
			err = fmt.Errorf("repeat creation: %v, %v", got, createErr)
			return
		}
		for _, other := range []string{"", "org.golang-gui.Other", "invalid id"} {
			if got, createErr := platform.NewPlatform(platform.DefaultName(), other); got != nil || createErr == nil {
				err = fmt.Errorf("conflicting ID %q: %v, %v", other, got, createErr)
				return
			}
		}
	}()
	if err != nil {
		panic(err)
	}
}
