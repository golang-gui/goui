// 全屏状态、工作区域与菜单栏回归，需要真实桌面及原生窗口管理器。
// 启动：go run ./tests/window/gui/fullscreen [-auto] [-chrome=native|integrated|none]
// 可用 GOUI_PLAT_PAINTER、GOUI_PLAT_SCALE 选择绘图后端与应用缩放。
// 初始：普通窗口，顶部为 Short/View/Long 菜单；正文显示当前状态与请求错误。
//
// 操作与预期：
//  1. View/Fullscreen 进入全屏，再打开三个菜单：菜单正文紧贴菜单栏底边，
//     不留下桌面面板高度的空隙；长短交替及 Esc 关闭不残留旧内容。
//  2. View/Normal 退出：恢复普通窗口装饰、大小和位置；再进入全屏仍正常。
//  3. 全屏中重复 Fullscreen、Show：保持全屏。Hide 后 Show 恢复显示。
//  4. 最大化后进入全屏，再请求 Normal：得到普通窗口，不是恢复最大化。
//  5. 全屏中 Minimized 后 Show 恢复全屏，再请求 Normal；快速 Fullscreen/Normal：
//     最终状态与最后请求一致，不能因为重复 toggle 反向切换。
//  6. 分别用三种 Chrome、1x/2x 重复；多屏时在各显示器重复并检查边缘退避。
//
// -auto 在 GUI 线程请求状态并等待实际观察，独立查询原生屏幕/客户区核对
// WorkAreaAt、Position(nil)、普通位置恢复和正常输入路径打开的菜单。
// 自动输入经过 Window.DispatchEvent；不替代原生指针、外观或多屏验收。
// 不支持全屏、超时、观察不符或提前关闭均非零退出，不记为跳过或通过。
// 副作用：只创建/激活自身窗口、菜单并改变其状态；不改系统设置或剪贴板。
// 复位：重启。退出：View/Quit 或关闭窗口；自动模式完成后自行退出。
package main

import (
	"flag"
	"fmt"
	"image"
	"math"
	"os"
	"runtime"
	"time"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/theme/modern"
)

type checkStep struct {
	name  string
	start func() error
	ready func() bool
	check func() error
}

var paintedScale float32

type paintProbe struct{ gui.WidgetBase }

func (p *paintProbe) Measure(c layout.Constraint) layout.Measurement {
	return layout.Measured(c.Clamp(geometry.Size{Width: 1, Height: 1}))
}
func (p *paintProbe) Paint(draw gui.Painter) {
	if paintedScale == draw.PixelScale() {
		return
	}
	img, err := draw.NewImage(image.NewRGBA(image.Rect(0, 0, 1, 1)))
	if err != nil {
		panic(err)
	}
	fmt.Printf("PAINTER image=%T scale=%g\n", img, draw.PixelScale())
	img.Destroy()
	paintedScale = draw.PixelScale()
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL", err)
		os.Exit(1)
	}
}

