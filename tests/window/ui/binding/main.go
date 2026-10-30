// 声明式自动读取追踪、局部更新与属性绑定窗口验收，不依赖 DevServer。
// 环境：Linux X11 / Windows / macOS 桌面，记录后端和缩放。
// 启动：go run ./tests/window/ui/binding；-auto 经正常窗口输入路径自动断言后退出。
// 可用 GOUI_PLAT_PAINTER=software/opengl、GOUI_PLAT_SCALE=1/2 重复。
// 初始：主窗及镜像窗、空输入框，Global=1，Local=1，属性探针 Update=1。
// 操作与逐项预期：
//  1. 输入文字：两个窗口的标签和多行编辑框同步；光标/选区保持正确。
//     F6 输出 CHECK PASS，Global 与探针 Update 不增加，Local 随输入更新。
//  2. 点击 Increment：计算标签增加，输入框身份、内容和光标保留。
//  3. 点击 Cycle：中间内容依次 Label -> TextInput -> 空 -> Label，前后标签
//     始终在原相对位置；Overlay 在奇数阶段出现，浮层容器填满区域，文字居中；
//     底层文字保持左上角，两段文字应清晰分开。Fill 只控制尺寸，不提供背景遮挡。
//  4. 点击 Rebuild：Global 和探针 Update 各增加一次，所有同类型控件保持身份。
//  5. 关闭镜像窗后输入：主窗继续同步，不复活镜像窗；再次 Rebuild 会按声明重建。
//  6. 构建均不声明 Observe。Overlay 显示 count；可见时 Increment 更新居中文本，
//     隐藏时 Increment 不执行 Overlay 构建。F6 的 overlayBuild 可核对条件依赖。
//
// -auto 检查输入回写、通知合并、属性更新与 Build 的范围、空输出、槽位、Snapshot、
// 全量更新、窗口关闭和可见／隐藏时的动态依赖。它不替代不同字体/主题下的人工外观观察。
// 平台差异：仅装饰和字体可能不同，更新语义一致；无真实系统设置/剪贴板副作用。
// 复位：重新启动。退出：关闭主窗（同时关闭镜像窗），或 -auto 完成后退出。
package main

import (
	"flag"
	"fmt"
	"log"
	"math"
	"time"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/theme/modern"
	"github.com/golang-gui/goui/ui"
)

// 作为自定义 WidgetView 验证 BindValue，Update 次数与属性 setter 次数分开。
type boundProbe struct {
	ui.ViewBase[boundProbe]
	text             *ui.State[string]
	updates, applies *int
}

func probe(text *ui.State[string], updates, applies *int) *boundProbe {
	v := &boundProbe{text: text, updates: updates, applies: applies}
	v.Self = v
	return v.ID("probe")
}
func (*boundProbe) Mount(ui.BuildContext) gui.Widget { return gui.NewLabel("") }
func (v *boundProbe) Update(ctx ui.BuildContext, widget gui.Widget) {
	*v.updates++
	ui.BindValue(ctx, "text", v.text, func(value string) { *v.applies++; widget.(*gui.Label).SetText(value) })
}
func (*boundProbe) Unmount(ui.BuildContext, gui.Widget) {}

func require(ok bool, message string) {
	if !ok {
		panic(message)
	}
}

