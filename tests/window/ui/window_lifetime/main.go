// NoKeepRunning 与应用退出清理窗口验收。
//
// 环境：Linux/X11、Windows 或 macOS 桌面和正常事件循环；记录 OS/WM、后端与倍率。
// 启动：go run ./tests/window/ui/window_lifetime [-auto] [-only-tools]。
// GOUI_PLAT_PAINTER=software/opengl（Windows 另可 direct2d），GOUI_PLAT_SCALE=1/2。
// 初始：默认有两个普通窗口、一个声明式不保活窗口和一个命令式不保活窗口。
// -only-tools 仅创建后两者；辅助身份不改变原生装饰、任务栏条目或输入能力。
//
// 操作与预期：
//  1. 在声明式辅助窗口点击“计数”，数字更新且窗口不重建；四个窗口均可输入和关闭。
//  2. 点击主窗口“关闭另一普通窗口”，剩余普通窗口及两个辅助窗口继续运行。
//  3. 点击“关闭声明式辅助”，主窗口和命令式辅助继续运行；快照不再包含已关闭窗口。
//  4. 点击“关闭主窗口”，第一次否决并显示关闭请求次数；第二次允许关闭。
//     最后一个普通窗口关闭后自动退出，剩余命令式辅助及控件树也须销毁一次。
//  5. -only-tools 模式依次关闭两个辅助窗口；无窗口后事件循环仍运行。
//     手动模式在控制台输入 Ctrl+C 结束；自动模式确认下一次 Timer 仍执行后显式 Quit。
//
// -auto 经 Snapshot 定位按钮并派发 PointerEvent；按绘制、关闭通知与窗口数量推进，
// 20 秒超时。检查 GUI/UI 混合归属、窗口身份、控件树、原生资源释放和一次退出通知。
// 自动输入不代替窗口管理器实际关闭按钮、原生焦点或多平台桌面验收。
// 平台差异：窗口装饰、初始位置和字体由平台决定，本例不判断其外观或关闭时序快慢。
// 副作用：仅创建自身窗口、投递输入及标准输出；不读写用户文件、剪贴板或系统设置。
// 复位：重新启动。退出：普通模式关闭最后一个普通窗口，或点击“退出应用”。
package main

import (
	"flag"
	"fmt"
	"image"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/ui"
)

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func require(ok bool, message string) {
	if !ok {
		panic(message)
	}
}

func requestClose(w gui.Window) {
	if w != nil {
		must(w.RequestClose())
	}
}

type scenario struct {
	app                  ui.App
	auto, onlyTools      bool
	initialized, passed  bool
	showTool             bool
	phase, count, closes int
	shutdowns            int
	deadline             time.Time
	main, declared       gui.Window
	other, imperative    gui.Window
	windows              []gui.Window
	roots                []*gui.LinearBox
	frames, destroyed    map[string]int
	backend              string
}

func (s *scenario) frame(id string) func(gui.Painter, geometry.Rectangle) {
	return func(p gui.Painter, _ geometry.Rectangle) {
		s.frames[id]++
		if s.backend == "" {
			img, err := p.NewImage(image.NewRGBA(image.Rect(0, 0, 1, 1)))
			must(err)
			s.backend = fmt.Sprintf("%T", img)
			img.Destroy()
			requested := os.Getenv("GOUI_PLAT_PAINTER")
			require(requested == "" || strings.Contains(strings.ToLower(s.backend), requested), "绘图后端发生回退")
			fmt.Printf("PAINTER os=%s backend=%s scale=%g\n", runtime.GOOS, s.backend, p.PixelScale())
		}
	}
}

func (s *scenario) track(w gui.Window) {
	s.windows = append(s.windows, w)
	s.roots = append(s.roots, w.Widget().(*gui.LinearBox))
}

