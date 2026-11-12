// 正文持续请求布局时的窗口拖动回归，不使用 DevServer。
//
// 环境：Linux/X11（记录 WM）、Windows 或 macOS 可交互桌面，记录后端与缩放。
// 启动：go run ./tests/window/gui/chrome_layout -case header
// case 为 header/tabs/button；-pending=false 提供没有持续布局请求的对照。
// 初始：Integrated 720×420 DIP，第一行为 HeaderBar，第二行为含两个标签的
// HeaderBar，正文 Count 按钮下方显示操作说明。每次 Arrange 都请求下一帧布局。
//
// 操作与逐项预期：
//  1. 等待 READY，窗口激活后按 S（或 F6），记录初始位置并开始检查。
//  2. header：在首行左侧空白/标题处按住，移动至少40 DIP再释放；
//     tabs：在第二行两个标签右侧的空白处做同样拖动。不要落在标签或窗口按钮上。
//     预期窗口第一次按压即可移动，不必等待“布局干净”，标签选择和 Count 均不变。
//  3. button：只点击正文 Count 一次，预期次数为1，窗口不移动。
//  4. 按 F（或 F8）结束；程序断言窗口位移、计数、标签选择、布局请求和 Snapshot，
//     通过输出 PASS 并退出0，否则退出1。提前关闭不算通过。
//  5. 三种 case 分别重启验收；以 -pending=false 对照，复测 Software/OpenGL 和
//     1x/2x。Windows 走原生命中，X11/macOS 走 GUI 手势及原生 BeginMove。
//
// 连续布局请求只用于复现，窗口保持正常帧调度，不在命中回调中同步布局或绘制。
// 平台差异：Linux Integrated 可回退到 GUI 装饰；以 READY 的实际模式为准。
// macOS 先激活，功能键被系统占用时用 S/F。原生移动可能暂时阻塞应用帧。
// 副作用：仅本窗口、内存状态和日志，不修改文件、剪贴板或系统配置。
// 复位：重启。退出：F/F8 或关闭窗口；退出销毁窗口及其控件树。
package main

import (
	"errors"
	"flag"
	"fmt"
	"image"
	"math"
	"os"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/theme/modern"
	"github.com/golang-gui/goui/widgets"
)

type pendingBody struct {
	gui.WidgetBase
	pending  bool
	requests int
}

func (b *pendingBody) Arrange(rect geometry.Rectangle) {
	b.WidgetBase.Arrange(rect)
	if b.pending {
		b.requests++
		b.RequestLayout()
	}
}

func main() {
	which := flag.String("case", "header", "header, tabs or button")
	pending := flag.Bool("pending", true, "request another layout from every body Arrange")
	flag.Parse()
	if err := run(*which, *pending); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL:", err)
		os.Exit(1)
	}
}

