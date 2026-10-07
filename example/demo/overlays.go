package main

import (
	"fmt"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/theme/modern"
	"github.com/golang-gui/goui/ui"
)

func (d *demo) overlaysPage() ui.View {
	long := make([]*ui.MenuItemView, 45)
	for i := range long {
		long[i] = ui.MenuItem(fmt.Sprintf("长菜单项目 %02d", i+1), func() { d.report(fmt.Sprintf("选中菜单项目 %d", i+1)) })
	}
	base := page("菜单与覆盖层", "菜单使用原生 Popup；浮动面板和遮罩是普通 Overlay 子树。遮罩阻挡指针，不代表完整的键盘模态或焦点圈定。",
		section("PopoverMenu",
			ui.HWrap(ui.MenuButton("操作菜单").Menu(ui.MenuItem("执行操作", func() { d.report("菜单操作已执行") }), ui.MenuItem("禁用项目", nil).Enabled(false), ui.MenuSeparator(), ui.MenuItem("显示通知", func() { d.showToast("菜单触发了一条 Toast") })),
				ui.MenuButton("长菜单 / 桌面边缘退避").Menu(long...),
				ui.Button("通用 Popover").ID("popover-anchor").OnClick(d.openPopover)).Spacing(12).LineSpacing(8)),
		section("Overlay",
			ui.HWrap(ui.Button("浮动查找面板").ID("open-panel").OnClick(func() { d.panel = !d.panel; d.app.RequestUpdate() }),
				ui.Button("遮罩确认 Dialog").ID("open-dialog").OnClick(func() {
					if d.popup != nil {
						d.popup.Hide()
					}
					d.mask = true
					d.app.RequestUpdate()
				}),
				ui.Button("Toast（3 秒）").ID("open-toast").OnClick(func() { d.showToast("这是窗口内通知，不创建原生窗口") })).Spacing(12).LineSpacing(8),
			ui.TextInput().BindText(&d.input), ui.Button("背景操作").OnClick(func() { d.report("背景按钮收到了点击") }),
			ui.Label("面板之外仍可点击和滚动；遮罩展开后，本内容页的背景指针输入被覆盖。")),
		section("可滚动背景", ui.VBox(chipsCopy(25)...).Spacing(12)),
	)
	panel := surface(ui.VBox(ui.Label("查找").Style("demo-subheading"), ui.TextInput().BindText(&d.input), ui.Button("关闭面板").OnClick(func() { d.panel = false; d.app.RequestUpdate() })).Padding(16).Spacing(12).CrossAlign(layout.CrossStretch)).Style("demo-card").MinWidth(300).MaxWidth(330).Visible(d.panel)
	dialog := surface(ui.VBox(ui.Label("确认示例操作？").Style("demo-subheading"), ui.Label("这不是系统对话框；确认只更新操作反馈。"),
		ui.HBox(ui.Button("取消").OnClick(func() { d.mask = false; d.app.RequestUpdate() }), ui.Button("确认").Style(modern.Primary).OnClick(func() { d.mask = false; d.showToast("操作已确认") })).Spacing(12).MainAlign(layout.MainEnd)).Padding(24).Spacing(16).CrossAlign(layout.CrossStretch)).Style("demo-card").MaxWidth(450)
	mask := ui.Overlay(surface(nil).Style("demo-mask"), ui.OverlayItem(dialog).OnPosition(center)).Visible(d.mask)
	toast := surface(ui.HBox(ui.Label(d.toast)).Padding(12)).Style("demo-card").Visible(d.toast != "").MinHeight(48).MinWidth(280)
	return ui.Overlay(base,
		ui.OverlayItem(panel).OnPosition(func(a, s geometry.Size, p *geometry.Point) { *p = geometry.Point{X: max(0, a.Width-s.Width-20), Y: 20} }),
		ui.OverlayItem(mask).Fill(true),
		ui.OverlayItem(toast).OnPosition(func(a, s geometry.Size, p *geometry.Point) {
			*p = geometry.Point{X: max(0, (a.Width-s.Width)/2), Y: max(0, a.Height-s.Height-20)}
		}),
	).ID("page-overlays").MainWeight(1)
}
func (d *demo) openPopover() {
	if d.popup != nil {
		d.popup.Destroy()
		d.popup = nil
	}
	anchor := d.app.FindWidget("main", "popover-anchor")
	if anchor == nil {
		return
	}
	p := gui.NewPopover(anchor, nil)
	box := gui.NewLinearBox(layout.DirectionVertical)
	box.SetPadding(16)
	box.SetSpacing(12)
	box.AddChild(gui.NewLabel("通用 Popover"))
	box.AddChild(gui.NewLabel("由公开 GUI 接口创建，锚点跟随窗口移动。"))
	button := gui.NewButton()
	button.SetChild(gui.NewLabel("关闭"))
	button.ConnectClicked(func() { p.Hide() })
	box.AddChild(button)
	p.SetWidget(box)
	p.SetPlacement(gui.PopoverPlacementBottom)
	p.ConnectDismissRequest(p.Hide)
	d.popup = p
	d.result("显示 Popover", p.Show())
}
