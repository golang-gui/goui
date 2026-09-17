package glib

import (
	"errors"
	"fmt"
	"runtime"
	"unsafe"

	"github.com/goexlib/cgo"
)

var (
	glib    = cgo.NewLazyLibrary("libglib-2.0.so.0")
	gobject = cgo.NewLazyLibrary("libgobject-2.0.so.0")
	gio     = cgo.NewLazyLibrary("libgio-2.0.so.0")

	gObjectRef   = gobject.NewSymbol("g_object_ref")
	gObjectUnref = gobject.NewSymbol("g_object_unref")
	gObjectGet   = gobject.NewSymbol("g_object_get")

	gFree          = glib.NewSymbol("g_free")
	gFilenameToURI = glib.NewSymbol("g_filename_to_uri")
	gErrorFree     = glib.NewSymbol("g_error_free")

	gAppInfoLaunchDefaultForURI = gio.NewSymbol("g_app_info_launch_default_for_uri")
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
	cName := cgo.CStringTemp(name)
	var value int32
	_, _, err := gObjectGet.CallRaw(o.GObject, uintptr(cName), uintptr(unsafe.Pointer(&value)), 0)
	runtime.KeepAlive(cName)
	return value != 0, err
}

func (o Object) IntProperty(name string) (int32, error) {
	cName := cgo.CStringTemp(name)
	var value int32
	_, _, err := gObjectGet.CallRaw(o.GObject, uintptr(cName), uintptr(unsafe.Pointer(&value)), 0)
	runtime.KeepAlive(cName)
	return value, err
}

func (o Object) StringProperty(name string) (string, error) {
	cName := cgo.CStringTemp(name)
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

// GError is the native GLib error layout. Copy its diagnostic before ErrorFree.
type GError struct {
	Domain  uint32
	Code    int32
	Message *byte
}

func (e *GError) Error() string {
	return fmt.Sprintf("GLib domain %d, code %d: %s", e.Domain, e.Code,
		cgo.GoString(unsafe.Pointer(e.Message)))
}

func ErrorFree(err *GError) {
	gErrorFree.CallRaw(uintptr(unsafe.Pointer(err)))
	runtime.KeepAlive(err)
}

// FilenameToURI converts an absolute native filename to a file URI, copying and
// freeing GLib's result. InitURI must have succeeded before this call.
func FilenameToURI(filename string) (string, error) {
	name := cgo.CStringTemp(filename)
	var nativeErr *GError
	// CallRaw takes uintptrs; pin Go storage so stack growth cannot invalidate
	// the input or the native GError** output slot during lazy symbol dispatch.
	var pin runtime.Pinner
	pin.Pin(name)
	pin.Pin(&nativeErr)
	defer pin.Unpin()
	ret, _, err := gFilenameToURI.CallRaw(uintptr(name), 0, uintptr(unsafe.Pointer(&nativeErr)))
	runtime.KeepAlive(name)
	if nativeErr != nil {
		err = errors.New(nativeErr.Error())
		ErrorFree(nativeErr)
	}
	if ret != 0 {
		defer gFree.CallRaw(ret)
	}
	if err != nil {
		return "", err
	}
	if ret == 0 {
		return "", errors.New("g_filename_to_uri returned no URI")
	}
	return cgo.GoString(cgo.Pointer(ret)), nil
}

// AppInfoLaunchDefaultForURI reports GIO's synchronous launch-request result.
// context is a GAppLaunchContext pointer, or zero for the native default.
func AppInfoLaunchDefaultForURI(uri string, context uintptr) error {
	value := cgo.CStringTemp(uri)
	var nativeErr *GError
	var pin runtime.Pinner
	pin.Pin(value)
	pin.Pin(&nativeErr)
	defer pin.Unpin()
	ret, _, err := gAppInfoLaunchDefaultForURI.CallRaw(uintptr(value), context, uintptr(unsafe.Pointer(&nativeErr)))
	runtime.KeepAlive(value)
	if nativeErr != nil {
		err = errors.New(nativeErr.Error())
		ErrorFree(nativeErr)
		return err
	}
	if err != nil {
		return err
	}
	if ret == 0 {
		return errors.New("g_app_info_launch_default_for_uri failed without a GError")
	}
	return nil
}
