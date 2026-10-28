// 原生宿主终止与初始居中降级验收，不使用 DevServer。
//
// 环境：Linux/X11（有 WM 和无 WM 的独立会话分别运行）、Windows x64 或 macOS arm64
// 桌面；从 main 初始化应用。记录 OS、WM、后端、缩放。可设置 GOUI_PLAT_PAINTER 和
// GOUI_PLAT_SCALE；透明模式需要 compositor，Windows OpenGL 需支持 WGL 的桌面。
// 启动：go run ./tests/window/gui/window_lifecycle [-auto] [-platform] [-transparent]
// 默认验证 GUI Window；-platform 验证平台 Window、Popup 及各自 Painter。
// -center 在 GUI 模式请求初始居中；在没有 _NET_WORKAREA 的 X11 会话仍必须创建成功，
// 显式 WorkAreaAt 则仍应报告 ErrUnsupported（-no-workarea 自动断言这一点）。
//
// 初始：控制窗口和目标窗口可见。目标持续请求重绘；GUI 模式还有保留内容的 Popover。
// 操作与预期：
//  1. 点击“Request close”：仅增加关闭请求计数，目标仍可见、仍能重绘。
//  2. 点击“Native close”：Linux 通过另一 X 连接 XDestroyWindow，Windows 在 GUI 线程
//     DestroyWindow，macOS 直接 NSWindow.close（绕过 shouldClose）；只收到一次终止，
//     GUI 目标从 Application.Windows 消失，关闭请求计数不增加；控制窗口仍可操作。
//  3. -auto 额外检查：平台模式先单独终止 Popup，再终止 Window；每个表面终止恰好一次，Painter 释放后
//     重复 Destroy 安全。GUI 模式重新 Show 同一个 Popover 后再关闭目标，确认 Closed
//     通知及最终退出均完成。程序根据真实通知计数断言，不发送伪造 DestroyEvent。
//  4. 点击“Quit”或关闭控制窗口：退出，不残留窗口；重新运行即可复位。
//
// -auto 每 400ms 执行一个阶段：关闭否决、Popup 终止/重开、目标终止、迟到重绘与重复
// Destroy、控制窗口最后关闭。缺少任何通知或重复通知均非零退出；8 秒超时失败。
// Linux 外部终止的通知异步到达；Win32/AppKit 可以同步重入。 macOS 不要求销毁窗口的
// Objective-C 存储先于通知释放，Windows 不跨线程调用 DestroyWindow。
// 副作用：仅创建、激活并销毁本程序窗口，不改系统设置/剪贴板；不会调用 XKillClient。
// 此例使用原生故障注入验证生命周期，不能替代 WM 交互、任意驱动故障或多屏验收。
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/graphics"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL:", err)
		os.Exit(1)
	}
}

