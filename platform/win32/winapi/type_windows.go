package winapi

import (
	"syscall"
	"unsafe"
)

// C types
type (
	Cshort   = int16
	Cushort  = uint16
	Cint     = int32
	Cuint    = uint32
	Clong    = int32
	Culong   = uint32
	Cintptr  = int64
	Cuintptr = uint64
	Cvoidptr = unsafe.Pointer
)

// Windows API Basics
type (
	BYTE     = byte
	BOOL     = Cint
	ATOM     = Cushort
	INT      = Cint
	UINT     = Cuint
	LONG     = Clong
	LONG_PTR = Cintptr
	UINT_PTR = Cuintptr
	WORD     = Cushort
	DWORD    = Culong
	LRESULT  = LONG_PTR
	WCHAR    = uint16
	LPCWSTR  = *uint16
	LPWSTR   = *uint16
	LPVOID   = Cvoidptr
)

const (
	FALSE = 0
	TRUE  = 1
)

// Windows API Objects
type (
	WPARAM = UINT_PTR
	LPARAM = LONG_PTR

	WNDPROC   = uintptr
	HINSTANCE = syscall.Handle
	HMODULE   = syscall.Handle
	HWND      = syscall.Handle
	HMENU     = syscall.Handle
	HGDIOBJ   = syscall.Handle
	HDC       = syscall.Handle
	HBITMAP   = syscall.Handle
	HICON     = syscall.Handle
	HCURSOR   = syscall.Handle
	HBRUSH    = syscall.Handle
)

//Windows API Structs

type POINT struct {
	X, Y LONG
}

type RECT struct {
	Left, Top, Right, Bottom LONG
}
type LPRECT = *RECT

type MSG struct {
	Hwnd    HWND
	Message UINT
	WParam  WPARAM
	LParam  LPARAM
	Time    DWORD
	Pt      POINT
}
type LPMSG = *MSG

type PAINTSTRUCT struct {
	Hdc       HDC
	Erase     BOOL
	Paint     RECT
	Restore   BOOL
	IncUpdate BOOL
	reserved  [32]BYTE
}
type LPPAINTSTRUCT = *PAINTSTRUCT

type WNDCLASSEX struct {
	Size       UINT
	Style      UINT
	WndProc    WNDPROC
	ClsExtra   Cint
	WndExtra   Cint
	Instance   HINSTANCE
	Icon       HICON
	Cursor     HCURSOR
	Background HBRUSH
	MenuName   LPCWSTR
	ClassName  LPCWSTR
	IconSm     HICON
}

const Sizeof_WNDCLASSEX = 80

type CREATESTRUCT struct {
	CreateParams LPVOID
	Instance     HINSTANCE
	Menu         HMENU
	HwndParent   HWND
	CY           Cint
	CX           Cint
	Style        LONG
	Name         LPCWSTR
	Class        LPWSTR
	ExStyle      DWORD
}
type LPCREATESTRUCT = *CREATESTRUCT

type FLASHWINFO struct {
	Size    UINT
	Hwnd    HWND
	Flags   DWORD
	Count   UINT
	Timeout DWORD
}
type PFLASHWINFO = *FLASHWINFO

type BITMAPINFOHEADER struct {
	Size          DWORD
	Width         LONG
	Height        LONG
	Planes        WORD
	BitCount      WORD
	Compression   DWORD
	SizeImage     DWORD
	XPelsPerMeter LONG
	YPelsPerMeter LONG
	ClrUsed       DWORD
	ClrImportant  DWORD
}

const Sizeof_BITMAPINFOHEADER = 40

type PBITMAPINFOHEADER = *BITMAPINFOHEADER
type LPBITMAPINFOHEADER = *BITMAPINFOHEADER

type BITMAPINFO struct {
	Header BITMAPINFOHEADER
	Colors RGBQUAD
}

type RGBQUAD struct {
	Blue     BYTE
	Green    BYTE
	Red      BYTE
	Reserved BYTE
}
