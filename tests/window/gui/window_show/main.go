// Window.Show 窗口唤起验收，不使用 DevServer。
//
// 环境：支持前台激活的 Windows/macOS 桌面，或有 EWMH 窗口管理器的 Linux/X11。
// 启动：go run ./tests/window/gui/window_show [-auto] [-chrome=native|integrated|none]
// 可设置 GOUI_PLAT_PAINTER=software|opengl（Windows 另有 direct2d），
// GOUI_PLAT_SCALE=1|2。记录实际系统、窗口管理器、后端与缩放。
// 初始：两个重叠窗口；Target 有文本“keep selection”及选区，Controller 在前台。
//
// 操作与逐项预期：
//  1. 在 Controller 点击 Show target：已经显示的 Target 前置并获得焦点，
//     输入框的逻辑焦点、文字和选区保留；重复调用不会重置这些内容。
//  2. 点击 Hide target，再点击 Show target：Target 重新显示并激活。
//  3. 点击 Minimize target，再点击 Show target：取消最小化并激活。
//  4. 点击 Maximize target，再从 Controller 点击 Show target：保持最大化。
//     随后 Minimize target、Show target：恢复最大化，而不是变成普通窗口。
//  5. 点击 Normal target，再点击 Show target twice：保持普通模式与编辑状态。
//  6. 在 Target 点击 Popup：浮层出现，Target 保持原生焦点，输入框选区保留；
//     点击 Hide popup 后可继续编辑。Popup 不应调用普通窗口的激活请求。
//  7. 先在输入框编辑、选择文字，再重复步骤 1～5：新文字与选区也保持。
//     对比终端 Focus/State 日志与实际前台窗口，不凭 Show 返回值判定获得焦点。
//  8. 可用 -auto -fullscreen（当前 Linux 实现）验证全屏窗口重复 Show 保持全屏。
//
// -auto 使用 GUI Timer 等待原生焦点与状态事件，逐项断言选区/逻辑焦点、隐藏、
// 最小化、最大化恢复、重复调用、Popup 不激活和已销毁窗口错误；PASS 后自动退出。
// 系统可能拒绝后台应用激活；这符合 Show 契约，但不能记为交互验收通过。
// 自动模式要求本测试应用有正常的前台权限；可从桌面终端启动或先点击其窗口。
// 平台差异：Windows/macOS 当前没有通用全屏状态请求，默认不验此项。
// 副作用：只创建/激活本进程两个窗口与 Popup，不改系统设置、剪贴板或外部文件。
// 复位：重启；退出：关闭任一窗口或点击 Exit。自动模式建议外部设置 45 秒超时。
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform"
	"github.com/golang-gui/goui/theme/modern"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL", err)
		os.Exit(1)
	}
}

