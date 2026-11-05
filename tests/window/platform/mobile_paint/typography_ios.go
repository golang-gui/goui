package main

import (
	ffi "github.com/goexlib/cgo"
	"github.com/golang-gui/goui/platform"
	"github.com/golang-gui/goui/platform/typography"
)

func newTypography(p platform.Platform) (typography.Context, error) {
	context, err := p.NewTypography()
	if err == nil {
		expect("CoreText 不加载桌面 frameworks", ffi.CallRet[int32](symbol("probe_desktop_loaded")) == 0)
	}
	return context, err
}
