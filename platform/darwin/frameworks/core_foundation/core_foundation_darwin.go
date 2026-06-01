package core_foundation

import (
	"runtime"
	"unsafe"

	"github.com/golang-gui/goui/platform/darwin/frameworks/utils"

	"github.com/goexlib/cgo"
)

var framework utils.Framework

func InitCoreFoundation() (err error) {
	framework, err = utils.LoadSystemFramework("CoreFoundation")
	if err != nil {
		return
	}

	err = framework.LoadFunctions(functions)
	if err != nil {
		return
	}

	err = framework.LoadConstants(constants)
	if err != nil {
		return
	}

	return nil
}

type (
	CFIndex = int
	Uint8   = uint8
)

type CFTypeRef uintptr

func CFRelease(ref CFTypeRef) {
	fnCFRelease(ref)
}

type (
	CFDataRef      = CFTypeRef
	CFAllocatorRef = CFTypeRef
)

func CFDataCreate(allocator CFAllocatorRef, bytes []byte) CFDataRef {
	return fnCFDataCreate(allocator, cgo.CSlice(bytes), CFIndex(len(bytes)))
}

type CFNumberRef = CFTypeRef

type CFNumberType int32

const (
	KCFNumberSInt8Type    CFNumberType = 1
	KCFNumberSInt16Type   CFNumberType = 2
	KCFNumberSInt32Type   CFNumberType = 3
	KCFNumberSInt64Type   CFNumberType = 4
	KCFNumberFloat32Type  CFNumberType = 5
	KCFNumberFloat64Type  CFNumberType = 6
	KCFNumberCharType     CFNumberType = 7
	KCFNumberShortType    CFNumberType = 8
	KCFNumberIntType      CFNumberType = 9
	KCFNumberLongType     CFNumberType = 10
	KCFNumberLongLongType CFNumberType = 11
	KCFNumberFloatType    CFNumberType = 12
	KCFNumberDoubleType   CFNumberType = 13
	KCFNumberCGFloatType  CFNumberType = 16
)

func CFNumberCreateFloat64(value float64) CFNumberRef {
	return fnCFNumberCreate(0, KCFNumberFloat64Type, unsafe.Pointer(&value))
}

func CFNumberCreateInt32(value int32) CFNumberRef {
	return fnCFNumberCreate(0, KCFNumberSInt32Type, unsafe.Pointer(&value))
}

func CFNumberCreateInt(value int) CFNumberRef {
	v := int64(value)
	return fnCFNumberCreate(0, KCFNumberSInt64Type, unsafe.Pointer(&v))
}

type CFStringRef = CFTypeRef

type CFStringEncoding uint32

const (
	KCFStringEncodingUTF8    CFStringEncoding = 0x08000100
	KCFStringEncodingASCII   CFStringEncoding = 0x0600
	KCFStringEncodingUTF16   CFStringEncoding = 0x0100
	KCFStringEncodingUTF16BE CFStringEncoding = 0x10000100
	KCFStringEncodingUTF16LE CFStringEncoding = 0x14000100
)

func CFStringCreateWithBytes(bytes []byte, encoding CFStringEncoding) CFStringRef {
	if len(bytes) == 0 {
		return fnCFStringCreateWithBytes(0, nil, 0, encoding, false)
	}
	return fnCFStringCreateWithBytes(0, unsafe.Pointer(&bytes[0]), CFIndex(len(bytes)), encoding, false)
}

func CFStringCreateWithString(s string) CFStringRef {
	if len(s) == 0 {
		return fnCFStringCreateWithBytes(0, nil, 0, KCFStringEncodingUTF8, false)
	}
	ret := fnCFStringCreateWithBytes(0, unsafe.Pointer(unsafe.StringData(s)), CFIndex(len(s)), KCFStringEncodingUTF8, false)
	runtime.KeepAlive(s)
	return ret
}

func CFStringGetLength(str CFStringRef) int {
	return int(fnCFStringGetLength(str))
}

