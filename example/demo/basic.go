package main

import (
	"fmt"

	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/theme/modern"
	"github.com/golang-gui/goui/ui"
	wui "github.com/golang-gui/goui/widgets/ui"
)

func (d *demo) basicPage() ui.View {
	change := func() { d.app.RequestUpdate() }
	return page("基础控件", "点击、悬停、禁用和 Tab 焦点；图标与文本随主题独立解析自己的样式。",
		section("按钮与可用状态",
			wui.Switch("启用下方控件").Checked(d.enabled).OnChange(func(v bool) { d.enabled = v; change() }),
			ui.HWrap(ui.Button("普通按钮").OnClick(func() { d.report("普通按钮已点击") }),
				ui.Button("主要操作").Style(modern.Primary).OnClick(func() { d.report("主要操作已点击") }),
				ui.Button().Content(ui.HBox(ui.Icon(d.icons[0]).Size(18), ui.Label("带图标")).Spacing(8)).OnClick(func() { d.report("图标按钮已点击") }),
			).Spacing(12).LineSpacing(12).Enabled(d.enabled),
			ui.Button("始终禁用").Enabled(false)),
		section("复选、单选、按钮式选择与开关",
			ui.HWrap(
				wui.CheckButton("复选框").Checked(d.checked).OnChange(func(v wui.CheckState) { d.checked = v == wui.CheckChecked; change() }),
				wui.CheckButton("三态初值").CheckState(d.mixed).OnChange(func(v wui.CheckState) { d.mixed = v; change() }),
				wui.CheckButton("按钮式").Appearance(wui.CheckAppearanceButton).Checked(d.toggle).OnChange(func(v wui.CheckState) { d.toggle = v == wui.CheckChecked; change() }),
				wui.Switch("开关").Checked(d.switched).OnChange(func(v bool) { d.switched = v; change() }),
			).Spacing(18).LineSpacing(12).Enabled(d.enabled),
			ui.HBox(wui.CheckButton("选项 A").Group(d.group).Checked(d.radio == 0).OnChange(func(v wui.CheckState) {
				if v == wui.CheckChecked {
					d.radio = 0
					change()
				}
			}),
				wui.CheckButton("选项 B").Group(d.group).Checked(d.radio == 1).OnChange(func(v wui.CheckState) {
					if v == wui.CheckChecked {
						d.radio = 1
						change()
					}
				})).Spacing(20).Enabled(d.enabled)),
		section("DropDown：默认与自定义项目",
			wui.DropDown(d.choices).ID("basic-dropdown").Selected(d.drop).OnSelected(func(i int) { d.drop = i; d.report(fmt.Sprintf("默认列表选择第 %d 项", i+1)) }).OnOpenError(func(e error) { d.result("展开下拉框", e) }).Enabled(d.enabled),
			wui.DropDown(d.choices).Selected(d.fancyDrop).Item(func(i int, item wui.DropDownItem) ui.View {
				return ui.HBox(ui.Icon(d.icons[i%len(d.icons)]).Size(22), ui.Label(item.Text)).Spacing(12).Padding(6)
			}).
				OnSelected(func(i int) { d.fancyDrop = i; change() }).OnOpenError(func(e error) { d.result("展开自定义列表", e) }).Enabled(d.enabled),
			ui.Label("禁用可以来自控件自身或祖先；启用后保留选择值。选项中的禁用项目不可选择。").Style(modern.MutedText)),
		section("文本布局", ui.HBox(ui.Label("Baseline"), ui.TextInput().Text("文字基线对齐").MinWidth(240)).CrossAlign(layout.CrossBaseline).Spacing(12),
			ui.Label("Label 可以换行；较长文字按测量约束计算，滚动页面不会强行挤压全部内容。")),
	)
}
