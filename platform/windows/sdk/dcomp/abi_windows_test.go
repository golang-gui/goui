package dcomp

import (
	"testing"
	"unsafe"
)

func TestVtableLayout(t *testing.T) {
	slot := unsafe.Sizeof(uintptr(0))
	checks := []struct {
		name   string
		offset uintptr
		index  uintptr
	}{
		{"Device.Commit", unsafe.Offsetof(DeviceClass{}.Commit), 3},
		{"Device.CreateTargetForHwnd", unsafe.Offsetof(DeviceClass{}.CreateTargetForHwnd), 6},
		{"Device.CreateVisual", unsafe.Offsetof(DeviceClass{}.CreateVisual), 7},
		{"Target.SetRoot", unsafe.Offsetof(TargetClass{}.SetRoot), 3},
		{"Visual.SetContent", unsafe.Offsetof(VisualClass{}.SetContent), 15},
	}
	for _, c := range checks {
		if c.offset != c.index*slot {
			t.Errorf("%s offset=%d, want %d", c.name, c.offset, c.index*slot)
		}
	}
}
