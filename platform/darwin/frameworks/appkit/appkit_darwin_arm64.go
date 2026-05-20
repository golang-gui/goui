package appkit

import (
	"github.com/ebitengine/purego/objc"
	. "github.com/golang-gui/goui/platform/darwin/frameworks/foundation"
)

func makeNSViewDrawRect(f func(self NSView, rect NSRect)) any {
	// aarch64 use double-register put floats-struct argument
	return func(self objc.ID, cmd objc.SEL, x, y, w, h float64) {
		f(Cast[NSView](self), NSMakeRect(x, y, w, h))
	}
}
