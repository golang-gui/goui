// SplitView 两区／嵌套布局与输入验收，不使用 DevServer。
//
// 环境：Linux/X11、Windows 或 macOS 的可交互桌面；记录 OS、Linux WM、
// 实际绘制后端与缩放。仓库根目录启动：
//
//	go run ./tests/window/widgets/split_view
//
// 默认为 UI 声明绑定；加 -gui 验证纯 GUI 控件。GOUI_PLAT_PAINTER=software
// 或 opengl，Windows 可选 direct2d；GOUI_PLAT_SCALE=2 请求逻辑 2x。
// -fallback-style 改用显式 GUI+widgets 兜底样式；环境变量只代表请求。
// 初始：Native 900×600 DIP 窗口，Modern 亮色；左侧最小 120 DIP，右侧最小
// 240 DIP，编辑区与终端上下分区，终端初始固定 140 DIP。左侧有可输入内容。
// 默认外层以两侧测量初始化比例，不保证居中；窗口最小提示为 360×260 DIP。
//
// 操作与预期：
//  1. 在左右分隔线以及左右 4 DIP 内按住拖动；两侧实时分配且线跟随，无初始
//     跳动或拖动中重布局回弹。悬停显示缩放光标；默认分隔线始终为 1 DIP 中性色。
//     上下分隔条同理，拖到容器外再松手不继续拖动。内容互不越过分区裁剪。
//     Windows 回归：持续快速左右往返拖动至少 20 秒，再操作上下分隔条；窗口
//     应持续响应，不出现 0xC0000005。松手后点 Snapshot，几何断言仍应通过。
//  2. 点击分隔条后按主轴方向键、Shift+方向键，分别移动 1／10 DIP；Home／End
//     到两侧 MinSize 允许的位置。不相关方向键／Control+方向键不调整分区。
//     拖动中按 Esc，恢复按下前的大小；松手不会重新应用已取消的位置。
//     拖动结束并移开鼠标，分隔线不保留高亮或变粗；焦点仍在，可继续用方向键调整。
//  3. 拖动后缩小再放大窗口，比例模式保持比例，Start／End 模式保持对应首选
//     DIP 尺寸；不足时临时压缩、恢复后回到偏好。窗口小于内容最小总和时不得
//     产生负尺寸或崩溃。嵌套上下分区仍可单独操作。
//  4. 点 Mode 在内容建立的比例／Start 220／End 400／Ratio 0.5 间切换。
//     纯 GUI 的首选值由用户拖动更新；UI 的 OnResize 更新相应模型再重建。
//     点 Rebuild 不重置非受控比例、输入文字、选择或 ScrollView 的位置。
//  5. 点 Hide/Show 隐藏左侧：右侧填满、外层线消失；再显示恢复尺寸策略。
//     Toggle Dark 切换亮暗主题的中性色分隔条，不改变比例或内容。
//  6. 点 Snapshot：终端打印两侧实际大小及 separator 范围；assertions 检查
//     两侧之和加 1 DIP 等于容器主轴，真实内容子树与 separator 的边界及范围。
//     显示 ASSERT PASS 仅代表这些几何断言，不能代替上述桌面输入／外观验收。
//
// 平台差异：本例不扩展窗口装饰，macOS 需先激活窗口；没有全局 Tab 导航。
// 副作用：仅本例窗口、内存文字和日志；不写业务文件，不操作剪贴板或系统设置。
// 复位：重启。退出：关闭窗口；任一几何断言失败时 panic，返回非零状态。
package main

import (
	"flag"
	"fmt"
	"log"
	"math"
	"runtime"
	"strings"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/style"
	"github.com/golang-gui/goui/theme/modern"
	"github.com/golang-gui/goui/ui"
	"github.com/golang-gui/goui/widgets"
	widgetstyle "github.com/golang-gui/goui/widgets/style"
	wui "github.com/golang-gui/goui/widgets/ui"
)

var modeNames = []string{"Content ratio", "Start 220", "End 400", "Ratio 0.5"}
var document = strings.Repeat("SplitView text: edit, select and scroll independently.\n", 80)

