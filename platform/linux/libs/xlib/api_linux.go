package xlib

import (
	"runtime"

	"github.com/goexlib/cgo"
)

var (
	libx11 = cgo.NewLazyLibrary("libX11.so.6")

	xOpenDisplay            = libx11.NewSymbol("XOpenDisplay")
	xCloseDisplay           = libx11.NewSymbol("XCloseDisplay")
	xConnectionNumber       = libx11.NewSymbol("XConnectionNumber")
	xPending                = libx11.NewSymbol("XPending")
	xQLength                = libx11.NewSymbol("XQLength")
	xFlush                  = libx11.NewSymbol("XFlush")
	xSendEvent              = libx11.NewSymbol("XSendEvent")
	xNextEvent              = libx11.NewSymbol("XNextEvent")
	xPeekEvent              = libx11.NewSymbol("XPeekEvent")
	xInternAtom             = libx11.NewSymbol("XInternAtom")
	xDefaultScreen          = libx11.NewSymbol("XDefaultScreen")
	xDefaultScreenOfDisplay = libx11.NewSymbol("XDefaultScreenOfDisplay")
	xDefaultVisual          = libx11.NewSymbol("XDefaultVisual")
	xDefaultDepth           = libx11.NewSymbol("XDefaultDepth")
	xDefaultRootWindow      = libx11.NewSymbol("XDefaultRootWindow")
	xRootWindow             = libx11.NewSymbol("XRootWindow")
	xCreateWindow           = libx11.NewSymbol("XCreateWindow")
	xGetWindowAttributes    = libx11.NewSymbol("XGetWindowAttributes")
	xDestroyWindow          = libx11.NewSymbol("XDestroyWindow")
	xMapWindow              = libx11.NewSymbol("XMapWindow")
	xIconifyWindow          = libx11.NewSymbol("XIconifyWindow")
	xUnmapWindow            = libx11.NewSymbol("XUnmapWindow")
	xWithdrawWindow         = libx11.NewSymbol("XWithdrawWindow")
	xUngrabPointer          = libx11.NewSymbol("XUngrabPointer")
	xMoveWindow             = libx11.NewSymbol("XMoveWindow")
	xResizeWindow           = libx11.NewSymbol("XResizeWindow")
	xTranslateCoordinates   = libx11.NewSymbol("XTranslateCoordinates")
	xClearArea              = libx11.NewSymbol("XClearArea")
	xStoreName              = libx11.NewSymbol("XStoreName")
	xSetTransientForHint    = libx11.NewSymbol("XSetTransientForHint")
	xSetWMProtocols         = libx11.NewSymbol("XSetWMProtocols")
	xSetWMNormalHints       = libx11.NewSymbol("XSetWMNormalHints")
	xSetClassHint           = libx11.NewSymbol("XSetClassHint")
	xDeleteProperty         = libx11.NewSymbol("XDeleteProperty")
	xChangeProperty         = libx11.NewSymbol("XChangeProperty")
	xGetWindowProperty      = libx11.NewSymbol("XGetWindowProperty")
	xSetSelectionOwner      = libx11.NewSymbol("XSetSelectionOwner")
	xGetSelectionOwner      = libx11.NewSymbol("XGetSelectionOwner")
	xConvertSelection       = libx11.NewSymbol("XConvertSelection")
	xChangeWindowAttributes = libx11.NewSymbol("XChangeWindowAttributes")
	xCreateColormap         = libx11.NewSymbol("XCreateColormap")
	xFreeColormap           = libx11.NewSymbol("XFreeColormap")
	xCreateGC               = libx11.NewSymbol("XCreateGC")
	xFreeGC                 = libx11.NewSymbol("XFreeGC")
	xCreatePixmap           = libx11.NewSymbol("XCreatePixmap")
	xFreePixmap             = libx11.NewSymbol("XFreePixmap")
	xCreateImage            = libx11.NewSymbol("XCreateImage")
	xDestroyImage           = libx11.NewSymbol("XDestroyImage")
	xPutImage               = libx11.NewSymbol("XPutImage")
	xFree                   = libx11.NewSymbol("XFree")
	xLookupKeysym           = libx11.NewSymbol("XLookupKeysym")
	xkbLookupKeySym         = libx11.NewSymbol("XkbLookupKeySym")
	xkbKeysymToModifiers    = libx11.NewSymbol("XkbKeysymToModifiers")
	xRefreshKeyboardMapping = libx11.NewSymbol("XRefreshKeyboardMapping")
	xKeysymToKeycode        = libx11.NewSymbol("XKeysymToKeycode")
	xGetModifierMapping     = libx11.NewSymbol("XGetModifierMapping")
	xFreeModifiermap        = libx11.NewSymbol("XFreeModifiermap")
	xCreateFontCursor       = libx11.NewSymbol("XCreateFontCursor")
	xCreatePixmapCursor     = libx11.NewSymbol("XCreatePixmapCursor")
	xDefineCursor           = libx11.NewSymbol("XDefineCursor")
	xUndefineCursor         = libx11.NewSymbol("XUndefineCursor")
	xFreeCursor             = libx11.NewSymbol("XFreeCursor")
	xCreateBitmapFromData   = libx11.NewSymbol("XCreateBitmapFromData")
)

