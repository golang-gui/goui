// UI TabView 跨窗口子树移交验收，不使用 DevServer。
//
// 环境：Linux/X11、Windows 或 macOS 可交互桌面，记录 OS、窗口管理器、实际
// 绘制后端与缩放。启动命令在仓库根目录运行：
//
//	go run ./tests/window/widgets/ui_tab_transfer
//
// 初始：source 有 A/B 两页，target 有 C；输入框初值为 retained A/B/C。
// 默认 Native 装饰、-expect transferred；每个模式独立启动，不串联修改后的状态。
//
// 主要操作与预期：
//  1. 激活 source，选中 A 输入框并输入 edited A（全选后替换）。
//  2. 抓住 Document A 标签中部，先向下拖出栏，再拖到 target 标签栏松开。
//     预期 source 仅剩 B；target 显示 A，输入文本仍为 edited A。
//     控制台 TRANSFER 仅一次。不是创建另一个输入框并拷贝文本。
//  3. 在 target 点击 Rebuild。预期 A 文本不变，仍能编辑；将内容改成 after move。
//  4. 在 target 点击 Verify and close source。检查页面/输入框/State 身份、
//     View Mount=1/Unmount=0、文本通知连接；关闭源窗口后目标仍正常。
//     关闭源的 UI 重建完成后输出 PASS，程序自动退出。
//  5. 以 -expect canceled 重新启动：不要执行步骤 1，不修改初始 retained A。
//     拖离源栏，确认原生预览出现后保持按下、按 Esc、再松开，点击任一窗口 Verify。
//     预期零次转移、零次额外 GUI/View 卸载，源仍持有 A；不要只点击 Verify，
//     状态未变化不能证明执行过取消手势。此模式不执行步骤 3、4 的编辑流程。
//  6. 以 -expect detached 重新启动：初始只有 source。按步骤 1 编辑，再把 A
//     拖到所有测试窗口外的桌面空白处；松开后不要再移动源窗口或调整缩放。
//     目标窗口通过 UI 声明创建，正常布局后定位、移交；执行步骤 3、4。
//     Verify 额外检查目标标签的原抓取点与释放点相差不超过 1 物理像素。
//
// 可选回归（每项重新启动，仅执行该模式）：
//  1. -expect prepare-failure / -expect prepare-cancel：不要执行主要步骤 1，
//     保持初始 retained A，拖 A 到桌面。
//     准备窗口短暂出现后关闭；failure 使用同 ID 的空页面触发 UI 移交拒绝，
//     cancel 主动取消请求。两者均检查请求不可复用，源仍有 A/B。
//     点击源 Verify，断言原页面从未发生 GUI 或 View 卸载。不要在准备中移动源窗口。
//  2. -expect target-destroy：按主要步骤 1、2 操作。A 的 GUI Unmount 回调立即销毁目标，
//     源应恢复 A/B、A 仍选中。在源点击 Rebuild，将原输入框改成 after move，点击 Verify。
//     预期零次 OnTransfer/DetachRequest；原 State 和连接保留，GUI Mount/Unmount=2/1，
//     View Unmount=0；目标声明已移除，不会在重建时复活。
//  3. -expect source-destroy：按主要步骤 1、2 操作。A 在目标的 GUI Mount 回调立即销毁源。
//     在目标执行主要步骤 3、4，预期一次 OnTransfer，原 State/输入框仍可重建和编辑，
//     源声明正常移除；源 View 的销毁不能清理正在移交的 A 的协调记录。
//  4. -same-window：source/target 在同一个 Root 内，按主要步骤 1–4 验证，但将 A
//     放在 C 右半侧，预期目标顺序 C/A；验证按钮为 Verify and remove source，
//     移除源容器而非关闭窗口，目标仍保留原页面和 State。
//     可加 -target-first 将目标放左侧，验证目标先于源重建的情况；只支持 transferred
//     和 canceled。取消时按主要步骤 5 操作，同样不能先修改 retained A。
//     Integrated 时两侧 TabBar 都是 HeaderBar 的 child，标签拖动不应移动窗口；
//     2x 同窗口模式建议桌面至少 2400×900 物理像素，不代表真实跨屏混合 DPI。
//
// 可加 -chrome integrated 检查标签拖动与 HeaderBar 拖窗不会混淆。
// 可选逻辑 2x：POSIX 使用 GOUI_PLAT_SCALE=2 go run ./tests/window/widgets/ui_tab_transfer；
// PowerShell 先设 $env:GOUI_PLAT_SCALE='2' 再启动，结束后恢复原值；不代表混合 DPI。
// 平台差异：macOS 先激活窗口再操作，原生预览/取消动画由系统决定；Linux Integrated
// 需要合成器，应记录是否回退为 Native。全选用 Ctrl+A，macOS 用 Cmd+A。
// 结果：Verify 自动断言页面/输入框/State 身份、连接、生命周期和重建结果，成功输出
// PASS；错误日志或未完成以非零状态退出。预览、输入体验和真实释放点需独立观察。
// 副作用：仅创建、移动、关闭测试窗口，不改设置、剪贴板或文件。
// 复位：重新启动。退出：Verify 自动退出；关闭窗口不算验收，返回失败状态。
package main

