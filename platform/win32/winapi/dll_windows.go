package winapi

import "syscall"

var (
	user32Dll   = syscall.NewLazyDLL("user32.dll")
	kernel32Dll = syscall.NewLazyDLL("kernel32.dll")
	gdi32Dll    = syscall.NewLazyDLL("gdi32.dll")

	//Kernel
	procGetModuleHandleW = kernel32Dll.NewProc("GetModuleHandleW")

	//Window
	procRegisterClassExW     = user32Dll.NewProc("RegisterClassExW")
	procCreateWindowExW      = user32Dll.NewProc("CreateWindowExW")
	procDestroyWindow        = user32Dll.NewProc("DestroyWindow")
	procEnableWindow         = user32Dll.NewProc("EnableWindow")
	procShowWindow           = user32Dll.NewProc("ShowWindow")
	procUpdateWindow         = user32Dll.NewProc("UpdateWindow")
	procSetParent            = user32Dll.NewProc("SetParent")
	procGetWindowRect        = user32Dll.NewProc("GetWindowRect")
	procSetWindowPos         = user32Dll.NewProc("SetWindowPos")
	procGetWindowTextW       = user32Dll.NewProc("GetWindowTextW")
	procGetWindowTextLengthW = user32Dll.NewProc("GetWindowTextLengthW")
	procSetWindowTextW       = user32Dll.NewProc("SetWindowTextW")
	procBringWindowToTop     = user32Dll.NewProc("BringWindowToTop")
	procGetClientRect        = user32Dll.NewProc("GetClientRect")
	procInvalidateRect       = user32Dll.NewProc("InvalidateRect")
	procFlashWindowEx        = user32Dll.NewProc("FlashWindowEx")
	procCloseWindow          = user32Dll.NewProc("CloseWindow")

	// DPI
	procGetDpiForWindow               = user32Dll.NewProc("GetDpiForWindow")
	procSetProcessDpiAwarenessContext = user32Dll.NewProc("SetProcessDpiAwarenessContext")

	//Message
	procGetMessageW      = user32Dll.NewProc("GetMessageW")
	procWaitMessage      = user32Dll.NewProc("WaitMessage")
	procPeekMessageW     = user32Dll.NewProc("PeekMessageW")
	procTranslateMessage = user32Dll.NewProc("TranslateMessage")
	procDispatchMessageW = user32Dll.NewProc("DispatchMessageW")
	procPostMessageW     = user32Dll.NewProc("PostMessageW")
	procPostQuitMessage  = user32Dll.NewProc("PostQuitMessage")
	procDefWindowProcW   = user32Dll.NewProc("DefWindowProcW")

	procBeginPaint = user32Dll.NewProc("BeginPaint")
	procEndPaint   = user32Dll.NewProc("EndPaint")

	procGetDC     = user32Dll.NewProc("GetDC")
	procReleaseDC = user32Dll.NewProc("ReleaseDC")

	// Resource
	procLoadCursorW = user32Dll.NewProc("LoadCursorW")

	//GDI
	procCreateCompatibleDC = gdi32Dll.NewProc("CreateCompatibleDC")
	procDeleteDC           = gdi32Dll.NewProc("DeleteDC")

	procCreateCompatibleBitmap = gdi32Dll.NewProc("CreateCompatibleBitmap")
	procSelectObject           = gdi32Dll.NewProc("SelectObject")
	procDeleteObject           = gdi32Dll.NewProc("DeleteObject")

	procCreateDIBitmap = gdi32Dll.NewProc("CreateDIBitmap")
	procSetDIBits      = gdi32Dll.NewProc("SetDIBits")

	procBitBlt     = gdi32Dll.NewProc("BitBlt")
	procStretchBlt = gdi32Dll.NewProc("StretchBlt")
)
