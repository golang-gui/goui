package main

import (
	"github.com/goexlib/cgo"
	"github.com/golang-gui/goui/platform/android/libs/egl"
)

func nativeGLSymbol(name string) uintptr {
	if address := egl.GetProcAddress(name); address != 0 {
		return address
	}
	library, err := cgo.Dlopen("libGLESv3.so", cgo.RTLD_NOW)
	must(err)
	defer cgo.Dlclose(library)
	address, err := cgo.Dlsym(library, name)
	must(err)
	return address
}
