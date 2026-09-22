// WrapBox 桌面验证（无 DevServer 依赖）。
//
// 环境：Windows/macOS/Linux 桌面，记录系统、绘制后端和缩放；建议分别使用 1x/2x。
// 启动：go run ./tests/window/ui/wrap
// 初始：780x650 原生窗口，显式最小尺寸 400x600；上方是自动换行的按钮，
// 下方包含基线、竖向换列和滚动示例。
// 操作与预期：
//  1. 连续缩窄/放宽窗口，上方按钮按原顺序换行/合行，无重叠；短按钮在所在行居中。
//     必须能缩到 400 DIP 宽，此时上方按钮分成多行；再放宽到 780 DIP 应合行。
//     不应被“全部按钮单行宽度”的窗口最小尺寸卡住。
//  2. 点击任意项目，顶部显示其名称及点击次数；重建后继续点击，只递增一次。
//  3. 点击 Toggle last，末项消失/恢复并重新排布；点击 Alignment，行内对齐循环切换，
//     行与行整体仍靠上，不应把所有行一起垂直居中。
//  4. 调整宽度观察不同字号的 Baseline 行：同一行文字基线一致，换行后各自对齐。
//     竖向示例 V1～V8 从上向下排列，超过限定高度后从左到右换列；400 DIP 宽时全部可见。
//  5. 滚动底部区域，可以看到所有编号；内容显式 MaxWidth(360)，不会随视口自动铺宽。
//     这是当前双向 ScrollView 的约束语义，不是响应式宽度示例。
//  6. 关闭窗口退出。重新运行复位；不写文件、不改变系统设置。
//
// 平台差异：字体度量可能改变具体换行阈值，顺序、间距和命中行为应一致。
package main

import (
	"fmt"
	"log"

	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/style"
	"github.com/golang-gui/goui/theme/modern"
	"github.com/golang-gui/goui/ui"
)

func main() {
	count, alignment := 0, 0
	selected := "none"
	showLast := true
	if err := ui.Run("org.golang-gui.Wrap", func(app ui.App) ui.RootView {
		items := []ui.View{}
		names := []string{"Go", "Windows", "Linux", "macOS", "A longer tool button", "Short", "Last"}
		for i, name := range names {
			if i == len(names)-1 && !showLast {
				continue
			}
			items = append(items, ui.Button(name).Name(name).Padding(float32(6+i%3*4)).OnClick(func() { selected = name; count++; app.RequestUpdate() }))
		}
		rows := make([]ui.View, 30)
		for i := range rows {
			rows[i] = ui.Button(fmt.Sprintf("Item %02d", i+1)).MinSize(90, 30)
		}
		columns := make([]ui.View, 8)
		for i := range columns {
			columns[i] = ui.Label(fmt.Sprintf("V%d", i+1)).MinSize(90, 24)
		}
		sheet := style.Sheet(append(modern.Rules(modern.Options{}),
			style.Name("wrap-small").FontSize(14), style.Name("wrap-large").FontSize(26),
		)...)
		// Unbounded measurement describes the preferred single-line width, not
		// the minimum width of a wrapping layout. Override the window's current
		// automatic size hint explicitly so the WM permits this demonstration.
		return ui.Root().StyleSheet(sheet).Windows(ui.Window("wrap").Title("GOUI WrapBox").Size(780, 650).MinSize(100, 600).Content(
			ui.VBox(
				ui.Label(fmt.Sprintf("Clicked: %s (%d); alignment=%d", selected, count, alignment)),
				ui.HWrap(ui.Button("Toggle last").OnClick(func() { showLast = !showLast; app.RequestUpdate() }), ui.Button("Alignment").OnClick(func() { alignment = (alignment + 1) % 4; app.RequestUpdate() })).Spacing(8),
				ui.HWrap(items...).Spacing(8).LineSpacing(6).MainAlign(layout.MainAlign(alignment)),
				ui.HWrap(ui.Label("Baseline").Style("wrap-small"), ui.Label("Large text").Style("wrap-large"), ui.Label("Another baseline").Style("wrap-small")).Spacing(12).LineSpacing(6).CrossAlign(layout.CrossBaseline),
				ui.VWrap(columns...).Spacing(4).LineSpacing(14).MinHeight(80).MaxHeight(80),
				ui.Label("Scroll: content max width 360 DIP"),
				ui.ScrollView(ui.HWrap(rows...).Spacing(8).LineSpacing(8).MaxWidth(360)),
			).Spacing(12).Padding(12).CrossAlign(layout.CrossStretch),
		))
	}); err != nil {
		log.Fatal(err)
	}
}