func OpenDisplay(name string) Display {
	cName := cgo.CStringTemp(name)
	ret, _, _ := xOpenDisplay.CallRaw(uintptr(cName))
	runtime.KeepAlive(cName)
	return Display(ret)
}

func (d Display) PeekEvent() (event Event) {
	xPeekEvent.CallRaw(uintptr(d), uintptr(cgo.Pointer(&event)))
	return
}

func (d Display) Close() {
	xCloseDisplay.CallRaw(uintptr(d))
}

func (d Display) ConnectionNumber() int32 {
	ret, _, _ := xConnectionNumber.CallRaw(uintptr(d))
	return int32(ret)
}

func (d Display) Pending() int {
	ret, _, _ := xPending.CallRaw(uintptr(d))
	return int(ret)
}

func (d Display) QLength() int {
	ret, _, _ := xQLength.CallRaw(uintptr(d))
	return int(ret)
}

func (d Display) SendEvent(w Window, propagate bool, eventMask uint64, event *Event) Status {
	ret, _, _ := xSendEvent.CallRaw(uintptr(d), uintptr(w), uintptr(cgo.CBool(propagate)), uintptr(eventMask), uintptr(cgo.Pointer(event)))
	return Status(ret)
}

func (d Display) NextEvent() (ev Event) {
	xNextEvent.CallRaw(uintptr(d), uintptr(cgo.Pointer(&ev)))
	return
}

func (d Display) InternAtom(name string, onlyIfExists bool) Atom {
	cName := cgo.CStringTemp(name)
	ret, _, _ := xInternAtom.CallRaw(uintptr(d), uintptr(cName), uintptr(cgo.CBool(onlyIfExists)))
	runtime.KeepAlive(cName)
	return Atom(ret)
}

func (d Display) Flush() {
	xFlush.CallRaw(uintptr(d))
}

func (d Display) DefaultScreen() int {
	ret, _, _ := xDefaultScreen.CallRaw(uintptr(d))
	return int(ret)
}

func (d Display) DefaultScreenOfDisplay() *Screen {
	ret, _, _ := xDefaultScreenOfDisplay.CallRaw(uintptr(d))
	return (*Screen)(cgo.Pointer(ret))
}

func (d Display) DefaultVisual(screen int) *Visual {
	ret, _, _ := xDefaultVisual.CallRaw(uintptr(d), uintptr(screen))
	return (*Visual)(cgo.Pointer(ret))
}

func (d Display) DefaultDepth(screen int) (depth int) {
	ret, _, _ := xDefaultDepth.CallRaw(uintptr(d), uintptr(screen))
	return int(ret)
}

