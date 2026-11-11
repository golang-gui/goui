package main

import (
	"fmt"
	"slices"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/theme/modern"
	"github.com/golang-gui/goui/widgets"
)

// 转移源的 Current 信号在同步 Drop 尚未返回时销毁空窗口，覆盖最后一页的收尾顺序。
func runSourceCloseRepeat(chrome gui.WindowChromeMode, nativeSourceClose bool) error {
	app, err := gui.NewApplication("org.golang-gui.TabSourceCloseProbe")
	if err != nil {
		return err
	}
	app.SetStyleSheet(modern.Sheet(modern.Options{}))
	var windows []gui.Window
	defer func() {
		for _, w := range windows {
			w.Destroy()
		}
	}()
	var failure error
	verified := false
	fail := func(err error) {
		if failure == nil {
			failure = err
		}
		app.Post(app.Quit)
	}
	input := gui.NewTextInput()
	input.SetText("retained A")
	a := widgets.NewTabPage("Document A", input)
	b := widgets.NewTabPage("Document B", gui.NewLabel("B"))
	c := widgets.NewTabPage("Document C", gui.NewLabel("C"))
	mounts, unmounts, transfers, closed, targets := 0, 0, 0, 0, 0
	input.ConnectMount(func() { mounts++ })
	input.ConnectUnmount(func() { unmounts++ })
	main := widgets.NewTabView()
	main.AppendPage(b)
	main.AppendPage(c)
	var target *widgets.TabView
	var build func(string, *widgets.TabView, bool) (gui.Window, error)
	build = func(title string, view *widgets.TabView, closeEmpty bool) (gui.Window, error) {
		w, err := app.NewWindow(&gui.WindowOptions{Size: geometry.Size{Width: 500, Height: 320}, Chrome: chrome})
		if err != nil {
			return nil, err
		}
		windows = append(windows, w)
		if err := w.SetTitle(title); err != nil {
			return nil, err
		}
		w.ConnectCloseRequest(func(*bool) { fail(fmt.Errorf("window closed before verification: %s", title)) })
		bar := widgets.NewTabBar()
		bar.SetView(view)
		bar.SetReorderable(true)
		bar.SetTransferable(true)
		bar.ConnectTransferError(func(err error) { fail(fmt.Errorf("%s transfer: %w", title, err)) })
		// 只有实际转移到本 View 后的 Current 才计数；初始装配不触发。
		view.ConnectCurrent(func(page *widgets.TabPage) {
			if page == a {
				transfers++
				fmt.Printf("TRANSFER %d into %s\n", transfers, title)
			}
			if closeEmpty && len(view.Pages()) == 0 {
				closed++
				fmt.Printf("CLOSE %d %s during Drop\n", closed, title)
				if nativeSourceClose {
					w.PlatformWindow().Destroy()
				} else {
					w.Destroy()
				}
			}
		})
		box := gui.NewLinearBox(layout.DirectionVertical)
		box.SetCrossAlign(layout.CrossStretch)
		if chrome == gui.WindowChromeIntegrated {
			header := gui.NewHeaderBar()
			header.SetChild(bar)
			box.AddChild(header)
		} else {
			box.AddChild(bar)
		}
		if !closeEmpty {
			create := gui.NewButton()
			create.SetChild(gui.NewLabel("New target"))
			create.ConnectClicked(func() {
				if target != nil && len(target.Pages()) != 0 {
					fail(fmt.Errorf("previous target still owns its page"))
					return
				}
				targets++
				target = widgets.NewTabView()
				next, err := build(fmt.Sprintf("GOUI SESSION TARGET %d", targets), target, true)
				if err == nil {
					err = next.Show()
				}
				if err != nil {
					fail(err)
				}
			})
			verify := gui.NewButton()
			verify.SetChild(gui.NewLabel("Verify and exit"))
			verify.ConnectClicked(func() {
				if transfers != 5 || closed != 3 || targets != 2 || mounts != 6 || unmounts != 5 ||
					!slices.Equal(main.Pages(), []*widgets.TabPage{b, c, a}) || main.Current() != a ||
					a.Child() != input || input.Text() != "retained A" || a.Destroyed() || input.Destroyed() || a.Root() != w {
					fail(fmt.Errorf("transfers=%d closed=%d targets=%d mounts/unmounts=%d/%d pages=%d text=%q",
						transfers, closed, targets, mounts, unmounts, len(main.Pages()), input.Text()))
					return
				}
				verified = true
				fmt.Println("PASS five transfers, three source closes, retained page/input and lifecycle")
				app.Quit()
			})
			box.AddChild(create)
			box.AddChild(verify)
		}
		box.AddChild(view)
		w.SetWidget(box)
		return w, nil
	}
	mainWindow, err := build("GOUI SESSION MAIN", main, false)
	if err != nil {
		return err
	}
	detached := widgets.NewTabView()
	detached.AppendPage(a)
	detachedWindow, err := build("GOUI SESSION DETACHED", detached, true)
	if err != nil {
		return err
	}
	for _, w := range []gui.Window{mainWindow, detachedWindow} {
		if err := w.Show(); err != nil {
			return err
		}
	}
	app.Run()
	if failure != nil {
		return failure
	}
	if !verified {
		return fmt.Errorf("continuous transfer scenario was not verified")
	}
	return nil
}
