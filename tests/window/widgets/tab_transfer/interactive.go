package main

import (
	"fmt"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/theme/modern"
	"github.com/golang-gui/goui/widgets"
)

// Position only after the first normal frame: the WM may reparent a just
// mapped X11 window asynchronously. No delay or forced layout is required.
type positionedColumn struct {
	gui.WidgetBase
	after func()
}

func (c *positionedColumn) Paint(gui.Painter) {
	if after := c.after; after != nil {
		c.after = nil
		gui.App.Post(after)
	}
}

// 连续体验模式：双向移动、添加页面和取消均可重复操作，不做一次性退出判定。
func runInteractive(chrome gui.WindowChromeMode) error {
	app, err := gui.NewApplication("org.golang-gui.TabTransferDemo")
	if err != nil {
		return err
	}
	app.SetStyleSheet(modern.Sheet(modern.Options{}))
	var windows []gui.Window
	defer func() {
		for _, window := range windows {
			window.Destroy()
		}
	}()
	serial := 0
	for _, title := range []string{"GOUI TRANSFER LEFT", "GOUI TRANSFER RIGHT"} {
		window, err := app.NewWindow(&gui.WindowOptions{Size: geometry.Size{Width: 500, Height: 320}, Chrome: chrome})
		if err != nil {
			return err
		}
		windows = append(windows, window)
		window.SetMinSize(geometry.Size{Width: 400, Height: 280})
		_ = window.SetTitle(title)
		view := widgets.NewTabView()
		addPage := func() {
			serial++
			input := gui.NewTextInput()
			input.SetText(fmt.Sprintf("Retained text %d", serial))
			view.AppendPage(widgets.NewTabPage(fmt.Sprintf("Document %d", serial), input))
		}
		addPage()
		addPage()
		bar := widgets.NewTabBar()
		bar.SetView(view)
		bar.SetReorderable(true)
		bar.SetTransferable(true)
		status := gui.NewLabel("Move any tab between windows, including the last.")
		bar.ConnectTransferError(func(err error) {
			fmt.Println("ERROR:", err)
			if !status.Destroyed() {
				status.SetText(err.Error())
			}
		})
		add := gui.NewButton()
		add.SetChild(gui.NewLabel("Add tab"))
		add.ConnectClicked(addPage)
		column := new(positionedColumn)
		column.SetLayoutManager(&layout.LinearLayout{Direction: layout.DirectionVertical, CrossAlign: layout.CrossStretch, Spacing: 8})
		var top gui.Widget = bar
		if chrome == gui.WindowChromeIntegrated {
			header := gui.NewHeaderBar()
			header.SetChild(bar)
			top = header
		}
		for _, child := range []gui.Widget{top, status, add, view} {
			column.WidgetBase.AddChild(column, child)
		}
		position := geometry.Point{X: float32(50 + (len(windows)-1)*550), Y: 100}
		column.after = func() {
			if !column.Destroyed() {
				if err := window.SetPosition(nil, position); err != nil {
					status.SetText(err.Error())
					fmt.Println("ERROR:", err)
				}
			}
		}
		window.SetWidget(column)
		if err := window.Show(); err != nil {
			return err
		}
	}
	app.Run()
	return nil
}
