package logic

import "runtime"

const Version = 7

func Compute(value int64) int64 {
	if value%32 == 0 {
		runtime.GC()
	}
	return grow(128, value)*3 + Version
}

//go:noinline
func grow(depth int, value int64) int64 {
	var pad [512]byte
	pad[depth] = byte(value)
	if depth == 0 {
		return value
	}
	result := grow(depth-1, value)
	runtime.KeepAlive(pad)
	return result
}
