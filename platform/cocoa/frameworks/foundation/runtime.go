package foundation

import (
	"github.com/golang-gui/goui/core/cgo"
)

func initAutoReleasePool() (err error) {
	err = cgo.RegisterLibFunc(&objc_autoreleasePoolPush, handle, "objc_autoreleasePoolPush")
	if err != nil {
		return
	}
	return cgo.RegisterLibFunc(&objc_autoreleasePoolPop, handle, "objc_autoreleasePoolPop")
}

var (
	objc_autoreleasePoolPush func() uintptr
	objc_autoreleasePoolPop  func(pool uintptr)
)

func AutoReleasePool(block func()) {
	pool := objc_autoreleasePoolPush()
	defer objc_autoreleasePoolPop(pool)
	block()
}
