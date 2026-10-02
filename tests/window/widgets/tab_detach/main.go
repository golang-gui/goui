// TabView 拖出创建新窗口验收，不使用 DevServer。
//
// 环境：Linux/X11、Windows、macOS 可交互桌面；记录 OS、窗口管理器、实际后端
// 和缩放。启动命令在仓库根目录运行：
//
//	go run ./tests/window/widgets/tab_detach
//
// 初始：一个源窗口，A/B 两页；A 的输入框为 retained text。窗口只由本程序创建。
// 默认 Native 装饰、连续体验模式；可加 -chrome integrated 验证 HeaderBar 集成。
// 默认模式操作：拖出第一张标签后，将源窗口最后一张标签拖到桌面，预期释放后恢复，
// 无新窗口、无进程退出；拖入已有新窗口的标签栏则允许转移，源栏变空但窗口不自动关闭。
// 拖动期间源标签隐藏并留空槽，页面内容保持显示；目标栏标签动画让位，不显示竖线。
// 点击 Add tab 后可继续拖出；新窗口也支持相同操作。
// 拖拽预览不能替代鼠标光标；Esc 取消应保留原页面。关闭全部窗口正常退出。
// 以下严格验收以 -expect detached 启动，不要连续进行第二次移交后再 Verify。
//
// 主要操作与预期：
//  1. 激活源窗口，抓住 Document A 标签中部，拖到所有测试窗口外的桌面空白处松开。
//     默认模式：出现新窗口，A 在新窗口中，源仅剩 B；原文本保留。
//     新窗口按源内容区尺寸加本例布局开销创建，定位和移交后再显示；
//     不应先显示在系统默认位置再移动，也不应先恢复源标签再出现新窗口。
//     初次位置扣除已知标签栏原点和抓取偏移，不承诺不同布局/跨屏 DPI 精确对齐。
//  2. 点击任一窗口的 Verify and exit。程序检查原页面/输入框身份、挂载次数、
//     文本、选择和定位，再销毁源窗口，确认目标内容仍然存活。输出 PASS 后退出。
//  3. 以 -expect canceled 重新启动：先拖出源栏，观察到原生预览后保持左键按下，
//     按 Esc 再松开，然后 Verify。预期不产生新窗口请求，源仍为 A/B，
//     Mount=1、Unmount=0。不要只点击 Verify，静态断言不代替实际取消操作。
//  4. 以 -expect prepare-failure 重新启动，按步骤 1 拖出。
//     预期目标已创建但尚未显示时模拟准备失败，未接收页面的空目标关闭；
//     Verify 检查源不变且没有页面卸载。这不是操作系统创建窗口失败的注入。
//
// 可选拒绝回归（每项重新启动）：
//  1. -expect rejected：额外出现 REJECT TARGET，拖 A 到该窗口中释放再 Verify。
//     目标在协商阶段拒绝（不接收 Drop）；无新窗口请求，源身份和挂载次数不变。
//  2. -reject-target：仍使用默认 detached 预期。先将 A 拖入 REJECT TARGET，
//     保持按住，再移到全部测试窗口外的桌面释放并 Verify；须正常创建窗口。
//     这两项都会断言实际进入了拒绝目标，不能不拖动就点击 Verify 蒙混通过。
//
// 可选逻辑 2x：POSIX 使用 GOUI_PLAT_SCALE=2 go run ./tests/window/widgets/tab_detach；
// PowerShell 先设 $env:GOUI_PLAT_SCALE='2' 再启动，结束后恢复原值；不代表混合 DPI。
// 平台差异：macOS 先激活窗口再操作；允许拖出建窗的会话不播放失败回弹动画，
// Esc 仍恢复源标签。Windows 在窗口外应显示普通光标，不显示不可放置标记；
// 原生目标接受或本应用目标拒绝时仍使用原生反馈。
// Linux Integrated 需要合成器，应记录实际装饰是否回退为 Native。
// 不要在准备定位时移动源窗口；用例将其视为失效并报错。系统可能限制位置，
// 用例以 1 DIP 原生整数舍入容差检查请求位置和内容区尺寸。
// 严格模式结果：Verify 自动检查原对象、归属、文本、挂载次数和定位；成功 PASS、失败 FAIL，
// 失败或未完成返回非零状态。预览外观和实际释放点由操作者独立观察核对。
// 副作用：只创建、移动、销毁自身窗口，不改系统设置或剪贴板。
// 复位：重启程序。退出：默认模式关闭全部窗口；严格模式 Verify 自动退出，直接关闭算未验收。
// 本例只覆盖命令式窗口准备与移交；声明式子树协调另见 ../ui_tab_transfer。
package main