func run() error {
	auto := flag.Bool("auto", false, "自动执行并断言")
	raw := flag.Bool("platform", false, "直接验证平台表面")
	alpha := flag.Bool("transparent", false, "透明目标")
	center := flag.Bool("center", false, "请求初始居中")
	noArea := flag.Bool("no-workarea", false, "断言工作区查询不支持")
	flag.Parse()
	app, err := gui.NewApplication("org.golang-gui.NativeLifecycle")
	if err != nil {
		return err
	}
	keeper, err := app.NewWindow(&gui.WindowOptions{Size: geometry.Size{Width: 460, Height: 280}, Center: *center})
	if err != nil {
		return err
	}
	defer keeper.Destroy()
	keeper.SetTitle("GOUI lifecycle controller")
	keeper.ConnectCloseRequest(func(close *bool) {
		*close = false
		app.Quit() // 退出后由 defer 释放所有目标，不留下仍否决关闭的测试窗口。
	})
	box := gui.NewLinearBox(layout.DirectionVertical)
	box.SetPadding(16)
	box.SetSpacing(10)
	keeper.SetWidget(box)
	status := gui.NewLabel("Waiting for native termination")
	box.AddChild(status)
	var result error
	check := func(ok bool, message string) {
		if !ok && result == nil {
			result = errors.New(message)
			app.Quit()
		}
	}
	var victim platform.Window
	var popup platform.Popup
	var target gui.Window
	var pop gui.Popover
	var paints, closes, destroys, popClosed int
	var rawPaint, rawPopupPaint graphics.Painter
	var rawSize, popupSize events.SizeEvent
	paint := func(surface platform.Surface, p graphics.Painter, size events.SizeEvent) {
		if p == nil || surface.NativeHandle() == 0 || size.Width <= 0 {
			return
		}
		paints++
		p.Begin(size.PixelWidth, size.PixelHeight, size.PixelWidth/size.Width)
		p.Clear(graphics.RGB(50, 110, 160))
		p.FillRoundRect(geometry.Rect(15, 15, 120, 70), 12, graphics.RGB(240, 245, 250))
		p.End()
	}
	if *raw {
		victim, err = app.Platform().NewWindow(geometry.Size{Width: 320, Height: 220}, func(e platform.Event) {
			switch e := e.(type) {
			case events.SizeEvent:
				rawSize = e
			case events.CloseEvent:
				closes++
				status.SetText(fmt.Sprintf("Close requests: %d", closes))
			case events.PaintEvent:
				if victim != nil {
					paint(victim, rawPaint, rawSize)
				}
			case events.DestroyEvent:
				destroys++
				status.SetText(fmt.Sprintf("Native terminated: %d", destroys))
				check(victim.NativeHandle() == 0, "Window 终止时 NativeHandle 非零")
			}
		}, platform.WindowOptions{Chrome: platform.WindowChromeNative, Transparent: *alpha})
		if err != nil {
			return err
		}
		defer victim.Destroy()
		victim.SetTitle("Native target — keep painting")
		rawPaint, err = app.Platform().NewPainter(victim)
		if err != nil {
			return err
		}
		defer rawPaint.Destroy()
		fmt.Println("BACKEND", rawPaint.Name())
		if want := map[string]string{"opengl": "OpenGL", "software": "Software", "direct2d": "Direct2D"}[os.Getenv("GOUI_PLAT_PAINTER")]; want != "" && rawPaint.Name() != want {
			return fmt.Errorf("requested %s, actual painter %s", want, rawPaint.Name())
		}
		popup, err = app.Platform().NewPopup(victim, geometry.Size{Width: 180, Height: 100}, func(e platform.Event) {
			switch e := e.(type) {
			case events.SizeEvent:
				popupSize = e
			case events.PaintEvent:
				if popup != nil {
					paint(popup, rawPopupPaint, popupSize)
				}
			case events.DestroyEvent:
				popClosed++
				check(popup.NativeHandle() == 0, "Popup 终止时 NativeHandle 非零")
			}
		}, platform.PopupOptions{Transparent: *alpha})
		if err != nil {
			return err
		}
		defer popup.Destroy()
		rawPopupPaint, err = app.Platform().NewPainter(popup)
		if err != nil {
			return err
		}
		defer rawPopupPaint.Destroy()
		if err = victim.Show(); err != nil {
			return err
		}
		popup.SetPosition(30, 50)
		if err = popup.Show(); err != nil {
			return err
		}
	} else {
		target, err = app.NewWindow(&gui.WindowOptions{Size: geometry.Size{Width: 340, Height: 230}, Center: *center, Transparent: *alpha})
		if err != nil {
			return err
		}
		defer target.Destroy()
		target.SetTitle("GUI target — keep painting")
		anchor := gui.NewLabel("Native close bypasses veto; controller survives")
		target.SetWidget(anchor)
		target.ConnectCloseRequest(func(close *bool) {
			closes++
			*close = false
			status.SetText(fmt.Sprintf("Close requests: %d", closes))
		})
		target.ConnectDestroy(func() {
			destroys++
			status.SetText(fmt.Sprintf("Native terminated: %d", destroys))
		})
		target.ConnectFrame(func(time.Time) { paints++ })
		victim = target.PlatformWindow()
		pop = gui.NewPopover(anchor, &gui.PopoverOptions{Transparent: *alpha})
		defer pop.Destroy()
		pop.SetWidget(gui.NewLabel("Reusable Popover content"))
		pop.ConnectClosed(func() { popClosed++ })
		if err = target.Show(); err != nil {
			return err
		}
	}
	if *noArea {
		_, e := victim.(platform.DesktopWindow).WorkAreaAt(geometry.Point{})
		if !errors.Is(e, platform.ErrUnsupported) {
			return fmt.Errorf("显式 WorkAreaAt 应返回 ErrUnsupported，实际 %v", e)
		}
	}
	addButton := func(text string, f func()) {
		b := gui.NewButton()
		b.SetChild(gui.NewLabel(text))
		b.ConnectClicked(f)
		box.AddChild(b)
	}
	addButton("Request close", func() { victim.RequestClose() })
	addButton("Native close", func() {
		if victim.NativeHandle() != 0 {
			if e := destroyHost(victim.NativeHandle()); e != nil {
				result = e
				app.Quit()
			}
		}
	})
	addButton("Quit", app.Quit)
	if err = keeper.Show(); err != nil {
		return err
	}
	refresh := app.NewTimer()
	defer refresh.Stop()
	refresh.ConnectTimeout(func() {
		if victim.NativeHandle() != 0 {
			victim.RequestPaint()
		}
		if popup != nil && popup.NativeHandle() != 0 {
			popup.RequestPaint()
		}
	})
	if err = refresh.Start(16 * time.Millisecond); err != nil {
		return err
	}
	if *auto {
		stage := 0
		steps := app.NewTimer()
		defer steps.Stop()
		steps.ConnectTimeout(func() {
			switch stage {
			case 0:
				check(paints > 0, "目标没有执行绘制")
				victim.RequestClose()
				check(destroys == 0, "关闭请求销毁了被否决的窗口")
				if pop != nil {
					if e := pop.Show(); e != nil {
						result = e
					}
				} else {
					if e := destroyHost(popup.NativeHandle()); e != nil {
						result = e
					}
				}
			case 1:
				check(closes == 1 && destroys == 0, "关闭请求没有被否决")
				if pop != nil {
					check(pop.Visible(), "Popover 没有显示")
					pop.Hide()
					if e := pop.Show(); e != nil {
						result = e
					}
				} else {
					check(popClosed == 1, "Popup 终止通知不是一次")
					popup.Destroy()
					check(popClosed == 1, "重复 Destroy 重复发出 Popup 终止")
				}
			case 2:
				victim.RequestPaint() // 保留已排队的绘制请求，再由原生终止宿主。
				if e := destroyHost(victim.NativeHandle()); e != nil {
					result = e
				}
			case 3:
				check(destroys == 1 && closes == 1 && victim.NativeHandle() == 0, "Window 终止通知、关闭请求计数或句柄无效化错误")
				if target != nil {
					check(len(app.Windows()) == 1, "GUI 仍保留已终止窗口")
					check(!pop.Visible() && popClosed == 2, "所有者终止后 Popover 未关闭")
				}
				if rawPaint != nil {
					// 模拟终止后迟到的帧提交；不能重新绑定或呈现已失效的宿主。
					rawPaint.Begin(rawSize.PixelWidth, rawSize.PixelHeight, 1)
					rawPaint.Clear(graphics.Color{})
					rawPaint.End()
					rawPaint.Destroy()
					rawPaint = nil
				}
				if rawPopupPaint != nil {
					rawPopupPaint.Destroy()
					rawPopupPaint = nil
				}
				victim.Destroy()
				victim.RequestPaint()
				if target != nil {
					target.Destroy()
				}
			case 4:
				check(destroys == 1, "重复 Destroy 产生重复通知")
				keeper.RequestPaint()
				if result != nil {
					app.Quit()
					return
				}
				fmt.Printf("PASS native lifecycle: platform=%v transparent=%v paints=%d close=%d destroy=%d popupClosed=%d\n", *raw, *alpha, paints, closes, destroys, popClosed)
				keeper.Destroy()
			}
			if result != nil {
				app.Quit()
			}
			stage++
		})
		if err = steps.Start(400 * time.Millisecond); err != nil {
			return err
		}
		timeout := app.NewTimer()
		defer timeout.Stop()
		timeout.ConnectTimeout(func() { result = errors.New("生命周期验收超时"); app.Quit() })
		timeout.StartOnce(8 * time.Second)
	}
	app.Run()
	return result
}
