package wgl

import (
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/windows/sdk/winapi"
	"github.com/golang-gui/goui/platform/windows/win32"
	"strings"
	"testing"
)

func TestOpengl32(t *testing.T) {
	plat, err := win32.NewPlatform()
	if err != nil {
		t.Fatal(err)
	}

	win, err := plat.NewWindow(func(event events.Event) {

	})
	if err != nil {
		t.Fatal(err)
	}

	hdc, err := winapi.GetDC(winapi.HWND(win.NativeHandle()))
	if err != nil {
		t.Fatal(err)
	}

	err = Init(hdc)
	if err != nil {
		t.Fatal(err)
	}

	if GetExtensionsStringEXT != nil {
		extens, _ := GetExtensionsStringEXT()
		for _, ext := range strings.Split(extens, " ") {
			println(ext)
		}
	}
}
