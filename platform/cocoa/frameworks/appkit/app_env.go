package appkit

import (
	"github.com/ebitengine/purego/objc"
	"github.com/golang-gui/goui/platform/cocoa/frameworks/foundation"
)

func initNSApplication() {
	_NSApplication.Class = objc.GetClass("NSApplication")
	_NSApplication.sharedApplication = objc.RegisterName("sharedApplication")
	_NSApplication.sendEvent = objc.RegisterName("sendEvent:")
	_NSApplication.postEvent = objc.RegisterName("postEvent:atStart:")
	_NSApplication.nextEvent = objc.RegisterName("nextEventMatchingMask:untilDate:inMode:dequeue:")
}

var _NSApplication struct {
	objc.Class
	sharedApplication objc.SEL
	sendEvent         objc.SEL
	postEvent         objc.SEL
	nextEvent         objc.SEL
}

type NSApplication objc.ID

var NSApp NSApplication

func NSApplication_SharedApplication() NSApplication {
	NSApp = NSApplication(objc.ID(_NSApplication.Class).Send(_NSApplication.sharedApplication))
	return NSApp
}

func (a NSApplication) SendEvent(event NSEvent) {
	objc.ID(a).Send(_NSApplication.sendEvent, event)
}

func (a NSApplication) PostEvent(event NSEvent, atStart bool) {
	objc.ID(a).Send(_NSApplication.postEvent, event, atStart)
}

func (a NSApplication) NextEvent(mask NSEventMask, untilDate foundation.NSDate, inMode foundation.NSRunLoopMode, dequeue bool) NSEvent {
	event := objc.ID(a).Send(_NSApplication.nextEvent, mask, untilDate, inMode, dequeue)
	return NSEvent(event)
}
