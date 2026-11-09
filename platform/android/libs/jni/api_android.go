package jni

import (
	"unsafe"

	"github.com/goexlib/cgo"
)

// JNI 1.6 native-interface slot numbers, including its four reserved entries.
const (
	vmDetachCurrentThread         = 5
	vmGetEnv                      = 6
	vmAttachCurrentThreadAsDaemon = 7
	slotGetVersion                = 4
	slotFindClass                 = 6
	slotExceptionOccurred         = 15
	slotExceptionClear            = 17
	slotPushLocalFrame            = 19
	slotPopLocalFrame             = 20
	slotNewGlobalRef              = 21
	slotDeleteGlobalRef           = 22
	slotDeleteLocalRef            = 23
	slotNewObjectA                = 30
	slotGetObjectClass            = 31
	slotGetMethodID               = 33
	slotCallObjectMethodA         = 36
	slotCallBooleanMethodA        = 39
	slotCallIntMethodA            = 51
	slotCallFloatMethodA          = 57
	slotCallVoidMethodA           = 63
	slotGetStaticMethodID         = 113
	slotCallStaticObjectMethodA   = 116
	slotCallStaticIntMethodA      = 131
	slotGetStaticFieldID          = 144
	slotGetStaticObjectField      = 145
	slotNewString                 = 163
	slotGetStringLength           = 164
	slotGetArrayLength            = 171
	slotNewObjectArray            = 172
	slotGetObjectArrayElement     = 173
	slotSetObjectArrayElement     = 174
	slotNewIntArray               = 179
	slotGetIntArrayRegion         = 203
	slotGetFloatArrayRegion       = 205
	slotSetIntArrayRegion         = 211
	slotGetStringRegion           = 220
	slotExceptionCheck            = 228
)

