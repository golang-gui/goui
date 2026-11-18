package main

import (
	"fmt"
	"unsafe"

	"github.com/golang-gui/goui/platform"
	"github.com/golang-gui/goui/platform/windows/sdk/com"
	"github.com/golang-gui/goui/platform/windows/sdk/shell"
	"github.com/golang-gui/goui/platform/windows/sdk/winapi"
	"golang.org/x/sys/windows"
)

func checkNativeProcessApplicationIdentity(plat platform.Platform) {
	var got string
	var err error
	func() {
		var id winapi.LPWSTR
		err = shell.GetCurrentProcessExplicitAppUserModelID(&id)
		if err == nil {
			got = windows.UTF16PtrToString(id)
			com.CoTaskMemFree(unsafe.Pointer(id))
		}
	}()
	if err != nil || got != testApplicationID {
		panic(fmt.Errorf("native process ID = %q, %v; want %q", got, err, testApplicationID))
	}
}

func checkIdentity(plat platform.Platform) {
	checkApplicationIdentity(plat)
	checkNativeProcessApplicationIdentity(plat)
}
