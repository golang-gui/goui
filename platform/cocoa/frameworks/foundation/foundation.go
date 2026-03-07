package foundation

import "github.com/golang-gui/goui/platform/cocoa/frameworks/common"

var handle uintptr

func Init(load common.LoadFunc) (err error) {
	handle, err = load("Foundation")
	if err != nil {
		return
	}

	initNSDate()
	err = initNSRunLoopMode()
	if err != nil {
		return
	}

	err = initAutoReleasePool()
	if err != nil {
		return
	}

	return
}
