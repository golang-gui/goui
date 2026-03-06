//go:build !windows

package cgo

import (
	"github.com/ebitengine/purego"
	"syscall"
)

const (
	RTLD_DEFAULT = purego.RTLD_DEFAULT
	RTLD_LAZY    = purego.RTLD_LAZY
	RTLD_NOW     = purego.RTLD_NOW
	RTLD_LOCAL   = purego.RTLD_LOCAL
	RTLD_GLOBAL  = purego.RTLD_GLOBAL
)

// Dlopen examines the dynamic library or bundle file specified by path.
//
// The mode was ignored on Windows.
func Dlopen(path string, mode int) (uintptr, error) {
	return purego.Dlopen(path, mode)
}

// Dlsym takes a "handle" of a dynamic library returned by Dlopen and the symbol name.
func Dlsym(handle uintptr, name string) (uintptr, error) {
	return purego.Dlsym(handle, name)
}

// Dlclose decrements the reference count on the dynamic library handle.
func Dlclose(handle uintptr) error {
	return purego.Dlclose(handle)
}

func NewCallback(fn any) uintptr {
	return purego.NewCallback(fn)
}

func Call(fn uintptr, args ...uintptr) (r1, r2 uintptr, err syscall.Errno) {
	r1, r2, e := purego.SyscallN(fn, args...)
	return r1, r2, syscall.Errno(e)
}
