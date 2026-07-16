package xlib

import (
	"runtime"

	"github.com/goexlib/cgo"
)

var (
	xOpenIM             = libx11.NewSymbol("XOpenIM")
	xCloseIM            = libx11.NewSymbol("XCloseIM")
	xCreateIC           = libx11.NewSymbol("XCreateIC")
	xDestroyIC          = libx11.NewSymbol("XDestroyIC")
	xSetICFocus         = libx11.NewSymbol("XSetICFocus")
	xUnsetICFocus       = libx11.NewSymbol("XUnsetICFocus")
	xutf8LookupString   = libx11.NewSymbol("Xutf8LookupString")
	xutf8ResetIC        = libx11.NewSymbol("Xutf8ResetIC")
	xFilterEvent        = libx11.NewSymbol("XFilterEvent")
	xSetLocaleModifiers = libx11.NewSymbol("XSetLocaleModifiers")
	xSetICValues        = libx11.NewSymbol("XSetICValues")
	xVaCreateNestedList = libx11.NewSymbol("XVaCreateNestedList")

	// XCreateIC / XSetICValues option-name C strings, kept alive for the process
	// lifetime.
	xnInputStyle        = cgo.CString("inputStyle")
	xnClientWindow      = cgo.CString("clientWindow")
	xnFocusWindow       = cgo.CString("focusWindow")
	xnPreeditAttributes = cgo.CString("preeditAttributes")
	xnSpotLocation      = cgo.CString("spotLocation")

	// spotBuf backs the XNSpotLocation pointer. The nested list keeps the pointer
	// across XVaCreateNestedList -> XSetICValues, so it must stay put: a
	// package-level var has a stable address and x11 is single-threaded (all IC
	// calls are on the platform thread), so reusing one buffer is safe.
	spotBuf XPoint

	// A real (non-NULL) empty C string. Both setlocale and XSetLocaleModifiers
	// treat "" as "use the environment" but treat NULL as "query, don't set", so
	// we must pass this rather than cgo.CString("") (which returns NULL).
	emptyCString = []byte{0}
)

// SetLocaleModifiers wires the XMODIFIERS-based input-method selection (e.g.
// "@im=fcitx"). Pass "" to read the XMODIFIERS environment variable.
func SetLocaleModifiers(modifiers string) {
	if modifiers == "" {
		// Real empty string reads XMODIFIERS; NULL would only query (see above).
		xSetLocaleModifiers.CallRaw(uintptr(cgo.CSlice(emptyCString)))
		return
	}
	c := cgo.CString(modifiers)
	xSetLocaleModifiers.CallRaw(uintptr(c))
	runtime.KeepAlive(c)
}

// OpenIM opens the display's input method. Returns 0 if no input method is
// available (callers then fall back to plain keysym translation).
func OpenIM(d Display) XIM {
	ret, _, _ := xOpenIM.CallRaw(uintptr(d), 0, 0, 0)
	return XIM(ret)
}

func (im XIM) Close() {
	if im == 0 {
		return
	}
	xCloseIM.CallRaw(uintptr(im))
}

// CreateIC creates a root-style input context bound to window. Returns 0 on
// failure.
func (im XIM) CreateIC(window Window) XIC {
	if im == 0 {
		return 0
	}
	style := uintptr(ximPreeditNothing | ximStatusNothing)
	ret, _, _ := xCreateIC.CallRaw(
		uintptr(im),
		uintptr(xnInputStyle), style,
		uintptr(xnClientWindow), uintptr(window),
		uintptr(xnFocusWindow), uintptr(window),
		0,
	)
	return XIC(ret)
}

func (ic XIC) Destroy() {
	if ic == 0 {
		return
	}
	xDestroyIC.CallRaw(uintptr(ic))
}

func (ic XIC) SetFocus() {
	if ic == 0 {
		return
	}
	xSetICFocus.CallRaw(uintptr(ic))
}

func (ic XIC) UnsetFocus() {
	if ic == 0 {
		return
	}
	xUnsetICFocus.CallRaw(uintptr(ic))
}

// ResetIC clears any in-progress composition, discarding it. The returned string
// (freed here) is ignored; committing on blur is a future refinement.
func (ic XIC) ResetIC() {
	if ic == 0 {
		return
	}
	ret, _, _ := xutf8ResetIC.CallRaw(uintptr(ic))
	if ret != 0 {
		Free((*byte)(cgo.Pointer(ret)))
	}
}

// SetSpot updates the input context's XNSpotLocation so the candidate window
// follows the caret. x/y are pixels relative to the focus window, with y at the
// bottom of the caret line. This is the well-worn root-style + spot approach
// (used by st and others): even under XIMPreeditNothing, fcitx/ibus honor the
// spot for candidate placement, so no font set / over-the-spot style is needed.
func (ic XIC) SetSpot(x, y int16) {
	if ic == 0 {
		return
	}
	spotBuf.X, spotBuf.Y = x, y
	list, _, _ := xVaCreateNestedList.CallRaw(0, uintptr(xnSpotLocation), uintptr(cgo.Pointer(&spotBuf)), 0)
	if list == 0 {
		return
	}
	xSetICValues.CallRaw(uintptr(ic), uintptr(xnPreeditAttributes), list, 0)
	Free((*byte)(cgo.Pointer(list)))
}

// Utf8LookupString feeds a key-press event to the input context and returns the
// committed UTF-8 text (empty if none), the keysym, and the lookup status.
func (ic XIC) Utf8LookupString(event *KeyEvent) (text string, keysym KeySym, status Status) {
	buf := make([]byte, 64)
	for {
		n, _, _ := xutf8LookupString.CallRaw(
			uintptr(ic),
			uintptr(cgo.Pointer(event)),
			uintptr(cgo.Pointer(&buf[0])),
			uintptr(len(buf)),
			uintptr(cgo.Pointer(&keysym)),
			uintptr(cgo.Pointer(&status)),
		)
		count := int(n)
		if status == XBufferOverflow && count > len(buf) {
			buf = make([]byte, count)
			continue
		}
		if count > 0 && count <= len(buf) {
			text = string(buf[:count])
		}
		return
	}
}

// FilterEvent gives the input method a chance to consume an event (composition,
// candidate navigation). Returns true if it did, meaning the caller must drop
// the event. Pass window=0 to use the event's own window.
func FilterEvent(event *Event, window Window) bool {
	ret, _, _ := xFilterEvent.CallRaw(uintptr(cgo.Pointer(event)), uintptr(window))
	return ret != 0
}
