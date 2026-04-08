package xlib

import (
	"github.com/goexlib/cgo"
	"runtime"
)

var (
	libx11 = cgo.NewLazyLibrary("libX11.so.6")

	xOpenDisplay            = libx11.NewSymbol("XOpenDisplay")
	xCloseDisplay           = libx11.NewSymbol("XCloseDisplay")
	xPending                = libx11.NewSymbol("XPending")
	xQLength                = libx11.NewSymbol("XQLength")
	xFlush                  = libx11.NewSymbol("XFlush")
	xSendEvent              = libx11.NewSymbol("XSendEvent")
	xNextEvent              = libx11.NewSymbol("XNextEvent")
	xInternAtom             = libx11.NewSymbol("XInternAtom")
	xDefaultScreen          = libx11.NewSymbol("XDefaultScreen")
	xDefaultScreenOfDisplay = libx11.NewSymbol("XDefaultScreenOfDisplay")
	xDefaultVisual          = libx11.NewSymbol("XDefaultVisual")
	xDefaultDepth           = libx11.NewSymbol("XDefaultDepth")
	xDefaultRootWindow      = libx11.NewSymbol("XDefaultRootWindow")
	xRootWindow             = libx11.NewSymbol("XRootWindow")
	xCreateWindow           = libx11.NewSymbol("XCreateWindow")
	xDestroyWindow          = libx11.NewSymbol("XDestroyWindow")
	xMapWindow              = libx11.NewSymbol("XMapWindow")
	xUnmapWindow            = libx11.NewSymbol("XUnmapWindow")
	xStoreName              = libx11.NewSymbol("XStoreName")
	xSetTransientForHint    = libx11.NewSymbol("XSetTransientForHint")
	xSetWMProtocols         = libx11.NewSymbol("XSetWMProtocols")
	xDeleteProperty         = libx11.NewSymbol("XDeleteProperty")
	xChangeProperty         = libx11.NewSymbol("XChangeProperty")
	xChangeWindowAttributes = libx11.NewSymbol("XChangeWindowAttributes")
	xCreateGC               = libx11.NewSymbol("XCreateGC")
	xFreeGC                 = libx11.NewSymbol("XFreeGC")
	xCreatePixmap           = libx11.NewSymbol("XCreatePixmap")
	xFreePixmap             = libx11.NewSymbol("XFreePixmap")
	xCreateImage            = libx11.NewSymbol("XCreateImage")
	xDestroyImage           = libx11.NewSymbol("XDestroyImage")
	xPutImage               = libx11.NewSymbol("XPutImage")
)

func OpenDisplay(name string) Display {
	cName := cgo.CString(name)
	ret, _, _ := xOpenDisplay.CallRaw(uintptr(cName))
	runtime.KeepAlive(cName)
	return Display(ret)
}

func (d Display) Close() {
	xCloseDisplay.CallRaw(uintptr(d))
}

func (d Display) ConnectionNumber() int32 {
	ret, _, _ := xQLength.CallRaw(uintptr(d))
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
	cName := cgo.CString(name)
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

func (d Display) MapWindow(w Window) {
	xMapWindow.CallRaw(uintptr(d), uintptr(w))
}

func (d Display) UnmapWindow(w Window) {
	xUnmapWindow.CallRaw(uintptr(d), uintptr(w))
}

func (d Display) StoreName(w Window, name string) {
	cName := cgo.CString(name)
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

func (d Display) SetWMProtocols(w Window, protocols []Atom) Status {
	ret, _, _ := xSetWMProtocols.CallRaw(uintptr(d), uintptr(w), uintptr(cgo.CSlice(protocols)), uintptr(len(protocols)))
	return Status(ret)
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
