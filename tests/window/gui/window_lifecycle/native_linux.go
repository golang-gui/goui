package main

import (
	"fmt"
	"unsafe"

	"github.com/golang-gui/goui/platform/linux/libs/xlib"
)

func destroyHost(handle uintptr) error {
	// 核对 XDestroyWindowEvent 的 LP64 ABI，避免 window 字段错位。
	if unsafe.Sizeof(xlib.DestroyWindowEvent{}) != 48 || unsafe.Offsetof(xlib.DestroyWindowEvent{}.Window) != 40 {
		return fmt.Errorf("XDestroyWindowEvent ABI mismatch")
	}
	// 新连接代表另一个 X client。Close 等待本连接请求完成，不使用 XKillClient。
	display := xlib.OpenDisplay("")
	if display == 0 {
		return fmt.Errorf("open external X client failed")
	}
	display.DestroyWindow(xlib.Window(handle))
	display.Close()
	return nil
}
