package appkit

import (
	"github.com/ebitengine/purego/objc"
	"github.com/golang-gui/goui/core/cgo"
	"github.com/golang-gui/goui/platform/cocoa/frameworks/foundation"
	"github.com/golang-gui/goui/platform/cocoa/frameworks/objcrt"
)

var _NSEvent struct {
	objc.Class
	otherEventWithType objc.SEL
}

func initNSEvent() {
	_NSEvent.Class = objc.GetClass("NSEvent")
	_NSEvent.otherEventWithType = objc.RegisterName("otherEventWithType:location:modifierFlags:timestamp:windowNumber:context:subtype:data1:data2:")
}

type NSEvent objcrt.NSObject

type NSEventType foundation.NSUInteger

// TODO: event types
const NSEventTypeApplicationDefined NSEventType = 15

type NSEventModifierFlags int

func NSEvent_otherEventWithType(eventType NSEventType, location foundation.NSPoint, modifierFlags NSEventModifierFlags, timestamp foundation.NSTimeInterval, windowNumber foundation.NSInteger, context NSGraphicsContext, subtype cgo.Short, data1, data2 foundation.NSInteger) NSEvent {
	event := objc.ID(_NSEvent.Class).Send(_NSEvent.otherEventWithType, eventType, location, modifierFlags, timestamp, windowNumber, context, subtype, data1, data2)
	return NSEvent(event)
}

type NSEventMask uint

// TODO: other event mask
const NSEventMaskAny NSEventMask = (9223372036854775807*2 + 1)
