//go:build android || ios

// Package entry provides the precompiled standard-cgo ABI entry support.
// It has no native UI, window, event-loop or graphics responsibilities.
package entry

/*
#include <stddef.h>
*/
import "C"

import (
	"github.com/golang-gui/goui/platform/internal/mobileentry"
	"unsafe"
)

//export goui_run
func goui_run() C.int { return C.int(mobileentry.Run()) }

//export ANativeActivity_onCreate
func ANativeActivity_onCreate(activity unsafe.Pointer, saved unsafe.Pointer, size C.size_t) {
	mobileentry.Create(uintptr(activity), uintptr(saved), uintptr(size))
}
