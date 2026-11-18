// ListRowGroup 连接分组窗口验收，需要真实桌面、系统字体和绘图后端，不使用 DevServer。
// 启动：go run ./tests/window/widgets/list_row_group [-auto]；可加 -basic、-dark。
// -uniform 改为统一行高场景，步骤和预期见 uniform.go；可组合 -auto。
// 后端：GOUI_PLAT_PAINTER=software/opengl（Windows 可 direct2d），GOUI_PLAT_SCALE=1/2。
// 独立 X11 会话没有输入法服务时，用 XMODIFIERS=@im=none 选择本地 XIM。
// 初始：亮色 Modern，两组行。设置组四行（开关行、动作行、禁用行、版本行）上下相连，
// 无行间距；探针组四行使用 seam-row 样式（不透明白底、2 DIP 半透明红边、8 DIP 圆角），
// 只用于像素断言。
// 操作及预期：
//  1. 首帧：分组角色为 vbox、无动作、不获得焦点；每行仍是 listitem，保留自己的
//     前缀/尾部和点击动作；相邻行上下紧贴、左右同宽，行之间没有间距。
//  2. 点击第二行正文只激活该行一次，第一、三行计数不变；悬停第二行只改变它自己的
//     背景，不扩散到相邻行。禁用行点击不激活、不产生按压。
//  3. 点击 Hide middle：中间行隐藏后不占位，上下两行立即贴合并共用一条边框，隐藏行
//     不再参与连接。点击 Reverse 后行顺序反转，行实例与其内容保持不变。
//  4. Tab 进入分组后可见完整焦点轮廓：共享边也被焦点框补齐，不出现开口；外角圆角
//     仍与未聚焦时一致，鼠标点击不留下键盘焦点装饰。
//  5. 探针组 1x/2x：上外边框、两条共享接缝、下外边框均为单次绘制的边框色，接缝不能
//     比外边框更深；外侧四角为透明（露出后方窗口），内侧接缝两端是直角且边框连续，
//     行内部是行背景色。
//  6. 切换 Basic/Modern、亮/暗后重复 1～5：每行仍用自己的 StyleName 解析样式，
//     分组不向子行提供样式，也不因主题重建替换行实例。
//
// 自动模式（-auto）依次执行上述步骤，逐步断言并在结束时输出 AUTO PASS 后退出；
// Check 按钮（F6）可随时重跑静态断言并输出 BOUNDS 与 ASSERT PASS。断言失败 panic。
// 平台差异：字体与抗锯齿边缘不同，像素断言只取远离 AA 的整数采样点并允许 2 字节误差。
// 副作用：仅测试窗口、焦点、指针与终端输出，不写文件、不修改系统设置或剪贴板。
// 复位：重启。退出：关闭窗口；自动模式提前关闭视为验收未完成。
package main

