// Package gles binds the core OpenGL ES entry points used outside NanoVG.
package gles

import "github.com/goexlib/cgo"

var (
	libgles     = cgo.NewLazyLibrary("libGLESv3.so")
	glFlush     = libgles.NewSymbol("glFlush")
	glGetString = libgles.NewSymbol("glGetString")
)

const Extensions uint32 = 0x1f03

func Flush() { cgo.Call(glFlush.Addr()) }

// GetString copies a string owned by the current GL context.
func GetString(name uint32) string {
	return cgo.GoString(cgo.Pointer(cgo.CallRet[uintptr](glGetString.Addr(), name)))
}

// Lookup resolves a core library symbol when eglGetProcAddress returns NULL.
func Lookup(name string) (uintptr, error) {
	if err := libgles.Load(); err != nil {
		return 0, err
	}
	return cgo.Dlsym(libgles.Handle(), name)
}
