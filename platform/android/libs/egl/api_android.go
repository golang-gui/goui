package egl

import "github.com/goexlib/cgo"

var (
	libegl                  = cgo.NewLazyLibrary("libEGL.so")
	eglGetDisplay           = libegl.NewSymbol("eglGetDisplay")
	eglInitialize           = libegl.NewSymbol("eglInitialize")
	eglTerminate            = libegl.NewSymbol("eglTerminate")
	eglChooseConfig         = libegl.NewSymbol("eglChooseConfig")
	eglGetConfigAttrib      = libegl.NewSymbol("eglGetConfigAttrib")
	eglCreateContext        = libegl.NewSymbol("eglCreateContext")
	eglCreatePbufferSurface = libegl.NewSymbol("eglCreatePbufferSurface")
	eglCreateWindowSurface  = libegl.NewSymbol("eglCreateWindowSurface")
	eglDestroyContext       = libegl.NewSymbol("eglDestroyContext")
	eglDestroySurface       = libegl.NewSymbol("eglDestroySurface")
	eglMakeCurrent          = libegl.NewSymbol("eglMakeCurrent")
	eglSwapBuffers          = libegl.NewSymbol("eglSwapBuffers")
	eglSwapInterval         = libegl.NewSymbol("eglSwapInterval")
	eglGetError             = libegl.NewSymbol("eglGetError")
	eglGetProcAddress       = libegl.NewSymbol("eglGetProcAddress")
)

func GetDisplay(native uintptr) Display { return cgo.CallRet[Display](eglGetDisplay.Addr(), native) }

// Initialize permits nil version outputs, as specified by EGL.
func Initialize(display Display, major, minor *int32) Boolean {
	return cgo.CallRet[Boolean](eglInitialize.Addr(), display, major, minor)
}
func Terminate(display Display) Boolean { return cgo.CallRet[Boolean](eglTerminate.Addr(), display) }

// Attribute lists are borrowed, EGL_NONE-terminated, or empty for native NULL.
// ChooseConfig accepts an empty configs slice to query the available count.
func ChooseConfig(display Display, attrs []int32, configs []Config) (count int32, ok Boolean) {
	ok = cgo.CallRet[Boolean](eglChooseConfig.Addr(), display, attributes(attrs), configPointer(configs), int32(len(configs)), &count)
	return
}
func GetConfigAttrib(display Display, config Config, name int32) (value int32, ok Boolean) {
	ok = cgo.CallRet[Boolean](eglGetConfigAttrib.Addr(), display, config, name, &value)
	return
}
func CreateContext(display Display, config Config, share Context, attrs []int32) Context {
	return cgo.CallRet[Context](eglCreateContext.Addr(), display, config, share, attributes(attrs))
}
func CreatePbufferSurface(display Display, config Config, attrs []int32) Surface {
	return cgo.CallRet[Surface](eglCreatePbufferSurface.Addr(), display, config, attributes(attrs))
}
func CreateWindowSurface(display Display, config Config, window uintptr, attrs []int32) Surface {
	return cgo.CallRet[Surface](eglCreateWindowSurface.Addr(), display, config, window, attributes(attrs))
}
func DestroyContext(display Display, context Context) Boolean {
	return cgo.CallRet[Boolean](eglDestroyContext.Addr(), display, context)
}
func DestroySurface(display Display, surface Surface) Boolean {
	return cgo.CallRet[Boolean](eglDestroySurface.Addr(), display, surface)
}
func MakeCurrent(display Display, draw, read Surface, context Context) Boolean {
	return cgo.CallRet[Boolean](eglMakeCurrent.Addr(), display, draw, read, context)
}
func SwapBuffers(display Display, surface Surface) Boolean {
	return cgo.CallRet[Boolean](eglSwapBuffers.Addr(), display, surface)
}
func SwapInterval(display Display, interval int32) Boolean {
	return cgo.CallRet[Boolean](eglSwapInterval.Addr(), display, interval)
}
func GetError() int32 { return cgo.CallRet[int32](eglGetError.Addr()) }
func GetProcAddress(name string) uintptr {
	return cgo.CallRet[uintptr](eglGetProcAddress.Addr(), cgo.CStringTemp(name))
}
