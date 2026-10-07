package main

import (
	"fmt"

	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/ui"
	wui "github.com/golang-gui/goui/widgets/ui"
)

func (d *demo) layoutPage() ui.View {
	chips := make([]ui.View, 18)
	for i := range chips {
		chips[i] = ui.Button(fmt.Sprintf("流式项目 %02d", i+1)).OnClick(func() { d.report(fmt.Sprintf("WrapBox 项目 %d", i+1)) })
	}
	vertical := make([]ui.View, 8)
	for i := range vertical {
		vertical[i] = ui.Label(fmt.Sprintf("VWrap %02d", i+1))
	}
	return page("布局与滚动", "拖动分隔条、缩放窗口，观察权重、基线和换行；滚动条保留胶囊滑块及足够命中宽度。",
		section("独立 ScrollBar（GUI 控件的本地 UI 绑定）", scrollBar(d.scrollValue, func(value float32) { d.scrollValue = value; d.app.RequestUpdate() }), ui.Label(fmt.Sprintf("当前位置：%.0f / 100", d.scrollValue))),
		section("HBox / VBox / Baseline",
			ui.HBox(ui.Label("用户名"), ui.TextInput().Text("GOUI").MainWeight(1), ui.Button("确定")).CrossAlign(layout.CrossBaseline).Spacing(12),
			ui.HBox(surface(ui.HBox(ui.Label("权重 1")).Padding(12)).Style("demo-card").MainWeight(1), surface(ui.HBox(ui.Label("权重 2")).Padding(12)).Style("demo-card").MainWeight(2)).Spacing(12)),
		section("HWrap：窗口变窄后折行", ui.HWrap(chips...).Spacing(8).LineSpacing(8)),
		section("VWrap：固定高度内换列", ui.VWrap(vertical...).Spacing(8).LineSpacing(20).MinHeight(95).MaxHeight(95)),
		section("SplitView：两侧滚动内容",
			wui.HSplit(ui.ScrollView(ui.VBox(ui.Label("左侧"), ui.Label("StartSize 初值 200 DIP"), ui.TextInput().Text("拖动分隔条后改变内容")).Spacing(12).Padding(12)),
				wui.VSplit(ui.ScrollView(ui.VBox(ui.Label("右上"), ui.Label("自动尺寸 + 用户拖动"))).MinHeight(70),
					ui.ScrollView(ui.VBox(chipsCopy(12)...).Spacing(10).Padding(12))).Ratio(d.layoutRatio).
					OnResize(func(start, end float32) {
						if start+end > 0 {
							d.layoutRatio = start / (start + end)
						}
					})).StartSize(d.layoutStart).OnResize(func(start, _ float32) { d.layoutStart = start }).MinHeight(280).MaxHeight(280)),
	)
}
func chipsCopy(n int) []ui.View {
	rows := make([]ui.View, n)
	for i := range rows {
		rows[i] = ui.Label(fmt.Sprintf("滚动行 %02d", i+1))
	}
	return rows
}
