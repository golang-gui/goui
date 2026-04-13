package appkit

import (
	"github.com/ebitengine/purego/objc"
	"github.com/golang-gui/goui/platform/cocoa/frameworks/foundation"
)

func makeNSViewDrawRect(f func(self NSView, rect foundation.NSRect)) any {
	// aarch64 use double-register put floats-struct argument
	return func(self objc.ID, cmd objc.SEL, x, y, w, h float64) {
		f(foundation.Cast[NSView](self), foundation.NSRect{
			Origin: foundation.NSPoint{
				X: x,
				Y: y,
			},
			Size: foundation.NSSize{
				Width:  w,
				Height: h,
			},
		})
	}
}
