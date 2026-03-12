package cgo

import "unsafe"

func GetExternVariant[T any](libraryHandle uintptr, name string) (pv *T, err error) {
	symbol, err := Dlsym(libraryHandle, name)
	if err != nil {
		return
	}
	return (*T)(unsafe.Pointer(symbol)), nil
}
