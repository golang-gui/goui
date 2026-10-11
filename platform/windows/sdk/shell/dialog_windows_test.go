package shell

import "testing"

func TestFileDialogCancellationHRESULT(t *testing.T) {
	// HRESULT_FROM_WIN32(ERROR_CANCELLED): severity=1, FACILITY_WIN32=7,
	// ERROR_CANCELLED=1223. This must be distinguished from system failures.
	hr := HRESULT_CANCEL
	if uint32(hr) != uint32(0x800704C7) || hr.Code() != 1223 || hr.Facility() != 7 || !hr.Failed() {
		t.Fatalf("incorrect cancellation HRESULT: %x", int32(HRESULT_CANCEL))
	}
}
