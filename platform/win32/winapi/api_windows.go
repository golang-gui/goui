package winapi

import (
	"syscall"
	"unsafe"
)

type WindowProcFunc func(wnd HWND, message UINT, wParam WPARAM, lParam LPARAM) LRESULT

func MakeWindowProc(wndProcFn WindowProcFunc) WNDPROC {
	return syscall.NewCallback(wndProcFn)
}

func GetModuleHandle(name LPCWSTR) (HMODULE, error) {
	ret, _, err := syscall.SyscallN(procGetModuleHandleW.Addr(), uintptr(unsafe.Pointer(name)))
	if ret == 0 {
		return 0, err
	}
	return HMODULE(ret), nil
}

func RegisterClassEx(cls *WNDCLASSEX) (ATOM, error) {
	ret, _, err := syscall.SyscallN(procRegisterClassExW.Addr(), uintptr(unsafe.Pointer(cls)))
	if ret == 0 {
		return 0, err
	}
	return ATOM(ret), nil
}

func CreateWindowEx(exStyle DWORD, clsName, wndName LPCWSTR, style DWORD, x, y, w, h int, parent HWND, menu HMENU, inst HINSTANCE, param LPVOID) (HWND, error) {
	ret, _, err := syscall.SyscallN(procCreateWindowExW.Addr(),
		uintptr(exStyle),
		uintptr(unsafe.Pointer(clsName)),
		uintptr(unsafe.Pointer(wndName)),
		uintptr(style),
		uintptr(x), uintptr(y), uintptr(w), uintptr(h),
		uintptr(parent), uintptr(menu), uintptr(inst), uintptr(param))
	if ret == 0 || HWND(ret) == syscall.InvalidHandle {
		return 0, err
	}

	return HWND(ret), nil
}

func DestroyWindow(wnd HWND) BOOL {
	ret, _, _ := syscall.SyscallN(procDestroyWindow.Addr(), uintptr(wnd))
	return BOOL(ret)
}

func CloseWindow(wnd HWND) BOOL {
	ret, _, _ := syscall.SyscallN(procCloseWindow.Addr(), uintptr(wnd))
	return BOOL(ret)
}

func SetParent(wnd, parent HWND) (HWND, error) {
	ret, _, err := syscall.SyscallN(procSetParent.Addr(), uintptr(wnd), uintptr(parent))
	if ret == 0 {
		return 0, err
	}
	return HWND(ret), nil
}

func ShowWindow(wnd HWND, cmd int) BOOL {
	ret, _, _ := syscall.SyscallN(procShowWindow.Addr(), uintptr(wnd), uintptr(cmd))
	return BOOL(ret)
}

func UpdateWindow(wnd HWND) BOOL {
	ret, _, _ := syscall.SyscallN(procUpdateWindow.Addr(), uintptr(wnd))
	return BOOL(ret)
}

func EnableWindow(wnd HWND, enable BOOL) BOOL {
	ret, _, _ := syscall.SyscallN(procEnableWindow.Addr(), uintptr(wnd), uintptr(enable))
	return BOOL(ret)
}

func GetWindowRect(wnd HWND, rect LPRECT) error {
	ret, _, err := syscall.SyscallN(procGetWindowRect.Addr(), uintptr(wnd), uintptr(unsafe.Pointer(rect)))
	if ret == FALSE {
		return err
	}
	return nil
}

func SetWindowPos(wnd, insertAfter HWND, x, y, cx, cy int, flags UINT) error {
	ret, _, err := syscall.SyscallN(procSetWindowPos.Addr(),
		uintptr(wnd), uintptr(insertAfter),
		uintptr(x), uintptr(y), uintptr(cx), uintptr(cy), uintptr(flags))
	if ret == FALSE {
		return err
	}

	return nil
}

