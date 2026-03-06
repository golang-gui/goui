package cgo

import (
	"errors"
	"path/filepath"
	"syscall"
)

// Dummy RTLD flags
const (
	RTLD_DEFAULT = 0
	RTLD_LAZY    = 0
	RTLD_NOW     = 0
	RTLD_LOCAL   = 0
	RTLD_GLOBAL  = 0
)

// Dlopen examines the dynamic library or bundle file specified by path.
//
// The mode was ignored on Windows.
func Dlopen(path string, mode int) (uintptr, error) {
	if len(path) == 0 {
		return 0, errors.New("invalid library path")
	}
	handle, err := syscall.LoadLibrary(filepath.ToSlash(path))
	if err != nil {
		return 0, err
	}
	return uintptr(handle), nil
}

// Dlsym takes a "handle" of a dynamic library returned by Dlopen and the symbol name.
func Dlsym(handle uintptr, name string) (uintptr, error) {
	if handle == 0 {
		return 0, errors.New("invalid handle")
	}
	return syscall.GetProcAddress(syscall.Handle(handle), name)
}

// Dlclose decrements the reference count on the dynamic library handle.
func Dlclose(handle uintptr) error {
	return syscall.FreeLibrary(syscall.Handle(handle))
}

func NewCallback(fn any) uintptr {
	return syscall.NewCallback(fn)
}

func Call(fn uintptr, args ...uintptr) (r1, r2 uintptr, err syscall.Errno) {
	return syscall.SyscallN(fn, args...)
}
