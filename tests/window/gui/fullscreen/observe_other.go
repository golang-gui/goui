//go:build (!linux && !windows && !darwin) || android || ios

package main

import (
	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform"
)

func nativeArea(platform.Window) (geometry.Rectangle, geometry.Size, error) {
	return geometry.Rectangle{}, geometry.Size{}, platform.ErrUnsupported
}
