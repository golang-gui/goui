package main

import (
	"encoding/json"
	"fmt"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/ui"
)

func (d *demo) windowPage() ui.View {
	return fullPage("窗口与语义检查", "主窗口使用 Integrated。平台不具备相应能力时按 GUI 契约退回；窗口状态请求不承诺成功。",
		ui.HWrap(ui.Button("普通 Native 窗口").ID("native-window").OnClick(func() { d.openWindow(gui.WindowChromeNative, false) }),
			ui.Button("Integrated 窗口").OnClick(func() { d.openWindow(gui.WindowChromeIntegrated, false) }),
			ui.Button("透明 None 窗口").ID("transparent-window").OnClick(func() { d.openWindow(gui.WindowChromeNone, true) })).Spacing(12).LineSpacing(8),
		ui.HWrap(ui.Button("请求最大化").OnClick(func() {
			d.app.FindWindow("main").RequestState(gui.WindowStateMaximized)
			d.report("已请求最大化")
		}),
			ui.Button("请求普通状态").OnClick(func() {
				d.app.FindWindow("main").RequestState(gui.WindowStateNormal)
				d.report("已请求普通状态")
			}),
			ui.Button("读取位置 / 工作区").OnClick(d.inspectPosition), ui.Button("读取快照").ID("snapshot").OnClick(d.inspectSnapshot),
			ui.Button("按 ID 查找").OnClick(func() {
				w := d.app.FindWidget("main", "snapshot")
				if w != nil {
					d.report(fmt.Sprintf("snapshot → %T · 角色 %s", w, w.Snapshot().Role))
				}
			})).Spacing(10).LineSpacing(8),
		ui.ScrollView(ui.TextView().ID("snapshot-text").Model(d.inspection).ReadOnly(true).WrapMode(ui.WrapWordChar).Padding(12)).MainWeight(1),
	)
}
func (d *demo) inspectSnapshot() {
	w := d.app.FindWindow("main")
	if w == nil {
		return
	}
	data, err := json.MarshalIndent(w.Snapshot(), "", "  ")
	if err == nil {
		d.inspection.SetText(string(data))
	}
	d.result("读取语义快照", err)
}
func (d *demo) inspectPosition() {
	w := d.app.FindWindow("main")
	pos, err := w.Position(nil)
	if err != nil {
		d.result("读取位置", err)
		return
	}
	area, err := w.WorkAreaAt(geometry.Point{})
	if err != nil {
		d.result("读取工作区", err)
		return
	}
	d.report(fmt.Sprintf("客户内容原点相对工作区：%.0f, %.0f DIP；工作区：%.0f×%.0f DIP；状态：%v", pos.X, pos.Y, area.Width, area.Height, w.State()))
}
func (d *demo) openWindow(chrome gui.WindowChromeMode, transparent bool) {
	w, err := gui.App.NewWindow(&gui.WindowOptions{Size: geometry.Size{Width: 440, Height: 300}, Chrome: chrome, Transparent: transparent})
	if err != nil {
		d.result("创建窗口", err)
		return
	}
	if err := w.SetTitle("GOUI · 独立窗口"); err != nil {
		w.Destroy()
		d.result("设置窗口标题", err)
		return
	}
	if transparent {
		w.SetStyleName("demo-transparent-window")
	}
	box := gui.NewLinearBox(layout.DirectionVertical)
	box.SetSpacing(16)
	box.SetPadding(20)
	box.AddChild(gui.NewLabel("通过 GUI 公共接口创建的独立窗口"))
	box.AddChild(gui.NewLabel("主题变化会重新绘制；关闭主窗口退出整个示例。"))
	if transparent {
		box.AddChild(gui.NewLabel("此窗口独立使用透明背景，其他窗口的背景不变。"))
	}
	move := gui.NewButton()
	move.SetChild(gui.NewLabel("移动到主窗口右下方"))
	move.ConnectClicked(func() {
		d.result("相对定位", w.SetPosition(d.app.FindWindow("main"), geometry.Point{X: 240, Y: 160}))
	})
	box.AddChild(move)
	closeButton := gui.NewButton()
	closeButton.SetChild(gui.NewLabel("关闭此窗"))
	closeButton.ConnectClicked(func() { w.Destroy() })
	box.AddChild(closeButton)
	header := gui.NewHeaderBar()
	title := "GOUI · 可以拖动标题区域"
	if chrome == gui.WindowChromeNative {
		title = "GOUI · 请拖动系统原生标题栏"
	}
	header.SetChild(gui.NewLabel(title))
	column := gui.NewLinearBox(layout.DirectionVertical)
	column.SetCrossAlign(layout.CrossStretch)
	if transparent {
		column.SetPadding(12)
	} // 子树留在圆角内部，不假装背景绘制能够裁剪后续孩子。
	column.AddChild(header)
	column.AddChild(box)
	panel := new(surfaceWidget)
	panel.SetStyleName("demo-card")
	panel.SetLayoutManager(layout.NewFillLayout())
	panel.SetChild(column)
	if transparent {
		panel.SetMainWeight(1)
		outer := gui.NewLinearBox(layout.DirectionVertical)
		outer.SetPadding(16)
		outer.SetCrossAlign(layout.CrossStretch)
		outer.AddChild(panel)
		w.SetWidget(outer)
	} else {
		w.SetWidget(panel)
	}
	w.ConnectState(func(state gui.WindowState) { d.report(fmt.Sprintf("独立窗口实际上报状态：%v", state)) })
	d.app.RequestUpdate()
	// 先选定此窗口的背景样式、创建内容，再显示；不展示默认位置后再定位。
	d.app.Post(func() {
		positionErr := w.SetPosition(d.app.FindWindow("main"), geometry.Point{X: 180, Y: 120})
		if err := w.Show(); err != nil {
			w.Destroy()
			d.result("显示窗口", err)
			return
		}
		if positionErr != nil {
			d.result("窗口已显示，但相对定位", positionErr)
		} else {
			d.report("独立窗口已显示（可尝试拖动标题区域）")
		}
	})
}
