package glib

import "github.com/goexlib/cgo"

var (
	glib = cgo.NewLazyLibrary("libglib-2.0.so.0")

	gObjectRef   = glib.NewSymbol("g_object_ref")
	gObjectUnref = glib.NewSymbol("g_object_unref")
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

type GSList[T any] struct {
	Data *T
	Next *GSList[T]
}
