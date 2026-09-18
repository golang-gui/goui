// Icon 窗口验证：样式驱动的矢量／自定义绘制，通过 Software 生成并缓存位图，不使用 DevServer。
//
// 环境：桌面会话及相应绘制库；记录 OS、Linux 桌面环境、后端和缩放比例。
// 启动：go run ./tests/window/ui/icon
// 可用 GOUI_PLAT_PAINTER=software/opengl（Windows 另有 direct2d）选择后端，
// GOUI_PLAT_SCALE=1 或 2 选择缩放。ui.Run 负责锁定 GUI 主线程。
// 初始：亮色、蓝色主题、24 DIP；显示搜索、Path 勾号、双色标记及图标按钮。
//
// 操作与预期：
//  1. 点击 Light/Dark。普通搜索图标随正文变亮／暗，Accent 勾号保持可辨，
//     双色标记的橙色部分不变，另一部分随前景变化，无旧颜色残留。
//  2. 点击 Accent。蓝／紫／绿循环，只有主题色图标跟随；普通图标不变。
//  3. 点击 Size。24/40/16 DIP 循环，圆形仍为圆形，Path 比例不变；在 2x 下
//     重新运行检查斜线和曲线边缘，不应出现放大低分辨率位图的模糊。
//  4. 点击 Replace。最后一个图标在加号和减号间切换，验证同源闭包更新。
//  5. 点击搜索图标按钮，每次计数加一；悬停／按下只改变按钮背景，图标不
//     继承按钮样式；独立图标无点击行为，不出现焦点边框。
//  6. 调整窗口大小，图标不溢出控件边界；切换主题后重复以上操作。
//
// 平台差异：字体／窗口装饰由现有平台处理；三后端图标几何和颜色应一致，
// 允许边缘栅格化细微差异。半透明画刷专项验证见 tests/window/graphics/alpha。
// 副作用：无系统设置、剪贴板或文件修改。复位：重启。退出：关闭窗口。
package main

import (
	"fmt"
	"image/color"

	"github.com/golang-gui/goui/core/geometry"
	drawicon "github.com/golang-gui/goui/icon/draw"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/theme/modern"
	"github.com/golang-gui/goui/ui"
)

func search(p graphics.Painter, size graphics.Size, c graphics.Color) {
	s := size.Width
	p.DrawEllipse(geometry.Point{X: s * .42, Y: s * .42}, s*.25, s*.25, s*.08, c)
	p.DrawLine(geometry.Point{X: s * .60, Y: s * .60}, geometry.Point{X: s * .88, Y: s * .88}, s*.08, c)
}

var checkPath = graphics.MoveTo(3, 12).LineTo(9, 18).LineTo(21, 5)

func check(p graphics.Painter, size graphics.Size, c graphics.Color) {
	p.SetTransform(geometry.Scale(size.Width/24, size.Height/24))
	p.DrawPath(checkPath, 2, c)
}

func mark(p graphics.Painter, size graphics.Size, c graphics.Color) {
	p.FillEllipse(geometry.Point{X: size.Width / 2, Y: size.Height / 2}, size.Width*.45, size.Height*.45,
		ui.Color{R: 1, G: .5, A: 1})
	p.FillRect(geometry.Rect(size.Width*.4, size.Height*.2, size.Width*.2, size.Height*.6), c)
}

func symbol(plus bool) *drawicon.Source {
	return &drawicon.Source{Draw: func(p graphics.Painter, size graphics.Size, c graphics.Color) {
		p.FillRect(geometry.Rect(size.Width*.2, size.Height*.45, size.Width*.6, size.Height*.1), c)
		if plus {
			p.FillRect(geometry.Rect(size.Width*.45, size.Height*.2, size.Width*.1, size.Height*.6), c)
		}
	}}
}

func main() {
	dark, plus := false, true
	searchSource, checkSource, markSource := &drawicon.Source{Draw: search}, &drawicon.Source{Draw: check}, &drawicon.Source{Draw: mark}
	plusSource, minusSource := symbol(true), symbol(false)
	accentIndex, sizeIndex, clicks := 0, 0, 0
	accents := []color.Color{color.RGBA{R: 40, G: 120, B: 220, A: 255}, color.RGBA{R: 160, G: 60, B: 200, A: 255}, color.RGBA{R: 20, G: 150, B: 80, A: 255}}
	sizes := []float32{24, 40, 16}
	if err := ui.Run("org.golang-gui.IconTest", func(app ui.App) ui.RootView {
		size := sizes[sizeIndex]
		symbolSource := minusSource
		if plus {
			symbolSource = plusSource
		}
		return ui.Root().StyleSheet(modern.Sheet(modern.Options{Dark: dark, AccentColor: accents[accentIndex]})).Windows(
			ui.Window("icons").Title("GOUI Icon validation").Size(720, 320).Chrome(ui.WindowChromeIntegrated).Content(
				ui.VBox(
					ui.HeaderBar(ui.Label("Drawing icons")),
					ui.VBox(
						ui.Label(fmt.Sprintf("Dark: %v | Accent: %d | Size: %.0f DIP | Clicks: %d", dark, accentIndex+1, size, clicks)),
						ui.HBox(
							ui.Button("Light/Dark").OnClick(func() { dark = !dark; app.RequestUpdate() }),
							ui.Button("Accent").OnClick(func() { accentIndex = (accentIndex + 1) % len(accents); app.RequestUpdate() }),
							ui.Button("Size").OnClick(func() { sizeIndex = (sizeIndex + 1) % len(sizes); app.RequestUpdate() }),
							ui.Button("Replace").OnClick(func() { plus = !plus; app.RequestUpdate() }),
						).Spacing(12),
						ui.HBox(
							ui.Icon(searchSource).Size(size),
							ui.Icon(checkSource).Size(size).Style(modern.AccentIcon),
							ui.Icon(markSource).Size(size),
							ui.Button().Content(ui.Icon(searchSource).Size(size)).Style(modern.Primary).OnClick(func() { clicks++; app.RequestUpdate() }),
							ui.Icon(symbolSource).Size(size),
						).Spacing(24).CrossAlign(layout.CrossCenter),
						ui.Label("Search | Accent path | Two-color | Icon button | Closure"),
					).Padding(16).Spacing(20),
				).CrossAlign(layout.CrossStretch),
			),
		)
	}); err != nil {
		panic(err)
	}
}
