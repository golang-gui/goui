package main

import (
	"fmt"
	"image"
	"runtime"
	"strings"
	"time"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/style"
	"github.com/golang-gui/goui/theme/basic"
	"github.com/golang-gui/goui/theme/modern"
	"github.com/golang-gui/goui/ui"
	"github.com/golang-gui/goui/widgets"
	wui "github.com/golang-gui/goui/widgets/ui"
)

// 统一行高窗口验收（与默认连接／像素场景独立）。
// 环境：桌面、系统字体；Software/OpenGL，Windows 可选 Direct2D；记录实际后端与缩放。
// 启动：go run ./tests/window/widgets/list_row_group -uniform [-auto] [-basic] [-dark]。
// 初始：800×680 DIP，两行、统一行高开启、宽度 560 DIP、普通字号与说明文字。
// 行组位于滚动区域内，增大字号后可纵向滚动查看全文；工具按钮始终位于视口外。
// 操作与逐项预期：
//  1. 两行等高，取说明行的自然高度；点击第一行原自然高度以下的新增区域，只增加
//     第一行计数；共享边界以下属于第二行。语义边界与真实分配一致。
//  2. Width 改为 280 DIP：说明换行，两行一起增高；Content 增加说明，两行再次增高。
//  3. Font 增大字号后两行继续增高；Hide 隐藏最高行后只剩短行自然高度，恢复后重新等高。
//  4. Uniform 关闭后两行恢复独立自然高度；Reset 恢复初始配置，行实例和计数不被重建。
//  5. Basic/Modern 与亮暗切换后重复检查：文字不裁切，接缝连续，边框内均可命中本行。
//
// Check/F6 在布局完成后核对自然高度、源尺寸属性、语义边界、身份及点击动作，失败 panic。
// -auto 通过 Window.DispatchEvent 点击按钮与行，执行 1～4；每阶段断言后输出 AUTO PASS
// 并退出。自动模式不替代外观验收；手工模式不派发模拟输入。
// 平台差异：原生字体行高可能不同，按当前实际自然测量断言，不能固定像素金图。
// 副作用：只有窗口、焦点、指针和终端输出；不改系统设置、文件或剪贴板。
// 复位：Reset 或重启。退出：关闭窗口；自动模式提前关闭视为未完成。
func runUniformRows(auto, basicTheme, dark bool) {
	uniform, narrow, extended, large, hidden := true, false, false, false, false
	started, complete, step := false, false, 0
	clicks := [2]int{}
	var app ui.App
	var original [2]*widgets.ListRow
	var wideHeight, narrowHeight, extendedHeight, largeHeight float32
	var painter string
	var scale float32
	find := func(id string) gui.Widget { return app.FindWidget("uniform", id) }
	check := func() {
		group := find("uniform-group").(*widgets.ListRowGroup)
		rows := [2]*widgets.ListRow{find("short").(*widgets.ListRow), find("long").(*widgets.ListRow)}
		require(group.UniformRows() == uniform, "声明式开关未更新")
		if original[0] == nil {
			original = rows
		}
		require(rows == original, "属性更新替换了行实例")
		natural := [2]float32{}
		for i, row := range rows {
			natural[i] = row.Measure(layout.Loose(geometry.Size{Width: group.Rect().Width, Height: layout.Inf})).Height
			require(row.MinSize() == (geometry.Size{Height: 48}) && row.MaxSize() == (geometry.Size{}), "统一布局改写了行尺寸属性")
		}
		require(rows[0].MainWeight() == 3 && rows[1].MainWeight() == 0, "统一布局改写了行权重")
		info := group.Snapshot()
		require(info.Role == gui.RoleVBox && len(info.Actions) == 0 && !info.Focusable && len(info.Children) == 2, "分组语义改变")
		y := float32(0)
		for i, row := range rows {
			require(info.Children[i].Visible == row.Visible(), "行可见语义不正确")
			if !row.Visible() {
				continue
			}
			height := natural[i]
			if uniform {
				height = max(natural[0], natural[1])
			}
			require(row.Rect() == geometry.Rect(0, y, group.Rect().Width, height), fmt.Sprintf("行 %d 分配错误：%v，期望高度 %g", i, row.Rect(), height))
			bounds := info.Children[i].Bounds
			require(bounds == geometry.Rect(info.Bounds.X, info.Bounds.Y+y, group.Rect().Width, height), "语义边界未使用统一行高")
			require(info.Children[i].Role == gui.RoleListItem && len(info.Children[i].Actions) == 1, "行角色或动作改变")
			y += height
		}
		require(group.Rect().Height == y, "分组自然高度有额外均分空间")
		fmt.Printf("ASSERT PASS uniform=%v width=%g hidden=%v large=%v extended=%v heights=[%g %g] clicks=%v\n", uniform, group.Rect().Width, hidden, large, extended, rows[0].Rect().Height, rows[1].Rect().Height, clicks)
	}
	must(ui.Run("org.golang-gui.ListRowUniformTest", func(a ui.App) ui.RootView {
		app = a
		if !started {
			started = true
			if auto {
				app.TickFunc(200*time.Millisecond, func() {
					win := app.FindWindow("uniform")
					if win == nil || find("uniform-group") == nil || find("uniform-group").Rect().Width == 0 || painter == "" {
						return
					}
					check()
					click := func(id string, fraction float32) {
						b := find(id).Snapshot().Bounds
						at := geometry.Point{X: b.X + 20, Y: b.Y + b.Height*fraction}
						for _, kind := range []events.EventType{events.PointerDown, events.PointerUp} {
							must(win.DispatchEvent(events.PointerEvent{EventType: kind, Position: at, Button: events.PointerButtonLeft}))
						}
					}
					height := find("short").Rect().Height
					switch step {
					case 0:
						wideHeight = height
						click("short", .9)
						click("long", .01)
						require(clicks == [2]int{1, 1}, "加高区域或共享边界点击归属错误")
						click("width", .5)
					case 1:
						require(height > wideHeight, "收窄没有增加折行高度")
						narrowHeight = height
						click("content", .5)
					case 2:
						require(height > narrowHeight, "内容更新没有重新计算高度")
						extendedHeight = height
						click("font", .5)
					case 3:
						require(height > extendedHeight, "字号更新没有重新计算高度")
						largeHeight = height
						click("hide", .5)
					case 4:
						require(height < largeHeight, "隐藏最高行没有恢复短行高度")
						click("hide", .5)
					case 5:
						require(height == largeHeight, "重新显示最高行没有恢复统一高度")
						click("uniform-toggle", .5)
					case 6:
						require(!uniform && height < find("long").Rect().Height, "关闭后未恢复自然高度")
						click("reset", .5)
					case 7:
						require(height == wideHeight && clicks == [2]int{1, 1}, "复位未恢复布局或丢失计数")
						fmt.Printf("AUTO PASS uniform rows; OS=%s painter=%s scale=%g\n", runtime.GOOS, painter, scale)
						complete = true
						app.Quit()
					}
					step++
				})
			}
		}
		update := func() { app.RequestUpdate() }
		font := float32(14)
		if large {
			font = 22
		}
		rules := basic.Rules()
		if !basicTheme {
			rules = modern.Rules(modern.Options{Dark: dark})
		}
		rules = append(rules, style.Name("list-row-title").FontSize(font), style.Name("list-row-subtitle").FontSize(font))
		width := float32(560)
		if narrow {
			width = 280
		}
		text := "Download updates automatically and keep the current application settings."
		if extended {
			text = strings.Repeat(text+" ", 2)
		}
		probe := &uniformProbeView{painter: &painter, scale: &scale}
		probe.Self = probe
		return ui.Root().StyleSheet(style.Sheet(rules...)).Windows(ui.Window("uniform").Title("GOUI Uniform List Rows").Chrome(ui.WindowChromeIntegrated).Size(800, 680).Center(true).Content(
			ui.VBox(
				ui.HeaderBar(ui.Label("Uniform natural row heights")),
				ui.HBox(
					ui.Button("Uniform").ID("uniform-toggle").OnClick(func() { uniform = !uniform; update() }),
					ui.Button("Width").ID("width").OnClick(func() { narrow = !narrow; update() }),
					ui.Button("Content").ID("content").OnClick(func() { extended = !extended; update() }),
					ui.Button("Font").ID("font").OnClick(func() { large = !large; update() }),
					ui.Button("Hide").ID("hide").OnClick(func() { hidden = !hidden; update() }),
				).Spacing(8),
				ui.HBox(
					ui.Button("Basic / Modern").ID("theme").OnClick(func() { basicTheme = !basicTheme; update() }),
					ui.Button("Light / Dark").OnClick(func() { dark = !dark; update() }),
					ui.Button("Reset").ID("reset").OnClick(func() { uniform, narrow, extended, large, hidden = true, false, false, false, false; update() }),
					ui.Button("Check (F6)").OnClick(check),
				).Spacing(8),
				ui.Label(fmt.Sprintf("Uniform %v | width %g | font %g | clicks %v", uniform, width, font, clicks)),
				ui.ScrollView(ui.VBox(
					wui.ListRowGroup(
						wui.ListRow("Updates").ID("short").MainWeight(3).Activatable(true).OnActivate(func() { clicks[0]++; update() }),
						wui.ListRow("Description").ID("long").Subtitle(text).Visible(!hidden).Activatable(true).OnActivate(func() { clicks[1]++; update() }),
					).ID("uniform-group").UniformRows(uniform).MinWidth(width).MaxWidth(width),
				).CrossAlign(layout.CrossStart)).MinWidth(600).MaxWidth(600).MainWeight(1),
				probe,
			).Padding(18).Spacing(12).CrossAlign(layout.CrossStart).Shortcuts(ui.Shortcut(ui.KeyF6).OnActivate(check)),
		))
	}))
	if auto {
		require(complete, "统一行高自动验收提前退出")
	}
}

