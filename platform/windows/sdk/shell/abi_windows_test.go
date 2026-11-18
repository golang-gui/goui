package shell

import (
	"testing"
	"unsafe"
)

func TestShellExecuteInfoLayout(t *testing.T) {
	var info SHELLEXECUTEINFO
	size, file, show, instance, process := uintptr(112), uintptr(24), uintptr(48), uintptr(56), uintptr(104)
	if unsafe.Sizeof(uintptr(0)) == 4 {
		size, file, show, instance, process = 60, 16, 28, 32, 56
	}
	if unsafe.Sizeof(info) != size || unsafe.Offsetof(info.File) != file ||
		unsafe.Offsetof(info.Show) != show || unsafe.Offsetof(info.Instance) != instance ||
		unsafe.Offsetof(info.Process) != process {
		t.Fatalf("SHELLEXECUTEINFOW ABI mismatch: %+v", info)
	}
}