func (d Display) DefaultRootWindow() Window {
	ret, _, _ := xDefaultRootWindow.CallRaw(uintptr(d))
	return Window(ret)
}

func (d Display) RootWindow(screen int) Window {
	ret, _, _ := xRootWindow.CallRaw(uintptr(d), uintptr(screen))
	return Window(ret)
}

func (d Display) CreateWindow(parent Window, x, y, width, height, borderWidth, depth, class int, visual *Visual, valueMask uint, attrs *SetWindowAttributes) Window {
	ret, _, _ := xCreateWindow.CallRaw(uintptr(d), uintptr(parent), uintptr(x), uintptr(y), uintptr(width), uintptr(height), uintptr(borderWidth), uintptr(depth), uintptr(class), uintptr(cgo.Pointer(visual)), uintptr(valueMask), uintptr(cgo.Pointer(attrs)))
	return Window(ret)
}

func (d Display) DestroyWindow(w Window) {
	xDestroyWindow.CallRaw(uintptr(d), uintptr(w))
}

func (d Display) GetWindowAttributes(w Window, attrs *WindowAttributes) Status {
	var pin runtime.Pinner
	pin.Pin(attrs)
	defer pin.Unpin()
	ret, _, _ := xGetWindowAttributes.CallRaw(uintptr(d), uintptr(w), uintptr(cgo.Pointer(attrs)))
	return Status(ret)
}

func (d Display) MapWindow(w Window) {
	xMapWindow.CallRaw(uintptr(d), uintptr(w))
}

func (d Display) UnmapWindow(w Window) {
	xUnmapWindow.CallRaw(uintptr(d), uintptr(w))
}

func (d Display) IconifyWindow(w Window, screen int) Status {
	ret, _, _ := xIconifyWindow.CallRaw(uintptr(d), uintptr(w), uintptr(screen))
	return Status(ret)
}

func (d Display) WithdrawWindow(w Window, screen int) Status {
	ret, _, _ := xWithdrawWindow.CallRaw(uintptr(d), uintptr(w), uintptr(screen))
	return Status(ret)
}

func (d Display) UngrabPointer(time Time) int32 {
	ret, _, _ := xUngrabPointer.CallRaw(uintptr(d), uintptr(time))
	return int32(ret)
}

func (d Display) MoveWindow(w Window, x, y int) {
	xMoveWindow.CallRaw(uintptr(d), uintptr(w), uintptr(x), uintptr(y))
}

func (d Display) ResizeWindow(w Window, width, height uint) {
	xResizeWindow.CallRaw(uintptr(d), uintptr(w), uintptr(width), uintptr(height))
}

// TranslateCoordinates maps (srcX, srcY) in src to the dest window's coordinate
// space. Used to place a popup at an owner-window-local point in root coords.
func (d Display) TranslateCoordinates(src, dest Window, srcX, srcY int) (destX, destY int) {
	destX, destY, _ = d.TranslateCoordinatesChecked(src, dest, srcX, srcY)
	return
}

// TranslateCoordinatesChecked also reports XTranslateCoordinates' Bool result
// (false when the windows are on different X screens).
func (d Display) TranslateCoordinatesChecked(src, dest Window, srcX, srcY int) (destX, destY int, ok bool) {
	var dx, dy int32
	var child Window
	var pin runtime.Pinner
	pin.Pin(&dx)
	pin.Pin(&dy)
	pin.Pin(&child)
	defer pin.Unpin()
	ret, _, _ := xTranslateCoordinates.CallRaw(uintptr(d), uintptr(src), uintptr(dest),
		uintptr(srcX), uintptr(srcY),
		uintptr(cgo.Pointer(&dx)), uintptr(cgo.Pointer(&dy)), uintptr(cgo.Pointer(&child)))
	return int(dx), int(dy), ret != 0
}

func (d Display) ClearArea(w Window, x, y int, width, height uint, exposures bool) {
	xClearArea.CallRaw(uintptr(d), uintptr(w), uintptr(x), uintptr(y), uintptr(width), uintptr(height), uintptr(cgo.CBool(exposures)))
}

