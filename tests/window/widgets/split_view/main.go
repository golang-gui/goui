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
// 240 DIP，编辑区与终端上下分区，终端初始固定 140 DIP。左侧有输入框和滚动列表。
// 分隔区初始3 DIP，线1 DIP；侧栏和终端使用同色背景，正文使用另一背景。
// 默认外层以两侧测量初始化比例，不保证居中；窗口最小提示为 360×260 DIP。
//
// 操作与预期：
//
//  1. 在默认3 DIP分隔区域的两侧留白和中线分别悬停、按下拖动：整个区域
//     都应立即显示调整光标并开始拖动，没有区域外的隐藏抓取带。线默认1 DIP，
//     两侧背景分别衔接侧栏和正文；区域外的滚动条、按钮及文本拖选正常工作。
//     上下分隔条同理，拖到容器外再松手不继续拖动。内容互不越过分区裁剪。
//     Windows 回归：持续快速左右往返拖动至少 20 秒，再操作上下分隔条；窗口
//     应持续响应，不出现 0xC0000005。松手后点 Snapshot，几何断言仍应通过。
//
//  2. 点击分隔条后按主轴方向键、Shift+方向键，分别移动 1／10 DIP；Home／End
//     到两侧 MinSize 允许的位置。不相关方向键／Control+方向键不调整分区。
//     拖动中按 Esc，恢复按下前的大小；松手不会重新应用已取消的位置。
//     拖动结束并移开鼠标，分隔线不保留高亮或变粗；焦点仍在，可继续用方向键调整。
//
//  3. 拖动后缩小再放大窗口，比例模式保持比例，Start／End 模式保持对应首选
//     DIP 尺寸；不足时临时压缩、恢复后回到偏好。窗口小于内容最小总和时不得
//     产生负尺寸或崩溃。嵌套上下分区仍可单独操作。
//
//  4. 点 Mode 在内容建立的比例／Start 220／End 400／Ratio 0.5 间切换。
//     纯 GUI 的首选值由用户拖动更新；UI 的 OnResize 更新相应模型再重建。
//     点 Rebuild 不重置非受控比例、输入文字、选择或 ScrollView 的位置。
//
//  5. 点 Hide/Show 隐藏左侧：右侧填满、外层线消失；再显示恢复尺寸策略。
//     Toggle Dark 在Modern下切换亮暗；Basic兜底保持亮色，不改变比例或内容。
//
//  6. 在左面板右侧滚动条靠近分隔线的 4 DIP 内拖动滑块：只滚动内容，分隔不移动。
//     对上下分隔线旁的横滚动条同样检查；邻近按钮点击仍激活。
//
//  7. 点 Snapshot：终端打印两侧实际大小及 separator 范围；assertions 检查
//     两侧之和加实际分隔区域厚度 等于容器主轴，真实内容子树与 separator 的边界及范围。
//     显示 ASSERT PASS 仅代表这些几何断言，不能代替上述桌面输入／外观验收。
//
//  8. Handle 按钮循环3／5／9／1.5 DIP；Line循环1／0／3 DIP。每次等待PRESENT
//     后点Snapshot：尺寸、语义和1／1.25／1.5／2x的像素断言必须通过。线宽钳制
//     在区域内，隐藏线后仍是连续的两色背景。拖动中按F7切换厚度，旧手势取消。
//     Rebuild、亮暗、隐藏／恢复后重复；1x、2x分别检查两侧没有第三色窄条或接缝。
//
// 平台差异：本例不扩展窗口装饰，macOS 需先激活窗口；Tab 沿用框架焦点遍历。
// 副作用：仅本例窗口、内存文字和日志；不写业务文件，不操作剪贴板或系统设置。
// 复位：重启。退出：关闭窗口；任一几何断言失败时 panic，返回非零状态。
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/color"
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
func splitColors(dark bool) (start, end, line color.RGBA) {
	if dark {
		return color.RGBA{44, 46, 51, 255}, color.RGBA{30, 31, 34, 255}, color.RGBA{79, 82, 89, 255}
	}
	return color.RGBA{235, 237, 241, 255}, color.RGBA{255, 255, 255, 255}, color.RGBA{196, 199, 206, 255}
}
func sheet(dark, fallback bool, lineWidth float32) style.StyleSheet {
	var rules []style.Rule
	if fallback {
		rules = append(gui.DefaultStyleRules(), widgetstyle.Rules()...)
	} else {
		rules = modern.Rules(modern.Options{Dark: dark})
	}
	start, end, line := splitColors(dark && !fallback)
	rules = append(rules,
		style.Name("sidebar-pane").BackgroundColor(start),
		style.Name("content-pane").BackgroundColor(end),
		style.Name("sidebar-split").Part("handle-start").BackgroundColor(start),
		style.Name("sidebar-split").Part("handle-end").BackgroundColor(end),
		style.Name("editor-split").Part("handle-start").BackgroundColor(end),
		style.Name("editor-split").Part("handle-end").BackgroundColor(start),
	)
	for _, name := range []string{"sidebar-split", "editor-split"} {
		rules = append(rules, style.Name(name).Part("separator").ForegroundColor(line).BorderWidth(lineWidth),
			style.Name(name).Part("separator").State(style.FocusVisible).ForegroundColor(color.RGBA{70, 130, 220, 255}))
	}
	return style.Sheet(rules...)
}
func cycleHandle(size float32) float32 {
	switch size {
	case 3:
		return 5
	case 5:
		return 9
	case 9:
		return 1.5
	default:
		return 3
	}
}
func cycleLine(width float32) float32 {
	if width == 1 {
		return 0
	}
	if width == 0 {
		return 3
	}
	return 1
}

