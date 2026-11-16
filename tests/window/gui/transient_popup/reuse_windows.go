package main

import (
	"log"
	"runtime"
	"unsafe"

	"github.com/golang-gui/goui/platform"
	"github.com/golang-gui/goui/platform/windows/sdk/winapi"
)

func init() { nativeDPIProbe = verifyPopupDPIRequests }

// 使用真实 HWND 及完整消息路径，独立断言 Popup 几何由显式请求控制。
// 此处不改变系统 DPI；不能代替真实混合 DPI 多屏迁移验收。
func verifyPopupDPIRequests(popup platform.Popup) {
	hwnd := winapi.HWND(popup.NativeHandle())
	logical, pixels, err := popup.ClientSize()
	must(err)
	var before winapi.RECT
	must(winapi.GetWindowRect(hwnd, &before))
	suggested := before
	suggested.Left += 19
	suggested.Right += 92
	suggested.Bottom += 47
	var pin runtime.Pinner
	pin.Pin(&suggested)
	winapi.SendMessage(hwnd, winapi.WM_DPICHANGED, winapi.WPARAM(192|192<<16), winapi.LPARAM(uintptr(unsafe.Pointer(&suggested))))
	pin.Unpin()
	var after winapi.RECT
	must(winapi.GetWindowRect(hwnd, &after))
	actual, backing, err := popup.ClientSize()
	must(err)
	if after != before || actual != logical || backing != pixels {
		log.Fatalf("DPI FAIL 系统建议矩形覆盖了显式 Popup 几何：before=%v after=%v logical=%v actual=%v", before, after, logical, actual)
	}
	log.Printf("DPI PASS alpha=%v explicit geometry preserved", popup.Transparent())
}