func (d Display) StoreName(w Window, name string) {
	cName := cgo.CStringTemp(name)
	xStoreName.CallRaw(uintptr(d), uintptr(w), uintptr(cName))
	runtime.KeepAlive(cName)
}

func (d Display) SetTransientForHint(w, propWin Window) {
	xSetTransientForHint.CallRaw(uintptr(d), uintptr(w), uintptr(propWin))
}

func (d Display) ChangeWindowAttributes(w Window, valueMask uint, attrs *SetWindowAttributes) {
	xChangeWindowAttributes.CallRaw(uintptr(d), uintptr(w), uintptr(valueMask), uintptr(cgo.Pointer(attrs)))
}

func (d Display) DeleteProperty(w Window, property Atom) {
	xDeleteProperty.CallRaw(uintptr(d), uintptr(w), uintptr(property))
}

func (d Display) ChangeProperty(w Window, property, typ Atom, format byte, mode PropertyChangeMode, data cgo.Pointer, nelements int) {
	xChangeProperty.CallRaw(uintptr(d), uintptr(w), uintptr(property), uintptr(typ), uintptr(format), uintptr(mode), uintptr(data), uintptr(nelements))
}

// GetWindowProperty preserves Xlib's returned type, format, lengths and buffer.
// The caller must Free a non-nil data buffer. For format 32, elements occupy
// native unsigned longs in memory, not packed uint32s.
func (d Display) GetWindowProperty(w Window, property Atom, offset, length int, delete bool, reqType Atom,
	actualType *Atom, actualFormat *int32, nitems, bytesAfter *uint, data **byte) int32 {
	// CallRaw transports addresses as uintptrs. Pin the output storage before
	// conversion so stack growth or GC cannot invalidate the native addresses.
	var pin runtime.Pinner
	pin.Pin(actualType)
	pin.Pin(actualFormat)
	pin.Pin(nitems)
	pin.Pin(bytesAfter)
	pin.Pin(data)
	defer pin.Unpin()
	ret, _, _ := xGetWindowProperty.CallRaw(uintptr(d), uintptr(w), uintptr(property),
		uintptr(offset), uintptr(length), uintptr(cgo.CBool(delete)), uintptr(reqType),
		uintptr(cgo.Pointer(actualType)), uintptr(cgo.Pointer(actualFormat)),
		uintptr(cgo.Pointer(nitems)), uintptr(cgo.Pointer(bytesAfter)), uintptr(cgo.Pointer(data)))
	return int32(ret)
}

func (d Display) SetSelectionOwner(selection Atom, owner Window, time Time) {
	xSetSelectionOwner.CallRaw(uintptr(d), uintptr(selection), uintptr(owner), uintptr(time))
}

func (d Display) GetSelectionOwner(selection Atom) Window {
	ret, _, _ := xGetSelectionOwner.CallRaw(uintptr(d), uintptr(selection))
	return Window(ret)
}

func (d Display) ConvertSelection(selection, target, property Atom, requestor Window, time Time) {
	xConvertSelection.CallRaw(uintptr(d), uintptr(selection), uintptr(target), uintptr(property), uintptr(requestor), uintptr(time))
}

// GetWindowPropertyBytes reads a window property as raw bytes (format 8),
// copies them into Go memory, frees the X buffer, and returns the data plus the
// property's actual type. Reads up to 64 MiB in one shot (no INCR support).
func (d Display) GetWindowPropertyBytes(w Window, property, reqType Atom) ([]byte, Atom) {
	var (
		actualType   Atom
		actualFormat int32
		nitems       uint64
		bytesAfter   uint64
		prop         uintptr
	)
	xGetWindowProperty.CallRaw(uintptr(d), uintptr(w), uintptr(property),
		uintptr(0), uintptr(1<<24), uintptr(cgo.CBool(false)), uintptr(reqType),
		uintptr(cgo.Pointer(&actualType)), uintptr(cgo.Pointer(&actualFormat)),
		uintptr(cgo.Pointer(&nitems)), uintptr(cgo.Pointer(&bytesAfter)), uintptr(cgo.Pointer(&prop)))
	if prop == 0 {
		return nil, actualType
	}
	out := make([]byte, nitems)
	if nitems > 0 {
		copy(out, []byte(cgo.GoStringN(cgo.Pointer(prop), int(nitems))))
	}
	Free((*byte)(cgo.Pointer(prop)))
	return out, actualType
}

