package appkit

import "github.com/golang-gui/goui/platform/cocoa/frameworks/common"

var handle uintptr

func Init(load common.LoadFunc) (err error) {
	handle, err = load("AppKit")
	if err != nil {
		return
	}
	initNSEvent()
	initNSApplication()
	return
}
