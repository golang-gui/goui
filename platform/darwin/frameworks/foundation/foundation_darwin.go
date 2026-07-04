package foundation

import (
	"runtime"
	"unsafe"

	. "github.com/golang-gui/goui/platform/darwin/frameworks/core_graphics"
	"github.com/golang-gui/goui/platform/darwin/frameworks/utils"

	"github.com/ebitengine/purego/objc"
	"github.com/goexlib/cgo"
)

var framework utils.Framework

func InitFoundation() (err error) {
	framework, err = utils.LoadSystemFramework("Foundation")
	if err != nil {
		return
	}

	initNSObject()
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
	ID = objc.ID

	NSInteger  = int
	NSUInteger = uint
	NSPoint    = CGPoint
	NSSize     = CGSize
	NSRect     = CGRect
)

func NSMakeRect(x, y, w, h CGFloat) (r NSRect) {
	return NSRect{
		Origin: NSPoint{x, y},
		Size:   NSSize{w, h},
	}
}

func initNSObject() {
	NSObjectClassId = NSObjectClass{objc.GetClass("NSObject")}
	NSObjectSel.Alloc = objc.RegisterName("alloc")
	NSObjectSel.Init = objc.RegisterName("init")
	NSObjectSel.New = objc.RegisterName("new")
	NSObjectSel.Retain = objc.RegisterName("retain")
	NSObjectSel.Release = objc.RegisterName("release")
	NSObjectSel.Autorelease = objc.RegisterName("autorelease")
	NSObjectSel.RetainCount = objc.RegisterName("retainCount")
	NSObjectSel.RespondsToSelector = objc.RegisterName("respondsToSelector:")
}

var (
	NSObjectClassId NSObjectClass
	NSObjectSel     struct {
		Alloc              objc.SEL
		Init               objc.SEL
		New                objc.SEL
		Retain             objc.SEL
		Release            objc.SEL
		Autorelease        objc.SEL
		RetainCount        objc.SEL
		RespondsToSelector objc.SEL
	}
)

type (
	NSObjectClass struct {
		objc.Class
	}
	NSObject struct {
		objc.ID
	}
	extendNSObject interface {
		isNSObject()
	}
)

func (c NSObjectClass) ID() objc.ID {
	return objc.ID(c.Class)
}

func (c NSObjectClass) Send(sel objc.SEL, args ...any) objc.ID {
	return objc.ID(c.Class).Send(sel, args...)
}

func (c NSObjectClass) Alloc() (o NSObject) {
	return Cast[NSObject](c.Send(NSObjectSel.Alloc))
}

func Cast[T extendNSObject](id objc.ID) (obj T) {
	(*NSObject)(cgo.Pointer(&obj)).ID = id
	return
}

func (o NSObject) isNSObject() {}

func (o NSObject) IsNil() bool {
	return o.ID == 0
}

func (o NSObject) Valid() bool {
	return o.ID != 0
}

func (o NSObject) Retain() {
	o.Send(NSObjectSel.Retain)
}

func (o NSObject) Release() {
	o.Send(NSObjectSel.Release)
}

func (o NSObject) AutoRelease() {
	o.Send(NSObjectSel.Autorelease)
}

func (o NSObject) RetainCount() uint {
	ret := o.Send(NSObjectSel.RetainCount)
	return uint(ret)
}

func (o NSObject) RespondsToSelector(sel string) bool {
	return objc.Send[bool](o.ID, NSObjectSel.RespondsToSelector, objc.RegisterName(sel))
}

// NSDate

func initNSDate() {
	NSDateClassId.Class = objc.GetClass("NSDate")
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
	NSDate      struct{ NSObject }
	NSDateClass struct{ NSObjectClass }
)

func (c NSDateClass) DistantFuture() (date NSDate) {
	date.ID = c.Send(NSDateSel.DistantFuture)
	return
}

func (c NSDateClass) DistantPast() (date NSDate) {
	date.ID = objc.ID(c.Class).Send(NSDateSel.DistantPast)
	return
}

type NSTimeInterval = float64

// NSString

func initNSString() {
	NSStringClassId.Class = objc.GetClass("NSString")
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
	NSString      struct{ NSObject }
	NSStringClass struct{ NSObjectClass }
)

func (c NSStringClass) Alloc() NSString {
	return Cast[NSString](c.NSObjectClass.Alloc().ID)
}

func (s NSString) InitWithBytes(bytes []byte, encoding NSStringEncoding) NSString {
	return Cast[NSString](s.Send(NSStringSel.InitWithBytes, cgo.CSlice(bytes), len(bytes), encoding))
}

func (s NSString) UTF8String() string {
	cStr := s.Send(NSStringSel.Utf8String)
	if cStr != 0 {
		return cgo.GoString(cgo.Pointer(cStr))
	}
	return ""
}

func ToNSString(s string) NSString {
	ns := NSStringClassId.Alloc().Send(NSStringSel.InitWithBytes, unsafe.StringData(s), len(s), NSUTF8StringEncoding)
	return Cast[NSString](ns)
}

type NSStringEncoding NSUInteger

const NSUTF8StringEncoding NSStringEncoding = 4

// NSNotification

func initNSNotification() {
	NSNotificationClassId.Class = objc.GetClass("NSNotification")
	NSNotificationSel.Object = objc.RegisterName("object")
}

var (
	NSNotificationSel struct {
		Object objc.SEL
	}
	NSNotificationClassId NSNotificationClass
)

type (
	NSNotification      struct{ NSObject }
	NSNotificationClass struct{ NSObjectClass }
)

func (n NSNotification) Object() objc.ID {
	return n.Send(NSNotificationSel.Object)
}

// AutoReleasePool

func initAutoReleasePool() (err error) {
	return framework.LoadFunctions([]utils.Function{
		{Name: "objc_autoreleasePoolPush", PFunc: &objc_autoreleasePoolPush},
		{Name: "objc_autoreleasePoolPop", PFunc: &objc_autoreleasePoolPop},
	})
}

var (
	objc_autoreleasePoolPush func() uintptr
	objc_autoreleasePoolPop  func(pool uintptr)
)

// @autoreleasepool
//
// The autorelease pool is thread-local: the token returned by push must be
// handed to pop on the same OS thread. A goroutine that has not locked its OS
// thread can be migrated between threads at any cgo-call boundary (including the
// calls inside block), so pin it for the pool's lifetime. Without this, pop may
// run on a different thread than push, corrupt that thread's pool stack, and
// crash inside objc_autoreleasePoolPop. LockOSThread nests safely, so callers
// already on a locked thread (e.g. the main UI thread) are unaffected.
func AutoReleasePool(block func()) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	pool := objc_autoreleasePoolPush()
	defer objc_autoreleasePoolPop(pool)
	block()
}

// NSRunLoopMode

func initNSRunLoopMode() (err error) {
	return framework.LoadConstants([]utils.Constant{
		utils.Const[NSRunLoopMode]{Name: "NSDefaultRunLoopMode", PVar: &NSDefaultRunLoopMode},
	})
}

type NSRunLoopMode NSString

var NSDefaultRunLoopMode NSRunLoopMode
