package xlib

import (
	"runtime"

	"github.com/goexlib/cgo"
)

var (
	xGrabServer          = libx11.NewSymbol("XGrabServer")
	xUngrabServer        = libx11.NewSymbol("XUngrabServer")
	xrmGetStringDatabase = libx11.NewSymbol("XrmGetStringDatabase")
	xrmGetResource       = libx11.NewSymbol("XrmGetResource")
	xrmDestroyDatabase   = libx11.NewSymbol("XrmDestroyDatabase")
)

func (d Display) GrabServer()   { xGrabServer.CallRaw(uintptr(d)) }
func (d Display) UngrabServer() { xUngrabServer.CallRaw(uintptr(d)) }

type ResourceDatabase uintptr

func GetStringDatabase(resources string) ResourceDatabase {
	cResources := cgo.CString(resources)
	ret, _, _ := xrmGetStringDatabase.CallRaw(uintptr(cResources))
	runtime.KeepAlive(cResources)
	return ResourceDatabase(ret)
}

func (db ResourceDatabase) Destroy() {
	xrmDestroyDatabase.CallRaw(uintptr(db))
}

func (db ResourceDatabase) GetResource(name, class string) (string, bool) {
	cName, cClass := cgo.CString(name), cgo.CString(class)
	var kind uintptr
	var value struct {
		Size    uint32
		Address uintptr
	}
	var pin runtime.Pinner
	pin.Pin(&kind)
	pin.Pin(&value)
	defer pin.Unpin()
	ret, _, _ := xrmGetResource.CallRaw(uintptr(db), uintptr(cName), uintptr(cClass),
		uintptr(cgo.Pointer(&kind)), uintptr(cgo.Pointer(&value)))
	runtime.KeepAlive(cName)
	runtime.KeepAlive(cClass)
	if ret == 0 || value.Address == 0 || value.Size == 0 {
		return "", false
	}
	return cgo.GoString(cgo.Pointer(value.Address)), true
}
