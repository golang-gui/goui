// TabView 跨窗口页面移交验收，不使用 DevServer。
//
// 环境：Linux/X11、Windows 或 macOS 可交互桌面，记录 OS、实际绘制后端、窗口
// 管理器及缩放。启动命令在仓库根目录运行：
//
//	go run ./tests/window/widgets/tab_transfer
//
// 默认连续体验：左右各两页，可重复双向拖动，包括将源最后一页移入另一窗口。
// 拖出时源标签隐藏但占位和页面内容保留；目标栏动画推开标签留空槽，不使用竖线。
// 在目标栏移动指针，空槽应稳定切换；移出或 Esc 取消后空槽收回，源标签恢复。
// 单标签在桌面释放不会新建窗口；成功转走后源栏为空，源窗口仍在。Add tab 可再次添加。
// Linux 拖出栏外仍须显示系统光标及独立预览；Esc 取消不改变归属。
// 默认模式关闭全部窗口退出，不自动判定 PASS。以下严格验收追加 -expect accepted；
// -editor、-same-window 或非默认 -target 仍隐含严格 accepted 模式。
//
// 初始：两个 Native 窗口，源有 A/B 两页、目标有 T；A 中输入框为 retained text。
// 两侧 TabBar 显式开启 Reorderable 和 Transferable。
//
// 主要操作与预期（默认 -expect accepted）：
//  1. 激活左窗，在 A 中输入内容（也可保持默认文本）。拖动 A 标签离开左侧标签栏，
//     进入右侧标签栏 T 左半边，观察插入标记，再释放。源退避到 B，目标选中 A；
//     标签预览跟随指针，原输入框文本不变。拖动不应移动窗口或触发额外点击。
//  2. 点击右侧 Verify and exit。程序断言页面、输入框对象身份、文本、归属、
//     选中状态、一次 Unmount/Mount；随后销毁源窗口，断言目标页面未被销毁并退出。
//  3. 重新启动并加 -expect canceled：拖离源栏，确认原生预览出现，再保持左键
//     拖到目标栏、按 Esc、松开；点击右侧 Verify and exit。
//     预期源 A/B 与目标 T 均未改变，无额外页面卸载/挂载。不能只点击 Verify，
//     未变化的状态本身不能证明执行过取消手势。
//
// 可选回归（每项重新启动，仅执行该模式，不要求穷举组合）：
//  1. -expect target-destroy：同主要步骤 1；A 卸载时立即销毁目标窗口。
//     预期 A 回到源窗口原位置且仍选中，文本不变；点击左侧 Verify and exit，
//     检查一次卸载后重新挂载、一次转移错误、没有拖出新窗口请求。
//  2. -expect source-destroy：同主要步骤 1；源选中项通知中立即销毁源窗口。
//     预期原 A 已归目标，仍可编辑；点击右侧 Verify，检查成功未被撤销、无转移错误。
//  3. -same-window：单窗口左右两组 TabView，按主要步骤 1、2 从左拖到右。
//     预期同样转移原页面，Verify 不销毁双方共用的窗口。
//  4. -target empty：目标栏初始没有标签，但保留一行接收区域；拖入其顶部空栏松开。
//     预期目标只包含原 A，自动选中。可与 -same-window 组合。
//  5. -target overflow：目标初始 T1..T8。拖 A 到目标栏右侧滚动箭头左边，保持按住
//     且不移动鼠标，等待标签自动滚到 T8。空槽位于 T8 后时松开，再 Verify。
//     预期 T1..T8 顺序不变，A 追加到最后且选中；必须依靠悬停计时滚动到隐藏标签。
//     此模式默认验收尾部插入，不能在中途松开。可与 -same-window 或 canceled 组合。
//     用例显式设置窗口最小尺寸，避免从整排标签期望宽度推导而撑宽窗口。
//  6. -same-window -target mirror：左右标签栏同时展示同一个 TabView 的 A/B/C，
//     页面内容仅在左侧。把左栏 A 先向下拖出，再放到右栏 B 的右半边（C 之前）。
//     预期两个栏都变为 B/A/C，A 仍选中，原输入框没有卸载/挂载；右侧 Verify 检查
//     正好一次 Moved(0,1)、零 CloseRequest。不要直接放到右栏最后。
//     -expect canceled 时按 Esc 取消，预期两栏保持 A/B/C，Moved 为零。
//  7. -chrome integrated：标签栏放在 HeaderBar 内，按主要步骤验证。
//     标签拖动必须启动页面转移而不是移动整个窗口；空白标题区域仍可移动窗口。
//     Linux 需要合成器；2x 同窗口模式建议桌面至少 2200×900 物理像素。
//  8. -editor：编辑页状态保留的独立验收，具体步骤见 editor.go；不可与 same-window、
//     target 等组合，仅支持 accepted/canceled。验证选区、滚动、撤销/重做和源关闭后输入。
//  9. -target relayout：目标 T1/T2/T3 初始各宽 100 DIP，原生拖动首次进入目标栏时
//     自动改为 140 DIP。按住 A，将指针停在目标栏左端向右约 175 DIP（初始 T2 右半，
//     变化后 T2 左半），等待标签变宽和空槽更新后释放，不再移动指针。
//     Verify 要求 T1/A/T2/T3，而非沿用旧插入位置 T1/T2/A/T3；取消模式仍保持原顺序。
//     控制台必须先输出 RELAYOUT，不能把未进入目标就取消算作本项通过。
//  10. 长时间悬停：按可选第 9 项进入目标栏后保持按住超过 35 秒，再验证投递或
//     Esc 取消。预期等待期间页面不移动、会话不报超时；释放/取消后结果仍同该项。
//     本例不自动断言持续时长；计时断言使用 platform/drag_end 的 -min-duration 35s。
//
// 可选逻辑 2x：POSIX 使用 GOUI_PLAT_SCALE=2 go run ./tests/window/widgets/tab_transfer；
// PowerShell 先设 $env:GOUI_PLAT_SCALE='2' 再启动，结束后恢复原值；不代表混合 DPI。
// 平台差异：macOS 先点击源窗口非控件区激活，必要时先激活目标再点击 Verify；原生
// 预览与取消动画由系统决定，不要求每个平台都出现回弹动画。记录实际装饰及后端。
// 严格模式结果：Verify 自动断言对象身份、顺序、选择和生命周期，成功输出 PASS 并退出，
// 失败输出 FAIL 并以非零状态退出；预览、让位动画、窗口是否误移动由操作者观察。
// 副作用：只创建/移动/销毁本例窗口，不修改剪贴板、文件或系统设置。
// 复位：重启。退出：默认模式关闭全部窗口；严格模式 Verify 自动结束，直接关闭视为未验收。
// 本例不覆盖拖出新窗口或声明式子树移交；对应例程为 ../tab_detach、../ui_tab_transfer。
package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"
	"slices"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/theme/modern"
	"github.com/golang-gui/goui/widgets"
)

