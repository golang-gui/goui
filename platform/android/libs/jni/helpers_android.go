package jni

import (
	"fmt"
	"unicode/utf16"
)

// NewString copies Go UTF-8 into a Java UTF-16 String, including NUL and non-BMP text.
func (e Env) NewString(text string) Object { return e.NewStringUTF16(utf16.Encode([]rune(text))) }

// String copies a Java String to Go UTF-8. text is a live non-null reference.
func (e Env) String(text Object) string {
	length := e.GetStringLength(text)
	if length <= 0 {
		return ""
	}
	chars := make([]uint16, int(length))
	e.GetStringRegion(text, 0, chars)
	return string(utf16.Decode(chars))
}
func (e Env) Strings(values []string) Object {
	class := e.FindClass("java/lang/String")
	if class == 0 {
		return 0
	}
	defer e.DeleteLocalRef(class)
	array := e.NewObjectArray(int32(len(values)), class, 0)
	if array == 0 {
		return 0
	}
	for i, value := range values {
		text := e.NewString(value)
		if text == 0 {
			e.DeleteLocalRef(array)
			return 0
		}
		e.SetObjectArrayElement(array, int32(i), text)
		e.DeleteLocalRef(text)
		if e.ExceptionCheck() {
			e.DeleteLocalRef(array)
			return 0
		}
	}
	return array
}
func (e Env) IntArray(values []int32) Object {
	array := e.NewIntArray(int32(len(values)))
	if array != 0 && len(values) > 0 {
		e.SetIntArrayRegion(array, 0, values)
	}
	return array
}
func (e Env) Ints(array Object) []int32 {
	if array == 0 {
		return nil
	}
	values := make([]int32, int(e.GetArrayLength(array)))
	if len(values) > 0 {
		e.GetIntArrayRegion(array, 0, values)
	}
	return values
}
func (e Env) Floats(array Object) []float32 {
	if array == 0 {
		return nil
	}
	values := make([]float32, int(e.GetArrayLength(array)))
	if len(values) > 0 {
		e.GetFloatArrayRegion(array, 0, values)
	}
	return values
}

// Exception consumes a pending Java exception before attempting ordinary JNI
// calls. A second exception while formatting it is cleared and reported safely.
func (e Env) Exception() error {
	if !e.ExceptionCheck() {
		return nil
	}
	throwable := e.ExceptionOccurred()
	e.ExceptionClear()
	if throwable == 0 {
		return fmt.Errorf("JNI exception")
	}
	defer e.DeleteLocalRef(throwable)
	class := e.GetObjectClass(throwable)
	if e.ExceptionCheck() {
		e.ExceptionClear()
		return fmt.Errorf("JNI exception (class unavailable)")
	}
	if class == 0 {
		return fmt.Errorf("JNI exception (class unavailable)")
	}
	defer e.DeleteLocalRef(class)
	method := e.GetMethodID(class, "toString", "()Ljava/lang/String;")
	if e.ExceptionCheck() {
		e.ExceptionClear()
		return fmt.Errorf("JNI exception (description unavailable)")
	}
	if method == 0 {
		return fmt.Errorf("JNI exception (description unavailable)")
	}
	message := e.CallObjectMethodA(throwable, method)
	if e.ExceptionCheck() {
		e.ExceptionClear()
		return fmt.Errorf("JNI exception (description unavailable)")
	}
	if message == 0 {
		return fmt.Errorf("JNI exception")
	}
	defer e.DeleteLocalRef(message)
	text := e.String(message)
	if e.ExceptionCheck() {
		e.ExceptionClear()
		return fmt.Errorf("JNI exception (description unavailable)")
	}
	return fmt.Errorf("JNI: %s", text)
}
