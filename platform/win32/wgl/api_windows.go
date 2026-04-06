package wgl

import (
	"runtime"
	"syscall"
	"unsafe"

	"github.com/goexlib/cgo"
	"github.com/golang-gui/goui/platform/win32/winapi"
)

var (
	opengl32 = syscall.NewLazyDLL("opengl32.dll")

	// WGL static
	wglCreateContext     = opengl32.NewProc("wglCreateContext")
	wglDeleteContext     = opengl32.NewProc("wglDeleteContext")
	wglGetProcAddress    = opengl32.NewProc("wglGetProcAddress")
	wglGetCurrentDC      = opengl32.NewProc("wglGetCurrentDC")
	wglGetCurrentContext = opengl32.NewProc("wglGetCurrentContext")
	wglMakeCurrent       = opengl32.NewProc("wglMakeCurrent")
	wglShareLists        = opengl32.NewProc("wglShareLists")

	// WGL dynamic
	wglGetExtensionsStringEXT    uintptr
	wglGetExtensionsStringARB    uintptr
	wglCreateContextAttribsARB   uintptr
	wglSwapIntervalEXT           uintptr
	wglGetPixelFormatAttribivARB uintptr
)

func Init(dc winapi.HDC) (err error) {
	err = opengl32.Load()
	if err != nil {
		return
	}
	err = wglCreateContext.Find()
	if err != nil {
		return
	}
	err = wglDeleteContext.Find()
	if err != nil {
		return
	}
	err = wglGetProcAddress.Find()
	if err != nil {
		return
	}
	err = wglGetCurrentDC.Find()
	if err != nil {
		return
	}
	err = wglGetCurrentContext.Find()
	if err != nil {
		return
	}
	err = wglMakeCurrent.Find()
	if err != nil {
		return
	}
	err = wglShareLists.Find()
	if err != nil {
		return
	}

	var pfd winapi.PIXELFORMATDESCRIPTOR
	pfd.Size = winapi.Sizeof_PIXELFORMATDESCRIPTOR
	pfd.Version = 1
	pfd.Flags = winapi.PFD_DRAW_TO_WINDOW | winapi.PFD_SUPPORT_OPENGL | winapi.PFD_DOUBLEBUFFER
	pfd.PixelType = winapi.PFD_TYPE_RGBA
	pfd.ColorBits = 24

	pixFormat, err := winapi.ChoosePixelFormat(dc, &pfd)
	if err != nil {
		return
	}

	err = winapi.SetPixelFormat(dc, pixFormat, &pfd)
	if err != nil {
		return
	}

	rc, err := CreateContext(dc)
	if err != nil {
		return
	}
	defer DeleteContext(rc)

	pdc := GetCurrentDC()
	prc := GetCurrentContext()

	err = MakeCurrent(dc, rc)
	if err != nil {
		return
	}
	defer MakeCurrent(pdc, prc)

	wglGetExtensionsStringEXT, _ = GetProcAddress("wglGetExtensionsStringEXT")
	if wglGetExtensionsStringEXT != 0 {
		GetExtensionsStringEXT = func() (string, error) {
			ret, _, err := syscall.SyscallN(wglGetExtensionsStringEXT)
			if ret == 0 {
				return "", err
			}
			return cgo.GoString(unsafe.Pointer(ret)), nil
		}
	}

	wglGetExtensionsStringARB, _ = GetProcAddress("wglGetExtensionsStringARB")
	if wglGetExtensionsStringARB != 0 {
		GetExtensionsStringARB = func(dc winapi.HDC) (string, error) {
			ret, _, err := syscall.SyscallN(wglGetExtensionsStringARB, uintptr(dc))
			if ret == 0 {
				return "", err
			}
			return cgo.GoString(unsafe.Pointer(ret)), nil
		}
	}

	wglCreateContextAttribsARB, _ = GetProcAddress("wglCreateContextAttribsARB")
	if wglCreateContextAttribsARB != 0 {
		CreateContextAttribsARB = func(hdc winapi.HDC, share HGLRC, attrs []int32) (HGLRC, error) {
			ret, _, err := syscall.SyscallN(wglCreateContextAttribsARB, uintptr(hdc), uintptr(share), uintptr(unsafe.Pointer(&attrs[0])))
			if ret == 0 {
				return 0, err
			}
			return HGLRC(ret), nil
		}
	}

	wglGetPixelFormatAttribivARB, _ = GetProcAddress("wglGetPixelFormatAttribivARB")
	if wglGetPixelFormatAttribivARB != 0 {
		GetPixelFormatAttribivARB = func(hdc winapi.HDC, pixelFormat, layerPlane winapi.INT, attrs, values []int32) error {
			ret, _, err := syscall.SyscallN(wglGetPixelFormatAttribivARB, uintptr(hdc), uintptr(pixelFormat), uintptr(layerPlane), uintptr(len(attrs)), uintptr(unsafe.Pointer(&attrs[0])), uintptr(unsafe.Pointer(&values[0])))
			if ret == 0 {
				return err
			}
			return nil
		}
	}

	wglSwapIntervalEXT, _ = GetProcAddress("wglSwapIntervalEXT")
	if wglSwapIntervalEXT != 0 {
		SwapIntervalEXT = func(vsync int) {
			syscall.SyscallN(wglSwapIntervalEXT, uintptr(vsync))
		}
	}

	return nil
}