import (
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"slices"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/theme/modern"
	"github.com/golang-gui/goui/ui"
	"github.com/golang-gui/goui/widgets"
	wui "github.com/golang-gui/goui/widgets/ui"
)

type pageState struct {
	input                            *gui.TextInput
	connection                       signal.Handle
	updates, notifications, unmounts int
	last                             string
	guiMounts, guiUnmounts           int
	lifecycle                        signal.Handles
}

type pageView struct {
	ui.ViewBase[pageView]
	key       string
	states    map[string]*pageState
	lifecycle func(*gui.TextInput, bool)
}

func (v *pageView) Build() ui.View { return v }
func (v *pageView) Mount(ctx ui.BuildContext) gui.Widget {
	if v.states[v.key] != nil {
		panic("page was recreated: " + v.key)
	}
	input := gui.NewTextInput()
	input.SetText("retained " + v.key)
	s := &pageState{input: input, last: input.Text()}
	s.connection = input.ConnectText(func(value string) { s.notifications++; s.last = value })
	s.lifecycle = signal.Handles{input.ConnectMount(func() {
		s.guiMounts++
		if v.lifecycle != nil {
			v.lifecycle(input, true)
		}
	}), input.ConnectUnmount(func() {
		s.guiUnmounts++
		if v.lifecycle != nil {
			v.lifecycle(input, false)
		}
	})}
	v.states[v.key] = s
	ctx.SetState(s)
	return input
}
func (v *pageView) Update(ctx ui.BuildContext, _ gui.Widget) {
	ctx.State().(*pageState).updates++
}
func (*pageView) Unmount(ctx ui.BuildContext, _ gui.Widget) {
	s := ctx.State().(*pageState)
	s.unmounts++
	s.connection.Disconnect()
	s.lifecycle.Disconnect()
}

