package core_foundation

import "github.com/golang-gui/goui/platform/cocoa/frameworks/common"

var handle uintptr

func Init(load common.LoadFunc) (err error) {
	handle, err = load("CoreFoundation")
	return
}