import (
	"flag"
	"fmt"
	"math"
	"os"
	"runtime"
	"slices"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/theme/modern"
	"github.com/golang-gui/goui/widgets"
)

// 同一种文档窗口结构在首次正常布局时记录自身的固定开销。
// 拖出只沿用内容区大小，加上本例标题栏/按钮/留白；不测量隐藏目标。
type frameProbe struct {
	gui.WidgetBase
	view                  *widgets.TabView
	initialSize, overhead geometry.Size
	sized                 bool
}

func (p *frameProbe) Arrange(rect geometry.Rectangle) {
	p.WidgetBase.Arrange(rect)
	if !p.sized && p.view.Rect().Width > 0 && p.view.Rect().Height > 0 {
		viewport := p.view.Rect().Size
		// 正常首轮布局可能被内容最小尺寸放大，不能把请求尺寸当作实际尺寸。
		// 只读源窗口已有的 DIP 边界；max 保留自绘模式的原生边框开销，
		// 不把最小尺寸放大误记成负开销，也不观察/布局隐藏目标。
		bounds := p.Window().Snapshot().Bounds.Size
		p.overhead = geometry.Size{Width: max(bounds.Width, p.initialSize.Width) - viewport.Width, Height: max(bounds.Height, p.initialSize.Height) - viewport.Height}
		p.sized = true
	}
}

func main() {
	runtime.LockOSThread()
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL:", err)
		os.Exit(1)
	}
}

