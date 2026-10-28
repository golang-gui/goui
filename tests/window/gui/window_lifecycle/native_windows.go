package main

import (
	"fmt"

	"github.com/golang-gui/goui/platform/windows/sdk/winapi"
)

func destroyHost(handle uintptr) error {
	// 与窗口同一个 GUI 线程，真实触发 WM_DESTROY/WM_NCDESTROY。
	if winapi.DestroyWindow(winapi.HWND(handle)) == 0 {
		return fmt.Errorf("native DestroyWindow failed")
	}
	return nil
}
