package main

import "github.com/golang-gui/goui/platform/darwin/frameworks/opengles"

func nativeGLSymbol(name string) uintptr {
	address, err := opengles.GetProcAddress(name)
	must(err)
	return address
}
