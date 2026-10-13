// Modern 样式展示。
// 环境：桌面窗口；Linux 需要 X11，后端和应用缩放可由 GOUI_PLAT_PAINTER / GOUI_PLAT_SCALE 指定。
// 启动：go run ./example/modern。初始：系统主题、系统主题色、启用 Modern。
// 操作与预期：
//  1. 切换 Mode 的 Light/Dark，文字、背景及语义色适配亮暗；切换 Accent 不改变 Warning/Error/Success。
//  2. 悬停/按下 Flat action 和 Menu，出现与普通按钮一致的中性底色和 1 DIP 描边，文字位置不跳动；点击 Flat action 更新输入框。
//  3. Disabled flat 无底色，不可点击或 Tab 聚焦；Flat action 的 Tab 聚焦显示 2 DIP 边框。
//  4. 切换 Bare GUI，清空自定义 StyleName，恢复 GUI 兜底，文字不丢失。
//  5. 打开菜单、滚动长菜单、改变窗口大小，检查菜单与控件无裁剪或透明色残留。
//  6. Accent 按钮使用主题色实底和显式 PrimaryContent；紫/绿使用白字，黄/白使用黑字。
//     常态/悬停/按下均不叠加普通描边，黑色仍有交互反馈；Disabled primary 使用中性底和弱化文字。
//     Primary 的 Tab 焦点亮色为深灰 #414147、暗色为浅灰 #D8D8DE，沿用 2 DIP 贴边框，实底和文字不变。
//     普通按钮/Flat 的焦点配色与几何不变。
//
// Card/Sidebar 的表面消费示例见 example/demo；布局容器不因 StyleName 自动绘制背景。
// 副作用：无系统配置/剪贴板/文件修改。复位：重新启动。退出：关闭窗口。
package main

import (
	"fmt"
	"image/color"

	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/style"
	"github.com/golang-gui/goui/theme/modern"
	"github.com/golang-gui/goui/ui"
)

func main() {
	mode, accentIndex := 0, 0
	text := "Hello, GOUI"
	modes := []string{"System", "Light", "Dark", "Bare GUI"}
	accents := []color.Color{nil, color.NRGBA{R: 140, G: 70, B: 220, A: 255}, color.NRGBA{R: 20, G: 160, B: 90, A: 255}, color.NRGBA{R: 255, G: 220, B: 70, A: 255}, color.White, color.Black}
	accentNames := []string{"System", "Purple", "Green", "Yellow", "White", "Black"}
	err := ui.Run("com.xugtek.goui", func(app ui.App) ui.RootView {
		settings := app.Settings()
		dark := mode == 2 || mode == 0 && settings.ColorScheme() == ui.ColorSchemeDark
		accent := accents[accentIndex]
		if accent == nil {
			accent = settings.AccentColor()
		}
		var sheet style.StyleSheet
		primary, primaryContent, muted, flat, warning, errorStyle, success := "", "", "", "", "", "", ""
		if mode != 3 {
			if mode == 0 && accentIndex == 0 {
				sheet = modern.FromSettings(settings)
			} else {
				sheet = modern.Sheet(modern.Options{
					Dark: dark, AccentColor: accent,
					FontFamily: settings.FontFamily(), FontSize: settings.FontSize(),
				})
			}
			primary, primaryContent, muted = modern.Primary, modern.PrimaryContent, modern.MutedText
			flat, warning, errorStyle, success = modern.Flat, modern.Warning, modern.Error, modern.Success
		}
		rows := make([]ui.View, 30)
		menuRows := make([]*ui.MenuItemView, 30)
		for i := range rows {
			rows[i] = ui.Label(fmt.Sprintf("Scrollable row %02d — rounded scrollbar", i+1))
			menuRows[i] = ui.MenuItem(fmt.Sprintf("Menu item %02d", i+1), nil)
		}
		edgeMenuRows := make([]*ui.MenuItemView, 120)
		for i := range edgeMenuRows {
			edgeMenuRows[i] = ui.MenuItem(fmt.Sprintf("Work-area constrained item %03d", i+1), nil)
		}
		return ui.Root().StyleSheet(sheet).Windows(
			ui.Window("modern").Title("GOUI Modern").Size(680, 560).
				Chrome(ui.WindowChromeIntegrated).Content(
				ui.VBox(
					ui.HeaderBar(ui.Label("GOUI Modern")),
					ui.VBox(
						ui.Label("Hover and press buttons to inspect their appearance.").Style(muted),
						ui.HBox(
							ui.Button("Mode: "+modes[mode]).OnClick(func() { mode = (mode + 1) % len(modes); app.RequestUpdate() }),
							ui.Button().Content(ui.Label("Accent: "+accentNames[accentIndex]).Style(primaryContent)).Style(primary).
								OnClick(func() { accentIndex = (accentIndex + 1) % len(accents); app.RequestUpdate() }),
							ui.Button().Content(ui.Label("Disabled primary").Style(primaryContent)).Style(primary).Enabled(false),
						).Spacing(12),
						ui.HWrap(
							ui.Button("Flat action").Style(flat).OnClick(func() { text = "Flat action clicked"; app.RequestUpdate() }),
							ui.Button("Disabled flat").Style(flat).Enabled(false),
							ui.Label("Warning").Style(warning), ui.Label("Error").Style(errorStyle), ui.Label("Success").Style(success),
						).Spacing(12).LineSpacing(8),
						ui.HBox(
							ui.MenuButton("Menu").Style(flat).Menu(
								ui.MenuItem("New document", func() { text = "New document"; app.RequestUpdate() }),
								ui.MenuItem("Unavailable command", nil).Enabled(false),
								ui.MenuSeparator(),
								ui.MenuItem("Clear text", func() { text = ""; app.RequestUpdate() }),
							),
							ui.MenuButton("Long menu").Menu(menuRows...),
						).Spacing(12),
						ui.TextInput().Text(text).OnText(func(value string) { text = value; app.RequestUpdate() }),
						ui.Label("System mode follows settings automatically; Bare GUI restores the fallback.").Style(muted),
						ui.ScrollView(ui.VBox(rows...).Spacing(10).Padding(8)),
					).Padding(16).Spacing(16).CrossAlign(layout.CrossStretch).MainWeight(1),
					ui.HBox(
						ui.Label("Move near screen edges, then open →").Style(muted),
						ui.MenuButton("Edge / long menu").Menu(edgeMenuRows...),
					).MainAlign(layout.MainEnd).Padding(8).Spacing(8),
				).CrossAlign(layout.CrossStretch),
			),
		)
	})
	if err != nil {
		panic(err)
	}
}
