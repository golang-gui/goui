package libc

import (
	"runtime"
	"unsafe"

	"github.com/goexlib/cgo"
)

var (
	libc       = cgo.NewLazyLibrary("libc.so.6")
	cSetLocale = libc.NewSymbol("setlocale")
	cMbstowcs  = libc.NewSymbol("mbstowcs")
)

const LC_CTYPE = 0

func SetLocale(lctype int, locale string) {
	// A non-NULL empty string selects the environment; NULL only queries.
	// CStringTemp("") returns nil, so explicitly supply the NUL byte.
	if locale == "" {
		locale = "\x00"
	}
	cLocale := cgo.CStringTemp(locale)
	cSetLocale.CallRaw(uintptr(lctype), uintptr(cLocale))
	runtime.KeepAlive(cLocale)
}

// Mbstowcs converts at most len(dst) characters from the current LC_CTYPE
// multibyte encoding into Linux wchar_t values. src must contain that many
// characters or an earlier NUL. The native return value is -1 on invalid input.
func Mbstowcs(dst []rune, src *byte) int {
	if len(dst) == 0 {
		return 0
	}
	var pin runtime.Pinner
	pin.Pin(&dst[0])
	defer pin.Unpin()
	n, _, _ := cMbstowcs.CallRaw(uintptr(unsafe.Pointer(&dst[0])), uintptr(unsafe.Pointer(src)), uintptr(len(dst)))
	runtime.KeepAlive(src)
	return int(n)
}
