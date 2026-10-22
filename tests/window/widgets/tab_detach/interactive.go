package main

import (
	"fmt"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/theme/modern"
	"github.com/golang-gui/goui/widgets"
)

// Continuous mode reports errors in the source window and terminal without
// exiting after another drag. New windows support adding/detaching pages too;
// a last page may move to an existing window but cannot create another window.
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
	var build func(*widgets.TabView, geometry.Size) (gui.Window, *widgets.TabBar, *frameProbe, *gui.Label, error)
	build = func(view *widgets.TabView, size geometry.Size) (gui.Window, *widgets.TabBar, *frameProbe, *gui.Label, error) {
		window, err := app.NewWindow(&gui.WindowOptions{Size: size, Chrome: chrome})
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
		content := &frameProbe{view: view, initialSize: size}
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
		bar.ConnectDetachRequest(func(request *widgets.TabDetachRequest, handled *bool) {
			origin, err := window.Position(nil)
			if err != nil {
				request.Cancel()
				report(err)
				return
			}
			targetView := widgets.NewTabView()
			target, _, targetContent, targetStatus, err := build(targetView, request.ContentSize.Add(content.overhead))
			if err != nil {
				request.Cancel()
				report(err)
				return
			}
			abort := func(err error) { request.Cancel(); target.Destroy(); report(err) }
			*handled = true
			target.ConnectCloseRequest(func(*bool) { request.Cancel() })
			prepare := func() {
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
				if observed != origin {
					abort(fmt.Errorf("source moved during preparation"))
					return
				}
				position := request.Position.Add(bar.Snapshot().Bounds.Pos.Add(request.Hotspot).Scale(-1))
				if err := target.SetPosition(window, position); err != nil {
					abort(err)
					return
				}
				if err := request.TransferTo(targetView, 0); err != nil {
					abort(err)
					return
				}
				targetStatus.SetText("Page retained. Move back or add another tab.")
				fmt.Println("DETACHED: original page transferred; continue or close windows")
				if err := target.Show(); err != nil {
					// Keep a target that already owns the original page. Return show failure
					// to the caller without destroying the editor content.
					report(err)
				}
			}
			prepare()
		})
		return window, bar, content, status, nil
	}
	view := widgets.NewTabView()
	addPage(view)
	addPage(view)
	window, _, _, _, err := build(view, geometry.Size{Width: 500, Height: 320})
	if err != nil {
		return err
	}
	if err := window.Show(); err != nil {
		return err
	}
	app.Run()
	return nil
}
