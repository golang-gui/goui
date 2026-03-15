package foundation

import (
	"unsafe"

	"github.com/ebitengine/purego/objc"
	"github.com/golang-gui/goui/core/cgo"
	"github.com/golang-gui/goui/platform/cocoa/frameworks/common"
	"github.com/golang-gui/goui/platform/cocoa/frameworks/core_graphics"
	"github.com/golang-gui/goui/platform/cocoa/frameworks/objcrt"
)

var handle uintptr

func Init(load common.LoadFunc) (err error) {
	handle, err = load("Foundation")
	if err != nil {
		return
	}

	initNSDate()
	initNSString()
	initNSNotification()

	err = initNSRunLoopMode()
	if err != nil {
		return
	}

	err = initAutoReleasePool()
	if err != nil {
		return
	}

	return
}

// Types

type (
	NSInteger  = int
	NSUInteger = uint
	NSPoint    = core_graphics.CGPoint
	NSSize     = core_graphics.CGSize
	NSRect     = core_graphics.CGRect
)

func NSMakeRect(x, y, w, h core_graphics.CGFloat) (r NSRect) {
	return NSRect{
		Origin: NSPoint{x, y},
		Size:   NSSize{w, h},
	}
}

// NSDate

func initNSDate() {
	NSDateClassId = NSDateClass(objc.GetClass("NSDate"))
	NSDateSel.DistantFuture = objc.RegisterName("distantFuture")
	NSDateSel.DistantPast = objc.RegisterName("distantPast")
}

var (
	NSDateClassId NSDateClass
	NSDateSel     struct {
		DistantFuture objc.SEL
		DistantPast   objc.SEL
	}
)

type (
	NSDate      objcrt.NSObject
	NSDateClass objc.Class
)

func (c NSDateClass) DistantFuture() NSDate {
	return NSDate(objc.ID(c).Send(NSDateSel.DistantFuture))
}

func (c NSDateClass) DistantPast() NSDate {
	return NSDate(objc.ID(c).Send(NSDateSel.DistantPast))
}

type NSTimeInterval = float64

// NSString

func initNSString() {
	NSStringClassId = NSStringClass(objc.GetClass("NSString"))
	NSStringSel.InitWithBytes = objc.RegisterName("initWithBytes:length:encoding:")
	NSStringSel.Utf8String = objc.RegisterName("UTF8String")
}

var (
	NSStringClassId NSStringClass
	NSStringSel     struct {
		InitWithBytes objc.SEL
		Utf8String    objc.SEL
	}
)

type (
	NSString      objcrt.NSObject
	NSStringClass objc.Class
)

func (c NSStringClass) Alloc() NSString {
	return (NSString)(objc.ID(c).Send(objcrt.NSObjectSel.Alloc))
}

func (s NSString) InitWithBytes(bytes []byte, encoding NSStringEncoding) NSString {
	ret := objc.ID(s).Send(NSStringSel.InitWithBytes, cgo.CSlice(bytes), len(bytes), encoding)
	return NSString(ret)
}

func (s NSString) UTF8String() string {
	cStr := objc.ID(s).Send(NSStringSel.Utf8String)
	if cStr != 0 {
		return cgo.GoString(cgo.Pointer(cStr))
	}
	return ""
}

func ToNSString(s string) NSString {
	ns := objc.ID(NSStringClassId.Alloc()).Send(NSStringSel.InitWithBytes, unsafe.StringData(s), len(s), NSUTF8StringEncoding)
	return NSString(ns)
}

type NSStringEncoding NSUInteger

const NSUTF8StringEncoding NSStringEncoding = 4

// NSNotification

func initNSNotification() {
	_NSNotification.Class = objc.GetClass("NSNotification")
	_NSNotification.object = objc.RegisterName("object")
}

var _NSNotification struct {
	objc.Class
	object objc.SEL
}

type NSNotification objcrt.NSObject

func (n NSNotification) Object() objc.ID {
	return objc.ID(n).Send(_NSNotification.object)
}

// AutoReleasePool

func initAutoReleasePool() (err error) {
	err = cgo.RegisterLibFunc(&objc_autoreleasePoolPush, handle, "objc_autoreleasePoolPush")
	if err != nil {
		return
	}
	return cgo.RegisterLibFunc(&objc_autoreleasePoolPop, handle, "objc_autoreleasePoolPop")
}

var (
	objc_autoreleasePoolPush func() uintptr
	objc_autoreleasePoolPop  func(pool uintptr)
)

// @autoreleasepooll
func AutoReleasePool(block func()) {
	pool := objc_autoreleasePoolPush()
	defer objc_autoreleasePoolPop(pool)
	block()
}

// NSRunLoopMode

func initNSRunLoopMode() (err error) {
	var pv *NSRunLoopMode
	if pv, err = cgo.GetExternVariant[NSRunLoopMode](handle, "NSDefaultRunLoopMode"); err != nil {
		return
	}

	NSDefaultRunLoopMode = *pv
	return nil
}

type NSRunLoopMode NSString

var NSDefaultRunLoopMode NSRunLoopMode
