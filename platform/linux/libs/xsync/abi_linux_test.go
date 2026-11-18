package xsync

import (
	"runtime"
	"testing"
	"unsafe"

	"github.com/goexlib/cgo"
)

// 用 libXext 原生纯计算函数核对 Value 的内存布局与按值参数传递。
// 不打开 Display、不创建原生 counter；支持 Linux x64/arm64 同一份测试。
func TestValueABI(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	var value Value
	if unsafe.Sizeof(value) != 8 || unsafe.Alignof(value) != 4 || unsafe.Offsetof(value.Lo) != 4 {
		t.Fatal("XSyncValue must contain int32 hi followed by uint32 lo")
	}
	hi := libXext.NewSymbol("XSyncValueHigh32")
	lo := libXext.NewSymbol("XSyncValueLow32")
	for _, value := range []Value{
		{},
		{Hi: 17, Lo: 29},
		{Hi: -17, Lo: 0x89abcdef},
		{Hi: -2147483648, Lo: 0xffffffff},
		{Hi: 2147483647, Lo: 0},
	} {
		gotHi := cgo.CallRet[int32](hi.Addr(), value)
		gotLo := cgo.CallRet[uint32](lo.Addr(), value)
		if gotHi != value.Hi || gotLo != value.Lo {
			t.Fatalf("value=%+v: hi=%d lo=%#x", value, gotHi, gotLo)
		}
	}
}
