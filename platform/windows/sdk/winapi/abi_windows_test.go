package winapi

import (
	"runtime"
	"syscall"
	"testing"
	"unsafe"

	"github.com/goexlib/cgo"
)

// 不创建窗口。用独立的整数 ABI 接收端检查 POINT 按值传参，避免两个结构体
// 桥接端犯相同错误时，单纯 round-trip 仍然通过。常量来自 Windows x64 ABI。
func TestPointValueABI(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	var point POINT
	if unsafe.Sizeof(point) != 8 || unsafe.Alignof(point) != 4 || unsafe.Offsetof(point.Y) != 4 {
		t.Fatal("POINT must contain two consecutive signed 32-bit coordinates")
	}
	var gotPoint, gotFlags uintptr
	window := syscall.NewCallback(func(value uintptr) uintptr {
		gotPoint = value
		return 0x1234567887654321
	})
	monitor := syscall.NewCallback(func(value, flags uintptr) uintptr {
		gotPoint, gotFlags = value, flags
		return 0x1234567887654321
	})
	for _, tc := range []struct {
		point POINT
		word  uintptr
	}{
		{POINT{X: 17, Y: 29}, 0x0000001d00000011},
		{POINT{X: -17, Y: 29}, 0x0000001dffffffef},
		{POINT{X: 17, Y: -29}, 0xffffffe300000011},
		{POINT{X: -2147483648, Y: 2147483647}, 0x7fffffff80000000},
	} {
		if result := cgo.CallRet[HWND](window, tc.point); result != HWND(0x1234567887654321) || gotPoint != tc.word {
			t.Fatalf("WindowFromPoint ABI: point=%v word=%#x result=%#x", tc.point, gotPoint, result)
		}
		if result := cgo.CallRet[HMONITOR](monitor, tc.point, DWORD(0x89abcdef)); result != HMONITOR(0x1234567887654321) || gotPoint != tc.word || gotFlags != 0x89abcdef {
			t.Fatalf("MonitorFromPoint ABI: point=%v word=%#x flags=%#x result=%#x", tc.point, gotPoint, gotFlags, result)
		}
	}
}

func TestWindowPlacementABI(t *testing.T) {
	var p WINDOWPLACEMENT
	if unsafe.Sizeof(p) != 44 || unsafe.Offsetof(p.ShowCmd) != 8 || unsafe.Offsetof(p.MinPosition) != 12 || unsafe.Offsetof(p.MaxPosition) != 20 || unsafe.Offsetof(p.NormalPosition) != 28 {
		t.Fatalf("WINDOWPLACEMENT ABI 不符：size=%d show=%d min=%d max=%d normal=%d", unsafe.Sizeof(p), unsafe.Offsetof(p.ShowCmd), unsafe.Offsetof(p.MinPosition), unsafe.Offsetof(p.MaxPosition), unsafe.Offsetof(p.NormalPosition))
	}
}

func TestWindowPlacementReportsInvalidHandle(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	p := WINDOWPLACEMENT{Length: 44}
	if err := GetWindowPlacement(0, &p); err == nil {
		t.Fatal("GetWindowPlacement 必须保留无效 HWND 错误")
	}
	if err := SetWindowPlacement(0, &p); err == nil {
		t.Fatal("SetWindowPlacement 必须保留无效 HWND 错误")
	}
}
