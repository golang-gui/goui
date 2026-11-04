package main

import (
	ffi "github.com/goexlib/cgo"
	"github.com/golang-gui/goui/platform/typography"
	"github.com/golang-gui/goui/platform/typography/coretext"
)

func newTypography() (typography.Context, error) {
	context, err := coretext.NewContext()
	if err == nil {
		expect("CoreText 不加载桌面 frameworks", ffi.CallRet[int32](symbol("probe_desktop_loaded")) == 0)
	}
	return context, err
}
