// Package dispatch binds libdispatch's C entry points; callers own callback state.
package dispatch

import "github.com/goexlib/cgo"

var library = cgo.NewLazyLibrary("/usr/lib/libSystem.B.dylib")
var async = library.NewSymbol("dispatch_async_f")
var mainQueue = library.NewSymbol("_dispatch_main_q")
var mainThread = library.NewSymbol("pthread_main_np")

func MainAsync(callback uintptr) { cgo.Call(async.Addr(), mainQueue.Addr(), uintptr(0), callback) }
func IsMainThread() bool         { return cgo.CallRet[int32](mainThread.Addr()) != 0 }