func (s *scenario) nativeWindow(id string, excluded bool) gui.Window {
	w, err := gui.App.NewWindow(&gui.WindowOptions{Size: geometry.Size{Width: 300, Height: 150}, NoKeepRunning: excluded})
	must(err)
	w.SetID(id)
	must(w.SetTitle("GOUI " + id))
	root := gui.NewLinearBox(layout.DirectionVertical)
	root.SetPadding(geometry.UniformInsets(12))
	root.SetSpacing(8)
	root.AddChild(gui.NewLabel(fmt.Sprintf("NoKeepRunning=%t", excluded)))
	button := gui.NewButton()
	button.SetID("close")
	button.SetChild(gui.NewLabel("关闭此窗口"))
	button.ConnectClicked(func() { must(w.RequestClose()) })
	root.AddChild(button)
	w.SetWidget(root)
	w.ConnectAfterPaint(s.frame(id))
	w.ConnectDestroy(func() { s.destroyed[id]++ })
	s.track(w)
	must(w.SetPosition(nil, geometry.Point{X: 700, Y: 60 + float32(len(s.windows))*170}))
	must(w.Show())
	return w
}

func (s *scenario) click(w gui.Window, id string) {
	widget := gui.FindWidget(w, id)
	require(widget != nil, "没有找到按钮："+id)
	bounds := widget.Snapshot().Bounds
	require(bounds.Width > 0 && bounds.Height > 0, "按钮尚未完成布局："+id)
	for _, kind := range []events.EventType{events.PointerMove, events.PointerDown, events.PointerUp} {
		e := events.PointerEvent{EventType: kind, Position: bounds.Center()}
		if kind != events.PointerMove {
			e.Button = events.PointerButtonLeft
		}
		if kind == events.PointerDown {
			e.Buttons = events.PointerButtonLeftDown
		}
		must(w.DispatchEvent(e))
	}
}

func (s *scenario) tick() {
	if s.auto && time.Now().After(s.deadline) {
		panic(fmt.Sprintf("窗口生命周期验收超时：phase=%d destroyed=%v", s.phase, s.destroyed))
	}
	if !s.initialized {
		s.declared, s.main = s.app.FindWindow("tool"), s.app.FindWindow("main")
		if s.declared == nil || s.frames["tool"] == 0 || !s.onlyTools && (s.main == nil || s.frames["main"] == 0) {
			return
		}
		s.track(s.declared)
		if s.main != nil {
			s.track(s.main)
		}
		s.imperative = s.nativeWindow("imperative-tool", true)
		if !s.onlyTools {
			s.other = s.nativeWindow("other", false)
		}
		must(s.declared.SetPosition(nil, geometry.Point{X: 400, Y: 50}))
		s.initialized = true
		return
	}
	if !s.auto {
		return
	}
	switch s.phase {
	case 0:
		if s.frames["imperative-tool"] == 0 || !s.onlyTools && s.frames["other"] == 0 {
			return
		}
		want := 4
		if s.onlyTools {
			want = 2
		}
		require(len(gui.App.Windows()) == want && len(gui.App.Snapshot().Surfaces) == want, "辅助窗口丢失登记或语义快照")
		s.click(s.declared, "count")
		s.phase++
	case 1:
		if s.count != 1 || gui.FindWidget(s.declared, "count-value").(*gui.Label).Text() != "计数=1" {
			return
		}
		require(s.app.FindWindow("tool") == s.declared, "声明重建替换了辅助窗口")
		if s.onlyTools {
			s.click(s.declared, "close")
			s.phase = 5
		} else {
			s.click(s.main, "close-other")
			s.phase++
		}
	case 2:
		if s.destroyed["other"] != 1 {
			return
		}
		require(len(gui.App.Windows()) == 3, "仍有普通窗口时应用已退出或窗口数量错误")
		s.click(s.main, "close-tool")
		s.phase++
	case 3:
		if s.destroyed["tool"] != 1 {
			return
		}
		require(s.app.FindWindow("tool") == nil && len(gui.App.Windows()) == 2, "关闭辅助后宿主状态错误")
		s.click(s.main, "close-main")
		s.phase++
	case 4:
		if s.closes != 1 {
			return
		}
		require(s.destroyed["main"] == 0 && len(gui.App.Windows()) == 2, "主窗口关闭否决无效")
		s.passed = true
		s.click(s.main, "close-main")
		s.phase = 7
	case 5:
		if s.destroyed["tool"] != 1 {
			return
		}
		require(len(gui.App.Windows()) == 1, "不保活窗口关闭影响了其他辅助窗口")
		s.click(s.imperative, "close")
		s.phase++
	case 6:
		if s.destroyed["imperative-tool"] != 1 {
			return
		}
		require(len(gui.App.Windows()) == 0, "辅助窗口关闭后仍被登记")
		// 无窗口后的下一次计时回调确实运行，才能确认未自动退出。
		s.passed = true
		s.app.Quit()
		s.phase++
	}
}

