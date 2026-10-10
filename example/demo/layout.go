package main

import (
	"fmt"

	"github.com/golang-gui/goui/example/demo/resource"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/theme/modern"
	"github.com/golang-gui/goui/ui"
	wui "github.com/golang-gui/goui/widgets/ui"
)

func (d *demo) layoutPage() ui.View {
	chips := make([]ui.View, 18)
	for i := range chips {
		chips[i] = ui.Button(fmt.Sprintf(resource.WrapItem(), i+1)).OnClick(func() { d.report(fmt.Sprintf(resource.WrapClicked(), i+1)) })
	}
	vertical := make([]ui.View, 8)
	for i := range vertical {
		vertical[i] = ui.Label(fmt.Sprintf("VWrap %02d", i+1))
	}
	return page("layout", resource.Layout(), resource.LayoutHint(),
		section(resource.ScrollBarTitle(), scrollBar(d.scrollValue, func(value float32) { d.scrollValue = value; d.app.RequestUpdate() }), ui.Label(fmt.Sprintf(resource.ScrollPosition(), d.scrollValue))),
		section("HBox / VBox / Baseline",
			ui.HBox(ui.Label(resource.Username()), ui.TextInput().BindText(&d.username).MainWeight(1), ui.Button(resource.OK())).CrossAlign(layout.CrossBaseline).Spacing(12),
			ui.HBox(surface(ui.HBox(ui.Label(resource.WeightOne())).Padding(12)).Style(modern.Card).MainWeight(1), surface(ui.HBox(ui.Label(resource.WeightTwo())).Padding(12)).Style(modern.Card).MainWeight(2)).Spacing(12)),
		section(resource.HWrapTitle(), ui.HWrap(chips...).Spacing(8).LineSpacing(8)),
		section(resource.VWrapTitle(), ui.VWrap(vertical...).Spacing(8).LineSpacing(20).MinHeight(95).MaxHeight(95)),
		section(resource.SplitTitle(),
			wui.HSplit(ui.ScrollView(ui.VBox(ui.Label(resource.Left()), ui.Label(resource.StartSizeHint()), ui.TextInput().ID("split-input").BindText(&d.splitInput)).Spacing(12).Padding(12)),
				wui.VSplit(ui.ScrollView(ui.VBox(ui.Label(resource.RightTop()), ui.Label(resource.AutoSize()))).MinHeight(70),
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
		rows[i] = ui.Label(fmt.Sprintf(resource.ScrollRow(), i+1))
	}
	return rows
}