func main() {
	runtime.LockOSThread()
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL:", err)
		os.Exit(1)
	}
}

func run() error {
	expect := flag.String("expect", "interactive", "interactive, accepted, canceled, target-destroy or source-destroy")
	sameWindow := flag.Bool("same-window", false, "two tab views in one native window")
	targetMode := flag.String("target", "single", "single, empty, overflow, relayout or mirror (requires -same-window)")
	chrome := flag.String("chrome", "native", "native or integrated")
	editor := flag.Bool("editor", false, "editor page, history and viewport retention")
	flag.Parse()
	if *expect == "interactive" && (*sameWindow || *targetMode != "single" || *editor) {
		*expect = "accepted"
	}
	if !slices.Contains([]string{"interactive", "accepted", "canceled", "target-destroy", "source-destroy"}, *expect) {
		return fmt.Errorf("invalid expectation")
	}
	if !slices.Contains([]string{"single", "empty", "overflow", "relayout", "mirror"}, *targetMode) ||
		(*targetMode == "mirror" && !*sameWindow) ||
		((*sameWindow || *targetMode != "single") && *expect != "accepted" && *expect != "canceled") {
		return fmt.Errorf("invalid target or combination with destruction mode")
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
	if *editor {
		if *sameWindow || *targetMode != "single" || (*expect != "accepted" && *expect != "canceled") {
			return fmt.Errorf("editor supports only accepted/canceled in separate windows")
		}
		return runEditorTransfer(decoration, *expect == "canceled")
	}
	app, err := gui.NewApplication("org.golang-gui.TabTransferProbe")
	if err != nil {
		return err
	}
	app.SetStyleSheet(modern.Sheet(modern.Options{}))
	options := &gui.WindowOptions{Size: geometry.Size{Width: 480, Height: 320}, Chrome: decoration}
	if *sameWindow {
		options.Size.Width = 1000
	}
	source, err := app.NewWindow(options)
	if err != nil {
		return err
	}
	defer source.Destroy()
	// Permit actual overflow instead of deriving a WM minimum from all tabs.
	source.SetMinSize(geometry.Size{Width: 400, Height: 280})
	target := source
	if !*sameWindow {
		target, err = app.NewWindow(options)
		if err != nil {
			return err
		}
		defer target.Destroy()
		target.SetMinSize(geometry.Size{Width: 400, Height: 280})
		_ = target.SetTitle("GOUI tabs TARGET")
		target.ConnectCloseRequest(func(*bool) { app.Quit() })
	}
	_ = source.SetTitle("GOUI tabs SOURCE")
	source.ConnectCloseRequest(func(*bool) { app.Quit() })
	sv, tv := widgets.NewTabView(), widgets.NewTabView()
	input := gui.NewTextInput()
	input.SetText("retained text")
	a, b, c := widgets.NewTabPage("Document A", input), widgets.NewTabPage("Document B", gui.NewLabel("B retained")), widgets.NewTabPage("Target T", gui.NewLabel("T retained"))
	mounts, unmounts := 0, 0
	destructions, transferErrors, detachRequests := 0, 0, 0
	input.ConnectMount(func() { mounts++ })
	input.ConnectUnmount(func() {
		unmounts++
		if *expect == "target-destroy" && destructions == 0 {
			destructions++
			target.Destroy()
		}
	})
	sv.AppendPage(a)
	sv.AppendPage(b)
	switch *targetMode {
	case "single":
		tv.AppendPage(c)
	case "overflow":
		for i := 1; i <= 8; i++ {
			title := fmt.Sprintf("Target T%d", i)
			tv.AppendPage(widgets.NewTabPage(title, gui.NewLabel(title)))
		}
	case "relayout":
		for i := 1; i <= 3; i++ {
			tv.AppendPage(widgets.NewTabPage(fmt.Sprintf("T%d", i), nil))
		}
	case "mirror":
		c.SetTitle("Document C")
		sv.AppendPage(c)
		tv = sv
	}
	sourcePages := sv.Pages()
	targetPages, targetCurrent := tv.Pages(), tv.Current()
	moves, closes := 0, 0
	var moveError error
	sv.ConnectMoved(func(page *widgets.TabPage, from, to int) {
		if *targetMode == "mirror" && (page != a || from != 0 || to != 1) {
			moveError = fmt.Errorf("unexpected move: page=%q %d -> %d", page.Title(), from, to)
		}
		moves++
	})
	sv.ConnectCloseRequest(func(*widgets.TabPage) { closes++ })
	sv.ConnectCurrent(func(page *widgets.TabPage) {
		if *expect == "source-destroy" && page == b && destructions == 0 {
			destructions++
			source.Destroy()
		}
	})
	text := "retained text"
	input.ConnectText(func(value string) { text = value })
	var failure error
	verified := false
	relayout := false
	check := gui.NewButton()
	check.SetChild(gui.NewLabel("Verify and exit"))
	check.ConnectClicked(func() {
		wantOwner, wantMounts, wantUnmounts := sv, 1, 0
		wantSource, wantTarget := slices.Clone(sourcePages), slices.Clone(targetPages)
		wantCurrent := targetCurrent
		wantSourceCurrent := a
		accepted := *expect == "accepted" || *expect == "source-destroy"
		if accepted {
			wantOwner, wantMounts, wantUnmounts = tv, 2, 1
			wantSource = []*widgets.TabPage{b}
			wantSourceCurrent = b
			index := 0
			if *targetMode == "overflow" {
				index = len(wantTarget)
			}
			if *targetMode == "relayout" {
				index = 1
			}
			wantTarget = slices.Insert(wantTarget, index, a)
			wantCurrent = a
			if *targetMode == "mirror" {
				wantOwner, wantMounts, wantUnmounts = sv, 1, 0
				wantSource, wantTarget = []*widgets.TabPage{b, a, c}, []*widgets.TabPage{b, a, c}
				wantSourceCurrent = a
			}
		}
		if *expect == "target-destroy" {
			wantMounts, wantUnmounts = 2, 1
			wantTarget = nil
		}
		if *expect == "source-destroy" {
			wantSource = nil
		}
		if a.Parent() != wantOwner || a.Child() != input || input.Text() != text || mounts != wantMounts || unmounts != wantUnmounts ||
			!slices.Equal(sv.Pages(), wantSource) || !slices.Equal(tv.Pages(), wantTarget) ||
			(!sv.Destroyed() && sv.Current() != wantSourceCurrent) ||
			(!tv.Destroyed() && tv.Current() != wantCurrent) || a.Destroyed() {
			failure = fmt.Errorf("identity/order/lifecycle mismatch: parent=%T mounts=%d unmounts=%d text=%q", a.Parent(), mounts, unmounts, input.Text())
		} else if accepted && !*sameWindow {
			source.Destroy()
			if a.Root() != target || a.Destroyed() || input.Text() != text {
				failure = fmt.Errorf("source destruction affected moved page")
			}
		}
		wantDestructions, wantErrors := 0, 0
		if *expect == "target-destroy" || *expect == "source-destroy" {
			wantDestructions = 1
		}
		if *expect == "target-destroy" {
			wantErrors = 1
			if !tv.Destroyed() || sv.Current() != a || !a.Visible() || b.Visible() || a.Root() != source {
				failure = fmt.Errorf("rollback lost source selection or root")
			}
		}
		if *expect == "source-destroy" && !sv.Destroyed() {
			failure = fmt.Errorf("source survived its destruction callback")
		}
		if destructions != wantDestructions || transferErrors != wantErrors || detachRequests != 0 {
			failure = fmt.Errorf("destructions=%d errors=%d detach=%d", destructions, transferErrors, detachRequests)
		}
		wantMoves := 0
		if *targetMode == "mirror" && accepted {
			wantMoves = 1
		}
		if moves != wantMoves || closes != 0 {
			failure = fmt.Errorf("moves=%d want=%d closes=%d", moves, wantMoves, closes)
		}
		if moveError != nil {
			failure = moveError
		}
		if *targetMode == "relayout" && !relayout {
			failure = fmt.Errorf("target layout was not changed during native drag")
		}
		verified = true
		app.Post(app.Quit)
	})
	build := func(view *widgets.TabView, extra gui.Widget, includePage bool) gui.Widget {
		bar := widgets.NewTabBar()
		bar.SetView(view)
		bar.SetReorderable(true)
		bar.SetTransferable(true)
		if *targetMode == "relayout" && view == tv {
			bar.SetTabWidthRange(100, 100)
			// Subscribe after the bar's normal Enter handler, so that the
			// original insertion marker exists before requesting new layout.
			for _, controller := range bar.EventControllers() {
				if drop, ok := controller.(*gui.DropTarget); ok {
					drop.ConnectEnter(func(*gui.DragMotion) {
						if !relayout {
							relayout = true
							bar.SetTabWidthRange(140, 140)
							fmt.Println("RELAYOUT target tabs 100 -> 140 DIP; drop BEFORE T2")
						}
					})
				}
			}
		}
		bar.ConnectTransferError(func(err error) {
			transferErrors++
			if *expect == "target-destroy" && destructions == 1 {
				fmt.Printf("EXPECTED transfer error: %v\n", err)
				return
			}
			failure = err
			app.Post(app.Quit)
		})
		bar.ConnectDetachRequest(func(request *widgets.TabDetachRequest) {
			detachRequests++
			request.Cancel()
		})
		column := gui.NewLinearBox(layout.DirectionVertical)
		column.SetMainWeight(1)
		column.SetCrossAlign(layout.CrossStretch)
		column.SetSpacing(8)
		column.SetPadding(12)
		if decoration == gui.WindowChromeIntegrated {
			header := gui.NewHeaderBar()
			header.SetChild(bar)
			column.AddChild(header)
		} else {
			column.AddChild(bar)
		}
		column.AddChild(extra)
		if includePage {
			column.AddChild(view)
		}
		return column
	}
	instruction := "Drag A to the left half of T"
	if *targetMode == "empty" {
		instruction = "Drop A into the empty top bar"
	} else if *targetMode == "overflow" {
		instruction = "Hover at right edge; drop AFTER T8"
	} else if *targetMode == "mirror" {
		instruction = "Drop A AFTER B in the right bar"
	} else if *targetMode == "relayout" {
		instruction = "Hold over T2; wait for resize; drop BEFORE T2"
	}
	if *sameWindow {
		row := gui.NewLinearBox(layout.DirectionHorizontal)
		row.SetCrossAlign(layout.CrossStretch)
		row.AddChild(build(sv, gui.NewLabel(instruction), true))
		row.AddChild(build(tv, check, *targetMode != "mirror"))
		source.SetWidget(row)
	} else if *expect == "target-destroy" {
		source.SetWidget(build(sv, check, true))
		target.SetWidget(build(tv, gui.NewLabel("Target closes during A Unmount"), true))
	} else {
		source.SetWidget(build(sv, gui.NewLabel(instruction), true))
		target.SetWidget(build(tv, check, true))
	}
	if err = source.Show(); err != nil {
		return err
	}
	if err = source.SetPosition(nil, geometry.Point{X: 50, Y: 100}); err != nil {
		return err
	}
	if !*sameWindow {
		if err = target.Show(); err != nil {
			return err
		}
		if err = target.SetPosition(nil, geometry.Point{X: 600, Y: 100}); err != nil {
			return err
		}
	}
	fmt.Printf("OS=%s painter=%s scale=%s chrome=%s expect=%s target=%s same-window=%v\n", runtime.GOOS, os.Getenv("GOUI_PLAT_PAINTER"), os.Getenv("GOUI_PLAT_SCALE"), *chrome, *expect, *targetMode, *sameWindow)
	app.Run()
	if failure != nil {
		return failure
	}
	if !verified {
		return fmt.Errorf("closed without verification")
	}
	fmt.Printf("PASS %s mounts=%d unmounts=%d destructions=%d errors=%d detach=%d moves=%d closes=%d text=%q\n", *expect, mounts, unmounts, destructions, transferErrors, detachRequests, moves, closes, text)
	return nil
}
