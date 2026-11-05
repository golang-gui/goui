package main

import (
	"github.com/golang-gui/goui/platform"
	native "github.com/golang-gui/goui/platform/android/libs/android"
	"github.com/golang-gui/goui/platform/typography"
	"unsafe"
)

func newTypography(p platform.Platform) (typography.Context, error) {
	expect("NDK Activity ARM64 size", unsafe.Sizeof(native.Activity{}) == 80)
	expect("NDK Activity SDK offset", unsafe.Offsetof(native.Activity{}.SDKVersion) == 48)
	expect("NDK callback table size", unsafe.Sizeof(native.ActivityCallbacks{}) == 128)
	expect("NDK native window buffer size", unsafe.Sizeof(native.WindowBuffer{}) == 48)
	return p.NewTypography()
}
