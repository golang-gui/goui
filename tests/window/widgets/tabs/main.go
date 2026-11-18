// TabBar/TabView 基础交互与外观窗口验证，不使用 DevServer。
//
// 环境：Linux/X11、Windows 或 macOS 可交互桌面；记录 OS、窗口管理器、实际绘制
// 后端和缩放比例。以下命令在仓库根目录运行：
//
//	go run ./tests/window/widgets/tabs
//
// 初始：请求 Integrated 装饰、800×480 DIP 窗口，最小 400×320 DIP；采用 Modern
// 亮色样式。Document one/two/three 三页中 one 选中，页内输入框为空，HeaderBar 内有
// 可排序标签栏。未开启跨窗口转移；拖出新窗口另见 ../tab_detach。
//
// 主要操作与预期：
//  1. 分别在 one 和 two 输入不同文字，切换标签。
//     预期文字、光标和选区保留在各自页面；点击 Rebuild 后内容不重置。
//  2. 在栏内按住标签移动超过 4 DIP。预期整张标签保持原抓取偏移跟随指针，
//     邻居平滑让位，反向移动不跳动；松开后 order 和 moved 日志只提交一次顺序变化。
//     短点击只选择；拖出栏外释放或按 Esc 取消，顺序不变，不创建窗口。
//  3. 反复点击 Add，再缩窄窗口。预期标签等宽从 240 向 112 DIP 收缩，容纳不下
//     最小宽度时才滚动；箭头和滚轮能查看隐藏标签，新选中标签可见。
//     重新放宽：空间充足时箭头消失，标签等宽增长且不超过 240 DIP。
//  4. 将指针移入内容区。未选中标签的文字/图标不能像禁用状态；选中标签显示关闭按钮，
//     未选中标签仅在悬停或关闭按钮自身具有键盘焦点时显示关闭按钮。
//     悬停、移出不改变标题和邻居位置；点击关闭只移除该页，不先选中该页。
//     先聚焦当前页输入框，再悬停未选中标签的关闭按钮：按住后移到内容区再松手，
//     预期关闭按钮在移出后隐藏，无 close/current/moved 日志，原输入焦点保留。
//     按住后移出整个窗口再松手也应如此；再次悬停并点击仍只关闭该页一次。
//  5. 在 Integrated 标签上拖动，预期排序而非移动窗口；拖动最后一个标签后的空白、
//     标签间 4 DIP 间隙或 HeaderBar padding，预期移动窗口。关闭/滚动按钮仍可点击。
//  6. 右键未选中标签或其关闭按钮区域，预期菜单标题指向该标签，选中页面不变，
//     不触发排序或关闭。点“关闭”后才发出与关闭按钮相同的 close 日志并移除目标页。
//     点“切换到此页”只切换目标页；Esc 或点菜单外部仅关闭菜单。
//     在标签主体取得焦点后按 Shift+F10，菜单属于焦点标签；macOS 另验 Control+单击。
//     栏内空白和间隙不显示标签菜单，仍可作为 Caption 拖动窗口。
//
// 可选外观与回归（按需选择，不必排列组合重复所有操作）：
//  1. 点击标签主体后按方向键、Home、End，观察选中标签与内容对应；点击 Toggle Dark，
//     检查文字、图标、边界和命中区域一致。选中标签使用中性色表面和细边框，
//     顶边不能被 Integrated 缩放边缘裁掉；悬停/按下关闭或滚动按钮不使标题移位。
//  2. 点击 Text size 切换 14/24 pt。预期标签文字在高度上完整、HeaderBar 随行高增长、
//     关闭按钮垂直居中；宽度不足时仅视觉裁剪，不自动省略或缩小字体。
//  3. 点击 Width mode 切换 112..240 / 240..240 DIP。固定宽度模式不收缩，仍可滚动。
//     相邻未选中且未悬停的标签之间显示 1×16 DIP 居中分隔线；悬停任一相邻标签时
//     对应线隐藏，拖动及回位过渡中全部隐藏，结束后按新顺序恢复。
//  4. 拖动未选中标签并释放，再移入内容区：关闭按钮应隐藏，标签仍未选中。
//     按 Esc 或栏外释放也应如此；主体焦点不会保留关闭按钮，关闭按钮自身焦点会。
//     不应遗留不可见却可点击的关闭区域。
//     悬停未选中标签后，从其主体按 Tab 将焦点移到关闭按钮，再移开鼠标：按钮应
//     保持显示，Space/Enter 仍可关闭；鼠标按压不会主动把焦点转给关闭按钮。
//  5. Indicator 按钮循环 Off/Dot/Diamond：前导图标和标题不变，选中与未选中标签的
//     尾部均显示自定义图标。悬停整个标签或 Tab 聚焦关闭按钮后切换叉号，移出后恢复
//     图标；标题和标签位置不跳动，按住移出再松手不关闭，正常点击仍只关闭一次。
//     切换 Closable 为 false：尾部图标始终显示，点击选择、拖动排序，不产生 close。
//     改回 true 或清除 Indicator 后恢复对应行为。Rebuild 不重置输入和选择。
//     Modern 亮暗/Basic 下单色状态图标跟随自己的 tab-indicator-icon 前景色；Text size
//     24pt 与逻辑 2x 下仍居中。这里只显示静态图标，不模拟 Modified/Loading 业务状态。
//  6. 以 -fallback-style 重启，显式组合 GUI 与 widgets 兜底样式。
//     Toggle Dark/Text size 在此模式不改变样式；检查文字可读、选择/悬停表面、
//     分隔线和关闭/滚动图标，以及前述选择/排序行为。
//  7. 从当前页输入框反复按 Tab/Shift+Tab，把焦点移到标签主体、关闭按钮和溢出箭头。
//     主体有键盘焦点时只有该标签轮廓显示 2 DIP 提示，选中底色不改变；关闭按钮或
//     滚动箭头聚焦时只有该按钮显示焦点框，不能同时把整个标签标成聚焦。
//     单纯悬停保留提示；鼠标点击清除键盘提示，即使实际焦点仍在同一个主体上。
//     在已选中/未选中标签、Modern 亮暗/Basic、1x/2x 下重复，文字/按钮位置不跳动。
//     F6 记录实际焦点、所属标签的选择态和窗口局部边界，不改变焦点或选择；
//     标签主体的 FocusVisible 不应伪装成父标签的 Focused，关闭聚焦标签后不留旧框。
//
// 可选逻辑 2x：POSIX 使用 GOUI_PLAT_SCALE=2 go run ./tests/window/widgets/tabs；
// PowerShell 先设 $env:GOUI_PLAT_SCALE='2' 再启动，结束后恢复该变量原值。
// 平台差异：macOS 先激活窗口再操作；Linux Integrated 需要合成器，记录是否回退
// 为 Native。Windows 可选 Direct2D/OpenGL/Software；装饰差异不等于绘制后端差异。
// 结果：本例为人工/Computer Use 观察，界面显示 Selected/rebuilds/order，控制台
// 输出 current/close/moved。逐项记录通过/失败，不以窗口正常关闭作为自动 PASS。
// 副作用：仅操作本例窗口和内存中的文字，不读写业务文件、不修改剪贴板或系统设置。
// 复位：重启程序。退出：关闭窗口。逻辑 2x 不代表跨显示器混合 DPI 验证。
package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"log"
	"slices"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/icon"
	icondraw "github.com/golang-gui/goui/icon/draw"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/style"
	"github.com/golang-gui/goui/theme/modern"
	"github.com/golang-gui/goui/ui"
	"github.com/golang-gui/goui/widgets"
	widgetstyle "github.com/golang-gui/goui/widgets/style"
	wui "github.com/golang-gui/goui/widgets/ui"
)

