package main

import (
	"encoding/json"
	"fmt"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/example/demo/resource"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/mlang"
	"github.com/golang-gui/goui/theme/modern"
	"github.com/golang-gui/goui/ui"
)

func (d *demo) windowPage() ui.View {
	return fullPage("windows", resource.WindowHeading(), resource.WindowHint(),
		ui.HWrap(ui.Button(resource.NativeWindow()).ID("native-window").OnClick(func() {
			d.openWindow(gui.WindowOptions{Chrome: gui.WindowChromeNative}, false)
		}),
			ui.Button(resource.IntegratedWindow()).OnClick(func() {
				d.openWindow(gui.WindowOptions{Chrome: gui.WindowChromeIntegrated}, false)
			}),
			ui.Button(resource.TransparentWindow()).ID("transparent-window").OnClick(func() {
				d.openWindow(gui.WindowOptions{Chrome: gui.WindowChromeNone, Transparent: true}, false)
			}),
			ui.Button(resource.SkipTaskbarWindow()).ID("skip-taskbar-window").OnClick(func() {
				d.openWindow(gui.WindowOptions{Chrome: gui.WindowChromeNative, SkipTaskbar: true}, false)
			}),
			ui.Button(resource.BorderBoxWindow()).ID("border-box-window").OnClick(func() {
				d.openWindow(gui.WindowOptions{Chrome: gui.WindowChromeNone, Transparent: true}, true)
			})).Spacing(12).LineSpacing(8),
		ui.HWrap(ui.Button(resource.Maximize()).OnClick(func() {
			d.app.FindWindow("main").RequestState(gui.WindowStateMaximized)
			d.report(resource.MaximizeRequested())
		}),
			ui.Button(resource.NormalState()).OnClick(func() {
				d.app.FindWindow("main").RequestState(gui.WindowStateNormal)
				d.report(resource.NormalRequested())
			}),
			ui.Button(resource.ReadPositionArea()).OnClick(d.inspectPosition), ui.Button(resource.ReadSnapshot()).ID("snapshot").OnClick(d.inspectSnapshot),
			ui.Button(resource.FindID()).OnClick(func() {
				w := d.app.FindWidget("main", "snapshot")
				if w != nil {
					d.report(fmt.Sprintf(resource.FoundWidget(), w, w.Snapshot().Role))
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
	d.result(resource.ReadSemantics(), err)
}
func (d *demo) inspectPosition() {
	w := d.app.FindWindow("main")
	pos, err := w.Position(nil)
	if err != nil {
		d.result(resource.ReadPosition(), err)
		return
	}
	area, err := w.WorkAreaAt(geometry.Point{})
	if err != nil {
		d.result(resource.ReadWorkArea(), err)
		return
	}
	d.report(fmt.Sprintf(resource.PositionFormat(), pos.X, pos.Y, area.Width, area.Height, w.State()))
}
func (d *demo) openWindow(options gui.WindowOptions, borderBox bool) {
	options.Size = geometry.Size{Width: 440, Height: 300}
	if options.SkipTaskbar || borderBox {
		options.Size = geometry.Size{Width: 500, Height: 380}
	}
	w, err := gui.App.NewWindow(&options)
	if err != nil {
		d.result(resource.CreateWindow(), err)
		return
	}
	if options.SkipTaskbar || borderBox {
		// These resizable examples intentionally scroll their prose. Do not
		// use its unbounded natural width as the window's minimum-size hint.
		w.SetMinSize(geometry.Size{Width: 280, Height: 220})
	}
	windowTitle := resource.AuxTitle
	if options.SkipTaskbar {
		windowTitle = resource.SkipTaskbarWindow
	} else if borderBox {
		windowTitle = resource.BorderBoxWindow
	}
	if err := w.SetTitle(windowTitle()); err != nil {
		w.Destroy()
		d.result(resource.SetTitle(), err)
		return
	}
	if options.Transparent {
		w.SetStyleName("demo-transparent-window")
	}
	box := gui.NewLinearBox(layout.DirectionVertical)
	box.SetSpacing(16)
	box.SetPadding(20)
	var labels []struct {
		widget *gui.Label
		text   mlang.Getter
	}
	label := func(text mlang.Getter) *gui.Label {
		widget := gui.NewLabel(text())
		widget.SetWrapMode(gui.WrapWordChar)
		labels = append(labels, struct {
			widget *gui.Label
			text   mlang.Getter
		}{widget, text})
		return widget
	}
	box.AddChild(label(resource.AuxDescription))
	box.AddChild(label(resource.AuxHint))
	if options.Transparent {
		box.AddChild(label(resource.TransparencyHint))
	}
	if options.SkipTaskbar {
		box.AddChild(label(resource.SkipTaskbarHint))
	}
	if borderBox {
		box.AddChild(label(resource.BorderBoxHint))
	}
	if options.SkipTaskbar || borderBox {
		box.AddChild(label(resource.AuxInputHint))
		// Window-local input stays alive when the application's declarations
		// rebuild. Translating labels must not replace user-owned text.
		input := gui.NewTextInput()
		box.AddChild(input)
	}
	move := gui.NewButton()
	move.SetChild(label(resource.MoveWindow))
	move.ConnectClicked(func() {
		d.result(resource.RelativePosition(), w.SetPosition(d.app.FindWindow("main"), geometry.Point{X: 240, Y: 160}))
	})
	box.AddChild(move)
	closeButton := gui.NewButton()
	closeButton.SetChild(label(resource.CloseWindow))
	closeButton.ConnectClicked(func() { w.Destroy() })
	box.AddChild(closeButton)
	header := gui.NewHeaderBar()
	title := resource.DragTitle
	if options.Chrome == gui.WindowChromeNative {
		title = resource.NativeDragTitle
	}
	header.SetChild(label(title))
	column := gui.NewLinearBox(layout.DirectionVertical)
	column.SetCrossAlign(layout.CrossStretch)
	if options.Transparent && !borderBox {
		column.SetPadding(12)
	} // 子树留在圆角内部，不假装背景绘制能够裁剪后续孩子。
	column.AddChild(header)
	// Keep descriptions and close controls reachable with long translations
	// or a resized window; HeaderBar remains outside the scrolling content.
	scroll := gui.NewScrollView()
	scroll.SetDirection(gui.ScrollVertical)
	scroll.SetMainWeight(1)
	scroll.SetChild(box)
	column.AddChild(scroll)
	w.SetWidget(auxWindowContent(column, options.Transparent, borderBox))
	w.ConnectState(func(state gui.WindowState) { d.report(fmt.Sprintf(resource.WindowState(), state)) })
	closed := false
	languageChanged := mlang.ConnectChanged(func() {
		if closed {
			return
		}
		if err := w.SetTitle(windowTitle()); err != nil {
			d.result(resource.SetTitle(), err)
		}
		for _, item := range labels {
			if closed {
				return
			}
			item.widget.SetText(item.text())
		}
	})
	w.ConnectDestroy(func() { closed = true; languageChanged.Disconnect() })
	d.app.RequestUpdate()
	// 先选定此窗口的背景样式、创建内容，再显示；不展示默认位置后再定位。
	d.app.Post(func() {
		if closed {
			return
		}
		positionErr := w.SetPosition(d.app.FindWindow("main"), geometry.Point{X: 180, Y: 120})
		if err := w.Show(); err != nil {
			w.Destroy()
			d.result(resource.ShowWindow(), err)
			return
		}
		if positionErr != nil {
			d.result(resource.PositionAfterShow(), positionErr)
		} else {
			d.report(resource.WindowShown())
		}
	})
}

// Window creation preferences and content composition are independent. The
// BorderBox must be the actual root, not wrapped inside the legacy transparent
// example's outer padding, or automatic resizing would intentionally be inactive.
func auxWindowContent(column gui.Widget, transparent, borderBox bool) gui.Widget {
	if borderBox {
		frame := gui.NewBorderBox()
		frame.SetStyleName(modern.Card) // Explicit demo appearance; the default stays invisible.
		frame.SetChild(column)
		return frame
	}
	panel := new(surfaceWidget)
	panel.SetStyleName(modern.Card)
	panel.SetLayoutManager(layout.NewFillLayout())
	panel.SetChild(column)
	if transparent {
		panel.SetMainWeight(1)
		outer := gui.NewLinearBox(layout.DirectionVertical)
		outer.SetPadding(16)
		outer.SetCrossAlign(layout.CrossStretch)
		outer.AddChild(panel)
		return outer
	}
	return panel
}
