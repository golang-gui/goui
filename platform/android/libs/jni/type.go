// Package jni binds the public JNI 1.6 invocation and native interfaces.
// Env belongs to the current attached OS thread. Returned object references are
// local unless explicitly retained as global references.
package jni

import "math"

type VM uintptr
type Env uintptr
type Object uintptr
type Method uintptr

// Value mirrors jvalue; A-method calls borrow its storage synchronously.
type Value uint64

func Ref(v Object) Value    { return Value(v) }
func Int(v int32) Value     { return Value(uint32(v)) }
func Float(v float32) Value { return Value(math.Float32bits(v)) }

const (
	Version16 int32 = 0x00010006
	OK        int32 = 0
	Detached  int32 = -2
)

// AttachArgs mirrors JavaVMAttachArgs. Name is an optional native UTF-8 string;
// Group, if supplied, must be a global ThreadGroup reference.
type AttachArgs struct {
	Version int32
	Name    uintptr
	Group   Object
}