func run() error {
	expect := flag.String("expect", "interactive", "interactive, detached, canceled, rejected or prepare-failure")
	rejectTarget := flag.Bool("reject-target", false, "also show a target rejecting native negotiation")
	chrome := flag.String("chrome", "native", "native or integrated")
	flag.Parse()
	if *expect == "interactive" && *rejectTarget {
		*expect = "detached"
	}
	if !slices.Contains([]string{"interactive", "detached", "canceled", "rejected", "prepare-failure"}, *expect) {
		return fmt.Errorf("invalid expectation")
	}
	decoration := gui.WindowChromeNative
	if *chrome == "integrated" {
		decoration = gui.WindowChromeIntegrated
	} else if *chrome != "native" {
		return fmt.Errorf("invalid chrome")
	}
	if *expect == "interactive" {
		return runInteractive(decoration)
	}
	app, err := gui.NewApplication("org.golang-gui.TabDetachProbe")
	if err != nil {
		return err
	}
	app.SetStyleSheet(modern.Sheet(modern.Options{}))
	options := &gui.WindowOptions{Size: geometry.Size{Width: 480, Height: 320}, Chrome: decoration}
	source, err := app.NewWindow(options)
	if err != nil {
		return err
	}
	defer source.Destroy()
	var target gui.Window
	defer func() {
		if target != nil {
			target.Destroy()
		}
	}()
	_ = source.SetTitle("GOUI DETACH SOURCE")
	source.ConnectCloseRequest(func(*bool) { app.Quit() })
	input := gui.NewTextInput()
	input.SetText("retained text")
	text := input.Text()
	input.ConnectText(func(value string) { text = value })
	a, b := widgets.NewTabPage("Document A", input), widgets.NewTabPage("Document B", gui.NewLabel("B retained"))
	sv := widgets.NewTabView()
	sv.AppendPage(a)
	sv.AppendPage(b)
	mounts, unmounts, requests := 0, 0, 0
	rejectVisits, rejectDrops := 0, 0
	input.ConnectMount(func() { mounts++ })
	input.ConnectUnmount(func() { unmounts++ })
	var failure error
	verified := false
	fail := func(err error) {
		if failure == nil {
			failure = err
		}
		app.Post(app.Quit)
	}
	var tv *widgets.TabView
	var requestedPosition geometry.Point
	var contentSize geometry.Size
	verify := func() {
		if (*rejectTarget || *expect == "rejected") && (rejectVisits == 0 || rejectDrops != 0) {
			fail(fmt.Errorf("expected negotiation refusal: visits=%d drops=%d", rejectVisits, rejectDrops))
			return
		}
		if *expect == "detached" {
			if requests != 1 || target == nil || tv == nil || a.Parent() != tv || a.Child() != input || input.Text() != text ||
				!slices.Equal(sv.Pages(), []*widgets.TabPage{b}) || !slices.Equal(tv.Pages(), []*widgets.TabPage{a}) || tv.Current() != a || mounts != 2 || unmounts != 1 {
				fail(fmt.Errorf("identity/order/lifecycle: requests=%d mounts=%d unmounts=%d", requests, mounts, unmounts))
				return
			}
			origin, err := target.Position(source)
			if err != nil {
				fail(err)
				return
			}
			self, err := target.Position(target)
			if err != nil {
				fail(err)
				return
			}
			if self != (geometry.Point{}) {
				fail(fmt.Errorf("client self-position=%v, want zero", self))
				return
			}
			if math.Abs(float64(origin.X-requestedPosition.X)) > 1 || math.Abs(float64(origin.Y-requestedPosition.Y)) > 1 {
				fail(fmt.Errorf("position=%v requested=%v", origin, requestedPosition))
				return
			}
			actualSize := tv.Rect().Size
			if math.Abs(float64(actualSize.Width-contentSize.Width)) > 1 || math.Abs(float64(actualSize.Height-contentSize.Height)) > 1 {
				fail(fmt.Errorf("content size=%v requested=%v", actualSize, contentSize))
				return
			}
			fmt.Printf("position=%v content=%v\n", origin, actualSize)
			source.Destroy()
			if a.Destroyed() || a.Root() != target || input.Text() != text {
				fail(fmt.Errorf("source destruction affected target"))
				return
			}
		} else {
			wantRequests := 0
			if *expect == "prepare-failure" {
				wantRequests = 1
			}
			if requests != wantRequests || target != nil || !slices.Equal(sv.Pages(), []*widgets.TabPage{a, b}) || a.Child() != input || input.Text() != text || mounts != 1 || unmounts != 0 {
				fail(fmt.Errorf("canceled/failed preparation changed source: requests=%d mounts=%d unmounts=%d", requests, mounts, unmounts))
				return
			}
		}
		verified = true
		app.Post(app.Quit)
	}
	build := func(view *widgets.TabView, label string, size geometry.Size) (*frameProbe, *widgets.TabBar) {
		bar := widgets.NewTabBar()
		bar.SetView(view)
		bar.SetReorderable(true)
		bar.SetTransferable(true)
		bar.ConnectTransferError(fail)
		button := gui.NewButton()
		button.SetChild(gui.NewLabel("Verify and exit"))
		button.ConnectClicked(verify)
		column := &frameProbe{view: view, initialSize: size}
		column.SetLayoutManager(&layout.LinearLayout{Direction: layout.DirectionVertical, CrossAlign: layout.CrossStretch, Padding: 12, Spacing: 8})
		column.WidgetBase.AddChild(column, bar)
		column.WidgetBase.AddChild(column, gui.NewLabel(label))
		column.WidgetBase.AddChild(column, button)
		column.WidgetBase.AddChild(column, view)
		return column, bar
	}
	content, sb := build(sv, "Drag A onto empty desktop, then Verify", options.Size)
	sb.ConnectDetachRequest(func(request *widgets.TabDetachRequest, handled *bool) {
		requests++
		if *expect == "rejected" {
			request.Cancel()
			fail(fmt.Errorf("local negotiation rejection triggered detach"))
			return
		}
		if requests != 1 {
			request.Cancel()
			fail(fmt.Errorf("duplicate detach request"))
			return
		}
		contentSize = request.ContentSize
		targetOptions := *options
		targetOptions.Size = contentSize.Add(content.overhead)
		// 本例目标标签栏与源使用相同留白，采用已知栏原点初次定位。
		requestedPosition = request.Position.Add(sb.Snapshot().Bounds.Pos.Add(request.Hotspot).Scale(-1))
		sourceOrigin, err := source.Position(nil)
		if err != nil {
			request.Cancel()
			fail(err)
			return
		}
		target, err = app.NewWindow(&targetOptions)
		if err != nil {
			request.Cancel()
			fail(err)
			return
		}
		_ = target.SetTitle("GOUI DETACHED TARGET")
		*handled = true
		target.ConnectCloseRequest(func(*bool) { request.Cancel(); app.Quit() })
		tv = widgets.NewTabView()
		targetContent, _ := build(tv, "Retained page in new window", targetOptions.Size)
		target.SetWidget(targetContent)
		abort := func(err error) { request.Cancel(); target.Destroy(); target = nil; fail(err) }
		prepare := func() {
			observed, err := source.Position(nil)
			if err != nil {
				abort(err)
				return
			}
			if observed != sourceOrigin {
				abort(fmt.Errorf("source moved during preparation"))
				return
			}
			if err := target.SetPosition(source, requestedPosition); err != nil {
				abort(err)
				return
			}
			if err := request.TransferTo(tv, 0); err != nil {
				abort(err)
				return
			}
			fmt.Printf("prepared target: position=%v content=%v\n", requestedPosition, contentSize)
		}
		if *expect == "prepare-failure" {
			request.Cancel()
			target.Destroy()
			target = nil
			fmt.Println("injected preparation failure; source retained")
			return
		}
		prepare()
		if target != nil {
			if err := target.Show(); err != nil {
				// 保留已接收原页面的目标，不因显示错误销毁编辑内容。
				fail(err)
			}
		}
	})
	source.SetWidget(content)
	if err := source.Show(); err != nil {
		return err
	}
	if err := source.SetPosition(nil, geometry.Point{X: 50, Y: 100}); err != nil {
		return err
	}
	if *rejectTarget || *expect == "rejected" {
		reject, err := app.NewWindow(&gui.WindowOptions{Size: geometry.Size{Width: 340, Height: 200}, Chrome: decoration})
		if err != nil {
			return err
		}
		defer reject.Destroy()
		_ = reject.SetTitle("GOUI REJECT TARGET")
		reject.ConnectCloseRequest(func(*bool) { app.Quit() })
		label := gui.NewLabel("Native negotiation rejects here")
		drop := gui.NewDropTarget(gui.LocalFormat("goui.widgets.tab-page"))
		drop.SetActions(gui.DragMove)
		refuse := func(e *gui.DragMotion) { rejectVisits++; e.Action = 0 }
		drop.ConnectEnter(refuse)
		drop.ConnectMotion(refuse)
		drop.ConnectDrop(func(*gui.DropRequest) { rejectDrops++ })
		label.AddEventController(drop)
		reject.SetWidget(label)
		if err := reject.Show(); err != nil {
			return err
		}
		if err := reject.SetPosition(source, geometry.Point{X: 540, Y: 0}); err != nil {
			return err
		}
	}
	fmt.Printf("OS=%s painter=%s scale=%s chrome=%s expect=%s\n", runtime.GOOS, os.Getenv("GOUI_PLAT_PAINTER"), os.Getenv("GOUI_PLAT_SCALE"), *chrome, *expect)
	app.Run()
	if failure != nil {
		return failure
	}
	if !verified {
		return fmt.Errorf("closed without verification")
	}
	fmt.Printf("PASS %s requests=%d mounts=%d unmounts=%d text=%q rejection-visits=%d drops=%d\n", *expect, requests, mounts, unmounts, text, rejectVisits, rejectDrops)
	return nil
}