// Static functions

type HGLRC = syscall.Handle

func CreateContext(hdc winapi.HDC) (HGLRC, error) {
	ret, _, err := syscall.SyscallN(wglCreateContext.Addr(), uintptr(hdc))
	if ret == 0 {
		return 0, err
	}
	return HGLRC(ret), nil
}

func DeleteContext(rc HGLRC) error {
	ret, _, err := syscall.SyscallN(wglDeleteContext.Addr(), uintptr(rc))
	if ret == 0 {
		return err
	}
	return nil
}

func GetProcAddress(symbol string) (fn uintptr, err error) {
	fn, err = getProcAddress(symbol)
	if err != nil || fn == 0 {
		return getProcAddressOpengl32(symbol)
	}
	return
}

func GetCurrentDC() winapi.HDC {
	ret, _, _ := syscall.SyscallN(wglGetCurrentDC.Addr())
	return winapi.HDC(ret)
}

func GetCurrentContext() HGLRC {
	ret, _, _ := syscall.SyscallN(wglGetCurrentContext.Addr())
	return HGLRC(ret)
}

func MakeCurrent(hdc winapi.HDC, rc HGLRC) error {
	ret, _, err := syscall.SyscallN(wglMakeCurrent.Addr(), uintptr(hdc), uintptr(rc))
	if ret == 0 {
		return err
	}
	return nil
}

func ShareLists(share, rc HGLRC) error {
	ret, _, err := syscall.SyscallN(wglShareLists.Addr(), uintptr(share), uintptr(rc))
	if ret == 0 {
		return err
	}
	return nil
}

func getProcAddress(symbol string) (uintptr, error) {
	s := cgo.CString(symbol)
	ret, _, err := syscall.SyscallN(wglGetProcAddress.Addr(), uintptr(s))
	if ret == 0 {
		return 0, err
	}
	runtime.KeepAlive(s)
	return ret, nil
}

func getProcAddressOpengl32(symbol string) (uintptr, error) {
	proc := opengl32.NewProc(symbol)
	err := proc.Find()
	if err != nil {
		return 0, err
	}
	return proc.Addr(), nil
}

// Dynamic functions

var (
	SwapIntervalEXT           func(vsync int)
	GetExtensionsStringEXT    func() (string, error)
	GetExtensionsStringARB    func(hdc winapi.HDC) (string, error)
	CreateContextAttribsARB   func(hdc winapi.HDC, share HGLRC, attrs []int32) (HGLRC, error)
	GetPixelFormatAttribivARB func(hdc winapi.HDC, pixelFormat, layerPlane winapi.INT, attrs, values []int32) error
)

