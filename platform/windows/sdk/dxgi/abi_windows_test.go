package dxgi

import (
	"testing"
	"unsafe"
)

func TestSwapChainDesc1ABILayout(t *testing.T) {
	if got, want := unsafe.Sizeof(SwapChainDesc1{}), uintptr(48); got != want {
		t.Fatalf("SwapChainDesc1 size = %d, want %d", got, want)
	}
	if got, want := unsafe.Sizeof(SwapChain1{}), unsafe.Sizeof(uintptr(0)); got != want {
		t.Fatalf("SwapChain1 interface size = %d, want %d", got, want)
	}
}

func TestScalingValues(t *testing.T) {
	if got, want := uint32(DXGI_SCALING_STRETCH), uint32(0); got != want {
		t.Fatalf("DXGI_SCALING_STRETCH = %d, want %d", got, want)
	}
	if got, want := uint32(DXGI_SCALING_NONE), uint32(1); got != want {
		t.Fatalf("DXGI_SCALING_NONE = %d, want %d", got, want)
	}
}
