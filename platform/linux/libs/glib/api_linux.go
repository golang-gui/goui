package glib

import (
	"runtime"
	"unsafe"

	"github.com/goexlib/cgo"
)

var (
	glib    = cgo.NewLazyLibrary("libglib-2.0.so.0")
	gobject = cgo.NewLazyLibrary("libgobject-2.0.so.0")

	gObjectRef   = gobject.NewSymbol("g_object_ref")
	gObjectUnref = gobject.NewSymbol("g_object_unref")
	gObjectGet   = gobject.NewSymbol("g_object_get")
	gFree        = glib.NewSymbol("g_free")
)

type Object struct {
	GObject uintptr
}

func (o Object) IsNull() bool {
	return o.GObject == 0
}

func (o Object) Valid() bool {
	return o.GObject != 0
}

func (o Object) Ref() {
	// void g_object_ref(GObject*)
	gObjectRef.CallRaw(o.GObject)
}

func (o Object) Unref() {
	//void g_object_unref(GObject*)
	gObjectUnref.CallRaw(o.GObject)
}

func (o Object) BoolProperty(name string) (bool, error) {
	cName := cgo.CString(name)
	var value int32
	_, _, err := gObjectGet.CallRaw(o.GObject, uintptr(cName), uintptr(unsafe.Pointer(&value)), 0)
	runtime.KeepAlive(cName)
	return value != 0, err
}

func (o Object) IntProperty(name string) (int32, error) {
	cName := cgo.CString(name)
	var value int32
	_, _, err := gObjectGet.CallRaw(o.GObject, uintptr(cName), uintptr(unsafe.Pointer(&value)), 0)
	runtime.KeepAlive(cName)
	return value, err
}

func (o Object) StringProperty(name string) (string, error) {
	cName := cgo.CString(name)
	var value uintptr
	_, _, err := gObjectGet.CallRaw(o.GObject, uintptr(cName), uintptr(unsafe.Pointer(&value)), 0)
	runtime.KeepAlive(cName)
	if err != nil {
		return "", err
	}
	if value == 0 {
		return "", nil
	}
	defer gFree.CallRaw(value)
	return cgo.GoString(cgo.Pointer(value)), nil
}

type GSList[T any] struct {
	Data *T
	Next *GSList[T]
}
