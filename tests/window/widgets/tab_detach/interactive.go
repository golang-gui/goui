package main

import (
	"fmt"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/theme/modern"
	"github.com/golang-gui/goui/widgets"
)

// 连续体验模式：错误显示在原窗口并写终端，不因第二次拖动退出。
// 每个新窗口同样支持添加页面及继续拖出；最后一页可转入已有窗口，但不再新建窗口。
func runInteractive(chrome gui.WindowChromeMode) error {
	app, err := gui.NewApplication("org.golang-gui.TabDetachDemo")
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
	addPage := func(view *widgets.TabView) {
		serial++
		input := gui.NewTextInput()
		input.SetText(fmt.Sprintf("Retained text %d", serial))
		view.AppendPage(widgets.NewTabPage(fmt.Sprintf("Document %d", serial), input))
	}
	var build func(*widgets.TabView) (gui.Window, *widgets.TabBar, *frameProbe, *gui.Label, error)
	build = func(view *widgets.TabView) (gui.Window, *widgets.TabBar, *frameProbe, *gui.Label, error) {
		window, err := app.NewWindow(&gui.WindowOptions{Size: geometry.Size{Width: 500, Height: 320}, Chrome: chrome})
		if err != nil {
			return nil, nil, nil, nil, err
		}
		windows = append(windows, window)
		window.SetMinSize(geometry.Size{Width: 400, Height: 280})
		_ = window.SetTitle(fmt.Sprintf("GOUI TAB DETACH %d", len(windows)))
		bar := widgets.NewTabBar()
		bar.SetView(view)
		bar.SetReorderable(true)
		bar.SetTransferable(true)
		status := gui.NewLabel("Last tab: move to a window, not the desktop.")
		report := func(err error) {
			fmt.Println("ERROR:", err)
			if !status.Destroyed() {
				status.SetText(err.Error())
			}
		}
		bar.ConnectTransferError(report)
		add := gui.NewButton()
		add.SetChild(gui.NewLabel("Add tab"))
		add.ConnectClicked(func() { addPage(view) })
		content := new(frameProbe)
		content.SetLayoutManager(&layout.LinearLayout{Direction: layout.DirectionVertical, CrossAlign: layout.CrossStretch, Spacing: 8})
		var top gui.Widget = bar
		if chrome == gui.WindowChromeIntegrated {
			header := gui.NewHeaderBar()
			header.SetChild(bar)
			top = header
		}
		for _, child := range []gui.Widget{top, status, add, view} {
			content.WidgetBase.AddChild(content, child)
		}
		window.SetWidget(content)
		bar.ConnectDetachRequest(func(request *widgets.TabDetachRequest) {
			origin, err := window.Position(nil)
			if err != nil {
				request.Cancel()
				report(err)
				return
			}
			sourceScale := content.scale
			targetView := widgets.NewTabView()
			placeholder := widgets.NewTabPage(request.Page.Title(), nil)
			targetView.AppendPage(placeholder)
			target, targetBar, targetContent, targetStatus, err := build(targetView)
			if err != nil {
				request.Cancel()
				report(err)
				return
			}
			abort := func(err error) { request.Cancel(); target.Destroy(); report(err) }
			target.ConnectCloseRequest(func(*bool) { request.Cancel() })
			targetContent.after = func(scale float32) {
				if targetContent.Destroyed() {
					request.Cancel()
					return
				}
				if content.Destroyed() {
					abort(fmt.Errorf("source closed during preparation"))
					return
				}
				observed, err := window.Position(nil)
				if err != nil {
					abort(err)
					return
				}
				if observed != origin || content.scale != sourceScale {
					abort(fmt.Errorf("source moved during preparation"))
					return
				}
				frame, err := target.Position(target)
				if err != nil {
					abort(err)
					return
				}
				anchor, ok := firstTab(targetBar.Snapshot())
				if !ok || scale <= 0 || sourceScale <= 0 {
					abort(fmt.Errorf("missing target layout"))
					return
				}
				position := request.Position.Add(anchor.Add(request.Hotspot).Add(frame.Scale(-1)).Scale(-scale / sourceScale))
				if err := target.SetPosition(window, position); err != nil {
					abort(err)
					return
				}
				targetView.RemovePage(placeholder)
				if err := request.TransferTo(targetView, 0); err != nil {
					abort(err)
					return
				}
				targetStatus.SetText("Page retained. Move back or add another tab.")
				fmt.Println("DETACHED: original page transferred; continue or close windows")
			}
			if err := target.Show(); err != nil {
				abort(err)
			}
		})
		return window, bar, content, status, nil
	}
	view := widgets.NewTabView()
	addPage(view)
	addPage(view)
	window, _, _, _, err := build(view)
	if err != nil {
		return err
	}
	if err := window.Show(); err != nil {
		return err
	}
	app.Run()
	return nil
}
