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

func TestFileDialogCancellationHRESULT(t *testing.T) {
	// HRESULT_FROM_WIN32(ERROR_CANCELLED): severity=1, FACILITY_WIN32=7,
	// ERROR_CANCELLED=1223. This must be distinguished from system failures.
	hr := HRESULT_CANCEL
	if uint32(hr) != uint32(0x800704C7) || hr.Code() != 1223 || hr.Facility() != 7 || !hr.Failed() {
		t.Fatalf("incorrect cancellation HRESULT: %x", int32(HRESULT_CANCEL))
	}
}
