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
//     新窗口先正常布局，随后按目标真实标签位置、原生边框和像素比例定位；
//     原抓取点应落在释放点（允许一物理像素舍入），不把释放点当作窗口左上角。
//  2. 点击任一窗口的 Verify and exit。程序检查原页面/输入框身份、挂载次数、
//     文本、选择和定位，再销毁源窗口，确认目标内容仍然存活。输出 PASS 后退出。
//  3. 以 -expect canceled 重新启动：先拖出源栏，观察到原生预览后保持左键按下，
//     按 Esc 再松开，然后 Verify。预期不产生新窗口请求，源仍为 A/B，
//     Mount=1、Unmount=0。不要只点击 Verify，静态断言不代替实际取消操作。
//  4. 以 -expect prepare-failure 重新启动，按步骤 1 拖出。
//     预期目标已创建显示但模拟准备失败，未接收页面的空目标关闭；
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
// 平台差异：macOS 先激活窗口再操作；取消动画由系统决定，不要求一定回弹。
// Linux Integrated 需要合成器，应记录实际装饰是否回退为 Native。
// 不要在准备定位时移动源窗口；用例将其视为失效并报错。系统可能限制位置，
// 定位断言失败不能通过放宽容差当作成功。
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

// A test-owned container observes a normal frame, then posts preparation to
// the event loop. It neither forces layout nor transfers a page inside Paint.
type frameProbe struct {
	gui.WidgetBase
	scale float32
	after func(float32)
}

func (p *frameProbe) Paint(painter gui.Painter) {
	p.scale = painter.PixelScale()
	if next := p.after; next != nil {
		p.after = nil
		scale := p.scale
		gui.App.Post(func() { next(scale) })
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
	var tb *widgets.TabBar
	var release, hotspot geometry.Point
	var ratio, sourceScale float32
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
			outer, err := target.Position(source)
			if err != nil {
				fail(err)
				return
			}
			frame, err := target.Position(target)
			if err != nil {
				fail(err)
				return
			}
			anchor, ok := firstTab(tb.Snapshot())
			if !ok {
				fail(fmt.Errorf("target tab not laid out"))
				return
			}
			actual := outer.Add(anchor.Add(hotspot).Add(frame.Scale(-1)).Scale(ratio))
			if math.Abs(float64(actual.X-release.X))*float64(sourceScale) > 1.01 || math.Abs(float64(actual.Y-release.Y))*float64(sourceScale) > 1.01 {
				fail(fmt.Errorf("grab point=%v release=%v ratio=%g", actual, release, ratio))
				return
			}
			fmt.Printf("position release=%v actual=%v ratio=%g\n", release, actual, ratio)
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
	build := func(view *widgets.TabView, label string) (*frameProbe, *widgets.TabBar) {
		bar := widgets.NewTabBar()
		bar.SetView(view)
		bar.SetReorderable(true)
		bar.SetTransferable(true)
		bar.ConnectTransferError(fail)
		button := gui.NewButton()
		button.SetChild(gui.NewLabel("Verify and exit"))
		button.ConnectClicked(verify)
		column := &frameProbe{}
		column.SetLayoutManager(&layout.LinearLayout{Direction: layout.DirectionVertical, CrossAlign: layout.CrossStretch, Padding: 12, Spacing: 8})
		column.WidgetBase.AddChild(column, bar)
		column.WidgetBase.AddChild(column, gui.NewLabel(label))
		column.WidgetBase.AddChild(column, button)
		column.WidgetBase.AddChild(column, view)
		return column, bar
	}
	content, sb := build(sv, "Drag A onto empty desktop, then Verify")
	sb.ConnectDetachRequest(func(request *widgets.TabDetachRequest) {
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
		release, hotspot, sourceScale = request.Position, request.Hotspot, content.scale
		sourceOrigin, err := source.Position(nil)
		if err != nil {
			request.Cancel()
			fail(err)
			return
		}
		target, err = app.NewWindow(options)
		if err != nil {
			request.Cancel()
			fail(err)
			return
		}
		_ = target.SetTitle("GOUI DETACHED TARGET")
		target.ConnectCloseRequest(func(*bool) { request.Cancel(); app.Quit() })
		tv = widgets.NewTabView()
		// Only a sizing placeholder, never a copy of the page's editing subtree.
		placeholder := widgets.NewTabPage(a.Title(), nil)
		tv.AppendPage(placeholder)
		targetContent, targetBar := build(tv, "Retained page in new window")
		tb = targetBar
		targetContent.after = func(scale float32) {
			if target == nil {
				return
			} // preparation may have been canceled after Show
			abort := func(err error) { request.Cancel(); target.Destroy(); target = nil; fail(err) }
			observed, err := source.Position(nil)
			if err != nil {
				abort(err)
				return
			}
			if observed != sourceOrigin || content.scale != sourceScale {
				abort(fmt.Errorf("source moved or changed DPI while preparing target"))
				return
			}
			frame, err := target.Position(target)
			if err != nil {
				abort(err)
				return
			}
			anchor, ok := firstTab(tb.Snapshot())
			if !ok || scale <= 0 || sourceScale <= 0 {
				abort(fmt.Errorf("missing target layout/scale"))
				return
			}
			ratio = scale / sourceScale
			position := release.Add(anchor.Add(hotspot).Add(frame.Scale(-1)).Scale(-ratio))
			if err := target.SetPosition(source, position); err != nil {
				abort(err)
				return
			}
			tv.RemovePage(placeholder)
			if err := request.TransferTo(tv, 0); err != nil {
				abort(err)
				return
			}
			fmt.Printf("prepared target: requested=%v release=%v ratio=%g\n", position, release, ratio)
		}
		target.SetWidget(targetContent)
		if err := target.Show(); err != nil {
			request.Cancel()
			fail(err)
			return
		}
		if *expect == "prepare-failure" {
			targetContent.after = nil
			request.Cancel()
			target.Destroy()
			target = nil
			fmt.Println("injected preparation failure; source retained")
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

func firstTab(info gui.WidgetInfo) (geometry.Point, bool) {
	if info.Role == widgets.RoleTab {
		return info.Bounds.Pos, info.Bounds.Width > 0 && info.Bounds.Height > 0
	}
	for _, child := range info.Children {
		if point, ok := firstTab(child); ok {
			return point, true
		}
	}
	return geometry.Point{}, false
}