func (d Display) SetWMProtocols(w Window, protocols []Atom) Status {
	ret, _, _ := xSetWMProtocols.CallRaw(uintptr(d), uintptr(w), uintptr(cgo.CSlice(protocols)), uintptr(len(protocols)))
	return Status(ret)
}

// SetWMNormalHints sets the WM_NORMAL_HINTS property on w. The hints struct is
// copied into the C call; the caller retains ownership.
func (d Display) SetWMNormalHints(w Window, hints *SizeHints) {
	xSetWMNormalHints.CallRaw(uintptr(d), uintptr(w), uintptr(cgo.Pointer(hints)))
}

// SetClassHint copies the two null-terminated strings into WM_CLASS.
func (d Display) SetClassHint(w Window, hint *ClassHint) int32 {
	ret, _, _ := xSetClassHint.CallRaw(uintptr(d), uintptr(w), uintptr(cgo.Pointer(hint)))
	runtime.KeepAlive(hint)
	return int32(ret)
}

func (d Display) CreateColormap(w Window, visual *Visual, alloc ColormapAlloc) Colormap {
	ret, _, _ := xCreateColormap.CallRaw(uintptr(d), uintptr(w), uintptr(cgo.Pointer(visual)), uintptr(alloc))
	return Colormap(ret)
}

func (d Display) FreeColormap(c Colormap) {
	xFreeColormap.CallRaw(uintptr(d), uintptr(c))
}

func (d Display) CreateGC(drawable Drawable, valueMask uint, values *GCValues) GC {
	ret, _, _ := xCreateGC.CallRaw(uintptr(d), uintptr(drawable), uintptr(valueMask), uintptr(cgo.Pointer(values)))
	return GC(ret)
}

func (d Display) FreeGC(gc GC) {
	xFreeGC.CallRaw(uintptr(d), uintptr(gc))
}

func (d Display) CreatePixmap(drawable Drawable, width, height, depth int) Pixmap {
	ret, _, _ := xCreatePixmap.CallRaw(uintptr(d), uintptr(drawable), uintptr(width), uintptr(height), uintptr(depth))
	return Pixmap(ret)
}

func (d Display) FreePixmap(pixmap Pixmap) {
	xFreePixmap.CallRaw(uintptr(d), uintptr(pixmap))
}

func (d Display) CreateImage(visual *Visual, depth, format, offset int, data cgo.Pointer, width, height, xpad, bytesPerLine int) *Image {
	ret, _, _ := xCreateImage.CallRaw(uintptr(d), uintptr(cgo.Pointer(visual)), uintptr(depth), uintptr(format), uintptr(offset), uintptr(data), uintptr(width), uintptr(height), uintptr(xpad), uintptr(bytesPerLine))
	return (*Image)(cgo.Pointer(ret))
}

func (d Display) PutImage(drawable Drawable, gc GC, image *Image, srcX, srcY, dstX, dstY, width, height int) {
	xPutImage.CallRaw(uintptr(d), uintptr(drawable), uintptr(gc), uintptr(cgo.Pointer(image)), uintptr(srcX), uintptr(srcY), uintptr(dstX), uintptr(dstY), uintptr(width), uintptr(height))
}

func (img *Image) Destroy() {
	xDestroyImage.CallRaw(uintptr(cgo.Pointer(img)))
}

func Free[T any](p *T) {
	xFree.CallRaw(uintptr(cgo.Pointer(p)))
}

