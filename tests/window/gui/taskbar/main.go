// SkipTaskbar window checks, without DevServer.
//
// Environment: interactive Linux/X11, Windows or macOS. Record OS/WM, backend
// and scale. Run:
//
//	go run ./tests/window/gui/taskbar [-chrome=none|native|integrated] [-auto]
//
// Use GOUI_PLAT_PAINTER=software/opengl (also direct2d on Windows) and
// GOUI_PLAT_SCALE=1 or 2. Initially the ordinary Native main window and an
// unrelated SkipTaskbar auxiliary window are visible. Show the main window first,
// position the hidden auxiliary window at client (480,80) DIP, then show it.
// The default None auxiliary window has a custom HeaderBar and 6 DIP resize
// hit area, without always-on-top or no-activate flags.
// Steps and expected results:
//  1. Windows/X11 taskbars have only the ordinary window's entry. Windows Alt+Tab
//     also excludes the auxiliary; X11 switcher behavior depends on the WM.
//     The macOS Dock remains application-wide without hiding the app or other windows.
//  2. Type taskbar-test in the auxiliary input, then click Count: text/count
//     appear, and the main window does not block auxiliary focus or input.
//  3. Drag its caption (custom HeaderBar for None), then right/bottom edges.
//     Movement and resizing work. None does not configure automatic resize cursors;
//     those are outside this check. Native/Integrated appearance varies; Native
//     content size must not shrink by a tool-window caption's height.
//  4. Reposition returns the auxiliary to (480,80); Check prints PASS with
//     observed client coordinates matching the request and nonzero window sizes.
//  5. Minimize auxiliary, then Show auxiliary restores input. A native macOS
//     minimized Dock thumbnail is allowed; do not hide the entire app to simulate it.
//  6. Closing the auxiliary leaves the main window usable. Quit releases both.
//
// -auto checks creation flags, positioning before show and observed position
// afterward, then exits after about 800 ms. It does not verify taskbar appearance,
// activation, input or real dragging. Record WM-adjusted position failures.
// Side effects: this program's windows and logs only, no settings or clipboard.
// Reset: restart. Exit: Quit or close the main window.
package main

import (
	"flag"
	"fmt"
	"math"
	"os"
	"runtime"
	"time"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL:", err)
		os.Exit(1)
	}
}

