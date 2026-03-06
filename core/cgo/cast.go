package cgo

import (
	"slices"
	"unsafe"
)

// Length Get the length of an array ending with zero
func Length[T comparable](p unsafe.Pointer) (n int) {
	var zero T
	step := unsafe.Sizeof(zero)
	for {
		pv := (*T)(unsafe.Pointer(uintptr(p) + uintptr(n)*step))
		if *pv == zero {
			break
		}
		n++
	}
	return
}

func Strlen(s unsafe.Pointer) (n int) {
	return Length[byte](s)
}

func GoSliceTemp[T comparable](p unsafe.Pointer) []T {
	return GoSliceNTemp[T](p, Length[T](p))
}

func GoSliceNTemp[T any](p unsafe.Pointer, n int) []T {
	return unsafe.Slice((*T)(p), n)
}

func GoStringTemp(s unsafe.Pointer) string {
	return GoStringNTemp(s, Strlen(s))
}

func GoStringNTemp(s unsafe.Pointer, n int) string {
	return unsafe.String((*byte)(s), n)
}

func GoSlice[T comparable](p unsafe.Pointer) []T {
	return GoSliceN[T](p, Length[T](p))
}

func GoSliceN[T any](p unsafe.Pointer, n int) []T {
	slice := GoSliceNTemp[T](p, n)
	return slices.Clone(slice)
}

func GoString(s unsafe.Pointer) string {
	return GoStringN(s, Strlen(s))
}

func GoStringN(s unsafe.Pointer, n int) string {
	return string(GoSliceNTemp[byte](s, n))
}

func CSlice[S ~[]E, E any](s S) unsafe.Pointer {
	return unsafe.Pointer(&s[0])
}

func CString(s string) unsafe.Pointer {
	buf := make([]byte, len(s)+1)
	copy(buf, s)
	return CSlice(buf)
}

func CBool(b bool) int8 {
	if b {
		return 1
	}
	return 0
}
