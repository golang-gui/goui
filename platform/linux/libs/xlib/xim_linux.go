package xlib

import (
	"runtime"
	"unsafe"

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
	xGetIMValues        = libx11.NewSymbol("XGetIMValues")
	xSetIMValues        = libx11.NewSymbol("XSetIMValues")

	// spotBuf backs the XNSpotLocation pointer. The nested list keeps the pointer
	// across XVaCreateNestedList -> XSetICValues, so it must stay put: a
	// package-level var has a stable address and x11 is single-threaded (all IC
	// calls are on the platform thread), so reusing one buffer is safe.
	spotBuf XPoint
)

// These option names are only borrowed while reading the varargs/nested list.
// Terminated Go constants avoid both per-call copies and process-lifetime C
// allocations. The actual pointers remain local to the calls below.
const (
	xnInputStyle           = "inputStyle\x00"
	xnClientWindow         = "clientWindow\x00"
	xnFocusWindow          = "focusWindow\x00"
	xnPreeditAttributes    = "preeditAttributes\x00"
	xnSpotLocation         = "spotLocation\x00"
	xnQueryInputStyle      = "queryInputStyle\x00"
	xnDestroyCallback      = "destroyCallback\x00"
	xnPreeditStartCallback = "preeditStartCallback\x00"
	xnPreeditDoneCallback  = "preeditDoneCallback\x00"
	xnPreeditDrawCallback  = "preeditDrawCallback\x00"
	xnPreeditCaretCallback = "preeditCaretCallback\x00"
)

// SetLocaleModifiers wires the XMODIFIERS-based input-method selection (e.g.
// "@im=fcitx"). Pass "" to read the XMODIFIERS environment variable.
func SetLocaleModifiers(modifiers string) {
	if modifiers == "" {
		// CStringTemp("") is NULL (query); a real empty string reads XMODIFIERS.
		modifiers = "\x00"
	}
	c := cgo.CStringTemp(modifiers)
	xSetLocaleModifiers.CallRaw(uintptr(c))
	runtime.KeepAlive(c)
}

// OpenIM opens the display's input method. Returns 0 if no input method is
// available. This does not provide a character-input fallback.
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

// SetDestroyCallback registers XNDestroyCallback. Xlib copies the descriptor;
// its callback/client-data targets must outlive the XIM. When called, Xlib
// destroys the XIM and all its XICs: the client must invalidate its handles,
// not call XCloseIM or XDestroyIC on them. Returns false on attribute rejection.
func (im XIM) SetDestroyCallback(callback XIMCallback) bool {
	if im == 0 {
		return false
	}
	name := cgo.CStringTemp(xnDestroyCallback)
	var pin runtime.Pinner
	pin.Pin(&callback)
	defer pin.Unpin()
	failed, _, _ := xSetIMValues.CallRaw(uintptr(im), uintptr(name), uintptr(unsafe.Pointer(&callback)), 0)
	runtime.KeepAlive(name)
	return failed == 0
}

// QueryInputStyles copies XNQueryInputStyle and frees Xlib's result. A nil slice
// means the query failed; style selection belongs to the caller.
func (im XIM) QueryInputStyles() []uintptr {
	if im == 0 {
		return nil
	}
	var styles *XIMStyles
	name := cgo.CStringTemp(xnQueryInputStyle)
	var pin runtime.Pinner
	pin.Pin(&styles)
	defer pin.Unpin()
	err, _, _ := xGetIMValues.CallRaw(uintptr(im), uintptr(name), uintptr(unsafe.Pointer(&styles)), 0)
	runtime.KeepAlive(name)
	if styles != nil {
		defer Free((*byte)(unsafe.Pointer(styles)))
	}
	if err != 0 || styles == nil {
		return nil
	}
	return append([]uintptr(nil), unsafe.Slice(styles.SupportedStyles, int(styles.CountStyles))...)
}

