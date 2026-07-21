package winapi

import (
	"syscall"
	"unsafe"
)

// HIMC is an input-method context handle.
type HIMC = syscall.Handle

// VK_PROCESSKEY is reported as the WM_KEYDOWN virtual key while the IME is
// processing a key (composition/candidate navigation); such keys must not be
// delivered as ordinary key events.
const VK_PROCESSKEY = 0xE5

// ImmGetCompositionStringW index flags.
const (
	GCS_COMPSTR   = 0x0008
	GCS_CURSORPOS = 0x0080
	GCS_RESULTSTR = 0x0800
)

// COMPOSITIONFORM / CANDIDATEFORM dwStyle flags.
const (
	CFS_POINT          = 0x0002
	CFS_FORCE_POSITION = 0x0020
	CFS_CANDIDATEPOS   = 0x0040
	CFS_EXCLUDE        = 0x0080
)

// ImmNotifyIME actions / composition-string values.
const (
	NI_COMPOSITIONSTR = 0x0015
	CPS_COMPLETE      = 0x0001
	CPS_CANCEL        = 0x0004
)

type COMPOSITIONFORM struct {
	DwStyle      DWORD
	PtCurrentPos POINT
	RcArea       RECT
}

type CANDIDATEFORM struct {
	DwIndex      DWORD
	DwStyle      DWORD
	PtCurrentPos POINT
	RcArea       RECT
}

var (
	imm32Dll = syscall.NewLazyDLL("imm32.dll")

	procImmGetContext            = imm32Dll.NewProc("ImmGetContext")
	procImmReleaseContext        = imm32Dll.NewProc("ImmReleaseContext")
	procImmGetCompositionStringW = imm32Dll.NewProc("ImmGetCompositionStringW")
	procImmSetCompositionWindow  = imm32Dll.NewProc("ImmSetCompositionWindow")
	procImmSetCandidateWindow    = imm32Dll.NewProc("ImmSetCandidateWindow")
	procImmNotifyIME             = imm32Dll.NewProc("ImmNotifyIME")
	procImmAssociateContext      = imm32Dll.NewProc("ImmAssociateContext")
)

func ImmGetContext(hwnd HWND) HIMC {
	ret, _, _ := syscall.SyscallN(procImmGetContext.Addr(), uintptr(hwnd))
	return HIMC(ret)
}

func ImmReleaseContext(hwnd HWND, himc HIMC) {
	syscall.SyscallN(procImmReleaseContext.Addr(), uintptr(hwnd), uintptr(himc))
}

// ImmGetCompositionString reads a composition component (GCS_COMPSTR or
// GCS_RESULTSTR) as UTF-16 and returns it decoded to a Go string.
func ImmGetCompositionString(himc HIMC, index DWORD) string {
	n, _, _ := syscall.SyscallN(procImmGetCompositionStringW.Addr(), uintptr(himc), uintptr(index), 0, 0)
	size := int(int32(n)) // byte length
	if size <= 0 {
		return ""
	}
	buf := make([]uint16, size/2)
	syscall.SyscallN(procImmGetCompositionStringW.Addr(), uintptr(himc), uintptr(index),
		uintptr(unsafe.Pointer(&buf[0])), uintptr(size))
	return syscall.UTF16ToString(buf)
}

// ImmGetCompositionCursorPos returns the caret position within the composition
// string, in UTF-16 code units.
func ImmGetCompositionCursorPos(himc HIMC) int {
	ret, _, _ := syscall.SyscallN(procImmGetCompositionStringW.Addr(), uintptr(himc), uintptr(GCS_CURSORPOS), 0, 0)
	return int(int32(ret))
}

func ImmSetCompositionWindow(himc HIMC, form *COMPOSITIONFORM) {
	syscall.SyscallN(procImmSetCompositionWindow.Addr(), uintptr(himc), uintptr(unsafe.Pointer(form)))
}

func ImmSetCandidateWindow(himc HIMC, form *CANDIDATEFORM) {
	syscall.SyscallN(procImmSetCandidateWindow.Addr(), uintptr(himc), uintptr(unsafe.Pointer(form)))
}

func ImmNotifyIME(himc HIMC, action, index, value DWORD) {
	syscall.SyscallN(procImmNotifyIME.Addr(), uintptr(himc), uintptr(action), uintptr(index), uintptr(value))
}

// ImmAssociateContext associates himc with hwnd and returns the previously
// associated context. Pass 0 to disable IME for the window (save the returned
// handle to restore later).
func ImmAssociateContext(hwnd HWND, himc HIMC) HIMC {
	ret, _, _ := syscall.SyscallN(procImmAssociateContext.Addr(), uintptr(hwnd), uintptr(himc))
	return HIMC(ret)
}