func run() error {
	auto := flag.Bool("auto", false, "自动检查实际状态、区域和恢复位置")
	chrome := flag.String("chrome", "native", "native、integrated 或 none")
	flag.Parse()
	modes := map[string]gui.WindowChromeMode{"native": gui.WindowChromeNative, "integrated": gui.WindowChromeIntegrated, "none": gui.WindowChromeNone}
	mode, ok := modes[*chrome]
	if !ok {
		return fmt.Errorf("未知 Chrome %q", *chrome)
	}
	app, err := gui.NewApplication("org.golanggui.FullscreenRegression")
	if err != nil {
		return err
	}
	app.SetStyleSheet(modern.FromSettings(app.Settings()))
	w, err := app.NewWindow(&gui.WindowOptions{Size: geometry.Size{Width: 620, Height: 360}, Chrome: mode, Center: true})
	if err != nil {
		return err
	}
	defer w.Destroy()
	_ = w.SetTitle("GOUI fullscreen and menus")
	native := w.PlatformWindow().(platform.WindowManagement)
	status := gui.NewLabel("View / Fullscreen, then open Short / Long")
	request := func(state gui.WindowState) error {
		err := native.RequestState(state)
		status.SetText(fmt.Sprintf("requested=%v observed=%v error=%v", state, w.State(), err))
		return err
	}
	box := gui.NewLinearBox(layout.DirectionVertical)
	group := gui.NewMenuButtonGroup()
	var buttons []*gui.MenuButton
	for i, name := range []string{"Short", "View", "Long"} {
		b := gui.NewMenuButton()
		b.SetChild(gui.NewLabel(name))
		menu := gui.NewMenu()
		if i == 1 {
			for _, entry := range []struct {
				name  string
				state gui.WindowState
			}{{"Normal", gui.WindowStateNormal}, {"Fullscreen", gui.WindowStateFullscreen}, {"Maximized", gui.WindowStateMaximized}, {"Minimized", gui.WindowStateMinimized}} {
				menu.Append(entry.name, func() { _ = request(entry.state) })
			}
			menu.Append("Show", func() { _ = w.Show() })
			menu.Append("Hide then show", func() {
				_ = request(gui.WindowStateHidden)
				time.AfterFunc(time.Second, func() { app.Post(func() { _ = w.Show() }) })
			})
			menu.Append("Quit", app.Quit)
		} else {
			for row := 0; row < 1+i*4; row++ {
				menu.Append(fmt.Sprintf("%s action %d", name, row+1), func() {})
			}
		}
		b.SetMenu(menu)
		group.AddChild(b)
		buttons = append(buttons, b)
	}
	box.AddChild(group)
	box.AddChild(status)
	box.AddChild(&paintProbe{})
	w.SetWidget(box)
	fullEvents := 0
	reverseOnFull := false
	var reversalErr error
	w.ConnectState(func(state gui.WindowState) {
		fmt.Println("STATE", state)
		if state == gui.WindowStateFullscreen {
			fullEvents++
			if reverseOnFull {
				reverseOnFull = false
				reversalErr = request(gui.WindowStateNormal)
			}
		}
	})
	complete := false
	w.ConnectDestroy(func() {
		if *auto && !complete {
			err = fmt.Errorf("验收完成前关闭窗口")
		}
		app.Quit()
	})
	if showErr := w.Show(); showErr != nil {
		return showErr
	}
	if !*auto {
		return app.Run()
	}
	var initialPosition geometry.Point
	var initialSize geometry.Size
	var steps []checkStep
	state := func(s gui.WindowState) func() bool { return func() bool { return w.State() == s } }
	checkFull := func() error {
		want, _, e := nativeArea(w.PlatformWindow())
		if e != nil {
			return e
		}
		got, e := w.WorkAreaAt(geometry.Point{})
		if e != nil {
			return e
		}
		if !nearRect(got, want) {
			return fmt.Errorf("全屏区域 got=%+v want=%+v", got, want)
		}
		pos, e := w.Position(nil)
		if e != nil {
			return e
		}
		if !near(pos.X, -want.X) || !near(pos.Y, -want.Y) {
			return fmt.Errorf("全屏 Position(nil)=%+v area=%+v", pos, want)
		}
		return nil
	}
	addState := func(name string, s gui.WindowState, check func() error) {
		steps = append(steps, checkStep{name, func() error {
			if err := request(s); err != nil {
				return err
			}
			if s == gui.WindowStateFullscreen {
				return request(s)
			}
			return nil
		}, state(s), check})
	}
	steps = append(steps, checkStep{"normal baseline", nil, func() bool { return w.State() == gui.WindowStateNormal && group.Rect().Height > 0 && paintedScale > 0 }, func() error {
		initialPosition, err = w.Position(nil)
		if err != nil {
			return err
		}
		_, initialSize, err = nativeArea(w.PlatformWindow())
		return err
	}})
	addState("enter fullscreen", gui.WindowStateFullscreen, checkFull)
	addState("repeat fullscreen", gui.WindowStateFullscreen, checkFull)
	steps = append(steps, checkStep{"show fullscreen", w.Show, state(gui.WindowStateFullscreen), checkFull})
	for _, b := range buttons {
		steps = append(steps, checkStep{"fullscreen menu", func() error {
			for _, button := range buttons {
				button.Close()
			}
			p := b.Snapshot().Bounds.Center()
			for _, typ := range []events.EventType{events.PointerDown, events.PointerUp} {
				down := events.PointerButtons(0)
				if typ == events.PointerDown {
					down = events.PointerButtonLeftDown
				}
				if e := w.DispatchEvent(events.PointerEvent{EventType: typ, Button: events.PointerButtonLeft, Buttons: down, Position: p}); e != nil {
					return e
				}
			}
			return nil
		}, func() bool { return b.Opened() }, func() error {
			if len(w.Popovers()) != 1 {
				return fmt.Errorf("应仅有一个共享菜单")
			}
			return checkFull()
		}})
	}
	addState("leave fullscreen", gui.WindowStateNormal, func() error {
		for _, b := range buttons {
			b.Close()
		}
		pos, e := w.Position(nil)
		if e != nil {
			return e
		}
		_, size, e := nativeArea(w.PlatformWindow())
		if e != nil {
			return e
		}
		if !near(pos.X, initialPosition.X) || !near(pos.Y, initialPosition.Y) || !near(size.Width, initialSize.Width) || !near(size.Height, initialSize.Height) {
			return fmt.Errorf("普通位置/尺寸恢复 got=%+v/%+v want=%+v/%+v", pos, size, initialPosition, initialSize)
		}
		return nil
	})
	if runtime.GOOS != "darwin" || mode != gui.WindowChromeNone {
		addState("maximize", gui.WindowStateMaximized, nil)
		addState("maximized to fullscreen", gui.WindowStateFullscreen, checkFull)
		addState("fullscreen to ordinary", gui.WindowStateNormal, nil)
		addState("fullscreen before minimize", gui.WindowStateFullscreen, checkFull)
		addState("fullscreen to minimized", gui.WindowStateMinimized, nil)
		steps = append(steps, checkStep{"show minimized fullscreen", w.Show, state(gui.WindowStateFullscreen), checkFull})
		addState("fullscreen to minimized again", gui.WindowStateMinimized, nil)
		addState("minimized to ordinary", gui.WindowStateNormal, nil)
	} else {
		fmt.Println("NOT APPLICABLE macOS None has no native zoom/miniaturization capability")
	}
	addState("fullscreen before hide", gui.WindowStateFullscreen, checkFull)
	addState("hide fullscreen", gui.WindowStateHidden, nil)
	steps = append(steps, checkStep{"show hidden fullscreen", w.Show, state(gui.WindowStateFullscreen), checkFull})
	addState("final ordinary", gui.WindowStateNormal, nil)
	steps = append(steps, checkStep{"latest target during fullscreen entry", func() error {
		if err := request(gui.WindowStateFullscreen); err != nil {
			return err
		}
		return request(gui.WindowStateNormal)
	}, state(gui.WindowStateNormal), nil})
	if runtime.GOOS == "darwin" {
		observedBefore := 0
		steps = append(steps, checkStep{"reverse after observed fullscreen", func() error {
			observedBefore = fullEvents
			reverseOnFull = true
			return request(gui.WindowStateFullscreen)
		}, func() bool {
			return fullEvents > observedBefore && w.State() == gui.WindowStateNormal
		}, func() error { return reversalErr }})
	}
	index, begun, stable := 0, false, 0
	deadline := time.Now().Add(12 * time.Second)
	done := make(chan struct{})
	defer close(done)
	go func() {
		ticker := time.NewTicker(80 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				app.Post(func() {
					if complete || err != nil {
						return
					}
					if index == len(steps) {
						complete = true
						fmt.Println("PASS fullscreen, work area, menus and restoration")
						app.Quit()
						return
					}
					s := steps[index]
					fail := func(e error) { err = fmt.Errorf("%s: %w", s.name, e); app.Quit() }
					if !begun {
						begun = true
						deadline = time.Now().Add(12 * time.Second)
						if s.start != nil {
							if e := s.start(); e != nil {
								fail(e)
								return
							}
						}
					}
					if time.Now().After(deadline) {
						fail(fmt.Errorf("等待实际状态超时，state=%v", w.State()))
						return
					}
					if s.ready != nil && !s.ready() {
						stable = 0
						return
					}
					stable++
					if stable < 4 {
						return
					}
					if s.check != nil {
						if e := s.check(); e != nil {
							fail(e)
							return
						}
					}
					fmt.Println("VERIFIED", s.name)
					index++
					begun = false
					stable = 0
				})
			}
		}
	}()
	if runErr := app.Run(); runErr != nil {
		return runErr
	}
	if err == nil && !complete {
		return fmt.Errorf("验收完成前退出事件循环")
	}
	return err
}

func near(a, b float32) bool { return math.Abs(float64(a-b)) <= 1 }
func nearRect(a, b geometry.Rectangle) bool {
	return near(a.X, b.X) && near(a.Y, b.Y) && near(a.Width, b.Width) && near(a.Height, b.Height)
}
