package libc

import "github.com/goexlib/cgo"

var (
	libc       = cgo.NewLazyLibrary("libc.so.6")
	cSetLocale = libc.NewSymbol("setlocale")
)

const LC_CTYPE = 0

// emptyCString is a real (non-NULL) empty C string. setlocale(cat, "") sets the
// locale from the environment, but setlocale(cat, NULL) only queries it; since
// cgo.CString("") returns NULL, pass this instead for the "" case.
var emptyCString = []byte{0}

func SetLocale(lctype int, locale string) {
	if locale == "" {
		cSetLocale.CallRaw(uintptr(lctype), uintptr(cgo.CSlice(emptyCString)))
	} else {
		cSetLocale.CallRaw(uintptr(lctype), uintptr(cgo.CString(locale)))
	}
}