func CFStringToString(str CFStringRef) string {
	if str == 0 {
		return ""
	}
	ptr := fnCFStringGetCStringPtr(str, KCFStringEncodingUTF8)
	if ptr != 0 {
		return cgo.GoString(cgo.Pointer(ptr))
	}
	length := fnCFStringGetLength(str)
	bufSize := length*4 + 1
	buf := make([]byte, bufSize)
	if fnCFStringGetCString(str, unsafe.Pointer(&buf[0]), bufSize, KCFStringEncodingUTF8) {
		return cgo.GoString(cgo.Pointer(&buf[0]))
	}
	return ""
}

type (
	CFAttributedStringRef        = CFTypeRef
	CFMutableAttributedStringRef = CFTypeRef
	CFMutableStringRef           = CFTypeRef
)

type CFRange struct {
	Location CFIndex
	Length   CFIndex
}

func CFRangeMake(location, length int) CFRange {
	return CFRange{Location: CFIndex(location), Length: CFIndex(length)}
}

func CFAttributedStringCreate(str CFStringRef, attributes CFDictionaryRef) CFAttributedStringRef {
	return fnCFAttributedStringCreate(0, str, attributes)
}

func CFAttributedStringCreateMutable(allocator CFAllocatorRef, maxLength int) CFMutableAttributedStringRef {
	return fnCFAttributedStringCreateMutable(allocator, CFIndex(maxLength))
}

func CFAttributedStringCreateMutableCopy(allocator CFAllocatorRef, maxLength int, aStr CFAttributedStringRef) CFMutableAttributedStringRef {
	return fnCFAttributedStringCreateMutableCopy(allocator, CFIndex(maxLength), aStr)
}

func CFAttributedStringReplaceString(aStr CFMutableAttributedStringRef, r CFRange, replacement CFStringRef) {
	fnCFAttributedStringReplaceString(aStr, r, replacement)
}

func CFAttributedStringSetAttribute(aStr CFMutableAttributedStringRef, r CFRange, attrName CFStringRef, value CFTypeRef) {
	fnCFAttributedStringSetAttribute(aStr, r, attrName, value)
}

func CFAttributedStringGetLength(aStr CFAttributedStringRef) int {
	return int(fnCFAttributedStringGetLength(aStr))
}

func CFAttributedStringBeginEditing(aStr CFMutableAttributedStringRef) {
	fnCFAttributedStringBeginEditing(aStr)
}

func CFAttributedStringEndEditing(aStr CFMutableAttributedStringRef) {
	fnCFAttributedStringEndEditing(aStr)
}

type CFURLRef = CFTypeRef
type CFURLPathStyle int32

const (
	KCFURLPOSIXPathStyle   CFURLPathStyle = 0
	KCFURLWindowsPathStyle CFURLPathStyle = 2
)

func CFURLCreateWithFileSystemPath(filePath string, pathStyle CFURLPathStyle, isDirectory bool) CFURLRef {
	cfStr := CFStringCreateWithString(filePath)
	defer CFRelease(cfStr)
	return fnCFURLCreateWithFileSystemPath(0, cfStr, pathStyle, isDirectory)
}

// CFArrayRef

type CFArrayRef = CFTypeRef

func CFArrayCreate(values []CFTypeRef) CFArrayRef {
	if len(values) == 0 {
		return fnCFArrayCreate(0, nil, 0, kCFTypeArrayCallBacks)
	}
	return fnCFArrayCreate(0, unsafe.Pointer(&values[0]), CFIndex(len(values)), kCFTypeArrayCallBacks)
}

func CFArrayGetCount(theArray CFArrayRef) int {
	return int(fnCFArrayGetCount(theArray))
}

func CFArrayGetValueAtIndex(theArray CFArrayRef, idx int) CFTypeRef {
	return CFTypeRef(uintptr(fnCFArrayGetValueAtIndex(theArray, CFIndex(idx))))
}

type CFDictionaryRef = CFTypeRef