func main() {
	expect := flag.String("expect", "transferred", "transferred, canceled, detached, prepare-failure, prepare-cancel, target-destroy or source-destroy")
	chrome := flag.String("chrome", "native", "native or integrated")
	sameWindow := flag.Bool("same-window", false, "two tab views in one UI Root")
	targetFirst := flag.Bool("target-first", false, "build target before source; requires same-window")
	flag.Parse()
	if !slices.Contains([]string{"transferred", "canceled", "detached", "prepare-failure", "prepare-cancel", "target-destroy", "source-destroy"}, *expect) {
		log.Fatal("invalid expectation")
	}
	decoration := ui.WindowChromeNative
	if *chrome == "integrated" {
		decoration = ui.WindowChromeIntegrated
	} else if *chrome != "native" {
		log.Fatal("invalid chrome")
	}
	if *sameWindow {
		if *expect != "transferred" && *expect != "canceled" {
			log.Fatal("same-window supports transferred or canceled")
		}
		if err := runSameWindow(*expect, decoration, *targetFirst); err != nil {
			log.Fatal(err)
		}
		return
	}
	if *targetFirst {
		log.Fatal("target-first requires same-window")
	}
	models := map[string][]string{"source": {"A", "B"}, "target": {"C"}}
	detaching := *expect == "detached" || *expect == "prepare-failure" || *expect == "prepare-cancel"
	if detaching {
		delete(models, "target")
	}
	frames := make(map[string]*frameWidget)
	states := make(map[string]*pageState)
	var original *widgets.TabPage
	transfers, builds := 0, 0
	requests := 0
	destructions := 0
	var pending *wui.TabDetachRequest
	var release, hotspot, sourceOrigin geometry.Point
	var sourceScale, ratio float32
	verified, positioned := false, false
	var failure error
	err := ui.Run("org.golang-gui.UITabTransfer", func(app ui.App) ui.RootView {
		builds++
		fail := func(err error) {
			if failure == nil {
				failure = err
			}
			log.Print("FAIL ", err)
			app.Quit()
		}
		transfer := func(change wui.TabTransfer) {
			transfers++
			from, to := models[change.SourceWindow], models[change.TargetWindow]
			if detaching {
				to = slices.DeleteFunc(to, func(key string) bool { return key == "preparing" })
			}
			index := slices.Index(from, change.PageID)
			if index < 0 || change.Index < 0 || change.Index > len(to) {
				fail(fmt.Errorf("invalid model transfer: %+v", change))
				return
			}
			models[change.SourceWindow] = slices.Delete(from, index, index+1)
			models[change.TargetWindow] = slices.Insert(to, change.Index, change.PageID)
			if *expect == "source-destroy" {
				delete(models, "source")
			}
			log.Printf("TRANSFER %+v", change)
			app.RequestUpdate()
		}
		lifecycle := func(input *gui.TextInput, mounted bool) {
			if destructions != 0 || original == nil {
				return
			}
			id := ""
			if *expect == "target-destroy" && !mounted {
				id = "target"
				delete(models, id)
			} else if *expect == "source-destroy" && mounted && input.Window() == app.FindWindow("target") {
				id = "source"
			}
			if id == "" {
				return
			}
			window := app.FindWindow(id)
			if window == nil {
				fail(fmt.Errorf("missing %s at lifecycle boundary", id))
				return
			}
			destructions++
			window.Destroy()
			app.RequestUpdate()
			log.Printf("DESTROY %s during GUI mounted=%v", id, mounted)
		}
		detach := func(request *wui.TabDetachRequest) {
			requests++
			if !detaching || requests != 1 || request.PageID != "A" {
				request.Cancel()
				fail(fmt.Errorf("unexpected detach request"))
				return
			}
			var err error
			sourceOrigin, err = request.Window.Position(nil)
			if err != nil {
				request.Cancel()
				fail(err)
				return
			}
			pending, release, hotspot = request, request.Position, request.Hotspot
			sourceScale = frames["source"].scale
			models["target"] = []string{"preparing"} // metadata only, never duplicate the editor
			if *expect == "prepare-failure" {
				models["target"] = []string{"A"} // conflicting page ID, no second editor
			}
			app.RequestUpdate()
		}
		prepare := func(frame *frameWidget) {
			request := pending
			if request == nil {
				return
			}
			target := app.FindWindow("target")
			view, _ := app.FindWidget("target", "documents").(*widgets.TabView)
			abort := func(err error) {
				pending = nil
				request.Cancel()
				delete(models, "target")
				app.RequestUpdate()
				if err != nil {
					fail(err)
				}
			}
			if target == nil || view == nil {
				abort(fmt.Errorf("target preparation missing UI ownership"))
				return
			}
			if *expect == "prepare-failure" || *expect == "prepare-cancel" {
				if *expect == "prepare-failure" {
					if len(view.Pages()) != 1 || view.Pages()[0] == original || view.Pages()[0].Child() != nil {
						abort(fmt.Errorf("missing independent same-ID target page"))
						return
					}
					if err := request.TransferTo(view, 0); err == nil {
						abort(fmt.Errorf("duplicate page ID accepted"))
						return
					} else {
						log.Printf("EXPECTED transfer rejection: %v", err)
					}
				} else {
					request.Cancel()
				}
				if request.TransferTo(view, 0) == nil {
					abort(fmt.Errorf("consumed request committed"))
					return
				}
				abort(nil)
				log.Printf("EXPECTED %s; prepared target discarded, source retained", *expect)
				return
			}
			origin, err := request.Window.Position(nil)
			if err != nil {
				abort(err)
				return
			}
			if origin != sourceOrigin || sourceScale <= 0 || frames["source"].scale != sourceScale || frame.scale <= 0 {
				abort(fmt.Errorf("source moved or changed scale during preparation"))
				return
			}
			border, err := target.Position(target)
			if err != nil {
				abort(err)
				return
			}
			anchor, ok := firstTab(target.Widget().Snapshot())
			if !ok {
				abort(fmt.Errorf("target placeholder not laid out"))
				return
			}
			ratio = frame.scale / sourceScale
			position := release.Add(anchor.Add(hotspot).Add(border.Scale(-1)).Scale(-ratio))
			if err := target.SetPosition(request.Window, position); err != nil {
				abort(err)
				return
			}
			if err := request.TransferTo(view, 0); err != nil {
				abort(err)
				return
			}
			pending = nil
			log.Printf("PREPARED UI target requested=%v release=%v ratio=%g", position, release, ratio)
		}
		verify := func() {
			state := states["A"]
			windowID := "target"
			if *expect == "canceled" || *expect == "prepare-failure" || *expect == "prepare-cancel" || *expect == "target-destroy" {
				windowID = "source"
			}
			view, _ := app.FindWidget(windowID, "documents").(*widgets.TabView)
			if view == nil || original == nil || !slices.Contains(view.Pages(), original) || original.Child() != state.input || state.unmounts != 0 || app.FindWidget(windowID, "A") != original {
				fail(fmt.Errorf("original page/input/State not retained"))
				return
			}
			if *expect == "target-destroy" {
				if destructions != 1 || transfers != 0 || requests != 0 || app.FindWindow("target") != nil ||
					len(view.Pages()) != 2 || view.Pages()[0] != original || view.Current() != original ||
					state.guiMounts != 2 || state.guiUnmounts != 1 || state.updates < 3 || state.notifications < 2 ||
					state.last != "after move" || state.input.Text() != "after move" {
					fail(fmt.Errorf("UI rollback identity/rebuild/lifecycle failed: destructions=%d transfers=%d requests=%d lifecycle=%d/%d updates=%d text=%q", destructions, transfers, requests, state.guiMounts, state.guiUnmounts, state.updates, state.input.Text()))
					return
				}
				verified = true
				log.Printf("PASS UI target-destroy + rebuild + editing; original page/State retained, GUI lifecycle=2/1, View Unmount=0")
				app.Quit()
				return
			}
			if windowID == "source" {
				wantRequests := 0
				if detaching {
					wantRequests = 1
				}
				if requests != wantRequests || transfers != 0 || state.input.Text() != "retained A" || state.guiMounts != 1 || state.guiUnmounts != 0 || (detaching && app.FindWindow("target") != nil) {
					fail(fmt.Errorf("cancel changed source"))
					return
				}
				verified = true
				log.Printf("PASS UI %s; original page/State retained without unmount", *expect)
				app.Quit()
				return
			}
			if transfers != 1 || state.updates < 3 || state.notifications < 2 || state.last != "after move" || state.input.Text() != "after move" {
				fail(fmt.Errorf("transfer/rebuild/edit checks: transfers=%d updates=%d notifications=%d text=%q", transfers, state.updates, state.notifications, state.input.Text()))
				return
			}
			if state.guiMounts != 2 || state.guiUnmounts != 1 {
				fail(fmt.Errorf("GUI lifecycle=%d/%d", state.guiMounts, state.guiUnmounts))
				return
			}
			if *expect == "source-destroy" && (destructions != 1 || app.FindWindow("source") != nil || requests != 0) {
				fail(fmt.Errorf("source lifecycle destruction not observed"))
				return
			}
			if *expect == "detached" {
				if requests != 1 || pending != nil {
					fail(fmt.Errorf("detach request not committed once"))
					return
				}
				source, target := app.FindWindow("source"), app.FindWindow("target")
				outer, err := target.Position(source)
				if err != nil {
					fail(err)
					return
				}
				border, err := target.Position(target)
				if err != nil {
					fail(err)
					return
				}
				anchor, ok := firstTab(target.Widget().Snapshot())
				actual := outer.Add(anchor.Add(hotspot).Add(border.Scale(-1)).Scale(ratio))
				if !ok || math.Abs(float64(actual.X-release.X))*float64(sourceScale) > 1.01 || math.Abs(float64(actual.Y-release.Y))*float64(sourceScale) > 1.01 {
					fail(fmt.Errorf("position actual=%v release=%v ratio=%g", actual, release, ratio))
					return
				}
				log.Printf("POSITION actual=%v release=%v ratio=%g", actual, release, ratio)
			}
			delete(models, "source")
			app.RequestUpdate()
			app.Post(func() {
				if app.FindWindow("source") != nil || original.Child() != state.input || state.input.Window() != app.FindWindow("target") || state.unmounts != 0 || state.input.Text() != "after move" {
					fail(fmt.Errorf("source destruction damaged target"))
					return
				}
				verified = true
				log.Printf("PASS UI transfer + rebuild + source close; same page/input/State, updates=%d notifications=%d", state.updates, state.notifications)
				app.Quit()
			})
		}
		var windows []ui.WindowView
		for _, id := range []string{"source", "target"} {
			keys, exists := models[id]
			if !exists {
				continue
			}
			var pages []*wui.TabPageView
			for _, key := range keys {
				if key == "preparing" || (id == "target" && *expect == "prepare-failure") {
					pages = append(pages, wui.TabPage(key, nil).Title("Document A"))
					continue
				}
				p := &pageView{key: key, states: states}
				if key == "A" {
					p.lifecycle = lifecycle
				}
				p.Self = p
				p.ID("input-" + key)
				pages = append(pages, wui.TabPage(key, p).Title("Document "+key))
			}
			bar := wui.TabBar("documents").ID("tab-bar").Reorderable(true).Transferable(true).OnTransferError(fail).OnDetachRequest(detach)
			var heading ui.View = bar
			if *chrome == "integrated" {
				heading = ui.HeaderBar(bar).Padding(6)
			}
			content := ui.VBox(
				heading,
				ui.Label(fmt.Sprintf("%s / builds %d", id, builds)),
				ui.HBox(ui.Button("Rebuild").OnClick(app.RequestUpdate), ui.Button("Verify and close source").OnClick(verify)).Spacing(8),
				wui.TabView(pages...).ID("documents").OnTransfer(transfer),
			).Spacing(12).Padding(12)
			frame := &frameView{key: id, frames: frames, child: content}
			frame.Self = frame
			if id == "target" && pending != nil {
				frame.after = prepare
			}
			windows = append(windows, ui.Window(id).Title("GOUI UI TRANSFER "+id).Chrome(decoration).Size(520, 340).Content(frame))
		}
		if !positioned {
			positioned = true
			app.Post(func() {
				source, target := app.FindWindow("source"), app.FindWindow("target")
				if source == nil || (!detaching && target == nil) {
					fail(fmt.Errorf("initial windows missing"))
					return
				}
				if err := source.SetPosition(nil, geometry.Point{X: 50, Y: 80}); err != nil {
					fail(err)
					return
				}
				// Independent initial requests: the source's asynchronous native
				// move need not have completed when the second request is sent.
				if target != nil {
					if err := target.SetPosition(nil, geometry.Point{X: 650, Y: 80}); err != nil {
						fail(err)
						return
					}
				}
				original = app.FindWidget("source", "documents").(*widgets.TabView).Pages()[0]
				log.Print("READY UI transfer")
			})
		}
		return ui.Root().StyleSheet(modern.Sheet(modern.Options{})).Windows(windows...)
	})
	if err != nil {
		failure = err
	}
	if failure != nil || !verified {
		fmt.Fprintln(os.Stderr, "FAIL: verification incomplete:", failure)
		os.Exit(1)
	}
}
