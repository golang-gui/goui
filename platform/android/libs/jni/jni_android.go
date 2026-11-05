// Package jni binds the public JNI invocation and native interfaces.
package jni

import (
	"fmt"
	"math"
	"runtime"
	"unicode/utf16"
	"unsafe"

	"github.com/goexlib/cgo"
)

type VM uintptr
type Env uintptr
type Object uintptr
type Method uintptr

// Value has the layout of the JNI jvalue union. A-method calls copy arguments
// synchronously and never retain this Go storage.
type Value uint64

func Ref(v Object) Value    { return Value(v) }
func Int(v int32) Value     { return Value(uint32(v)) }
func Float(v float32) Value { return Value(math.Float32bits(v)) }

const Version16 = 0x00010006

func function(handle uintptr, index int) uintptr {
	table := *(*uintptr)(cgo.Pointer(handle))
	return unsafe.Slice((*uintptr)(cgo.Pointer(table)), index+1)[index]
}

func (v VM) GetEnv() (Env, int32) {
	var result uintptr
	var pin runtime.Pinner
	pin.Pin(&result)
	defer pin.Unpin()
	status := cgo.CallRet[int32](function(uintptr(v), 6), uintptr(v), uintptr(unsafe.Pointer(&result)), int32(Version16))
	return Env(result), status
}

func (v VM) AttachCurrentThreadAsDaemon() (Env, int32) {
	var result uintptr
	var pin runtime.Pinner
	pin.Pin(&result)
	defer pin.Unpin()
	status := cgo.CallRet[int32](function(uintptr(v), 7), uintptr(v), uintptr(unsafe.Pointer(&result)), uintptr(0))
	return Env(result), status
}

func (v VM) DetachCurrentThread() int32 {
	return cgo.CallRet[int32](function(uintptr(v), 5), uintptr(v))
}

func (e Env) PushLocalFrame(capacity int32) int32 {
	return cgo.CallRet[int32](function(uintptr(e), 19), uintptr(e), capacity)
}
func (e Env) PopLocalFrame() { cgo.Call(function(uintptr(e), 20), uintptr(e), uintptr(0)) }
func (e Env) NewGlobalRef(obj Object) Object {
	return Object(cgo.CallRet[uintptr](function(uintptr(e), 21), uintptr(e), uintptr(obj)))
}
func (e Env) DeleteGlobalRef(obj Object) {
	cgo.Call(function(uintptr(e), 22), uintptr(e), uintptr(obj))
}
func (e Env) DeleteLocalRef(obj Object) { cgo.Call(function(uintptr(e), 23), uintptr(e), uintptr(obj)) }
func (e Env) ObjectClass(obj Object) Object {
	return Object(cgo.CallRet[uintptr](function(uintptr(e), 31), uintptr(e), uintptr(obj)))
}
func (e Env) Method(class Object, name, signature string, static bool) Method {
	index := 33
	if static {
		index = 113
	}
	return Method(cgo.CallRet[uintptr](function(uintptr(e), index), uintptr(e), uintptr(class), cgo.CStringTemp(name), cgo.CStringTemp(signature)))
}

func (e Env) invoke(index int, obj Object, method Method, args []Value) uintptr {
	var address uintptr
	var pin runtime.Pinner
	if len(args) != 0 {
		pin.Pin(&args[0])
		address = uintptr(unsafe.Pointer(&args[0]))
		defer pin.Unpin()
	}
	result := cgo.CallRet[uintptr](function(uintptr(e), index), uintptr(e), uintptr(obj), uintptr(method), address)
	runtime.KeepAlive(args)
	return result
}
func (e Env) NewObject(class Object, method Method, args ...Value) Object {
	return Object(e.invoke(30, class, method, args))
}
func (e Env) CallObject(obj Object, method Method, args ...Value) Object {
	return Object(e.invoke(36, obj, method, args))
}
func (e Env) CallStaticObject(class Object, method Method, args ...Value) Object {
	return Object(e.invoke(116, class, method, args))
}
func (e Env) CallInt(obj Object, method Method, args ...Value) int32 {
	return int32(e.invoke(51, obj, method, args))
}
func (e Env) CallVoid(obj Object, method Method, args ...Value) { e.invoke(63, obj, method, args) }