func CFDictionaryCreate(keys []CFTypeRef, values []CFTypeRef) CFDictionaryRef {
	count := CFIndex(len(keys))
	if count == 0 {
		return fnCFDictionaryCreate(0, nil, nil, 0, kCFTypeDictionaryKeyCallBacks, kCFTypeDictionaryValueCallBacks)
	}
	return fnCFDictionaryCreate(0, unsafe.Pointer(&keys[0]), unsafe.Pointer(&values[0]), count, kCFTypeDictionaryKeyCallBacks, kCFTypeDictionaryValueCallBacks)
}

type CFBooleanRef = CFTypeRef

type CFErrorRef = CFTypeRef

func CFErrorGetCode(err CFErrorRef) CFIndex {
	return fnCFErrorGetCode(err)
}

// constants

var (
	KCFAllocatorDefault       CFAllocatorRef
	KCFAllocatorSystemDefault CFAllocatorRef
	KCFAllocatorMalloc        CFAllocatorRef
	KCFAllocatorMallocZone    CFAllocatorRef
	KCFAllocatorNull          CFAllocatorRef
	KCFAllocatorUseContext    CFAllocatorRef
)

var (
	KCFBooleanTrue  CFBooleanRef
	KCFBooleanFalse CFBooleanRef
)

var (
	kCFTypeDictionaryKeyCallBacks   uintptr
	kCFTypeDictionaryValueCallBacks uintptr
)

var kCFTypeArrayCallBacks uintptr

var constants = []utils.Constant{
	utils.Const[CFAllocatorRef]{Name: "kCFAllocatorDefault", PVar: &KCFAllocatorDefault},
	utils.Const[CFAllocatorRef]{Name: "kCFAllocatorSystemDefault", PVar: &KCFAllocatorSystemDefault},
	utils.Const[CFAllocatorRef]{Name: "kCFAllocatorMalloc", PVar: &KCFAllocatorMalloc},
	utils.Const[CFAllocatorRef]{Name: "kCFAllocatorMallocZone", PVar: &KCFAllocatorMallocZone},
	utils.Const[CFAllocatorRef]{Name: "kCFAllocatorNull", PVar: &KCFAllocatorNull},
	utils.Const[CFAllocatorRef]{Name: "kCFAllocatorUseContext", PVar: &KCFAllocatorUseContext},

	utils.Const[CFBooleanRef]{Name: "kCFBooleanTrue", PVar: &KCFBooleanTrue},
	utils.Const[CFBooleanRef]{Name: "kCFBooleanFalse", PVar: &KCFBooleanFalse},

	utils.Const[uintptr]{Name: "kCFTypeDictionaryKeyCallBacks", PVar: &kCFTypeDictionaryKeyCallBacks},
	utils.Const[uintptr]{Name: "kCFTypeDictionaryValueCallBacks", PVar: &kCFTypeDictionaryValueCallBacks},

	utils.Const[uintptr]{Name: "kCFTypeArrayCallBacks", PVar: &kCFTypeArrayCallBacks},
}

// functions

var functions = []utils.Function{
	{Name: "CFRelease", PFunc: &fnCFRelease},
	{Name: "CFDataCreate", PFunc: &fnCFDataCreate},

	{Name: "CFNumberCreate", PFunc: &fnCFNumberCreate},

	{Name: "CFDictionaryCreate", PFunc: &fnCFDictionaryCreate},

	{Name: "CFStringCreateWithBytes", PFunc: &fnCFStringCreateWithBytes},
	{Name: "CFStringGetLength", PFunc: &fnCFStringGetLength},
	{Name: "CFStringGetCStringPtr", PFunc: &fnCFStringGetCStringPtr},
	{Name: "CFStringGetCString", PFunc: &fnCFStringGetCString},

	{Name: "CFAttributedStringCreate", PFunc: &fnCFAttributedStringCreate},
	{Name: "CFAttributedStringCreateMutable", PFunc: &fnCFAttributedStringCreateMutable},
	{Name: "CFAttributedStringCreateMutableCopy", PFunc: &fnCFAttributedStringCreateMutableCopy},
	{Name: "CFAttributedStringReplaceString", PFunc: &fnCFAttributedStringReplaceString},
	{Name: "CFAttributedStringSetAttribute", PFunc: &fnCFAttributedStringSetAttribute},
	{Name: "CFAttributedStringGetLength", PFunc: &fnCFAttributedStringGetLength},
	{Name: "CFAttributedStringGetMutableString", PFunc: &fnCFAttributedStringGetMutableString},
	{Name: "CFAttributedStringBeginEditing", PFunc: &fnCFAttributedStringBeginEditing},
	{Name: "CFAttributedStringEndEditing", PFunc: &fnCFAttributedStringEndEditing},

	{Name: "CFArrayCreate", PFunc: &fnCFArrayCreate},
	{Name: "CFArrayGetCount", PFunc: &fnCFArrayGetCount},
	{Name: "CFArrayGetValueAtIndex", PFunc: &fnCFArrayGetValueAtIndex},

	{Name: "CFURLCreateWithFileSystemPath", PFunc: &fnCFURLCreateWithFileSystemPath},

	{Name: "CFErrorGetCode", PFunc: &fnCFErrorGetCode},
}

