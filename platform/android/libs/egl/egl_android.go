// Package egl binds Android's public EGL API.
package egl

import (
	"github.com/goexlib/cgo"
	"runtime"
	"unsafe"
)

var library = cgo.NewLazyLibrary("libEGL.so")
var fnGetDisplay = library.NewSymbol("eglGetDisplay")

func GetDisplay(native uintptr) uintptr { return cgo.CallRet[uintptr](fnGetDisplay.Addr(), native) }

var fnInitialize = library.NewSymbol("eglInitialize")

func Initialize(display uintptr) int32 {
	return cgo.CallRet[int32](fnInitialize.Addr(), display, uintptr(0), uintptr(0))
}

var fnDestroyContext = library.NewSymbol("eglDestroyContext")

func DestroyContext(display, context uintptr) int32 {
	return cgo.CallRet[int32](fnDestroyContext.Addr(), display, context)
}

var fnDestroySurface = library.NewSymbol("eglDestroySurface")

func DestroySurface(display, surface uintptr) int32 {
	return cgo.CallRet[int32](fnDestroySurface.Addr(), display, surface)
}

var fnMakeCurrent = library.NewSymbol("eglMakeCurrent")

func MakeCurrent(display, draw, read, context uintptr) int32 {
	return cgo.CallRet[int32](fnMakeCurrent.Addr(), display, draw, read, context)
}

var fnSwapBuffers = library.NewSymbol("eglSwapBuffers")

func SwapBuffers(display, surface uintptr) int32 {
	return cgo.CallRet[int32](fnSwapBuffers.Addr(), display, surface)
}

var fnSwapInterval = library.NewSymbol("eglSwapInterval")

func SwapInterval(display uintptr, interval int32) int32 {
	return cgo.CallRet[int32](fnSwapInterval.Addr(), display, interval)
}

var fnError = library.NewSymbol("eglGetError")

func Error() int32 { return cgo.CallRet[int32](fnError.Addr()) }

var fnProc = library.NewSymbol("eglGetProcAddress")

func Proc(name string) uintptr { return cgo.CallRet[uintptr](fnProc.Addr(), cgo.CStringTemp(name)) }

var choose = library.NewSymbol("eglChooseConfig")

func ChooseConfig(display uintptr, attrs []int32) (uintptr, int32, int32) {
	var config uintptr
	var count int32
	var pin runtime.Pinner
	pin.Pin(&attrs[0])
	pin.Pin(&config)
	pin.Pin(&count)
	defer pin.Unpin()
	ok := cgo.CallRet[int32](choose.Addr(), display, uintptr(unsafe.Pointer(&attrs[0])), uintptr(unsafe.Pointer(&config)), int32(1), uintptr(unsafe.Pointer(&count)))
	return config, count, ok
}

var attrib = library.NewSymbol("eglGetConfigAttrib")

func ConfigAttrib(display, config uintptr, name int32) (int32, int32) {
	var value int32
	var pin runtime.Pinner
	pin.Pin(&value)
	defer pin.Unpin()
	ok := cgo.CallRet[int32](attrib.Addr(), display, config, name, uintptr(unsafe.Pointer(&value)))
	return value, ok
}

var createContext = library.NewSymbol("eglCreateContext")

func CreateContext(display, config uintptr, attrs []int32) uintptr {
	var pin runtime.Pinner
	pin.Pin(&attrs[0])
	defer pin.Unpin()
	return cgo.CallRet[uintptr](createContext.Addr(), display, config, uintptr(0), uintptr(unsafe.Pointer(&attrs[0])))
}

var createPbuffer = library.NewSymbol("eglCreatePbufferSurface")

func CreatePbuffer(display, config uintptr, attrs []int32) uintptr {
	var pin runtime.Pinner
	pin.Pin(&attrs[0])
	defer pin.Unpin()
	return cgo.CallRet[uintptr](createPbuffer.Addr(), display, config, uintptr(unsafe.Pointer(&attrs[0])))
}

var createWindow = library.NewSymbol("eglCreateWindowSurface")

func CreateWindow(display, config, window uintptr) uintptr {
	return cgo.CallRet[uintptr](createWindow.Addr(), display, config, window, uintptr(0))
}
