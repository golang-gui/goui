package winapi

import (
	"syscall"
	"unsafe"
)

const (
	DWM_BB_ENABLE     = 0x1
	DWM_BB_BLURREGION = 0x2
)

type DWM_BLURBEHIND struct {
	Flags                 DWORD
	Enable                BOOL
	Region                HGDIOBJ
	TransitionOnMaximized BOOL
}

var (
	dwmapiDll = syscall.NewLazyDLL("dwmapi.dll")

	procDwmGetColorizationColor      = dwmapiDll.NewProc("DwmGetColorizationColor")
	procDwmGetWindowAttribute        = dwmapiDll.NewProc("DwmGetWindowAttribute")
	procDwmDefWindowProc             = dwmapiDll.NewProc("DwmDefWindowProc")
	procDwmIsCompositionEnabled      = dwmapiDll.NewProc("DwmIsCompositionEnabled")
	procDwmEnableBlurBehindWindow    = dwmapiDll.NewProc("DwmEnableBlurBehindWindow")
	procDwmExtendFrameIntoClientArea = dwmapiDll.NewProc("DwmExtendFrameIntoClientArea")
)

// DwmDefWindowProc writes result when DWM handles the message. The error reports
// an unavailable native entry point, independently of the handled return value.
func DwmDefWindowProc(wnd HWND, message UINT, wParam WPARAM, lParam LPARAM, result *LRESULT) (BOOL, error) {
	if err := procDwmDefWindowProc.Find(); err != nil {
		return FALSE, err
	}
	ret, _, _ := syscall.SyscallN(procDwmDefWindowProc.Addr(), uintptr(wnd), uintptr(message),
		uintptr(wParam), uintptr(lParam), uintptr(unsafe.Pointer(result)))
	return BOOL(ret), nil
}

func DwmGetWindowAttribute(wnd HWND, attribute DWORD, value unsafe.Pointer, size DWORD) error {
	if err := procDwmGetWindowAttribute.Find(); err != nil {
		return err
	}
	ret, _, _ := syscall.SyscallN(procDwmGetWindowAttribute.Addr(), uintptr(wnd), uintptr(attribute),
		uintptr(value), uintptr(size))
	if int32(ret) < 0 {
		return syscall.Errno(uint32(ret))
	}
	return nil
}

func DwmIsCompositionEnabled(enabled *BOOL) error {
	if err := procDwmIsCompositionEnabled.Find(); err != nil {
		return err
	}
	ret, _, _ := syscall.SyscallN(procDwmIsCompositionEnabled.Addr(), uintptr(unsafe.Pointer(enabled)))
	if int32(ret) < 0 {
		return syscall.Errno(uint32(ret))
	}
	return nil
}

func DwmEnableBlurBehindWindow(wnd HWND, blur *DWM_BLURBEHIND) error {
	if err := procDwmEnableBlurBehindWindow.Find(); err != nil {
		return err
	}
	ret, _, _ := syscall.SyscallN(procDwmEnableBlurBehindWindow.Addr(), uintptr(wnd), uintptr(unsafe.Pointer(blur)))
	if int32(ret) < 0 {
		return syscall.Errno(uint32(ret))
	}
	return nil
}

func DwmExtendFrameIntoClientArea(wnd HWND, margins *MARGINS) error {
	if err := procDwmExtendFrameIntoClientArea.Find(); err != nil {
		return err
	}
	ret, _, _ := syscall.SyscallN(procDwmExtendFrameIntoClientArea.Addr(), uintptr(wnd), uintptr(unsafe.Pointer(margins)))
	if int32(ret) < 0 {
		return syscall.Errno(uint32(ret))
	}
	return nil
}

func DwmGetColorizationColor(colorization *DWORD, opaqueBlend *BOOL) error {
	if err := procDwmGetColorizationColor.Find(); err != nil {
		return err
	}
	ret, _, _ := syscall.SyscallN(
		procDwmGetColorizationColor.Addr(),
		uintptr(unsafe.Pointer(colorization)),
		uintptr(unsafe.Pointer(opaqueBlend)),
	)
	if ret != 0 {
		return syscall.Errno(ret)
	}
	return nil
}
