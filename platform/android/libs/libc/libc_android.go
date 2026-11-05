// Package libc binds the Bionic operations used by the Android native loop.
package libc

import (
	"github.com/goexlib/cgo"
	"runtime"
	"unsafe"
)

var library = cgo.NewLazyLibrary("libc.so")
var eventfd = library.NewSymbol("eventfd")
var read = library.NewSymbol("read")
var write = library.NewSymbol("write")
var closeFD = library.NewSymbol("close")

func EventFD() int32 { return cgo.CallRet[int32](eventfd.Addr(), uint32(0), int32(0x800|0x80000)) }
func Close(fd int32) { cgo.Call(closeFD.Addr(), fd) }
func Signal(fd int32) int64 {
	one := uint64(1)
	var pin runtime.Pinner
	pin.Pin(&one)
	defer pin.Unpin()
	return cgo.CallRet[int64](write.Addr(), fd, uintptr(unsafe.Pointer(&one)), uintptr(8))
}
func Drain(fd int32) int64 {
	var count uint64
	var pin runtime.Pinner
	pin.Pin(&count)
	defer pin.Unpin()
	return cgo.CallRet[int64](read.Addr(), fd, uintptr(unsafe.Pointer(&count)), uintptr(8))
}