func SetWindowText(wnd HWND, text LPCWSTR) error {
	ret, _, err := syscall.SyscallN(procSetWindowTextW.Addr(), uintptr(wnd), uintptr(unsafe.Pointer(text)))
	if ret == FALSE {
		return err
	}
	return nil
}

func GetWindowText(wnd HWND, text LPWSTR, maxLen INT) (INT, error) {
	ret, _, err := syscall.SyscallN(procGetWindowTextW.Addr(), uintptr(wnd), uintptr(unsafe.Pointer(text)), uintptr(maxLen))
	if ret == 0 {
		return 0, err
	}
	return INT(ret), nil
}

func GetWindowTextLength(wnd HWND) (INT, error) {
	ret, _, err := syscall.SyscallN(procGetWindowTextLengthW.Addr(), uintptr(wnd))
	if err != 0 {
		return 0, err
	}
	return INT(ret), nil
}

func BringWindowToTop(wnd HWND) (BOOL, error) {
	ret, _, err := syscall.SyscallN(procBringWindowToTop.Addr(), uintptr(wnd))
	if ret == FALSE {
		return FALSE, err
	}
	return BOOL(ret), nil
}

func GetClientRect(wnd HWND, rect LPRECT) error {
	ret, _, err := syscall.SyscallN(procGetClientRect.Addr(), uintptr(wnd), uintptr(unsafe.Pointer(rect)))
	if ret == FALSE {
		return err
	}
	return nil
}

func InvalidateRect(wnd HWND, rect LPRECT, erase BOOL) error {
	ret, _, err := syscall.SyscallN(procInvalidateRect.Addr(), uintptr(wnd), uintptr(unsafe.Pointer(rect)), uintptr(erase))
	if ret == FALSE {
		return err
	}
	return nil
}

func FlashWindowEx(pfwi PFLASHWINFO) BOOL {
	ret, _, _ := syscall.SyscallN(procFlashWindowEx.Addr(), uintptr(unsafe.Pointer(pfwi)))
	return BOOL(ret)
}

func BeginPaint(wnd HWND, paint LPPAINTSTRUCT) HDC {
	ret, _, _ := syscall.SyscallN(procBeginPaint.Addr(), uintptr(wnd), uintptr(unsafe.Pointer(paint)))
	return HDC(ret)
}

func EndPaint(wnd HWND, paint LPPAINTSTRUCT) BOOL {
	ret, _, _ := syscall.SyscallN(procEndPaint.Addr(), uintptr(wnd), uintptr(unsafe.Pointer(paint)))
	return BOOL(ret)
}

func LoadCursor(inst HINSTANCE, name LPCWSTR) (HCURSOR, error) {
	ret, _, err := syscall.SyscallN(procLoadCursorW.Addr(), uintptr(inst), uintptr(unsafe.Pointer(name)))
	if Cintptr(ret) == 0 {
		return FALSE, err
	}
	return HCURSOR(ret), nil
}

func GetMessage(msg LPMSG, wnd HWND, filterMin, filterMax UINT) (BOOL, error) {
	ret, _, err := syscall.SyscallN(procGetMessageW.Addr(),
		uintptr(unsafe.Pointer(msg)),
		uintptr(wnd),
		uintptr(filterMin), uintptr(filterMax))
	if Cintptr(ret) == 0 {
		return FALSE, err
	}

	return BOOL(ret), nil
}

func WaitMessage() (BOOL, error) {
	ret, _, err := syscall.SyscallN(procWaitMessage.Addr())
	if Cintptr(ret) == 0 {
		return FALSE, err
	}
	return BOOL(ret), nil
}

func PeekMessage(msg LPMSG, wnd HWND, filterMin, filterMax, removeMsg UINT) (BOOL, error) {
	ret, _, err := syscall.SyscallN(procPeekMessageW.Addr(),
		uintptr(unsafe.Pointer(msg)),
		uintptr(wnd),
		uintptr(filterMin), uintptr(filterMax), uintptr(removeMsg))
	if Cintptr(ret) == 0 {
		return FALSE, err
	}

	return BOOL(ret), nil
}