func main() {
	guiMode := flag.Bool("gui", false, "use imperative GUI instead of UI binding")
	fallback := flag.Bool("fallback-style", false, "compose GUI and widgets fallback rules")
	flag.Parse()
	var err error
	if *guiMode {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		err = runGUI(*fallback)
	} else {
		err = runUI(*fallback)
	}
	if err != nil {
		log.Fatal(err)
	}
}
func sheet(dark, fallback bool) style.StyleSheet {
	if fallback {
		return style.Sheet(append(gui.DefaultStyleRules(), widgetstyle.Rules()...)...)
	}
	return modern.Sheet(modern.Options{Dark: dark})
}
func assertSplit(view *widgets.SplitView) {
	a, b := view.Sizes()
	extent := view.Rect().Width
	if view.Direction() == layout.DirectionVertical {
		extent = view.Rect().Height
	}
	info := view.Snapshot()
	active := view.StartChild() != nil && view.StartChild().Visible() && view.EndChild() != nil && view.EndChild().Visible()
	if active {
		if math.Abs(float64(a+b+1-extent)) > .01 || a < 0 || b < 0 {
			panic("split allocation does not fill view")
		}
		separator := info.Children[len(info.Children)-1]
		if separator.Role != widgets.RoleSeparator || separator.Range == nil || separator.Range.Value != a || a < separator.Range.Min-.01 || a > separator.Range.Max+.01 {
			panic("separator snapshot/range incorrect")
		}
		bounds, parent := separator.Bounds, info.Bounds
		if bounds.X < parent.X-.01 || bounds.Y < parent.Y-.01 || bounds.X+bounds.Width > parent.X+parent.Width+.01 || bounds.Y+bounds.Height > parent.Y+parent.Height+.01 {
			panic("separator hit bounds escaped split view")
		}
		hitWidth := bounds.Width
		if view.Direction() == layout.DirectionVertical {
			hitWidth = bounds.Height
		}
		if hitWidth < 0 || hitWidth > 9.01 || separator.Range.Direction != view.Direction() {
			panic("separator hit width/direction incorrect")
		}
		fmt.Printf("separator bounds=%v range=%+v\n", separator.Bounds, *separator.Range)
	}
	fmt.Printf("ASSERT PASS %s sizes=(%.1f,%.1f) extent=%.1f\n", view.ID(), a, b, extent)
}
func runUI(fallback bool) error {
	model := ui.NewTextModel(document)
	mode, dark, hidden, rebuilds := 0, false, false, 0
	startSize, endSize, ratio, terminal := float32(220), float32(400), float32(.5), float32(140)
	return ui.Run("org.golang-gui.SplitViewProbe", func(app ui.App) ui.RootView {
		left := ui.VBox(ui.Label("Sidebar / left"), ui.TextInput().ID("sidebar-input"), ui.Label("Minimum: 120 DIP")).Padding(12).Spacing(8).MinSize(120, 0).Hidden(hidden)
		right := wui.VSplit(
			ui.ScrollView(ui.TextView().ID("editor").Model(model)).MinSize(0, 100),
			ui.ScrollView(ui.Label(strings.Repeat("Terminal output\n", 40))).MinSize(0, 60),
		).ID("inner").MinSize(240, 0).EndSize(terminal).OnResize(func(_, end float32) { terminal = end; app.RequestUpdate() })
		split := wui.HSplit(left, right).ID("outer").OnResize(func(start, end float32) {
			switch mode {
			case 1:
				startSize = start
			case 2:
				endSize = end
			case 3:
				ratio = start / (start + end)
			}
			fmt.Printf("outer resize %.1f %.1f\n", start, end)
			app.RequestUpdate()
		})
		switch mode {
		case 1:
			split.StartSize(startSize)
		case 2:
			split.EndSize(endSize)
		case 3:
			split.Ratio(ratio)
		}
		return ui.Root().StyleSheet(sheet(dark, fallback)).Windows(
			ui.Window("split").Title("GOUI SplitView UI").Size(900, 600).MinSize(360, 260).Content(
				ui.VBox(
					ui.HBox(
						ui.Button("Mode").OnClick(func() { mode = (mode + 1) % 4; startSize, endSize, ratio = 220, 400, .5; app.RequestUpdate() }),
						ui.Button("Rebuild").OnClick(func() { rebuilds++; app.RequestUpdate() }),
						ui.Button("Hide/Show").OnClick(func() { hidden = !hidden; app.RequestUpdate() }),
						ui.Button("Toggle Dark").OnClick(func() { dark = !dark; app.RequestUpdate() }),
						ui.Button("Snapshot").OnClick(func() {
							assertSplit(app.FindWidget("split", "outer").(*widgets.SplitView))
							assertSplit(app.FindWidget("split", "inner").(*widgets.SplitView))
						}),
					).Spacing(8).Padding(8),
					ui.Label(fmt.Sprintf("%s | Rebuilds=%d | Drag; arrows / Shift / Home / End / Esc", modeNames[mode], rebuilds)),
					split,
				).CrossAlign(layout.CrossStretch),
			),
		)
	})
}
func runGUI(fallback bool) error {
	app, err := gui.NewApplication("org.golang-gui.SplitViewGUIProbe")
	if err != nil {
		return err
	}
	app.SetStyleSheet(sheet(false, fallback))
	window, err := app.NewWindow(&gui.WindowOptions{Size: geometry.Size{Width: 900, Height: 600}})
	if err != nil {
		return err
	}
	defer window.Destroy()
	if err = window.SetTitle("GOUI SplitView GUI"); err != nil {
		return err
	}
	window.SetMinSize(geometry.Size{Width: 360, Height: 260})
	left := gui.NewLinearBox(layout.DirectionVertical)
	left.SetPadding(12)
	left.SetSpacing(8)
	left.SetMinSize(geometry.Size{Width: 120})
	left.AddChild(gui.NewLabel("Sidebar / left"))
	left.AddChild(gui.NewTextInput())
	left.AddChild(gui.NewLabel("Minimum: 120 DIP"))
	editor := gui.NewTextView()
	editor.SetModel(gui.NewTextModel(document))
	scroll := gui.NewScrollView()
	scroll.SetChild(editor)
	scroll.SetMinSize(geometry.Size{Height: 100})
	terminal := gui.NewScrollView()
	terminal.SetChild(gui.NewLabel(strings.Repeat("Terminal output\n", 40)))
	terminal.SetMinSize(geometry.Size{Height: 60})
	inner := widgets.NewSplitView(layout.DirectionVertical)
	inner.SetID("inner")
	inner.SetStartChild(scroll)
	inner.SetEndChild(terminal)
	inner.SetEndSize(140)
	inner.SetMinSize(geometry.Size{Width: 240})
	outer := widgets.NewSplitView(layout.DirectionHorizontal)
	outer.SetID("outer")
	outer.SetStartChild(left)
	outer.SetEndChild(inner)
	outer.ConnectResize(func(a, b float32) { fmt.Printf("outer resize %.1f %.1f\n", a, b) })
	inner.ConnectResize(func(a, b float32) { fmt.Printf("inner resize %.1f %.1f\n", a, b) })
	root := gui.NewLinearBox(layout.DirectionVertical)
	root.SetCrossAlign(layout.CrossStretch)
	toolbar := gui.NewLinearBox(layout.DirectionHorizontal)
	toolbar.SetPadding(8)
	toolbar.SetSpacing(8)
	mode, dark, rebuilds := 0, false, 0
	status := gui.NewLabel(modeNames[0])
	button := func(text string, fn func()) {
		b := gui.NewButton()
		b.SetChild(gui.NewLabel(text))
		b.ConnectClicked(fn)
		toolbar.AddChild(b)
	}
	button("Mode", func() {
		mode = (mode + 1) % 4
		switch mode {
		case 0: // Retain current proportional allocation, like UI omission.
			a, b := outer.Sizes()
			if a+b > 0 {
				outer.SetRatio(a / (a + b))
			}
		case 1:
			outer.SetStartSize(220)
		case 2:
			outer.SetEndSize(400)
		case 3:
			outer.SetRatio(.5)
		}
		status.SetText(modeNames[mode])
	})
	button("Rebuild", func() {
		rebuilds++
		status.SetText(fmt.Sprintf("%s | Rebuilds=%d", modeNames[mode], rebuilds))
		root.RequestLayout()
	})
	button("Hide/Show", func() { left.SetVisible(!left.Visible()) })
	button("Toggle Dark", func() { dark = !dark; app.SetStyleSheet(sheet(dark, fallback)) })
	button("Snapshot", func() { assertSplit(outer); assertSplit(inner) })
	root.AddChild(toolbar)
	root.AddChild(status)
	root.AddChild(outer)
	window.SetWidget(root)
	if err = window.Show(); err != nil {
		return err
	}
	app.Run()
	return nil
}