func run() error {
	auto := flag.Bool("auto", false, "自动检查后退出")
	fullscreen := flag.Bool("fullscreen", false, "同时检查全屏模式；要求平台支持该请求")
	chrome := flag.String("chrome", "native", "native、integrated 或 none")
	flag.Parse()
	modes := map[string]gui.WindowChromeMode{"native": gui.WindowChromeNative, "integrated": gui.WindowChromeIntegrated, "none": gui.WindowChromeNone}
	mode, ok := modes[*chrome]
	if !ok {
		return fmt.Errorf("未知 chrome %q", *chrome)
	}
	app, err := gui.NewApplication("org.golang-gui.WindowShow")
	if err != nil {
		return err
	}
	app.SetStyleSheet(modern.FromSettings(app.Settings()))
	target, err := app.NewWindow(&gui.WindowOptions{Size: geometry.Size{Width: 500, Height: 230}, Chrome: mode})
	if err != nil {
		return err
	}
	defer target.Destroy()
	controller, err := app.NewWindow(&gui.WindowOptions{Size: geometry.Size{Width: 500, Height: 420}, Chrome: mode})
	if err != nil {
		return err
	}
	defer controller.Destroy()
	_ = target.SetTitle("GOUI Show Target")
	_ = controller.SetTitle("GOUI Show Controller")
	if !*auto {
		target.ConnectDestroy(app.Quit)
	}
	controller.ConnectDestroy(app.Quit)

	input := gui.NewTextInput()
	input.SetText("keep selection")
	selection := gui.TextSelection{Anchor: 1, Caret: 5}
	input.SetSelection(selection)
	content := gui.NewLinearBox(layout.DirectionVertical)
	content.SetSpacing(8)
	content.SetPadding(geometry.UniformInsets(16))
	if mode == gui.WindowChromeIntegrated {
		content.AddChild(gui.NewHeaderBar())
	}
	content.AddChild(gui.NewLabel("Target: Show preserves text, selection and mode"))
	content.AddChild(input)
	popup := gui.NewPopover(input, nil)
	defer popup.Destroy()
	popup.SetWidget(gui.NewLabel("Popup keeps owner focus"))
	popup.SetPlacement(gui.PopoverPlacementBottom)
	addButton := func(box *gui.LinearBox, label string, action func()) {
		button := gui.NewButton()
		button.SetChild(gui.NewLabel(label))
		button.ConnectClicked(action)
		box.AddChild(button)
	}
	addButton(content, "Popup", func() {
		if err := popup.Show(); err != nil {
			fmt.Println("POPUP", err)
		}
	})
	addButton(content, "Hide popup", popup.Hide)
	target.SetWidget(content)
	if !target.SetFocusedWidget(input) {
		return errors.New("无法设置输入框逻辑焦点")
	}

	status := gui.NewLabel("Controller: use Show to bring Target forward")
	panel := gui.NewLinearBox(layout.DirectionVertical)
	panel.SetSpacing(6)
	panel.SetPadding(geometry.UniformInsets(16))
	if mode == gui.WindowChromeIntegrated {
		panel.AddChild(gui.NewHeaderBar())
	}
	panel.AddChild(status)
	show := func() {
		if err := target.Show(); err != nil {
			fmt.Println("SHOW", err)
		}
	}
	addButton(panel, "Show target", show)
	addButton(panel, "Hide target", func() { target.RequestState(gui.WindowStateHidden) })
	addButton(panel, "Minimize target", func() { target.RequestState(gui.WindowStateMinimized) })
	addButton(panel, "Maximize target", func() { target.RequestState(gui.WindowStateMaximized) })
	addButton(panel, "Normal target", func() { target.RequestState(gui.WindowStateNormal) })
	addButton(panel, "Show target twice", func() { show(); show() })
	addButton(panel, "Exit", app.Quit)
	controller.SetWidget(panel)
	for _, win := range []gui.Window{target, controller} {
		win.ConnectFocus(func(focused bool) {
			fmt.Printf("FOCUS %s %v selection=%+v\n", win.Title(), focused, input.Selection())
		})
		win.ConnectState(func(state gui.WindowState) { fmt.Printf("STATE %s %v\n", win.Title(), state) })
	}
	// 定位只便于观察两个窗口；不将工作区支持作为 Show 的必要条件。
	_ = target.SetPosition(nil, geometry.Point{X: 100, Y: 100})
	_ = controller.SetPosition(target, geometry.Point{X: 140, Y: 100})
	if err := target.Show(); err != nil {
		return err
	}
	if err := controller.Show(); err != nil {
		return err
	}
	fmt.Printf("READY OS=%s painter=%q scale=%q chrome=%s\n", runtime.GOOS, os.Getenv("GOUI_PLAT_PAINTER"), os.Getenv("GOUI_PLAT_SCALE"), *chrome)
	var result error
	completed := false
	if *auto {
		type step struct {
			name  string
			run   func() error
			ready func() bool
		}
		state := func(s gui.WindowState) func() bool { return func() bool { return target.State() == s } }
		active := func(s gui.WindowState) func() bool {
			return func() bool { return target.Focused() && target.State() == s }
		}
		request := func(s gui.WindowState) func() error { return func() error { target.RequestState(s); return nil } }
		steps := []step{
			{"controller initially foreground", nil, controller.Focused},
			{"visible target foreground", target.Show, active(gui.WindowStateNormal)},
			{"repeat target Show", target.Show, active(gui.WindowStateNormal)},
			{"put controller in front", controller.Show, controller.Focused},
			{"visible target foreground again", target.Show, active(gui.WindowStateNormal)},
			{"hide target", request(gui.WindowStateHidden), state(gui.WindowStateHidden)},
			{"show hidden target", target.Show, active(gui.WindowStateNormal)},
			{"minimize target", request(gui.WindowStateMinimized), state(gui.WindowStateMinimized)},
			{"restore minimized target", target.Show, active(gui.WindowStateNormal)},
			{"maximize target", request(gui.WindowStateMaximized), state(gui.WindowStateMaximized)},
			{"controller over maximized target", controller.Show, controller.Focused},
			{"show maximized target", target.Show, active(gui.WindowStateMaximized)},
			{"minimize maximized target", request(gui.WindowStateMinimized), state(gui.WindowStateMinimized)},
			{"restore maximized target", target.Show, active(gui.WindowStateMaximized)},
			{"request normal target", request(gui.WindowStateNormal), state(gui.WindowStateNormal)},
			{"show normal target", target.Show, active(gui.WindowStateNormal)},
			{"show Popup", popup.Show, func() bool { return popup.Visible() && target.Focused() }},
			{"hide Popup", func() error { popup.Hide(); return nil }, func() bool { return !popup.Visible() && target.Focused() }},
		}
		if *fullscreen {
			steps = append(steps,
				step{"fullscreen target", request(gui.WindowStateFullscreen), state(gui.WindowStateFullscreen)},
				step{"show fullscreen target", target.Show, active(gui.WindowStateFullscreen)},
				step{"exit fullscreen", request(gui.WindowStateNormal), state(gui.WindowStateNormal)})
		}
		steps = append(steps, step{"destroyed target unavailable", func() error {
			target.Destroy()
			if err := target.Show(); !errors.Is(err, platform.ErrUnavailable) {
				return fmt.Errorf("已销毁窗口 Show=%v", err)
			}
			return nil
		}, nil})
		timer := app.NewTimer()
		defer timer.Stop()
		index, started := 0, false
		var began time.Time
		timer.ConnectTimeout(func() {
			if index == len(steps) {
				timer.Stop()
				completed = true
				fmt.Println("PASS Window.Show")
				app.Quit()
				return
			}
			current := steps[index]
			if !started {
				started, began = true, time.Now()
				if current.run != nil {
					if err := current.run(); err != nil {
						result = fmt.Errorf("%s: %w", current.name, err)
						timer.Stop()
						app.Quit()
						return
					}
				}
			}
			// 让原生事件至少处理一轮；单凭函数返回或逻辑 Visible 不能通过断言。
			if time.Since(began) < 150*time.Millisecond {
				return
			}
			if current.ready != nil && !current.ready() {
				if time.Since(began) > 4*time.Second {
					result = fmt.Errorf("%s: 超时，target state=%v focused=%v controller focused=%v", current.name, target.State(), target.Focused(), controller.Focused())
					timer.Stop()
					app.Quit()
				}
				return
			}
			if index < len(steps)-1 && (input.Text() != "keep selection" || input.Selection() != selection || target.FocusedWidget() != input) {
				result = fmt.Errorf("%s: 编辑状态被重置：text=%q selection=%+v focused=%T", current.name, input.Text(), input.Selection(), target.FocusedWidget())
				timer.Stop()
				app.Quit()
				return
			}
			fmt.Println("PASS", current.name)
			index++
			started = false
		})
		if err := timer.Start(40 * time.Millisecond); err != nil {
			return err
		}
	}
	app.Run()
	if *auto && result == nil && !completed {
		return errors.New("窗口在自动断言完成前退出")
	}
	return result
}
