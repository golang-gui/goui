// macOS 专属的原生输入辅助；通过 AppKit 事件队列验证菜单等价键的 Repeat，
// 不直接调用 GUI 分发或业务动作，也不依赖外部输入源是否生成自动重复。
package main

import (
	"github.com/ebitengine/purego/objc"
	"github.com/golang-gui/goui/gui"
	. "github.com/golang-gui/goui/platform/darwin/frameworks/appkit"
	. "github.com/golang-gui/goui/platform/darwin/frameworks/foundation"
)

func init() {
	postNativeRepeats = func(host gui.Window) {
		AutoReleasePool(func() {
			window := Cast[NSWindow](objc.ID(host.PlatformWindow().NativeHandle()))
			text := ToNSString("1")
			defer text.Release()
			for range 3 {
				event := NSEventClassId.KeyEventWithType(NSEventTypeKeyDown, NSPoint{}, NSEventModifierFlagCommand, 0, window.WindowNumber(), NSGraphicsContext{}, text, text, true, 18)
				require(event.Valid() && event.IsARepeat() && event.KeyCode() == 18 && event.ModifierFlags() == NSEventModifierFlagCommand && event.CharactersIgnoringModifiers().UTF8String() == "1", "原生重复事件字段不完整")
				NSApp.PostEvent(event, false)
			}
		})
	}
}
