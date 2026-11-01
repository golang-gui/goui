package main

import (
	"fmt"

	"github.com/ebitengine/purego/objc"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/platform/darwin/frameworks/appkit"
	"github.com/golang-gui/goui/platform/darwin/frameworks/foundation"
)

func prepareNativeTrace() error { return nil }

func startNativeTrace(_ gui.Window, out *traceLog, ready func(error)) (func() error, error) {
	block := objc.NewBlock(func(_ objc.Block, id objc.ID) objc.ID {
		e := foundation.Cast[appkit.NSEvent](id)
		kind := e.Type()
		detail := ""
		switch kind {
		case 10, 11, 12: // keyDown、keyUp、flagsChanged。
			detail = fmt.Sprintf(" keycode=%d", e.KeyCode())
			if kind != 12 {
				detail += fmt.Sprintf(" repeat=%t", e.IsARepeat())
			}
		case 22: // scrollWheel；这些属性只在滚动事件上查询。
			detail = fmt.Sprintf(" delta=%g,%g precise=%t phase=%#x momentumPhase=%#x inverted=%t", e.ScrollingDeltaX(), e.ScrollingDeltaY(), e.HasPreciseScrollingDeltas(), e.Phase(), e.MomentumPhase(), e.IsDirectionInvertedFromDevice())
		}
		if kind >= 1 && kind <= 9 || kind == 22 || kind >= 25 && kind <= 27 {
			detail += fmt.Sprintf(" locationInWindow=%v", e.LocationInWindow())
		}
		if kind >= 1 && kind <= 4 || kind == 6 || kind == 7 || kind >= 25 && kind <= 27 {
			detail += fmt.Sprintf(" buttonNumber=%d", e.ButtonNumber())
		}
		out.record("NATIVE", "appkit type=%d time=%.9f windowNumber=%d modifiers=%#x%s", kind, e.Timestamp(), e.WindowNumber(), e.ModifierFlags(), detail)
		return id // 原样返回；不取消、不替换输入。
	})
	monitor := appkit.NSEventClassId.AddLocalMonitor(appkit.NSEventMaskAny, block)
	if !monitor.Valid() {
		block.Release()
		return nil, fmt.Errorf("install NSEvent local monitor failed")
	}
	out.record("ENV", "native=AppKit local NSEvent monitor; AppKit coordinates are bottom-up points, GOUI coordinates are top-down DIP")
	ready(nil)
	return func() error {
		appkit.NSEventClassId.RemoveMonitor(monitor)
		block.Release()
		return nil
	}, nil
}