func main() {
	auto := flag.Bool("auto", false, "自动派发输入并验证退出")
	only := flag.Bool("only-tools", false, "仅创建不保活窗口")
	flag.Parse()
	s := &scenario{auto: *auto, onlyTools: *only, showTool: true, deadline: time.Now().Add(20 * time.Second), frames: map[string]int{}, destroyed: map[string]int{}}
	started := false
	err := ui.Run("org.golang-gui.WindowLifetime", func(app ui.App) ui.RootView {
		s.app = app
		if !started {
			started = true
			gui.App.ConnectShutdown(func() { s.shutdowns++ })
			app.TickFunc(20*time.Millisecond, s.tick)
		}
		var windows []ui.WindowView
		if !s.onlyTools {
			windows = append(windows, ui.Window("main").Title("GOUI Keep Running").Size(380, 330).
				OnAfterPaint(s.frame("main")).OnDestroy(func() { s.destroyed["main"]++ }).
				OnCloseRequest(func(allow *bool) { s.closes++; *allow = s.closes > 1; app.RequestUpdate() }).
				Content(ui.VBox(
					ui.Label(fmt.Sprintf("关闭请求=%d；第一次否决", s.closes)),
					ui.Button("关闭另一普通窗口").ID("close-other").OnClick(func() { requestClose(s.other) }),
					ui.Button("关闭声明式辅助").ID("close-tool").OnClick(func() {
						if w := app.FindWindow("tool"); w != nil {
							must(w.RequestClose())
						}
					}),
					ui.Button("关闭主窗口").ID("close-main").OnClick(func() { requestClose(app.FindWindow("main")) }),
					ui.Button("退出应用").OnClick(app.Quit),
				).Padding(12).Spacing(8).CrossAlign(layout.CrossStretch)))
		}
		if s.showTool {
			windows = append(windows, ui.Window("tool").Title("GOUI NoKeepRunning").Size(280, 220).NoKeepRunning(true).
				OnAfterPaint(s.frame("tool")).OnDestroy(func() { s.destroyed["tool"]++; s.showTool = false; app.RequestUpdate() }).
				Content(ui.VBox(
					ui.Label("声明式 NoKeepRunning=true"),
					ui.Label(fmt.Sprintf("计数=%d", s.count)).ID("count-value"),
					ui.Button("计数").ID("count").OnClick(func() { s.count++; app.RequestUpdate() }),
					ui.Button("关闭此窗口").ID("close").OnClick(func() { requestClose(app.FindWindow("tool")) }),
					ui.Button("退出应用").OnClick(app.Quit),
				).Padding(12).Spacing(8)))
		}
		return ui.Root().Windows(windows...)
	})
	must(err)
	if *auto {
		require(s.passed && s.shutdowns == 1, "自动验收提前退出或重复退出通知")
		require(len(gui.App.Windows()) == 0 && len(gui.App.Snapshot().Surfaces) == 0, "Run 返回后仍有窗口或快照")
		for i, w := range s.windows {
			require(s.destroyed[w.ID()] == 1 && w.PlatformWindow() == nil && s.roots[i].Destroyed(), "退出未释放窗口及控件树："+w.ID())
		}
		fmt.Printf("WINDOW LIFETIME PASS only-tools=%t windows=%d\n", *only, len(s.windows))
	}
}