func LookupKeysym(event *KeyEvent, index int) KeySym {
	ret, _, _ := xLookupKeysym.CallRaw(uintptr(cgo.Pointer(event)), uintptr(index))
	return KeySym(ret)
}

func (d Display) XkbLookupKeySym(keycode uint8, state uint32) (sym KeySym, modifiers uint32, ok bool) {
	ret, _, _ := xkbLookupKeySym.CallRaw(uintptr(d), uintptr(keycode), uintptr(state),
		uintptr(cgo.Pointer(&modifiers)), uintptr(cgo.Pointer(&sym)))
	ok = ret != 0
	return
}

func RefreshKeyboardMapping(event *MappingEvent) {
	xRefreshKeyboardMapping.CallRaw(uintptr(cgo.Pointer(event)))
}

func (d Display) XkbKeysymToModifiers(sym KeySym) uint32 {
	ret, _, _ := xkbKeysymToModifiers.CallRaw(uintptr(d), uintptr(sym))
	return uint32(ret)
}

func (d Display) KeysymToKeycode(keysym KeySym) KeyCode {
	ret, _, _ := xKeysymToKeycode.CallRaw(uintptr(d), uintptr(keysym))
	return KeyCode(ret)
}

func (d Display) GetModifierMapping() *ModifierKeymap {
	ret, _, _ := xGetModifierMapping.CallRaw(uintptr(d))
	return (*ModifierKeymap)(cgo.Pointer(ret))
}

func FreeModifiermap(mapping *ModifierKeymap) int {
	ret, _, _ := xFreeModifiermap.CallRaw(uintptr(cgo.Pointer(mapping)))
	return int(ret)
}

// CreateFontCursor creates a cursor from the X standard cursor font (cursorfont.h).
// The shape parameter is a font index (e.g., XC_xterm = 152, XC_hand2 = 60).
func (d Display) CreateFontCursor(shape uint) Cursor {
	ret, _, _ := xCreateFontCursor.CallRaw(uintptr(d), uintptr(shape))
	return Cursor(ret)
}

// CreatePixmapCursor creates a custom cursor from source/mask pixmaps and colors.
// Used to create transparent (hidden) cursors: pass a 1x1 transparent bitmap for both.
func (d Display) CreatePixmapCursor(source, mask Pixmap, foreground, background *XColor, x, y uint) Cursor {
	ret, _, _ := xCreatePixmapCursor.CallRaw(
		uintptr(d),
		uintptr(source),
		uintptr(mask),
		uintptr(cgo.Pointer(foreground)),
		uintptr(cgo.Pointer(background)),
		uintptr(x),
		uintptr(y),
	)
	return Cursor(ret)
}

// CreateBitmapFromData creates a 1-bit depth pixmap from bitmap data.
// Used for cursor masks. Data is a byte slice where each byte is a row of 8 bits.
func (d Display) CreateBitmapFromData(drawable Drawable, data []byte, width, height uint) Pixmap {
	ret, _, _ := xCreateBitmapFromData.CallRaw(
		uintptr(d),
		uintptr(drawable),
		uintptr(cgo.Pointer(&data[0])),
		uintptr(width),
		uintptr(height),
	)
	return Pixmap(ret)
}

// DefineCursor sets the cursor for the window. The cursor is displayed when the
// pointer is in the window.
func (d Display) DefineCursor(w Window, cursor Cursor) {
	xDefineCursor.CallRaw(uintptr(d), uintptr(w), uintptr(cursor))
}

// UndefineCursor removes the cursor definition for the window, reverting to the
// parent window's cursor.
func (d Display) UndefineCursor(w Window) {
	xUndefineCursor.CallRaw(uintptr(d), uintptr(w))
}

// FreeCursor deletes the cursor and frees its storage. Do not free cursors that
// are still defined on any window.
func (d Display) FreeCursor(cursor Cursor) {
	xFreeCursor.CallRaw(uintptr(d), uintptr(cursor))
}