// 在帧结束后的正常输入回调中运行；不会在Paint内嵌套离屏绘制。
func checkPixels(view *widgets.SplitView, dark, fallback bool, lineWidth float32) {
	info := view.Snapshot()
	if len(info.Children) != 3 {
		return
	}
	handle := info.Children[2].Bounds.Translate(geometry.Point{X: -info.Bounds.X, Y: -info.Bounds.Y})
	start, end, line := splitColors(dark && !fallback)
	if view.Direction() == layout.DirectionVertical {
		start, end = end, start
	}
	for _, scale := range []float32{1, 1.25, 1.5, 2} {
		img, err := gui.RenderWidget(view, scale)
		if err != nil {
			panic(err)
		}
		main, cross, extent, crossExtent := handle.X, handle.Y, handle.Width, handle.Height
		if view.Direction() == layout.DirectionVertical {
			main, cross, extent, crossExtent = handle.Y, handle.X, handle.Height, handle.Width
		}
		edge := func(value float32) int { return int(math.Floor(float64(value*scale) + .5)) }
		first, mid, last := edge(main), edge(main+extent/2), edge(main+extent)
		width := min(extent, max(0, lineWidth))
		lineFirst, lineLast := edge(main+(extent-width)/2), edge(main+(extent+width)/2)
		y := edge(cross + crossExtent/2)
		for x := first; x < last; x++ {
			want := start
			if x >= mid {
				want = end
			}
			if width > 0 && x >= lineFirst && x < lineLast {
				want = line
			}
			px, py := x, y
			if view.Direction() == layout.DirectionVertical {
				px, py = py, px
			}
			got := color.RGBAModel.Convert(img.At(px, py)).(color.RGBA)
			for i, value := range []uint8{got.R, got.G, got.B, got.A} {
				expected := [4]uint8{want.R, want.G, want.B, want.A}[i]
				if math.Abs(float64(value)-float64(expected)) > 1 {
					panic(fmt.Sprintf("Part像素 %s scale=%g (%d,%d)=%v want=%v", view.ID(), scale, px, py, got, want))
				}
			}
		}
	}
	fmt.Printf("PART PIXELS PASS %s handle=%g line=%g dark=%t\n", view.ID(), view.HandleSize(), lineWidth, dark)
}
func present(p gui.Painter, outer, inner *widgets.SplitView, controls map[string]gui.Widget, last *string, dark bool, lineWidth float32) {
	bounds := make(map[string]geometry.Rectangle)
	for name, w := range controls {
		if w != nil {
			bounds[name] = w.Snapshot().Bounds
		}
	}
	for name, v := range map[string]*widgets.SplitView{"outer": outer, "inner": inner} {
		info := v.Snapshot()
		if len(info.Children) == 3 {
			bounds[name] = info.Children[2].Bounds
		}
	}
	encoded, _ := json.Marshal(bounds)
	state := fmt.Sprintf("dark=%t handle=%g line=%g bounds=%s", dark, outer.HandleSize(), lineWidth, encoded)
	if state == *last {
		return
	}
	*last = state
	resource, err := p.NewImage(image.NewRGBA(image.Rect(0, 0, 1, 1)))
	if err != nil {
		panic(err)
	}
	fmt.Printf("PRESENT %s backend=%T scale=%g\n", state, resource, p.PixelScale())
	resource.Destroy()
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
		if math.Abs(float64(a+b+min(view.HandleSize(), extent)-extent)) > .01 || a < 0 || b < 0 {
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
		if hitWidth < 0 || math.Abs(float64(hitWidth-min(view.HandleSize(), extent))) > .01 || separator.Range.Direction != view.Direction() {
			panic("separator hit width/direction incorrect")
		}
		fmt.Printf("separator bounds=%v range=%+v\n", separator.Bounds, *separator.Range)
	}
	fmt.Printf("ASSERT PASS %s sizes=(%.1f,%.1f) extent=%.1f\n", view.ID(), a, b, extent)
}
func runUI(fallback bool) error {
	model := ui.NewTextModel(document)
	mode, dark, hidden, rebuilds := 0, false, false, 0
	handleSize, lineWidth := float32(3), float32(1)
	lastReport := ""
	startSize, endSize, ratio, terminal := float32(220), float32(400), float32(.5), float32(140)
	return ui.Run("org.golang-gui.SplitViewProbe", func(app ui.App) ui.RootView {
		left := ui.VBox(ui.Label("Sidebar / left"), ui.TextInput().ID("sidebar-input"), ui.Label("Minimum: 120 DIP"), ui.ScrollView(ui.Label(strings.Repeat("Side row\n", 100))).ID("sidebar-scroll")).Spacing(8).CrossAlign(layout.CrossStretch).Style("sidebar-pane").MinSize(120, 0).Hidden(hidden)
		right := wui.VSplit(
			ui.VBox(ui.ScrollView(ui.TextView().ID("editor").Model(model))).Style("content-pane").CrossAlign(layout.CrossStretch).MinSize(0, 100),
			ui.VBox(ui.ScrollView(ui.Label(strings.Repeat("Terminal output\n", 40)))).Style("sidebar-pane").CrossAlign(layout.CrossStretch).MinSize(0, 60),
		).ID("inner").Style("editor-split").HandleSize(handleSize).MinSize(240, 0).EndSize(terminal).OnResize(func(_, end float32) { terminal = end; app.RequestUpdate() })
		split := wui.HSplit(left, right).ID("outer").Style("sidebar-split").HandleSize(handleSize).OnResize(func(start, end float32) {
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
		return ui.Root().StyleSheet(sheet(dark, fallback, lineWidth)).Windows(
			ui.Window("split").Title("GOUI SplitView UI").Size(900, 600).MinSize(360, 260).
				Shortcuts(ui.Shortcut(ui.KeyF7).OnActivate(func() { handleSize = cycleHandle(handleSize); app.RequestUpdate() })).
				OnAfterPaint(func(p ui.Painter, _ geometry.Rectangle) {
					outer, _ := app.FindWidget("split", "outer").(*widgets.SplitView)
					inner, _ := app.FindWidget("split", "inner").(*widgets.SplitView)
					if outer != nil && inner != nil {
						present(p, outer, inner, map[string]gui.Widget{
							"snapshot": app.FindWidget("split", "snapshot"), "handle": app.FindWidget("split", "handle"), "line": app.FindWidget("split", "line"), "dark": app.FindWidget("split", "dark"), "sidebar-scroll": app.FindWidget("split", "sidebar-scroll"),
						}, &lastReport, dark, lineWidth)
					}
				}).Content(
				ui.VBox(
					ui.HBox(
						ui.Button("Mode").OnClick(func() { mode = (mode + 1) % 4; startSize, endSize, ratio = 220, 400, .5; app.RequestUpdate() }),
						ui.Button("Rebuild").OnClick(func() { rebuilds++; app.RequestUpdate() }),
						ui.Button("Hide/Show").OnClick(func() { hidden = !hidden; app.RequestUpdate() }),
						ui.Button("Toggle Dark").ID("dark").OnClick(func() { dark = !dark; app.RequestUpdate() }),
						ui.Button("Handle").ID("handle").OnClick(func() { handleSize = cycleHandle(handleSize); app.RequestUpdate() }),
						ui.Button("Line").ID("line").OnClick(func() { lineWidth = cycleLine(lineWidth); app.RequestUpdate() }),
						ui.Button("Snapshot").ID("snapshot").OnClick(func() {
							assertSplit(app.FindWidget("split", "outer").(*widgets.SplitView))
							assertSplit(app.FindWidget("split", "inner").(*widgets.SplitView))
							checkPixels(app.FindWidget("split", "outer").(*widgets.SplitView), dark, fallback, lineWidth)
							checkPixels(app.FindWidget("split", "inner").(*widgets.SplitView), dark, fallback, lineWidth)
							fmt.Printf("SIDEBAR SCROLL y=%g\n", app.FindWidget("split", "sidebar-scroll").(*gui.ScrollView).ScrollY())
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
	app.SetStyleSheet(sheet(false, fallback, 1))
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
	left.SetSpacing(8)
	left.SetCrossAlign(layout.CrossStretch)
	left.SetStyleName("sidebar-pane")
	left.SetMinSize(geometry.Size{Width: 120})
	left.AddChild(gui.NewLabel("Sidebar / left"))
	left.AddChild(gui.NewTextInput())
	left.AddChild(gui.NewLabel("Minimum: 120 DIP"))
	sideScroll := gui.NewScrollView()
	sideScroll.SetDirection(gui.ScrollVertical)
	sideScroll.SetChild(gui.NewLabel(strings.Repeat("Side row\n", 100)))
	left.AddChild(sideScroll)
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
	editorPane := gui.NewLinearBox(layout.DirectionVertical)
	editorPane.SetCrossAlign(layout.CrossStretch)
	editorPane.SetStyleName("content-pane")
	editorPane.AddChild(scroll)
	terminalPane := gui.NewLinearBox(layout.DirectionVertical)
	terminalPane.SetCrossAlign(layout.CrossStretch)
	terminalPane.SetStyleName("sidebar-pane")
	terminalPane.AddChild(terminal)
	inner.SetStyleName("editor-split")
	inner.SetStartChild(editorPane)
	inner.SetEndChild(terminalPane)
	inner.SetEndSize(140)
	inner.SetMinSize(geometry.Size{Width: 240})
	outer := widgets.NewSplitView(layout.DirectionHorizontal)
	outer.SetID("outer")
	outer.SetStyleName("sidebar-split")
	outer.SetStartChild(left)
	outer.SetEndChild(inner)
	outer.ConnectResize(func(a, b float32) { fmt.Printf("outer resize %.1f %.1f\n", a, b) })
	inner.ConnectResize(func(a, b float32) { fmt.Printf("inner resize %.1f %.1f\n", a, b) })
	root := gui.NewLinearBox(layout.DirectionVertical)
	root.SetCrossAlign(layout.CrossStretch)
	toolbar := gui.NewLinearBox(layout.DirectionHorizontal)
	toolbar.SetPadding(geometry.UniformInsets(8))
	toolbar.SetSpacing(8)
	mode, dark, rebuilds := 0, false, 0
	lineWidth := float32(1)
	controls := make(map[string]gui.Widget)
	lastReport := ""
	status := gui.NewLabel(modeNames[0])
	button := func(text string, fn func()) {
		b := gui.NewButton()
		b.SetChild(gui.NewLabel(text))
		b.ConnectClicked(fn)
		controls[strings.ToLower(strings.Split(text, " ")[0])] = b
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
	button("Toggle Dark", func() { dark = !dark; app.SetStyleSheet(sheet(dark, fallback, lineWidth)) })
	cycle := func() { next := cycleHandle(outer.HandleSize()); outer.SetHandleSize(next); inner.SetHandleSize(next) }
	button("Handle", cycle)
	button("Line", func() { lineWidth = cycleLine(lineWidth); app.SetStyleSheet(sheet(dark, fallback, lineWidth)) })
	shortcut := gui.NewShortcut(gui.KeyGesture{Key: gui.KeyF7})
	shortcut.ConnectActivate(cycle)
	window.Shortcuts().AddShortcut(shortcut)
	button("Snapshot", func() {
		assertSplit(outer)
		assertSplit(inner)
		checkPixels(outer, dark, fallback, lineWidth)
		checkPixels(inner, dark, fallback, lineWidth)
		fmt.Printf("SIDEBAR SCROLL y=%g\n", sideScroll.ScrollY())
	})
	root.AddChild(toolbar)
	root.AddChild(status)
	root.AddChild(outer)
	window.SetWidget(root)
	controls["sidebar-scroll"] = sideScroll
	window.ConnectAfterPaint(func(p gui.Painter, _ geometry.Rectangle) {
		present(p, outer, inner, controls, &lastReport, dark, lineWidth)
	})
	if err = window.Show(); err != nil {
		return err
	}
	app.Run()
	return nil
}
