package cocoa

import (
	"github.com/golang-gui/goui/platform/common"

	. "github.com/golang-gui/goui/platform/darwin/frameworks/appkit"
	. "github.com/golang-gui/goui/platform/darwin/frameworks/core_graphics"
	. "github.com/golang-gui/goui/platform/darwin/frameworks/foundation"
)

// pointsPerLogicalUnit maps GOUI coordinates to AppKit points. A scale override
// changes logical units per backing pixel; it cannot resize the native backing
// store independently of the view. Without an override, one logical unit is
// one AppKit point, including on Retina displays.
func pointsPerLogicalUnit(window NSWindow) CGFloat {
	if scale := common.GetPreferScale(); scale > 0 {
		return CGFloat(scale) / window.BackingScaleFactor()
	}
	return 1
}

func logicalContentSize(window NSWindow, width, height float32) NSSize {
	scale := pointsPerLogicalUnit(window)
	return NSSize{Width: CGFloat(width) * scale, Height: CGFloat(height) * scale}
}