func (e Env) NewString(text string) Object {
	chars := utf16.Encode([]rune(text))
	var address uintptr
	var pin runtime.Pinner
	if len(chars) > 0 {
		pin.Pin(&chars[0])
		defer pin.Unpin()
		address = uintptr(unsafe.Pointer(&chars[0]))
	}
	return Object(cgo.CallRet[uintptr](function(uintptr(e), 163), uintptr(e), address, int32(len(chars))))
}
func (e Env) String(text Object) string {
	length := cgo.CallRet[int32](function(uintptr(e), 164), uintptr(e), uintptr(text))
	if length <= 0 {
		return ""
	}
	chars := make([]uint16, int(length))
	var pin runtime.Pinner
	pin.Pin(&chars[0])
	defer pin.Unpin()
	cgo.Call(function(uintptr(e), 220), uintptr(e), uintptr(text), int32(0), length, uintptr(unsafe.Pointer(&chars[0])))
	return string(utf16.Decode(chars))
}
func (e Env) ArrayLength(array Object) int {
	return int(cgo.CallRet[int32](function(uintptr(e), 171), uintptr(e), uintptr(array)))
}
func (e Env) ArrayElement(array Object, index int) Object {
	return Object(cgo.CallRet[uintptr](function(uintptr(e), 173), uintptr(e), uintptr(array), int32(index)))
}

func (e Env) Strings(values []string) Object {
	class := Object(cgo.CallRet[uintptr](function(uintptr(e), 6), uintptr(e), cgo.CStringTemp("java/lang/String")))
	if class == 0 {
		return 0
	}
	defer e.DeleteLocalRef(class)
	array := Object(cgo.CallRet[uintptr](function(uintptr(e), 172), uintptr(e), int32(len(values)), uintptr(class), uintptr(0)))
	if array == 0 {
		return 0
	}
	for i, value := range values {
		text := e.NewString(value)
		if text == 0 {
			return 0
		}
		cgo.Call(function(uintptr(e), 174), uintptr(e), uintptr(array), int32(i), uintptr(text))
		e.DeleteLocalRef(text)
		if e.ExceptionCheck() {
			return 0
		}
	}
	return array
}

func (e Env) ExceptionCheck() bool {
	return cgo.CallRet[uint8](function(uintptr(e), 228), uintptr(e)) != 0
}
func (e Env) IntArray(values []int32) Object {
	a := Object(cgo.CallRet[uintptr](function(uintptr(e), 179), uintptr(e), int32(len(values))))
	if a == 0 || len(values) == 0 {
		return a
	}
	var pin runtime.Pinner
	pin.Pin(&values[0])
	defer pin.Unpin()
	cgo.Call(function(uintptr(e), 211), uintptr(e), uintptr(a), int32(0), int32(len(values)), uintptr(unsafe.Pointer(&values[0])))
	return a
}
func (e Env) Ints(array Object) []int32 {
	length := e.ArrayLength(array)
	result := make([]int32, length)
	if length == 0 {
		return result
	}
	var pin runtime.Pinner
	pin.Pin(&result[0])
	defer pin.Unpin()
	cgo.Call(function(uintptr(e), 203), uintptr(e), uintptr(array), int32(0), int32(length), uintptr(unsafe.Pointer(&result[0])))
	return result
}
func (e Env) Floats(array Object) []float32 {
	length := e.ArrayLength(array)
	result := make([]float32, length)
	if length == 0 {
		return result
	}
	var pin runtime.Pinner
	pin.Pin(&result[0])
	defer pin.Unpin()
	cgo.Call(function(uintptr(e), 205), uintptr(e), uintptr(array), int32(0), int32(length), uintptr(unsafe.Pointer(&result[0])))
	return result
}

// Exception consumes a pending Java exception. Calling JNI methods with an
// exception pending is restricted; clear it before obtaining its description.
func (e Env) Exception() error {
	if !e.ExceptionCheck() {
		return nil
	}
	throwable := Object(cgo.CallRet[uintptr](function(uintptr(e), 15), uintptr(e)))
	cgo.Call(function(uintptr(e), 17), uintptr(e))
	if throwable == 0 {
		return fmt.Errorf("JNI exception")
	}
	defer e.DeleteLocalRef(throwable)
	class := e.ObjectClass(throwable)
	defer e.DeleteLocalRef(class)
	method := e.Method(class, "toString", "()Ljava/lang/String;", false)
	message := Object(0)
	if method != 0 {
		message = e.CallObject(throwable, method)
	}
	if cgo.CallRet[uint8](function(uintptr(e), 228), uintptr(e)) != 0 {
		cgo.Call(function(uintptr(e), 17), uintptr(e))
		return fmt.Errorf("JNI exception (description unavailable)")
	}
	if message == 0 {
		return fmt.Errorf("JNI exception")
	}
	defer e.DeleteLocalRef(message)
	return fmt.Errorf("JNI: %s", e.String(message))
}
