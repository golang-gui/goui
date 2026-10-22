// Build-resource and application-packaging desktop checks, without DevServer.
// Environment: Windows x64, macOS arm64 or Linux X11 x64/arm64. Record OS/WM,
// backend and scale. Build from the repository root:
//
//	go run ./cmd/goui build -config tests/window/tools/packaging/goui.toml -target <OS/ARCH> -o <absolute-binary-path>
//
// The test icon has red/translucent green rectangles and transparent top/bottom
// margins, not the project logo. Initially a Native 520x240 DIP window displays
// the packaging-test title, text and a close button.
// Steps and expected results:
//
//  1. Windows: build windows/amd64. Explorer large/small icons use the test asset;
//     Properties/Details shows version 0.1.0.1 and the configured product/description.
//     Double-click opens no console. Caption/taskbar icons use the resource.
//     A new EXE filename may be needed to bypass the icon cache.
//
//  2. macOS: build darwin/arm64, then run:
//
//     go run ./cmd/goui package -config tests/window/tools/packaging/goui.toml -target darwin/arm64 -format app -binary <absolute-binary-path> -o <new-absolute-dir>/Packaging.app
//
//     plutil -lint <app>/Contents/Info.plist succeeds, Finder/Dock show the icon,
//     and open <app> launches without a Bundle ID mismatch. Bare Mach-O promises
//     no Dock icon. This sample has no signing identity; it does not verify
//     signing, notarization or Gatekeeper distribution.
//
//  3. Linux: build for the chosen architecture, then run (amd64 example):
//
//     go run ./cmd/goui package -config tests/window/tools/packaging/goui.toml -target linux/amd64 -format dir -binary <absolute-binary-path> -o <new-absolute-dir>
//
//     desktop-file-validate <dir>/usr/share/applications/org.golang-gui.PackagingTest.desktop
//     succeeds; usr/bin/goui-packaging-test runs. xprop WM_CLASS reports
//     res_name=goui-packaging-test and res_class=org.golang-gui.PackagingTest.
//     Launcher checks require separate, explicitly approved manual installation
//     to the matching usr paths and an icon-cache refresh. An unpacked directory
//     does not register itself. X11 does not set _NET_WM_ICON here, so title-bar
//     icons may differ between WMs.
//
//  4. Resize and close: normal painting, input and exit. Startup alone does not
//     verify icon integration.
//
// Platform differences: native decorations, icon caches and desktop associations
// vary; no ELF file icon is promised. Side effects: build/generate creates Windows
// syso/digests beside main and defaults to dist; put -o outputs outside the repo.
// package does not install, elevate or build; macOS signing runs only when
// configured. Reset: restart;
// if manually installed on Linux, record exact paths and remove only those files.
// Exit: close button or native close. -auto-close=2s probes startup only.
package main

import (
	"flag"
	"fmt"
	"runtime"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
)

func main() {
	runtime.LockOSThread()
	autoClose := flag.Duration("auto-close", 0, "自动关闭延时；0 表示手工验收")
	flag.Parse()
	app, err := gui.NewApplication("org.golang-gui.PackagingTest")
	check(err)
	win, err := app.NewWindow(&gui.WindowOptions{Size: geometry.Size{Width: 520, Height: 240}, Chrome: gui.WindowChromeNative})
	check(err)
	check(win.SetTitle("GOUI 打包验证"))
	box := gui.NewLinearBox(layout.DirectionVertical)
	box.SetSpacing(12)
	box.AddChild(gui.NewLabel("应用 ID: org.golang-gui.PackagingTest"))
	box.AddChild(gui.NewLabel("请检查 Explorer / Finder / 启动器的名称和图标。"))
	closeButton := gui.NewButton()
	closeButton.SetChild(gui.NewLabel("关闭"))
	closeButton.ConnectClicked(func() { check(win.RequestClose()) })
	box.AddChild(closeButton)
	win.SetWidget(box)
	check(win.Show())
	if *autoClose > 0 {
		timer := app.NewTimer()
		timer.ConnectTimeout(func() { win.Destroy() })
		check(timer.StartOnce(*autoClose))
	}
	fmt.Println("START: verify native icons separately; auto-close:", *autoClose)
	app.Run()
	fmt.Println("EXIT")
}

func check(err error) {
	if err != nil {
		panic(err)
	}
}