// 直接 Widget.Paint 中的资源类型记录真实宿主后端，不使用 DrawingArea 的离屏 Software Painter。
type uniformProbeView struct {
	ui.ViewBase[uniformProbeView]
	painter *string
	scale   *float32
}

func (v *uniformProbeView) Mount(ui.BuildContext) gui.Widget {
	return &uniformProbeWidget{painter: v.painter, scale: v.scale}
}
func (*uniformProbeView) Update(ui.BuildContext, gui.Widget)  {}
func (*uniformProbeView) Unmount(ui.BuildContext, gui.Widget) {}

type uniformProbeWidget struct {
	gui.WidgetBase
	painter *string
	scale   *float32
}

func (*uniformProbeWidget) Measure(c layout.Constraint) layout.Measurement {
	return layout.Measured(c.Clamp(geometry.Size{Width: 1, Height: 1}))
}
func (w *uniformProbeWidget) Paint(p gui.Painter) {
	if *w.painter != "" {
		return
	}
	resource, err := p.NewImage(image.NewRGBA(image.Rect(0, 0, 1, 1)))
	must(err)
	*w.painter, *w.scale = fmt.Sprintf("%T", resource), p.PixelScale()
	resource.Destroy()
	fmt.Printf("READY uniform rows; OS=%s painter=%s scale=%g\n", runtime.GOOS, *w.painter, *w.scale)
}
