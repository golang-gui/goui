//go:build !ios

package quartzcore

import (
	"github.com/ebitengine/purego/objc"
	. "github.com/golang-gui/goui/platform/darwin/frameworks/foundation"
	"github.com/golang-gui/goui/platform/darwin/frameworks/utils"
)

var CALayerSel struct{ SetNeedsDisplay, DisplayIfNeeded objc.SEL }

func InitQuartzCore() error {
	if _, err := utils.LoadSystemFramework("QuartzCore"); err != nil {
		return err
	}
	CALayerSel.SetNeedsDisplay = objc.RegisterName("setNeedsDisplay")
	CALayerSel.DisplayIfNeeded = objc.RegisterName("displayIfNeeded")
	return nil
}

type CALayer struct{ NSObject }

func (l CALayer) SetNeedsDisplay() { l.Send(CALayerSel.SetNeedsDisplay) }
func (l CALayer) DisplayIfNeeded() { l.Send(CALayerSel.DisplayIfNeeded) }