import (
	"flag"
	"fmt"
	"image/color"
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

func require(ok bool, message string) {
	if !ok {
		panic(message)
	}
}
func must(err error) {
	if err != nil {
		panic(err)
	}
}

func main() {
	auto := flag.Bool("auto", false, "通过正常事件分发自动验收")
	basicTheme := flag.Bool("basic", false, "使用 Basic")
	dark := flag.Bool("dark", false, "Modern 暗色")
	uniform := flag.Bool("uniform", false, "验证统一自然行高")
	flag.Parse()
	if *uniform {
		runUniformRows(*auto, *basicTheme, *dark)
		return
	}
	hidden, reverse, started, complete := false, false, false, false
	activations, switchChanges, step, tabSteps := [3]int{}, 0, 0, 0
	switched := false
	var app ui.App
	find := func(id string) gui.Widget { return app.FindWidget("rows", id) }
	// 探针颜色：不透明白底上的半透明红边一次覆盖后为预乘 (255,127,127,255)；
	// 若共享边被两行各画一次，会加深到约 (255,63,63,255)。
	borderInk := color.RGBA{R: 255, G: 127, B: 127, A: 255}
	surfaceInk := color.RGBA{R: 255, G: 255, B: 255, A: 255}
	near := func(got, want color.RGBA) bool {
		for i, v := range []uint8{got.R, got.G, got.B, got.A} {
			w := []uint8{want.R, want.G, want.B, want.A}[i]
			if delta := int(v) - int(w); delta < -2 || delta > 2 {
				return false
			}
		}
		return true
	}
	rows := func(group *widgets.ListRowGroup) []*widgets.ListRow {
		var out []*widgets.ListRow
		for _, child := range group.Children() {
			row, ok := child.(*widgets.ListRow)
			require(ok, "分组直接子控件不是 ListRow")
			out = append(out, row)
		}
		return out
	}
	visibleRows := func(group *widgets.ListRowGroup) []*widgets.ListRow {
		var out []*widgets.ListRow
		for _, row := range rows(group) {
			if row.Visible() {
				out = append(out, row)
			}
		}
		return out
	}
	// 探针像素断言：采样点都在远离抗锯齿边缘的整数 DIP 位置。
	sampleProbe := func() {
		probe := find("probe").(*widgets.ListRowGroup)
		for _, scale := range []float32{1, 2} {
			img, err := gui.RenderWidget(probe, scale)
			must(err)
			pixel := func(x, y int, want color.RGBA, what string) {
				got := color.RGBAModel.Convert(img.At(int(float32(x)*scale), int(float32(y)*scale))).(color.RGBA)
				require(near(got, want), fmt.Sprintf("%gx %s (%d,%d)=%v，预期 %v", scale, what, x, y, got, want))
			}
			probeRows := visibleRows(probe)
			require(len(probeRows) == 4, "探针行数量错误")
			width, height := int(probe.Rect().Width), int(probe.Rect().Height)
			middle := width / 2
			pixel(middle, 0, borderInk, "首行上外边框")
			for i := 1; i < len(probeRows); i++ {
				seam := int(probeRows[i].Rect().Y)
				pixel(middle, seam, borderInk, fmt.Sprintf("第 %d 条共享接缝", i))
			}
			pixel(middle, height-1, borderInk, "末行下外边框")
			pixel(middle, 24, surfaceInk, "行内部为表面色")
			pixel(0, 0, color.RGBA{}, "首行左上外角透明")
			pixel(0, height-1, color.RGBA{}, "末行左下外角透明")
			pixel(width-1, 0, color.RGBA{}, "首行右上外角透明")
			for i := 1; i < len(probeRows); i++ {
				seam := int(probeRows[i].Rect().Y)
				pixel(0, seam, borderInk, "内侧接缝左端为直角边框")
				pixel(width-1, seam, borderInk, "内侧接缝右端为直角边框")
			}
		}
	}
	// 焦点轮廓：共享边在聚焦后必须被焦点框补齐。比较同一采样点在聚焦与
	// 非聚焦下的颜色差异，避免把主题焦点色写进断言。
	sampleFocus := func(row *widgets.ListRow) {
		require(row.FocusVisible(), "Tab 焦点没有进入行")
		group := row.Parent().(*widgets.ListRowGroup)
		visible := visibleRows(group)
		// 采样与相邻行共享的那条边；单行或首尾不共享时退回自己的外边框。
		seam := int(row.Rect().Y + row.Rect().Height - 1)
		for i, candidate := range visible {
			if candidate != row {
				continue
			}
			if i == 0 && len(visible) > 1 {
				seam = int(row.Rect().Y)
			}
			break
		}
		sample := func() color.RGBA {
			img, err := gui.RenderWidget(group, 1)
			must(err)
			return color.RGBAModel.Convert(img.At(int(row.Rect().Width)/2, seam)).(color.RGBA)
		}
		focused := sample()
		win := app.FindWindow("rows")
		win.SetFocusedWidget(nil)
		plain := sample()
		win.SetFocusedWidget(row)
		require(!near(focused, plain), fmt.Sprintf("焦点轮廓没有补齐共享边：%v vs %v", focused, plain))
	}
	check := func() {
		group := find("settings-group").(*widgets.ListRowGroup)
		info := group.Snapshot()
		require(info.Role == gui.RoleVBox && len(info.Actions) == 0 && !info.Focusable, "分组语义错误")
		require(len(info.Children) == 4, "分组没有保留四行语义")
		for _, child := range info.Children {
			require(child.Role == gui.RoleListItem, "分组覆盖了行的语义角色")
		}
		probe := find("probe").(*widgets.ListRowGroup)
		require(probe.Snapshot().Role == gui.RoleVBox, "探针分组语义错误")
		for _, target := range []*widgets.ListRowGroup{group, probe} {
			visible := visibleRows(target)
			for i := 1; i < len(visible); i++ {
				above, below := visible[i-1], visible[i]
				require(above.Rect().Y+above.Rect().Height <= below.Rect().Y+0.01 && below.Rect().Y-above.Rect().Y-above.Rect().Height < 0.01, "行之间存在间距或重叠")
				require(above.Rect().Width == below.Rect().Width && above.Rect().X == below.Rect().X, "行宽度或左边界不一致")
			}
		}
		for _, id := range []string{"settings-updates", "settings-action", "settings-disabled", "settings-version"} {
			r := find(id).(*widgets.ListRow)
			fmt.Printf("BOUNDS %s %.1f %.1f %.1f %.1f\n", id, r.Rect().X, r.Rect().Y, r.Rect().Width, r.Rect().Height)
		}
		fmt.Printf("ASSERT PASS activations=%v switchChanges=%d switched=%v hidden=%v reverse=%v\n", activations, switchChanges, switched, hidden, reverse)
	}
	must(ui.Run("org.golang-gui.ListRowGroupTest", func(a ui.App) ui.RootView {
		app = a
		if !started {
			started = true
			if *auto {
				app.TickFunc(180*time.Millisecond, func() {
					win := app.FindWindow("rows")
					if win == nil || find("settings-group") == nil || find("settings-group").Rect().Width <= 0 {
						return
					}
					group := find("settings-group").(*widgets.ListRowGroup)
					point := func(id string) geometry.Point {
						r := find(id).Snapshot().Bounds
						return geometry.Point{X: r.X + r.Width/2, Y: r.Y + r.Height/2}
					}
					pointer := func(kind events.EventType, at geometry.Point) {
						must(win.DispatchEvent(events.PointerEvent{EventType: kind, Position: at, Button: events.PointerButtonLeft}))
					}
					click := func(id string) {
						at := point(id)
						pointer(events.PointerDown, at)
						pointer(events.PointerUp, at)
					}
					switch step {
					case 0:
						check()
						sampleProbe()
						click("settings-updates")
						require(activations == [3]int{}, "非激活行被执行")
					case 1:
						require(switchChanges == 0 && !switched, "点击行正文切换了尾部开关")
						click("settings-action")
					case 2:
						require(activations == [3]int{0, 1, 0}, fmt.Sprintf("动作行点击归属错误：%v", activations))
						// 悬停只影响命中的行，不通过分组扩散。
						pointer(events.PointerMove, point("settings-action"))
						first, second := find("settings-updates").(*widgets.ListRow), find("settings-action").(*widgets.ListRow)
						require(second.State().Has(style.Hovered) && !first.State().Has(style.Hovered), "悬停跨越了行边界")
						click("settings-disabled")
					case 3:
						require(activations == [3]int{0, 1, 0}, "禁用行被执行")
						click("hide")
					case 4:
						require(!find("settings-action").Visible(), "隐藏未生效")
						require(len(visibleRows(group)) == 3, "隐藏行仍参与连接")
						check()
						sampleProbe()
						click("reverse")
					case 5:
						require(group.Children()[0] == find("settings-version"), "没有按声明重排")
						require(group.Children()[3] == find("settings-updates"), "重排替换了行实例")
						// 恢复可见，让可聚焦的动作行重新参与 Tab 导航。
						click("hide")
					case 6:
						require(find("settings-action").Visible() && len(visibleRows(group)) == 4, "恢复可见失败")
						// Tab 依次访问行内控件与工具栏按钮，最多前进若干次找到行本身。
						must(win.DispatchEvent(events.KeyEvent{EventType: events.KeyDown, Key: events.KeyTab}))
					case 7:
						focused, ok := win.FocusedWidget().(*widgets.ListRow)
						if !ok || focused.Parent() != group {
							require(tabSteps < 8, "Tab 焦点始终没有进入分组内的行")
							tabSteps++
							must(win.DispatchEvent(events.KeyEvent{EventType: events.KeyDown, Key: events.KeyTab}))
							step-- // 停留在本步重试
							break
						}
						sampleFocus(focused)
						click("theme")
					case 8:
						check()
						sampleProbe()
						complete = true
						fmt.Println("AUTO PASS")
						app.Quit()
					}
					step++
				})
			}
		}
		update := func() { app.RequestUpdate() }
		rules := basic.Rules()
		if !*basicTheme {
			rules = modern.Rules(modern.Options{Dark: *dark})
		}
		// 探针规则显式定义，避免把主题色写进断言。
		rules = append(rules, style.Name("seam-row").BackgroundColor(color.White).
			BorderColor(color.NRGBA{R: 255, A: 128}).BorderWidth(2).Radius(8))
		sheet := style.Sheet(rules...)
		settings := []ui.View{
			wui.ListRow("Automatic updates").ID("settings-updates").Subtitle("Shared seam, independent switch.").
				Suffix(wui.Switch().ID("settings-updates-switch").Checked(switched).OnChange(func(v bool) {
					switched = v
					switchChanges++
					update()
				})),
			wui.ListRow("Check for updates").ID("settings-action").Subtitle("Activates only this row.").
				Suffix(ui.Label("›")).Activatable(true).OnActivate(func() {
				activations[1]++
				update()
			}).Visible(!hidden),
			wui.ListRow("Unavailable").ID("settings-disabled").Subtitle("Disabled rows keep their outline.").
				Enabled(false).Activatable(true).OnActivate(func() { activations[2]++ }),
			wui.ListRow("Application version").ID("settings-version").Suffix(ui.Label("0.8.0")),
		}
		if reverse {
			settings[0], settings[3] = settings[3], settings[0]
			settings[1], settings[2] = settings[2], settings[1]
		}
		probe := make([]ui.View, 4)
		for i := range probe {
			probe[i] = wui.ListRow("").Style("seam-row").MinHeight(48)
		}
		return ui.Root().StyleSheet(sheet).Windows(ui.Window("rows").Title("GOUI ListRowGroup").Chrome(ui.WindowChromeIntegrated).Size(720, 640).Center(true).Content(
			ui.VBox(
				ui.HeaderBar(ui.Label("Connected list rows")),
				ui.Label("Rows share one border and only the outer corners stay rounded."),
				wui.ListRowGroup(settings...).ID("settings-group"),
				wui.ListRowGroup(probe...).ID("probe").MinWidth(200).MaxWidth(200),
				ui.HBox(
					ui.Button("Hide middle").ID("hide").OnClick(func() { hidden = !hidden; update() }),
					ui.Button("Reverse").ID("reverse").OnClick(func() { reverse = !reverse; update() }),
					ui.Button("Basic / Modern").ID("theme").OnClick(func() { *basicTheme = !*basicTheme; update() }),
					ui.Button("Light / Dark").ID("dark").OnClick(func() { *dark = !*dark; update() }),
					ui.Button("Check (F6)").ID("check").OnClick(check),
				).Spacing(8),
				ui.Label(fmt.Sprintf("Activations %v | switch changes %d | hidden %v | reversed %v", activations, switchChanges, hidden, reverse)),
			).Padding(18).Spacing(14).CrossAlign(layout.CrossStart).Shortcuts(ui.Shortcut(ui.KeyF6).OnActivate(check)),
		))
	}))
	if *auto {
		require(complete, "自动验收提前退出")
	}
}
