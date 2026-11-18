package xrandr

import (
	"testing"
	"unsafe"
)

func TestMonitorInfoABI(t *testing.T) {
	if unsafe.Sizeof(uintptr(0)) != 8 {
		t.Skip("64-bit ABI")
	}
	var m MonitorInfo
	if unsafe.Sizeof(m) != 56 || unsafe.Offsetof(m.NOutput) != 16 || unsafe.Offsetof(m.X) != 20 || unsafe.Offsetof(m.Outputs) != 48 {
		t.Fatalf("invalid XRRMonitorInfo layout: size=%d outputs=%d", unsafe.Sizeof(m), unsafe.Offsetof(m.Outputs))
	}
}
