package glx

import (
	"runtime"
	"testing"

	"github.com/goexlib/cgo"
)

// 仅替换扩展函数地址，以原生回调模拟 C int 返回值，不打开 Display 或创建 GLX context。
// 返回寄存器高位刻意清零：不能依赖负 C int 的上半部恰好已经符号扩展。
func TestSwapIntervalSignedResult(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	sgi, mesa := glXSwapIntervalSGI, glXSwapIntervalMESA
	defer func() { glXSwapIntervalSGI, glXSwapIntervalMESA = sgi, mesa }()
	var argument, result uintptr
	fn := cgo.Symbol(cgo.NewCallback(func(interval uintptr) uintptr {
		argument = interval
		return result
	}))
	glXSwapIntervalSGI, glXSwapIntervalMESA = fn, fn
	for _, tc := range []struct {
		name string
		call func(int) int
	}{
		{"SGI", SwapIntervalSGI},
		{"MESA", SwapIntervalMESA},
	} {
		for _, want := range []int32{0, 7, -1, -2147483648} {
			result = uintptr(uint32(want))
			if got := tc.call(2); got != int(want) || argument != 2 {
				t.Fatalf("%s: argument=%d result=%d, want argument=2 result=%d", tc.name, argument, got, want)
			}
		}
	}
	glXSwapIntervalSGI, glXSwapIntervalMESA = 0, 0
	if SwapIntervalSGI(1) != 0 || SwapIntervalMESA(1) != 0 {
		t.Fatal("unavailable optional swap-control extension changed its fallback")
	}
}