var (
	fnCFRelease    func(ref CFTypeRef)
	fnCFDataCreate func(allocator CFAllocatorRef, bytes cgo.Pointer, length CFIndex) CFDataRef

	fnCFNumberCreate func(allocator CFAllocatorRef, theType CFNumberType, valuePtr unsafe.Pointer) CFNumberRef

	fnCFDictionaryCreate func(allocator CFAllocatorRef, keys unsafe.Pointer, values unsafe.Pointer, numValues CFIndex, keyCallBacks uintptr, valueCallBacks uintptr) CFDictionaryRef

	fnCFStringCreateWithBytes func(alloc CFAllocatorRef, bytes unsafe.Pointer, numBytes CFIndex, encoding CFStringEncoding, isExternalRepresentation bool) CFStringRef
	fnCFStringGetLength       func(str CFStringRef) CFIndex
	fnCFStringGetCStringPtr   func(str CFStringRef, encoding CFStringEncoding) uintptr
	fnCFStringGetCString      func(str CFStringRef, buffer unsafe.Pointer, bufferSize CFIndex, encoding CFStringEncoding) bool

	fnCFAttributedStringCreate            func(alloc CFAllocatorRef, str CFStringRef, attributes CFDictionaryRef) CFAttributedStringRef
	fnCFAttributedStringCreateMutable     func(alloc CFAllocatorRef, maxLength CFIndex) CFMutableAttributedStringRef
	fnCFAttributedStringCreateMutableCopy func(alloc CFAllocatorRef, maxLength CFIndex, aStr CFAttributedStringRef) CFMutableAttributedStringRef
	fnCFAttributedStringReplaceString     func(aStr CFMutableAttributedStringRef, r CFRange, replacement CFStringRef)
	fnCFAttributedStringSetAttribute      func(aStr CFMutableAttributedStringRef, r CFRange, attrName CFStringRef, value CFTypeRef)
	fnCFAttributedStringGetLength         func(aStr CFAttributedStringRef) CFIndex
	fnCFAttributedStringGetMutableString  func(aStr CFMutableAttributedStringRef) CFMutableStringRef
	fnCFAttributedStringBeginEditing      func(aStr CFMutableAttributedStringRef)
	fnCFAttributedStringEndEditing        func(aStr CFMutableAttributedStringRef)

	fnCFArrayCreate          func(allocator CFAllocatorRef, values unsafe.Pointer, numValues CFIndex, callBacks uintptr) CFArrayRef
	fnCFArrayGetCount        func(theArray CFArrayRef) CFIndex
	fnCFArrayGetValueAtIndex func(theArray CFArrayRef, idx CFIndex) unsafe.Pointer

	fnCFURLCreateWithFileSystemPath func(allocator CFAllocatorRef, filePath CFStringRef, pathStyle CFURLPathStyle, isDirectory bool) CFURLRef

	fnCFErrorGetCode func(err CFErrorRef) CFIndex
)
