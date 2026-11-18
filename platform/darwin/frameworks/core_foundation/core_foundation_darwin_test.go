package core_foundation

import (
	"bytes"
	"runtime"
	"sync"
	"testing"
	"unsafe"

	"github.com/golang-gui/goui/platform/darwin/frameworks/utils"
)

var initCFTest = sync.OnceValue(InitCoreFoundation)

func TestCollectionCallbackAddresses(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err := initCFTest(); err != nil {
		t.Fatal(err)
	}
	// macOS arm64 SDK：CFIndex 与函数指针均为八字节；导出对象是结构体，不是结构体指针变量。
	for _, tc := range []struct {
		name  string
		got   uintptr
		words int
	}{
		{"kCFTypeDictionaryKeyCallBacks", kCFTypeDictionaryKeyCallBacks, 6},
		{"kCFTypeDictionaryValueCallBacks", kCFTypeDictionaryValueCallBacks, 5},
		{"kCFTypeArrayCallBacks", kCFTypeArrayCallBacks, 5},
	} {
		address, err := framework.GetSymbol(tc.name)
		if err != nil {
			t.Fatal(err)
		}
		if tc.got == 0 || tc.got != address {
			t.Errorf("%s: callbacks=%#x, want exported object address %#x", tc.name, tc.got, address)
		}
		fields := unsafe.Slice((*uintptr)(unsafe.Pointer(address)), tc.words)
		if fields[0] != 0 {
			t.Errorf("%s: version=%d, want 0", tc.name, fields[0])
		}
		for i, callback := range fields[1:] {
			if callback == 0 {
				t.Errorf("%s: callback at byte offset %d is nil", tc.name, (i+1)*8)
			}
		}
	}
	// 普通引用常量仍须加载导出的值。
	address, err := framework.GetSymbol("kCFBooleanTrue")
	if err != nil {
		t.Fatal(err)
	}
	if KCFBooleanTrue == 0 || KCFBooleanTrue != *(*CFBooleanRef)(unsafe.Pointer(address)) || uintptr(KCFBooleanTrue) == address {
		t.Fatal("CFBoolean constant was loaded as an address instead of a reference value")
	}
}

func TestCollectionsRetainMembers(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err := initCFTest(); err != nil {
		t.Fatal(err)
	}
	var retainCount func(CFTypeRef) CFIndex
	if err := framework.LoadFunctions([]utils.Function{{Name: "CFGetRetainCount", PFunc: &retainCount}}); err != nil {
		t.Fatal(err)
	}
	// 非空 CFData 避开 CFNumber/CFString 的 tagged 或缓存引用，只比较测试拥有对象的相对引用计数。
	key := CFDataCreate(0, []byte("dictionary key"))
	value := CFDataCreate(0, []byte("collection value"))
	defer func() {
		if key != 0 {
			CFRelease(key)
		}
		if value != 0 {
			CFRelease(value)
		}
	}()
	k, v := retainCount(key), retainCount(value)
	array := CFArrayCreate([]CFTypeRef{value})
	defer func() {
		if array != 0 {
			CFRelease(array)
		}
	}()
	dictionary := CFDictionaryCreate([]CFTypeRef{key}, []CFTypeRef{array})
	defer CFRelease(dictionary)
	if retainCount(key) != k+1 || retainCount(value) != v+1 {
		t.Fatalf("collections did not retain members: key=%d want %d, value=%d want %d", retainCount(key), k+1, retainCount(value), v+1)
	}
	// 释放调用方的数组引用后，字典仍须保持数组及成员存活。
	var getValue func(CFDictionaryRef, CFTypeRef) CFTypeRef
	var getLength func(CFDataRef) CFIndex
	var getBytes func(CFDataRef) *byte
	if err := framework.LoadFunctions([]utils.Function{
		{Name: "CFDictionaryGetValue", PFunc: &getValue},
		{Name: "CFDataGetLength", PFunc: &getLength},
		{Name: "CFDataGetBytePtr", PFunc: &getBytes},
	}); err != nil {
		t.Fatal(err)
	}
	CFRelease(array)
	array = 0
	CFRelease(key)
	key = 0
	CFRelease(value)
	value = 0
	lookup := CFDataCreate(0, []byte("dictionary key"))
	defer CFRelease(lookup)
	stored := getValue(dictionary, lookup)
	if stored == 0 || CFArrayGetCount(stored) != 1 {
		t.Fatal("dictionary lost its key or nested array after releasing caller references")
	}
	data := CFArrayGetValueAtIndex(stored, 0)
	if !bytes.Equal(unsafe.Slice(getBytes(data), getLength(data)), []byte("collection value")) {
		t.Fatal("array lost its value after releasing the caller reference")
	}
}
