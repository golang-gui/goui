package main

import (
	"fmt"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform"
	"github.com/golang-gui/goui/platform/graphics/direct2d"
	"github.com/golang-gui/goui/platform/graphics/opengl"
	"github.com/golang-gui/goui/platform/windows/sdk/winapi"
)

type alphaHandle struct{ hwnd uintptr }

func (w alphaHandle) NativeHandle() uintptr { return w.hwnd }
func (alphaHandle) Transparent() bool       { return true }
func (alphaHandle) RequestPaint() error     { return nil }

func checkTransparentBackendOwnsWindowStyle(plat platform.Platform) {
	for _, backend := range []string{"direct2d", "opengl"} {
		for _, popup := range []bool{false, true} {
			func() {
				fmt.Println("CHECK", fmt.Sprintf("%s/popup=%v", backend, popup))
				var testErr error
				func() {
					testErr = checkBackendWindowStyle(plat, backend, popup)
				}()
				if testErr != nil {
					panic(testErr)
				}
			}()
		}
	}
}

func checkBackendWindowStyle(plat platform.Platform, backend string, usePopup bool) error {
	win, err := plat.NewWindow(geometry.Size{Width: 160, Height: 120}, func(platform.Event) {},
		platform.WindowOptions{Transparent: true})
	if err != nil {
		return err
	}
	defer win.Destroy()
	var surface platform.Surface = win
	if usePopup {
		popup, err := plat.NewPopup(win, geometry.Size{Width: 80, Height: 60}, func(platform.Event) {}, platform.PopupOptions{Transparent: true})
		if err != nil {
			return err
		}
		defer popup.Destroy()
		surface = popup
	}
	hwnd := winapi.HWND(surface.NativeHandle())
	before, err := winapi.GetWindowLong(hwnd, winapi.GWL_EXSTYLE)
	if err != nil {
		return err
	}
	if before&winapi.WS_EX_NOREDIRECTIONBITMAP != 0 {
		return fmt.Errorf("window constructor selected a composition-only style")
	}
	handle := alphaHandle{hwnd: surface.NativeHandle()}
	want := before
	if backend == "direct2d" {
		painter, err := direct2d.NewPainter(handle)
		if err != nil {
			return err
		}
		defer painter.Destroy()
		want |= winapi.WS_EX_NOREDIRECTIONBITMAP
	} else {
		ctx, err := opengl.NewContext(handle, nil, opengl.DefaultConfig)
		if err != nil {
			return err
		}
		defer ctx.Destroy()
	}
	after, err := winapi.GetWindowLong(hwnd, winapi.GWL_EXSTYLE)
	if err != nil {
		return err
	}
	if after != want {
		return fmt.Errorf("extended style=%#x, want %#x", after, want)
	}
	return nil
}

func checkNativeAlpha(plat platform.Platform) { checkTransparentBackendOwnsWindowStyle(plat) }