func function(handle uintptr, index int) uintptr {
	table := *(*uintptr)(cgo.Pointer(handle))
	return *(*uintptr)(unsafe.Add(cgo.Pointer(table), uintptr(index)*unsafe.Sizeof(uintptr(0))))
}
func (v VM) GetEnv(version int32) (out Env, status int32) {
	status = cgo.CallRet[int32](function(uintptr(v), vmGetEnv), v, &out, version)
	return
}
func (v VM) AttachCurrentThreadAsDaemon(args *AttachArgs) (out Env, status int32) {
	status = cgo.CallRet[int32](function(uintptr(v), vmAttachCurrentThreadAsDaemon), v, &out, args)
	return
}
func (v VM) DetachCurrentThread() int32 {
	return cgo.CallRet[int32](function(uintptr(v), vmDetachCurrentThread), v)
}
func (e Env) GetVersion() int32 { return cgo.CallRet[int32](function(uintptr(e), slotGetVersion), e) }
func (e Env) FindClass(name string) Object {
	return cgo.CallRet[Object](function(uintptr(e), slotFindClass), e, cgo.CStringTemp(name))
}
func (e Env) ExceptionOccurred() Object {
	return cgo.CallRet[Object](function(uintptr(e), slotExceptionOccurred), e)
}
func (e Env) ExceptionClear() { cgo.Call(function(uintptr(e), slotExceptionClear), e) }
func (e Env) PushLocalFrame(capacity int32) int32 {
	return cgo.CallRet[int32](function(uintptr(e), slotPushLocalFrame), e, capacity)
}
func (e Env) NewGlobalRef(obj Object) Object {
	return cgo.CallRet[Object](function(uintptr(e), slotNewGlobalRef), e, obj)
}
func (e Env) DeleteGlobalRef(obj Object) { cgo.Call(function(uintptr(e), slotDeleteGlobalRef), e, obj) }
func (e Env) DeleteLocalRef(obj Object)  { cgo.Call(function(uintptr(e), slotDeleteLocalRef), e, obj) }
func (e Env) GetObjectClass(obj Object) Object {
	return cgo.CallRet[Object](function(uintptr(e), slotGetObjectClass), e, obj)
}
func (e Env) GetMethodID(class Object, name, signature string) Method {
	return cgo.CallRet[Method](function(uintptr(e), slotGetMethodID), e, class, cgo.CStringTemp(name), cgo.CStringTemp(signature))
}
func (e Env) GetStaticMethodID(class Object, name, signature string) Method {
	return cgo.CallRet[Method](function(uintptr(e), slotGetStaticMethodID), e, class, cgo.CStringTemp(name), cgo.CStringTemp(signature))
}
func (e Env) GetStaticFieldID(class Object, name, signature string) Field {
	return cgo.CallRet[Field](function(uintptr(e), slotGetStaticFieldID), e, class, cgo.CStringTemp(name), cgo.CStringTemp(signature))
}
func (e Env) GetStaticObjectField(class Object, field Field) Object {
	return cgo.CallRet[Object](function(uintptr(e), slotGetStaticObjectField), e, class, field)
}
func (e Env) GetStringLength(text Object) int32 {
	return cgo.CallRet[int32](function(uintptr(e), slotGetStringLength), e, text)
}
func (e Env) GetArrayLength(array Object) int32 {
	return cgo.CallRet[int32](function(uintptr(e), slotGetArrayLength), e, array)
}
func (e Env) NewObjectArray(length int32, class, initial Object) Object {
	return cgo.CallRet[Object](function(uintptr(e), slotNewObjectArray), e, length, class, initial)
}
func (e Env) GetObjectArrayElement(array Object, index int32) Object {
	return cgo.CallRet[Object](function(uintptr(e), slotGetObjectArrayElement), e, array, index)
}
func (e Env) SetObjectArrayElement(array Object, index int32, value Object) {
	cgo.Call(function(uintptr(e), slotSetObjectArrayElement), e, array, index, value)
}
func (e Env) NewIntArray(length int32) Object {
	return cgo.CallRet[Object](function(uintptr(e), slotNewIntArray), e, length)
}
func (e Env) NewObjectA(obj Object, method Method, args ...Value) Object {
	return cgo.CallRet[Object](function(uintptr(e), slotNewObjectA), e, obj, method, valuesPointer(args))
}
func (e Env) CallObjectMethodA(obj Object, method Method, args ...Value) Object {
	return cgo.CallRet[Object](function(uintptr(e), slotCallObjectMethodA), e, obj, method, valuesPointer(args))
}
func (e Env) CallStaticObjectMethodA(obj Object, method Method, args ...Value) Object {
	return cgo.CallRet[Object](function(uintptr(e), slotCallStaticObjectMethodA), e, obj, method, valuesPointer(args))
}
func (e Env) CallIntMethodA(obj Object, method Method, args ...Value) int32 {
	return cgo.CallRet[int32](function(uintptr(e), slotCallIntMethodA), e, obj, method, valuesPointer(args))
}
func (e Env) CallBooleanMethodA(obj Object, method Method, args ...Value) bool {
	return cgo.CallRet[uint8](function(uintptr(e), slotCallBooleanMethodA), e, obj, method, valuesPointer(args)) != 0
}
func (e Env) CallFloatMethodA(obj Object, method Method, args ...Value) float32 {
	return cgo.CallRet[float32](function(uintptr(e), slotCallFloatMethodA), e, obj, method, valuesPointer(args))
}
func (e Env) CallStaticIntMethodA(obj Object, method Method, args ...Value) int32 {
	return cgo.CallRet[int32](function(uintptr(e), slotCallStaticIntMethodA), e, obj, method, valuesPointer(args))
}
func (e Env) CallVoidMethodA(obj Object, method Method, args ...Value) {
	cgo.Call(function(uintptr(e), slotCallVoidMethodA), e, obj, method, valuesPointer(args))
}
func (e Env) PopLocalFrame(result Object) Object {
	return cgo.CallRet[Object](function(uintptr(e), slotPopLocalFrame), e, result)
}
func (e Env) ExceptionCheck() bool {
	return cgo.CallRet[uint8](function(uintptr(e), slotExceptionCheck), e) != 0
}
func (e Env) NewStringUTF16(chars []uint16) Object {
	return cgo.CallRet[Object](function(uintptr(e), slotNewString), e, slicePointer(chars), int32(len(chars)))
}
func (e Env) GetStringRegion(text Object, start int32, chars []uint16) {
	cgo.Call(function(uintptr(e), slotGetStringRegion), e, text, start, int32(len(chars)), slicePointer(chars))
}
func (e Env) GetIntArrayRegion(array Object, start int32, values []int32) {
	cgo.Call(function(uintptr(e), slotGetIntArrayRegion), e, array, start, int32(len(values)), slicePointer(values))
}
func (e Env) GetFloatArrayRegion(array Object, start int32, values []float32) {
	cgo.Call(function(uintptr(e), slotGetFloatArrayRegion), e, array, start, int32(len(values)), slicePointer(values))
}
func (e Env) SetIntArrayRegion(array Object, start int32, values []int32) {
	cgo.Call(function(uintptr(e), slotSetIntArrayRegion), e, array, start, int32(len(values)), slicePointer(values))
}
func valuesPointer(values []Value) *Value { return slicePointer(values) }
func slicePointer[T any](values []T) *T {
	if len(values) == 0 {
		return nil
	}
	return &values[0]
}
