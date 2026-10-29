// Software 平直边缘像素对齐回归，不使用 DevServer。
//
// 环境：Windows、Linux X11 或 macOS 桌面；记录 OS/WM、实际后端与缩放。
// 启动：GOUI_PLAT_PAINTER=software GOUI_PLAT_SCALE=1 go run ./tests/window/ui/pixel_snap
// Windows PowerShell 先设置上述环境变量；另用 opengl/direct2d（仅 Windows）作对照。
// 初始：Modern 亮色 Integrated 窗口，计数 0；第二排按钮高度为 35.25 DIP。
//
// 操作与逐项预期：
//  1. Windows 100% 下观察标题栏最小化横线、最大化方框和平直按钮下边框。
//     1 DIP 直边应集中于一个物理像素，不应变成相邻两行/列的浅色条。
//     关闭符号的斜线及按钮圆角仍应抗锯齿，不能因修复变成锯齿。
//  2. 点击 Count 两次：计数变成 2。点击 Theme 切到暗色，再切回亮色；
//     计数保留，普通/Primary 及小数高度按钮的边框均完整，无浅色带或漏背景。
//  3. 拖动标题栏、改变窗口大小，再最大化/还原；控件保持可操作，边框不变虚。
//     Windows 还应检查还原状态的重叠方框。macOS 原生红绿灯不属于 Software 绘制。
//  4. 用 GOUI_PLAT_SCALE=1.25/1.5/2 重启。保留 1 DIP 宽度：非整数物理宽度
//     的内边缘存在部分覆盖是预期；外边界应与背景一致。整数像素宽度不得分摊。
//     环境变量只表示请求，终端输出实际图片后端与 PixelScale。
//
// 本例用于桌面呈现/操作；精确覆盖率、负坐标、Path/变换保真在 Software 包内断言。
// 副作用：仅创建窗口和输出日志，不改系统设置、文件或剪贴板。
// 复位：重启程序。退出：关闭主窗口。
package main

import (
	"fmt"
	"image"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/theme/modern"
	"github.com/golang-gui/goui/ui"
)

func main() {
	dark, clicks := false, 0
	var reportedScale float32
	if err := ui.Run("org.golang-gui.PixelSnap", func(app ui.App) ui.RootView {
		fmt.Printf("STATE dark=%t clicks=%d\n", dark, clicks)
		count := func() { clicks++; app.RequestUpdate() }
		return ui.Root().StyleSheet(modern.Sheet(modern.Options{Dark: dark})).Windows(
			ui.Window("pixel-snap").Title("GOUI pixel alignment").Size(720, 400).
				Chrome(ui.WindowChromeIntegrated).Content(ui.VBox(
				ui.HeaderBar(ui.Label("Pixel alignment | caption controls")).Padding(6),
				ui.VBox(
					ui.Label("Compare straight bottom borders and caption symbols."),
					ui.HBox(
						ui.Button("Count").MinWidth(160).OnClick(count),
						ui.Button("Primary").MinWidth(160).Style(modern.Primary).OnClick(count),
						ui.Button("Theme").MinWidth(160).OnClick(func() { dark = !dark; app.RequestUpdate() }),
					).Spacing(12),
					ui.HBox(
						ui.Button("35.25 DIP").MinWidth(160).MinHeight(35.25).MaxHeight(35.25).OnClick(count),
						ui.Button("35.25 DIP / Primary").MinWidth(210).MinHeight(35.25).MaxHeight(35.25).Style(modern.Primary).OnClick(count),
					).Spacing(12),
					ui.Label(fmt.Sprintf("Clicks: %d | dark: %t", clicks, dark)),
					ui.Label("Corners and diagonal strokes must remain antialiased."),
				).Padding(20).Spacing(13.25),
			)).OnAfterPaint(func(p ui.Painter, _ geometry.Rectangle) {
				if reportedScale == p.PixelScale() {
					return
				}
				native, err := p.NewImage(image.NewRGBA(image.Rect(0, 0, 1, 1)))
				if err != nil {
					panic(err)
				}
				fmt.Printf("BACKEND %T scale=%g\n", native, p.PixelScale())
				native.Destroy()
				reportedScale = p.PixelScale()
			}),
		)
	}); err != nil {
		panic(err)
	}
}