func main() {
	fallbackStyle := flag.Bool("fallback-style", false, "use explicit GUI + widgets fallback rules instead of Modern")
	flag.Parse()
	bitmap := image.NewRGBA(image.Rect(0, 0, 16, 16))
	for y := 2; y < 14; y++ {
		for x := 2; x < 14; x++ {
			bitmap.SetRGBA(x, y, color.RGBA{A: 255})
		}
	}
	iconSource, err := icon.NewImage(bitmap)
	if err != nil {
		log.Fatal(err)
	}
	keys := []string{"one", "two", "three"}
	current := "one"
	dark, serial, rebuilds := false, 3, 0
	fontSize := float32(14)
	fixedWidth := false
	indicatorMode, closable := 0, true
	indicators := []gui.IconSource{nil,
		&icondraw.Source{Draw: func(p graphics.Painter, size graphics.Size, foreground graphics.Color) {
			p.FillEllipse(geometry.Point{X: size.Width / 2, Y: size.Height / 2}, size.Width/5, size.Height/5, foreground)
		}},
		&icondraw.Source{Draw: func(p graphics.Painter, size graphics.Size, foreground graphics.Color) {
			path := graphics.MoveTo(size.Width/2, size.Height/5).
				LineTo(size.Width*4/5, size.Height/2).
				LineTo(size.Width/2, size.Height*4/5).
				LineTo(size.Width/5, size.Height/2).Close()
			p.FillPath(path, foreground)
		}},
	}
	err = ui.Run("org.golang-gui.TabsProbe", func(app ui.App) ui.RootView {
		minWidth := float32(112)
		if fixedWidth {
			minWidth = 240
		}
		pages := make([]*wui.TabPageView, 0, len(keys))
		for _, key := range keys {
			pages = append(pages, wui.TabPage(key,
				ui.VBox(ui.Label("Page: "+key), ui.TextInput().ID("input-"+key)).Spacing(8).Padding(16),
			).Title("Document "+key).Icon(iconSource).Closable(closable).IndicatorIcon(indicators[indicatorMode]))
		}
		sheet := modern.Sheet(modern.Options{Dark: dark, FontSize: fontSize})
		if *fallbackStyle {
			sheet = style.Sheet(append(gui.DefaultStyleRules(), widgetstyle.Rules()...)...)
		}
		return ui.Root().StyleSheet(sheet).Windows(
			ui.Window("tabs").Title("GOUI tabs acceptance").Chrome(ui.WindowChromeIntegrated).Size(800, 480).MinSize(400, 320).Content(
				ui.VBox(
					ui.HeaderBar(wui.TabBar("documents").TabWidthRange(minWidth, 240).Reorderable(true).
						ContextMenu(func(key string) []*ui.MenuItemView {
							fmt.Printf("menu=%s current=%s\n", key, current)
							return []*ui.MenuItemView{
								ui.MenuItem("Document "+key, nil).Enabled(false),
								ui.MenuSeparator(),
								ui.MenuItem("切换到此页", func() { current = key; app.RequestUpdate() }).Enabled(current != key),
								ui.MenuItem("关闭", func() {
									// Route through the same GUI request as the close button.
									if page, ok := app.FindWidget("tabs", key).(*widgets.TabPage); ok {
										if view, ok := app.FindWidget("tabs", "documents").(*widgets.TabView); ok {
											view.RequestClose(page)
										}
									}
								}).Enabled(closable),
							}
						}).
						OnContextMenuError(func(err error) { fmt.Printf("menu-error=%v\n", err) })).Padding(6),
					ui.HBox(
						ui.Button("Add").OnClick(func() {
							serial++
							key := fmt.Sprintf("page-%d", serial)
							keys = append(keys, key)
							current = key
							app.RequestUpdate()
						}),
						ui.Button("Rebuild").OnClick(func() { rebuilds++; app.RequestUpdate() }),
						ui.Button("Toggle Dark").OnClick(func() { dark = !dark; app.RequestUpdate() }),
						ui.Button("Text size").OnClick(func() {
							if fontSize == 14 {
								fontSize = 24
							} else {
								fontSize = 14
							}
							app.RequestUpdate()
						}),
					).Spacing(8).Padding(8),
					ui.HBox(
						ui.Button("Width mode").OnClick(func() { fixedWidth = !fixedWidth; app.RequestUpdate() }),
						ui.Label(fmt.Sprintf("Tab widths: %.0f..240 DIP", minWidth)),
						ui.Button("Indicator: "+[...]string{"Off", "Dot", "Diamond"}[indicatorMode]).OnClick(func() {
							indicatorMode = (indicatorMode + 1) % len(indicators)
							app.RequestUpdate()
						}),
						ui.Button(fmt.Sprintf("Closable: %t", closable)).OnClick(func() { closable = !closable; app.RequestUpdate() }),
					).Spacing(8).Padding(8),
					ui.Label(fmt.Sprintf("Selected=%s | rebuilds=%d | order=%v", current, rebuilds, keys)),
					wui.TabView(pages...).ID("documents").Current(current).
						OnCurrent(func(key string) { current = key; fmt.Printf("current=%s\n", key); app.RequestUpdate() }).
						OnCloseRequest(func(key string) {
							if i := slices.Index(keys, key); i >= 0 {
								if current == key {
									if i+1 < len(keys) {
										current = keys[i+1]
									} else if i > 0 {
										current = keys[i-1]
									} else {
										current = ""
									}
								}
								keys = slices.Delete(keys, i, i+1)
							}
							fmt.Printf("close=%s current=%s\n", key, current)
							app.RequestUpdate()
						}).
						OnMoved(func(key string, _, to int) {
							from := slices.Index(keys, key)
							if from >= 0 && to >= 0 && to < len(keys) {
								keys = slices.Delete(keys, from, from+1)
								keys = slices.Insert(keys, to, key)
								fmt.Printf("moved=%s %d->%d order=%v\n", key, from, to, keys)
								app.RequestUpdate()
							}
						}),
				).CrossAlign(layout.CrossStretch),
			).Shortcuts(ui.Shortcut(ui.KeyF6).OnActivate(func() { reportTabFocus(app) })),
		)
	})
	if err != nil {
		log.Fatal(err)
	}
}

// 手工检查的只读观测；仍通过真实 Tab/指针事件操作，不提供焦点动作捷径。
func reportTabFocus(app ui.App) {
	window := app.FindWindow("tabs")
	if window == nil {
		return
	}
	focused := window.FocusedWidget()
	if focused == nil {
		fmt.Println("focus=nil")
		return
	}
	var tab string
	var selected bool
	for owner := focused; owner != nil; owner = owner.Parent() {
		if info := owner.Snapshot(); info.Role == widgets.RoleTab {
			tab, selected = info.Text, info.Selected
			break
		}
	}
	fmt.Printf("focus=%T style=%q focus-visible=%t bounds=%v tab=%q selected=%t\n",
		focused, focused.StyleName(), focused.FocusVisible(), focused.Snapshot().Bounds, tab, selected)
}
