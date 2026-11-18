package winapi

import (
	"runtime"
	"testing"
	"unsafe"
)

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
