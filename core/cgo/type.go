package cgo

import "unsafe"

type (
	Char      = int8
	Uchar     = uint8
	Short     = int16
	Ushort    = uint16
	Int       = int32
	Uint      = uint32
	Long      = int32
	Ulong     = uint32
	LongLong  = int64
	UlongLong = uint64
	Ssize     = int
	Sizet     = uint
	Intptr    = int
	Uintptr   = uintptr
	Pointer   = unsafe.Pointer
)
