package foundation

import (
	"github.com/ebitengine/purego"
	"unsafe"
)

func initNSRunLoopMode() (err error) {
	sym, err := purego.Dlsym(handle, "NSDefaultRunLoopMode")
	if err != nil {
		return
	}

	NSDefaultRunLoopMode = getRonLoopVariable(sym)
	return nil
}

type NSRunLoopMode NSString

var NSDefaultRunLoopMode NSRunLoopMode

func getRonLoopVariable(symbol uintptr) NSRunLoopMode {
	ptr := (*uintptr)(unsafe.Pointer(symbol))
	return NSRunLoopMode(*ptr)
}
