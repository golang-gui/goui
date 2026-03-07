package foundation

import (
	"github.com/ebitengine/purego/objc"
	"github.com/golang-gui/goui/platform/cocoa/frameworks/objcrt"
)

var _NSDate struct {
	objc.Class
	distantFuture objc.SEL
	distantPast   objc.SEL
}

func initNSDate() {
	_NSDate.Class = objc.GetClass("NSDate")
	_NSDate.distantFuture = objc.RegisterName("distantFuture")
	_NSDate.distantPast = objc.RegisterName("distantPast")
}

type NSDate objcrt.NSObject

func NSDate_DistantFuture() NSDate {
	return NSDate(objc.ID(_NSDate.Class).Send(_NSDate.distantFuture))
}

func NSDate_DistantPast() NSDate {
	return NSDate(objc.ID(_NSDate.Class).Send(_NSDate.distantPast))
}

type NSTimeInterval = float64