func run(which string, pending bool) error {
	if which != "header" && which != "tabs" && which != "button" {
		return fmt.Errorf("unknown case %q", which)
	}
	app, err := gui.NewApplication("org.golanggui.ChromeLayout")
	if err != nil {
		return err
	}
	app.SetStyleSheet(modern.FromSettings(app.Settings()))
	win, err := app.NewWindow(&gui.WindowOptions{Size: geometry.Size{Width: 720, Height: 420}, Chrome: gui.WindowChromeIntegrated})
	if err != nil {
		return err
	}
	defer win.Destroy()
	win.ConnectDestroy(app.Quit)
	var chrome gui.ChromeInfo
	win.Chrome().ConnectInfo(func(info gui.ChromeInfo) { chrome = info })
	if err := win.SetTitle("GOUI pending chrome / " + which); err != nil {
		return err
	}
	if err := win.SetPosition(nil, geometry.Point{X: 100, Y: 100}); err != nil {
		return err
	}
	header := gui.NewHeaderBar()
	header.SetID("header")
	header.SetChild(gui.NewLabel("HeaderBar — drag here"))
	view := widgets.NewTabView()
	first := widgets.NewTabPage("Alpha", gui.NewLabel("Alpha page"))
	view.AppendPage(first)
	view.AppendPage(widgets.NewTabPage("Beta", gui.NewLabel("Beta page")))
	bar := widgets.NewTabBar()
	bar.SetView(view)
	// The view owns the pages; mount it in the body for normal lifetime cleanup.
	view.SetMainWeight(1)
	tabs := gui.NewHeaderBar()
	tabs.SetID("tabs-header")
	tabs.SetChild(bar)
	count := 0
	status := gui.NewLabel("S/F6 start — drag or click — F/F8 finish")
	button := gui.NewButton()
	button.SetChild(gui.NewLabel("Count"))
	button.SetMinSize(geometry.Size{Width: 150, Height: 40})
	button.ConnectClicked(func() { count++; status.SetText(fmt.Sprintf("Count: %d", count)) })
	content := gui.NewLinearBox(layout.DirectionVertical)
	content.SetPadding(geometry.UniformInsets(20))
	content.SetSpacing(12)
	content.AddChild(button)
	content.AddChild(status)
	content.AddChild(view)
	body := &pendingBody{pending: pending}
	body.SetID("body")
	body.SetMainWeight(1)
	body.SetLayoutManager(layout.NewFillLayout())
	body.WidgetBase.AddChild(body, content)
	root := gui.NewLinearBox(layout.DirectionVertical)
	root.SetCrossAlign(layout.CrossStretch)
	root.AddChild(header)
	root.AddChild(tabs)
	root.AddChild(body)
	win.SetWidget(root)
	ready, armed, finished := false, false, false
	var origin geometry.Point
	var failure error
	keys := gui.NewKeyEventController()
	keys.SetPhase(gui.PhaseCapture)
	keys.ConnectKeyDown(func(ctx gui.EventContext, e events.KeyEvent) {
		if e.Repeat {
			return
		}
		switch e.Key {
		case events.KeyS, events.KeyF6:
			ctx.StopPropagation()
			if !ready {
				return
			}
			origin, failure = win.Position(nil)
			armed = failure == nil
			fmt.Printf("ARM case=%s position=%v requests=%d\n", which, origin, body.requests)
		case events.KeyF, events.KeyF8:
			ctx.StopPropagation()
			position, err := win.Position(nil)
			distance := math.Hypot(float64(position.X-origin.X), float64(position.Y-origin.Y))
			valid := armed && err == nil && view.Current() == first && header.Snapshot().ID == "header" &&
				bar.Snapshot().Role == widgets.RoleTabBar && (!pending || body.requests > 0)
			if which == "button" {
				valid = valid && count == 1 && distance <= 1
			} else {
				valid = valid && count == 0 && distance >= 20
			}
			if !valid {
				failure = fmt.Errorf("case=%s armed=%t position=%v start=%v distance=%g count=%d requests=%d error=%v", which, armed, position, origin, distance, count, body.requests, err)
			} else {
				finished = true
				fmt.Printf("PASS case=%s position=%v distance=%g count=%d requests=%d\n", which, position, distance, count, body.requests)
			}
			win.Destroy()
		}
	})
	root.AddEventController(keys)
	win.ConnectAfterPaint(func(p gui.Painter, _ geometry.Rectangle) {
		if ready {
			return
		}
		resource, err := p.NewImage(image.NewRGBA(image.Rect(0, 0, 1, 1)))
		if err != nil {
			failure = err
			app.Post(win.Destroy)
			return
		}
		if !chrome.Enabled || chrome.Mode != gui.WindowChromeIntegrated {
			failure = fmt.Errorf("Integrated chrome unavailable: %+v", chrome)
			resource.Destroy()
			app.Post(win.Destroy)
			return
		}
		fmt.Printf("READY case=%s pending=%t platform=%s backend=%T scale=%g chrome=%v header=%v tabs=%v button=%v requests=%d\n",
			which, pending, app.Platform().Name(), resource, p.PixelScale(), chrome.Mode, header.Rect(), tabs.Rect(), button.Snapshot().Bounds, body.requests)
		resource.Destroy()
		ready = true
	})
	if err := win.Show(); err != nil {
		return err
	}
	if err := app.Run(); err != nil {
		return err
	}
	if failure != nil {
		return failure
	}
	if !finished {
		return errors.New("closed before verification completed")
	}
	return nil
}