func main() {
	auto := flag.Bool("auto", false, "自动验证后退出")
	flag.Parse()
	text, count, mode := ui.MakeState(""), ui.MakeState(0), ui.MakeState(0)
	global, local, updates, applies, overlayBuilds := 0, 0, 0, 0, 0
	overlayBefore := 0
	started, step := false, 0
	var original gui.Widget
	err := ui.Run("org.golang-gui.UIBinding", func(app ui.App) ui.RootView {
		global++
		check := func() {
			w := app.FindWindow("main")
			input := app.FindWidget("main", "input").(*gui.TextInput)
			require(input.Text() == text.Get(), "input/state mismatch")
			require(app.FindWidget("main", "probe").(*gui.Label).Text() == text.Get(), "custom binding mismatch")
			require(app.FindWidget("main", "multi").(*gui.TextView).Model().Text() == text.Get(), "multiline mismatch")
			require(original == nil || original == input, "input identity changed")
			if mirror := app.FindWindow("mirror"); mirror != nil {
				require(gui.FindWidget(mirror, "mirror-text").Snapshot().Text == text.Get(), "mirror snapshot mismatch")
			}
			fmt.Printf("CHECK PASS global=%d local=%d propertyUpdate=%d propertyApply=%d overlayBuild=%d text=%q\n", global, local, updates, applies, overlayBuilds, text.Get())
			for _, id := range []string{"input", "increment", "cycle", "rebuild"} {
				b := gui.FindWidget(w, id).Snapshot().Bounds
				fmt.Printf("BOUNDS %s %.1f %.1f %.1f %.1f\n", id, b.X, b.Y, b.Width, b.Height)
			}
		}
		if !started {
			started = true
			app.TimeoutFunc(300*time.Millisecond, func() {
				original = app.FindWidget("main", "input")
				check()
			})
			if *auto {
				var timer *ui.Timer
				timer = app.TickFunc(500*time.Millisecond, func() {
					win := app.FindWindow("main")
					click := func(id string) {
						b := gui.FindWidget(win, id).Snapshot().Bounds
						p := geometry.Point{X: b.X + b.Width/2, Y: b.Y + b.Height/2}
						for _, kind := range []events.EventType{events.PointerMove, events.PointerDown, events.PointerUp} {
							e := events.PointerEvent{EventType: kind, Position: p, Button: events.PointerButtonLeft}
							if err := win.DispatchEvent(e); err != nil {
								panic(err)
							}
						}
					}
					switch step {
					case 0:
						click("input")
						if err := win.CommitText("typed 中文 e\u0301"); err != nil {
							panic(err)
						}
						click("increment")
						click("increment")
					case 1:
						check()
						require(text.Get() == "typed 中文 e\u0301" && global == 1 && local == 2 && updates == 1 && applies == 2 && count.Get() == 2, "local update range/coalescing")
						click("cycle")
					case 2:
						_, ok := app.FindWidget("main", "dynamic").(*gui.TextInput)
						require(ok, "local output type replacement")
						o := app.FindWidget("main", "overlay").(*gui.Overlay)
						floating := gui.FindWidget(win, "floating")
						require(floating != nil && o.OverlayFill(floating) && floating.Rect().Size == o.Rect().Size, "local overlay slot/geometry")
						label := gui.FindWidget(win, "floating-text")
						require(label != nil, "missing overlay label")
						center, expected := label.Snapshot().Bounds.Center(), o.Snapshot().Bounds.Center()
						// DIP 中核对居中，容差只用于浮点运算误差，远小于一个像素。
						require(math.Abs(float64(center.X-expected.X)) < 0.001 && math.Abs(float64(center.Y-expected.Y)) < 0.001, "overlay label is not centered")
						click("cycle")
					case 3:
						require(app.FindWidget("main", "dynamic") == nil && app.FindWidget("main", "floating") == nil, "empty output")
						click("cycle")
					case 4:
						_, ok := app.FindWidget("main", "dynamic").(*gui.Label)
						require(ok, "restore empty output")
						click("rebuild")
					case 5:
						check()
						require(global == 2 && updates == 2, "explicit global update")
						app.FindWindow("mirror").Destroy()
						text.Set("after mirror close")
					case 6:
						check()
						require(app.FindWindow("mirror") == nil && global == 2 && updates == 2, "closed window resurrected")
						overlayBefore = overlayBuilds
						count.Update(func(value int) int { return value + 1 })
					case 7:
						require(overlayBuilds == overlayBefore+1, "visible overlay failed to track count")
						mode.Update(func(value int) int { return value + 1 })
					case 8:
						require(app.FindWidget("main", "floating") == nil, "overlay did not hide")
						overlayBefore = overlayBuilds
						count.Update(func(value int) int { return value + 1 })
					case 9:
						require(overlayBuilds == overlayBefore, "hidden overlay retained count dependency")
						check()
						timer.Stop()
						fmt.Println("UI BINDING PASS")
						win.Destroy()
					}
					step++
				})
			}
		}
		return ui.Root().StyleSheet(modern.FromSettings(app.Settings())).Windows(
			ui.Window("main").Title("GOUI UI binding").Size(680, 540).Center(true).
				Shortcuts(ui.Shortcut(ui.KeyF6).OnActivate(check)).OnDestroy(func() {
				if w := app.FindWindow("mirror"); w != nil {
					w.Destroy()
				}
			}).Content(
				ui.VBox(
					ui.Label(fmt.Sprintf("Global %d — F6 checks current counters", global)),
					ui.TextInput().ID("input").BindText(&text),
					probe(&text, &updates, &applies),
					ui.ScrollView(ui.TextView().ID("multi").BindText(&text)).MinHeight(70).MaxHeight(70),
					ui.Build(func() ui.View {
						local++
						return ui.Label(fmt.Sprintf("Local %d | count %d | text %s", local, count.Get(), text.Get()))
					}),
					ui.HBox(ui.Button("Increment").ID("increment").OnClick(func() { count.Update(func(v int) int { return v + 1 }) }), ui.Button("Cycle").ID("cycle").OnClick(func() { mode.Update(func(v int) int { return v + 1 }) }), ui.Button("Rebuild").ID("rebuild").OnClick(app.RequestUpdate)).Spacing(10),
					ui.VBox(ui.Label("before"), ui.Build(func() ui.View {
						switch mode.Get() % 3 {
						case 0:
							return ui.Label("dynamic label").ID("dynamic")
						case 1:
							return ui.TextInput().ID("dynamic")
						default:
							return nil
						}
					}), ui.Label("after")).Spacing(6),
					ui.Overlay(ui.Label("Overlay base"), ui.OverlayItem(ui.Build(func() ui.View {
						overlayBuilds++
						if mode.Get()%2 == 0 {
							return nil
						}
						return ui.HBox(ui.Label(fmt.Sprintf("Overlay Fill · count %d", count.Get())).ID("floating-text")).ID("floating").
							MainAlign(layout.MainCenter).CrossAlign(layout.CrossCenter)
					})).Fill(true)).ID("overlay").MinHeight(70).MaxHeight(70),
				).Padding(20).Spacing(12).CrossAlign(layout.CrossStretch),
			),
			ui.Window("mirror").Title("GOUI UI binding mirror").Size(320, 160).Content(ui.VBox(ui.Label("Shared source"), ui.Label().ID("mirror-text").BindText(&text)).Padding(20).Spacing(12)),
		)
	})
	if err != nil {
		log.Fatal(err)
	}
}