func TranslateMessage(msg LPMSG) BOOL {
	ret, _, _ := syscall.SyscallN(procTranslateMessage.Addr(), uintptr(unsafe.Pointer(msg)))
	return BOOL(ret)
}

func DispatchMessage(msg LPMSG) LRESULT {
	ret, _, _ := syscall.SyscallN(procDispatchMessageW.Addr(), uintptr(unsafe.Pointer(msg)))
	return LRESULT(ret)
}

func PostMessage(wnd HWND, msg UINT, wParam WPARAM, lParam LPARAM) error {
	ret, _, err := syscall.SyscallN(procPostMessageW.Addr(), uintptr(wnd), uintptr(msg), uintptr(wParam), uintptr(lParam))
	if ret == FALSE {
		return err
	}
	return nil
}

func PostQuitMessage(code int) {
	_, _, _ = syscall.SyscallN(procPostQuitMessage.Addr(), uintptr(code))
}

func DefWindowProc(wnd HWND, message UINT, wParam WPARAM, lParam LPARAM) LRESULT {
	ret, _, _ := syscall.SyscallN(procDefWindowProcW.Addr(), uintptr(wnd), uintptr(message), uintptr(wParam), uintptr(lParam))
	return LRESULT(ret)
}

func GetDC(hwnd HWND) (HDC, error) {
	ret, _, err := syscall.SyscallN(procGetDC.Addr(), uintptr(hwnd))
	if ret == 0 {
		return 0, err
	}
	return HDC(ret), nil
}

func ReleaseDC(hdc HDC) error {
	ret, _, err := syscall.SyscallN(procReleaseDC.Addr(), uintptr(hdc))
	if ret == 0 {
		return err
	}
	return nil
}

func CreateCompatibleDC(hdc HDC) HDC {
	ret, _, _ := syscall.SyscallN(procCreateCompatibleDC.Addr(), uintptr(hdc))
	return HDC(ret)
}

func DeleteDC(hdc HDC) BOOL {
	ret, _, _ := syscall.SyscallN(procDeleteDC.Addr(), uintptr(hdc))
	return BOOL(ret)
}

func CreateCompatibleBitmap(hdc HDC, w, h INT) HBITMAP {
	ret, _, _ := syscall.SyscallN(procCreateCompatibleBitmap.Addr(), uintptr(hdc), uintptr(w), uintptr(h))
	return HBITMAP(ret)
}

func SetDIBits(hdc HDC, bitmap HBITMAP, start, lines UINT, bits LPVOID, bmi *BITMAPINFO, colorUse UINT) (INT, error) {
	ret, _, err := syscall.SyscallN(procSetDIBits.Addr(), uintptr(hdc), uintptr(bitmap), uintptr(start), uintptr(lines), uintptr(bits), uintptr(unsafe.Pointer(bmi)), uintptr(colorUse))
	if ret == 0 {
		return 0, err
	}
	return INT(ret), nil
}

func SelectObject(hdc HDC, obj HGDIOBJ) HGDIOBJ {
	ret, _, _ := syscall.SyscallN(procSelectObject.Addr(), uintptr(hdc), uintptr(obj))
	return HGDIOBJ(ret)
}

func DeleteObject(obj HGDIOBJ) BOOL {
	ret, _, _ := syscall.SyscallN(procDeleteObject.Addr(), uintptr(obj))
	return BOOL(ret)
}

func BitBlt(hdc HDC, x, y, cx, cy INT, src HDC, x1, y1 INT, op DWORD) error {
	ret, _, err := syscall.SyscallN(procBitBlt.Addr(),
		uintptr(hdc),
		uintptr(x), uintptr(y), uintptr(cx), uintptr(cy),
		uintptr(src),
		uintptr(x1), uintptr(y1), uintptr(op))

	if ret == FALSE {
		return err
	}

	return nil
}