func run() error {
	chrome := flag.String("chrome", "none", "none, native or integrated")
	auto := flag.Bool("auto", false, "check native setup/position and exit")
	flag.Parse()
	modes := map[string]gui.WindowChromeMode{"none": gui.WindowChromeNone, "native": gui.WindowChromeNative, "integrated": gui.WindowChromeIntegrated}
	mode, ok := modes[*chrome]
	if !ok {
		return fmt.Errorf("invalid chrome %q", *chrome)
	}
	app, err := gui.NewApplication("org.golang-gui.TaskbarTest")
	if err != nil {
		return err
	}
	mainWindow, err := app.NewWindow(&gui.WindowOptions{Size: geometry.Size{Width: 440, Height: 360}})
	if err != nil {
		return err
	}
	defer mainWindow.Destroy()
	tool, err := app.NewWindow(&gui.WindowOptions{Size: geometry.Size{Width: 400, Height: 320}, Chrome: mode, SkipTaskbar: true})
	if err != nil {
		return err
	}
	defer tool.Destroy()
	mainWindow.SetTitle("GOUI Taskbar - ordinary")
	tool.SetTitle("GOUI Taskbar - skipped")
	mainWindow.ConnectDestroy(app.Quit)
	closed := false
	tool.ConnectDestroy(func() { closed = true })
	status := gui.NewLabel("辅助窗口可以激活并接收键盘输入")
	count := 0
	button := func(text string, action func()) *gui.Button {
		b := gui.NewButton()
		b.SetChild(gui.NewLabel(text))
		b.ConnectClicked(action)
		return b
	}
	toolContent := gui.NewLinearBox(layout.DirectionVertical)
	toolContent.SetPadding(12)
	toolContent.SetSpacing(10)
	input := gui.NewTextInput()
	input.ConnectText(func(text string) { fmt.Printf("INPUT %q\n", text) })
	toolContent.AddChild(input)
	toolContent.AddChild(button("计数", func() {
		count++
		status.SetText(fmt.Sprintf("计数=%d；输入=%s", count, input.Text()))
		fmt.Printf("CLICK count=%d text=%q\n", count, input.Text())
	}))
	toolContent.AddChild(button("最小化辅助", func() { tool.RequestState(gui.WindowStateMinimized) }))
	toolContent.AddChild(status)
	toolRoot := gui.NewLinearBox(layout.DirectionVertical)
	header := gui.NewHeaderBar()
	header.SetChild(gui.NewLabel("SkipTaskbar：拖动这里移动"))
	toolRoot.AddChild(header)
	toolRoot.AddChild(toolContent)
	tool.SetWidget(toolRoot)
	if mode == gui.WindowChromeNone {
		tool.Chrome().ConnectQueryRegion(func(p geometry.Point, region *gui.ChromeRegion) {
			r := toolRoot.Rect()
			left, right := p.X < 6, p.X >= r.Width-6
			top, bottom := p.Y < 6, p.Y >= r.Height-6
			switch {
			case top && left:
				*region = gui.ChromeRegionTopLeft
			case top && right:
				*region = gui.ChromeRegionTopRight
			case bottom && left:
				*region = gui.ChromeRegionBottomLeft
			case bottom && right:
				*region = gui.ChromeRegionBottomRight
			case top:
				*region = gui.ChromeRegionTop
			case bottom:
				*region = gui.ChromeRegionBottom
			case left:
				*region = gui.ChromeRegionLeft
			case right:
				*region = gui.ChromeRegionRight
			}
		})
	}
	want := geometry.Point{X: 480, Y: 80}
	position := func() error { return tool.SetPosition(mainWindow, want) }
	check := func() error {
		if closed {
			return fmt.Errorf("辅助窗口已关闭，请重新启动")
		}
		if err := checkNative(mainWindow, false); err != nil {
			return err
		}
		if err := checkNative(tool, true); err != nil {
			return err
		}
		got, err := tool.Position(mainWindow)
		if err != nil {
			return err
		}
		if math.Abs(float64(got.X-want.X)) > 1 || math.Abs(float64(got.Y-want.Y)) > 1 {
			return fmt.Errorf("observed client position %v, want %v", got, want)
		}
		fmt.Printf("PASS native setup and client position=%v\n", got)
		return nil
	}
	mainContent := gui.NewLinearBox(layout.DirectionVertical)
	mainContent.SetPadding(12)
	mainContent.SetSpacing(10)
	mainContent.AddChild(gui.NewLabel("普通窗口：任务栏对照"))
	mainContent.AddChild(button("重新定位", func() { fmt.Println("POSITION", position()) }))
	mainContent.AddChild(button("显示辅助", func() {
		if !closed {
			tool.RequestState(gui.WindowStateNormal)
			fmt.Println("SHOW", tool.Show())
		}
	}))
	mainContent.AddChild(button("检查", func() { fmt.Println("CHECK", check()) }))
	mainContent.AddChild(button("退出", app.Quit))
	mainWindow.SetWidget(mainContent)
	if err := mainWindow.SetPosition(nil, geometry.Point{X: 40, Y: 40}); err != nil {
		return err
	}
	if err := checkNative(mainWindow, false); err != nil {
		return err
	}
	if err := checkNative(tool, true); err != nil {
		return err
	}
	if err := mainWindow.Show(); err != nil {
		return err
	}
	fmt.Printf("OS=%s chrome=%s painter=%q scale=%q ordinary=%#x skipped=%#x\n", runtime.GOOS, *chrome,
		os.Getenv("GOUI_PLAT_PAINTER"), os.Getenv("GOUI_PLAT_SCALE"), mainWindow.PlatformWindow().NativeHandle(), tool.PlatformWindow().NativeHandle())
	var result error
	// Let the WM place the reference window first. SetPosition samples actual
	// geometry once; it does not follow an outstanding move on that reference.
	// These delays are observation/setup for the probe, not backend retries.
	timer := app.NewTimer()
	defer timer.Stop()
	timer.ConnectTimeout(func() {
		if *auto {
			result = check()
			app.Quit()
		}
	})
	showTimer := app.NewTimer()
	defer showTimer.Stop()
	showTimer.ConnectTimeout(func() {
		if result = position(); result == nil {
			result = tool.Show()
		}
		if result == nil && *auto {
			result = timer.StartOnce(400 * time.Millisecond)
		}
		if result != nil {
			app.Quit()
		}
	})
	if err := showTimer.StartOnce(400 * time.Millisecond); err != nil {
		return err
	}
	app.Run()
	return result
}
