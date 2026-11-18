package d2d1

import (
	"testing"
	"unsafe"
)

func TestD2D11ABILayouts(t *testing.T) {
	if got, want := unsafe.Sizeof(BitmapProperties1{}), uintptr(32); got != want {
		t.Fatalf("BitmapProperties1 size = %d, want %d", got, want)
	}
	if got, want := unsafe.Sizeof(DeviceContext{}), unsafe.Sizeof(uintptr(0)); got != want {
		t.Fatalf("DeviceContext interface size = %d, want %d", got, want)
	}
	if got, want := unsafe.Sizeof(CommandList{}), unsafe.Sizeof(uintptr(0)); got != want {
		t.Fatalf("CommandList interface size = %d, want %d", got, want)
	}
}
