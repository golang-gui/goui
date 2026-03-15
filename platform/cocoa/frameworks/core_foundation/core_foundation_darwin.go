package core_foundation

import (
	"github.com/golang-gui/goui/core/cgo"
	"github.com/golang-gui/goui/platform/cocoa/frameworks/common"
)

var handle uintptr

func Init(load common.LoadFunc) (err error) {
	handle, err = load("CoreFoundation")
	if err != nil {
		return
	}

	err = initCoreFoundation()
	if err != nil {
		return
	}

	return initCFData()
}

type (
	CFIndex = int32
	Uint8   = uint8
)

func initCoreFoundation() (err error) {
	err = cgo.RegisterLibFunc(&fnCFRelease, handle, "CFRelease")
	if err != nil {
		return
	}
	return
}

var (
	fnCFRelease func(ref CFTypeRef)
)

type CFTypeRef uintptr

func CFRelease(ref CFTypeRef) {
	fnCFRelease(ref)
}

func initCFData() (err error) {
	err = cgo.RegisterLibFunc(&fnCFDataCreate, handle, "CFDataCreate")
	if err != nil {
		return
	}
	return
}

var (
	fnCFDataCreate func(allocator CFAllocatorRef, bytes cgo.Pointer, length CFIndex) CFDataRef
)

type (
	CFDataRef      = CFTypeRef
	CFAllocatorRef = CFTypeRef
)

func CFDataCreate(allocator CFAllocatorRef, bytes []byte) CFDataRef {
	return fnCFDataCreate(allocator, cgo.CSlice(bytes), CFIndex(len(bytes)))
}
