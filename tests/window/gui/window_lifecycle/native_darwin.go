package main

import (
	"github.com/golang-gui/goui/platform/darwin/frameworks/appkit"
	"github.com/golang-gui/goui/platform/darwin/frameworks/foundation"
)

func destroyHost(handle uintptr) error {
	// 真正主线程调用 close，跳过 shouldClose；外层 pool 覆盖整个 AppKit close 栈。
	foundation.AutoReleasePool(func() { var w appkit.NSWindow; w.ID = foundation.ID(handle); w.Close() })
	return nil
}