// Constants

const (
	WGL_NUMBER_PIXEL_FORMATS_ARB                = 0x2000
	WGL_SUPPORT_OPENGL_ARB                      = 0x2010
	WGL_DRAW_TO_WINDOW_ARB                      = 0x2001
	WGL_PIXEL_TYPE_ARB                          = 0x2013
	WGL_TYPE_RGBA_ARB                           = 0x202b
	WGL_ACCELERATION_ARB                        = 0x2003
	WGL_NO_ACCELERATION_ARB                     = 0x2025
	WGL_RED_BITS_ARB                            = 0x2015
	WGL_RED_SHIFT_ARB                           = 0x2016
	WGL_GREEN_BITS_ARB                          = 0x2017
	WGL_GREEN_SHIFT_ARB                         = 0x2018
	WGL_BLUE_BITS_ARB                           = 0x2019
	WGL_BLUE_SHIFT_ARB                          = 0x201a
	WGL_ALPHA_BITS_ARB                          = 0x201b
	WGL_ALPHA_SHIFT_ARB                         = 0x201c
	WGL_ACCUM_BITS_ARB                          = 0x201d
	WGL_ACCUM_RED_BITS_ARB                      = 0x201e
	WGL_ACCUM_GREEN_BITS_ARB                    = 0x201f
	WGL_ACCUM_BLUE_BITS_ARB                     = 0x2020
	WGL_ACCUM_ALPHA_BITS_ARB                    = 0x2021
	WGL_DEPTH_BITS_ARB                          = 0x2022
	WGL_STENCIL_BITS_ARB                        = 0x2023
	WGL_AUX_BUFFERS_ARB                         = 0x2024
	WGL_STEREO_ARB                              = 0x2012
	WGL_DOUBLE_BUFFER_ARB                       = 0x2011
	WGL_SAMPLES_ARB                             = 0x2042
	WGL_FRAMEBUFFER_SRGB_CAPABLE_ARB            = 0x20a9
	WGL_CONTEXT_DEBUG_BIT_ARB                   = 0x00000001
	WGL_CONTEXT_FORWARD_COMPATIBLE_BIT_ARB      = 0x00000002
	WGL_CONTEXT_PROFILE_MASK_ARB                = 0x9126
	WGL_CONTEXT_CORE_PROFILE_BIT_ARB            = 0x00000001
	WGL_CONTEXT_COMPATIBILITY_PROFILE_BIT_ARB   = 0x00000002
	WGL_CONTEXT_MAJOR_VERSION_ARB               = 0x2091
	WGL_CONTEXT_MINOR_VERSION_ARB               = 0x2092
	WGL_CONTEXT_FLAGS_ARB                       = 0x2094
	WGL_CONTEXT_ES2_PROFILE_BIT_EXT             = 0x00000004
	WGL_CONTEXT_ROBUST_ACCESS_BIT_ARB           = 0x00000004
	WGL_LOSE_CONTEXT_ON_RESET_ARB               = 0x8252
	WGL_CONTEXT_RESET_NOTIFICATION_STRATEGY_ARB = 0x8256
	WGL_NO_RESET_NOTIFICATION_ARB               = 0x8261
	WGL_CONTEXT_RELEASE_BEHAVIOR_ARB            = 0x2097
	WGL_CONTEXT_RELEASE_BEHAVIOR_NONE_ARB       = 0
	WGL_CONTEXT_RELEASE_BEHAVIOR_FLUSH_ARB      = 0x2098
	WGL_CONTEXT_OPENGL_NO_ERROR_ARB             = 0x31b3
	WGL_COLORSPACE_EXT                          = 0x309d
	WGL_COLORSPACE_SRGB_EXT                     = 0x3089
)
