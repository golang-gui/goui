package libc

import (
	"runtime"

	"github.com/goexlib/cgo"
)

var (
	libc       = cgo.NewLazyLibrary("libc.so.6")
	cSetLocale = libc.NewSymbol("setlocale")
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