// CreateIC binds an input context to window using the requested native style.
// preedit supplies start/done/draw/caret callbacks in that order, or nil for a
// style without callbacks. Xlib copies the callback descriptors; their function
// and client-data targets must remain valid until Destroy. Returns 0 on failure.
func (im XIM) CreateIC(window Window, style uintptr, preedit *[4]XIMCallback) XIC {
	if im == 0 {
		return 0
	}
	inputStyle := cgo.CStringTemp(xnInputStyle)
	clientWindow := cgo.CStringTemp(xnClientWindow)
	focusWindow := cgo.CStringTemp(xnFocusWindow)
	var list uintptr
	if preedit != nil {
		var pin runtime.Pinner
		pin.Pin(preedit)
		defer pin.Unpin()
		start, done := cgo.CStringTemp(xnPreeditStartCallback), cgo.CStringTemp(xnPreeditDoneCallback)
		draw, caret := cgo.CStringTemp(xnPreeditDrawCallback), cgo.CStringTemp(xnPreeditCaretCallback)
		defer runtime.KeepAlive(start)
		defer runtime.KeepAlive(done)
		defer runtime.KeepAlive(draw)
		defer runtime.KeepAlive(caret)
		list, _, _ = xVaCreateNestedList.CallRaw(0,
			uintptr(start), uintptr(unsafe.Pointer(&preedit[0])),
			uintptr(done), uintptr(unsafe.Pointer(&preedit[1])),
			uintptr(draw), uintptr(unsafe.Pointer(&preedit[2])),
			uintptr(caret), uintptr(unsafe.Pointer(&preedit[3])), 0)
		if list == 0 {
			return 0
		}
		defer Free((*byte)(cgo.Pointer(list)))
	}
	// A null attribute name terminates varargs for the no-callback case.
	var preeditName unsafe.Pointer
	if list != 0 {
		preeditName = cgo.CStringTemp(xnPreeditAttributes)
	}
	ret, _, _ := xCreateIC.CallRaw(
		uintptr(im),
		uintptr(inputStyle), style,
		uintptr(clientWindow), uintptr(window),
		uintptr(focusWindow), uintptr(window),
		uintptr(preeditName), list, 0,
	)
	runtime.KeepAlive(inputStyle)
	runtime.KeepAlive(clientWindow)
	runtime.KeepAlive(focusWindow)
	runtime.KeepAlive(preeditName)
	runtime.KeepAlive(preedit)
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

// SetSpot sets XNSpotLocation in focus-window physical pixels and reports
// whether XSetICValues accepted the attributes. Support outside
// XIMPreeditPosition is an Xlib/input-method extension, not an XIM guarantee.
func (ic XIC) SetSpot(x, y int16) bool {
	if ic == 0 {
		return false
	}
	spotBuf.X, spotBuf.Y = x, y
	spotLocation := cgo.CStringTemp(xnSpotLocation)
	preeditAttributes := cgo.CStringTemp(xnPreeditAttributes)
	// A nested list may borrow its names until XSetICValues consumes it.
	defer runtime.KeepAlive(spotLocation)
	defer runtime.KeepAlive(preeditAttributes)
	list, _, _ := xVaCreateNestedList.CallRaw(0, uintptr(spotLocation), uintptr(cgo.Pointer(&spotBuf)), 0)
	if list == 0 {
		return false
	}
	failed, _, _ := xSetICValues.CallRaw(uintptr(ic), uintptr(preeditAttributes), list, 0)
	Free((*byte)(cgo.Pointer(list)))
	return failed == 0
}

// Utf8LookupString feeds a key-press event to the input context and returns the
// committed UTF-8 text (empty if none), the keysym, and the lookup status.
func (ic XIC) Utf8LookupString(event *KeyEvent) (text string, keysym KeySym, status Status) {
	// Xlib may invoke Go preedit callbacks during lookup. Pin all output slots
	// and borrowed input storage across possible Go stack growth in a callback.
	var pin runtime.Pinner
	pin.Pin(event)
	pin.Pin(&keysym)
	pin.Pin(&status)
	defer pin.Unpin()
	buf := make([]byte, 64)
	for {
		pin.Pin(&buf[0])
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
	var pin runtime.Pinner
	pin.Pin(event)
	defer pin.Unpin()
	ret, _, _ := xFilterEvent.CallRaw(uintptr(cgo.Pointer(event)), uintptr(window))
	return ret != 0
}
