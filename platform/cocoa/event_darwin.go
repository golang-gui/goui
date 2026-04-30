package cocoa

import (
	"github.com/golang-gui/goui/platform/cocoa/frameworks/appkit"
	"github.com/golang-gui/goui/platform/cocoa/frameworks/foundation"
)

type EventQueue struct {
}

func newEventQueue() (q EventQueue, err error) {
	return
}

func (q EventQueue) Destroy() {

}

func (q EventQueue) Post() {
	foundation.AutoReleasePool(func() {
		event := appkit.NSEventClassId.OtherEventWithType(appkit.NSEventTypeApplicationDefined, foundation.NSPoint{},
			0, 0, 0, appkit.NSGraphicsContext{}, 0, 0, 0)
		appkit.NSApp.PostEvent(event, true)
	})
}

func (q EventQueue) Poll() {
	foundation.AutoReleasePool(func() {
		event := appkit.NSApp.NextEvent(appkit.NSEventMaskAny, foundation.NSDateClassId.DistantPast(),
			foundation.NSDefaultRunLoopMode, true)
		if event.Valid() {
			appkit.NSApp.SendEvent(event)
		}
	})
}

func (q EventQueue) Wait() {
	foundation.AutoReleasePool(func() {
		event := appkit.NSApp.NextEvent(appkit.NSEventMaskAny, foundation.NSDateClassId.DistantFuture(),
			foundation.NSDefaultRunLoopMode, true)
		appkit.NSApp.SendEvent(event)
	})
}
