package bridge

import "C"

import "example.com/baseprobe/contract"

var handler func(int64) int64

var _ = contract.ABI

func Register(fn func(int64) int64) { handler = fn }

//export goui_base_probe
func goui_base_probe(value C.longlong) C.longlong { return C.longlong(handler(int64(value))) }
