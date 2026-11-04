package main

import (
	"encoding/json"
	"fmt"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/example/demo/resource"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/gui/diagnostics"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/mlang"
	"github.com/golang-gui/goui/theme/modern"
	"github.com/golang-gui/goui/ui"
)

// windowsDemo owns this feature for the lifetime of the gallery.
// host is borrowed for shared assets and application operations.
type windowsDemo struct {
	host               *demo
	inspection         *ui.TextModel
	inspectionRevision uint64
	latest             gui.Window
	capture            *diagnostics.Capture
	hud                *diagnostics.HUD
	hudVisible         ui.State[bool]
	capturing          ui.State[bool]
	captureWidgets     ui.State[bool]
	captureRevision    ui.State[int]
	captureResult      diagnostics.Snapshot
}

func (d *windowsDemo) View() ui.View { return ui.Build(d.view) }

func (d *windowsDemo) view() ui.View {
	return fullPage("windows", resource.WindowHeading(), resource.WindowHint(),
		ui.ScrollView(d.inspectorView()).ID("inspection-scroll").Direction(ui.ScrollVertical).MainWeight(1),
	)
}

// Scroll the page on short windows and bound the nested reader: a large JSON
// snapshot should not become the page's unbounded natural height.
func (d *windowsDemo) inspectorView() ui.View {
	return ui.VBox(
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
			d.host.app.FindWindow("main").RequestState(gui.WindowStateMaximized)
			d.host.report(resource.MaximizeRequested())
		}),
			ui.Button(resource.NormalState()).OnClick(func() {
				d.host.app.FindWindow("main").RequestState(gui.WindowStateNormal)
				d.host.report(resource.NormalRequested())
			}),
			ui.Button(resource.ShowExisting()).ID("show-existing").OnClick(d.showExisting),
			ui.Button(resource.ReadPositionArea()).OnClick(d.inspectPosition), ui.Button(resource.ReadSnapshot()).ID("snapshot").OnClick(d.inspectSnapshot),
			ui.Button(resource.FindID()).OnClick(func() {
				w := d.host.app.FindWidget("main", "snapshot")
				if w != nil {
					d.host.report(fmt.Sprintf(resource.FoundWidget(), w, w.Snapshot().Role))
				}
			})).Spacing(10).LineSpacing(8),
		d.diagnosticsView(),
		ui.ScrollView(ui.TextView().ID("snapshot-text").Model(d.inspection).ReadOnly(true).WrapMode(ui.WrapWordChar).Padding(12)).
			Style("text-view").MinHeight(320).MaxHeight(320),
	).Spacing(12).CrossAlign(layout.CrossStretch)
}
func (d *windowsDemo) inspectSnapshot() {
	w := d.host.app.FindWindow("main")
	if w == nil {
		return
	}
	data, err := json.MarshalIndent(w.Snapshot(), "", "  ")
	if err == nil {
		d.inspection.SetText(string(data))
	}
	d.host.result(resource.ReadSemantics(), err)
}
func (d *windowsDemo) inspectPosition() {
	w := d.host.app.FindWindow("main")
	pos, err := w.Position(nil)
	if err != nil {
		d.host.result(resource.ReadPosition(), err)
		return
	}
	area, err := w.WorkAreaAt(geometry.Point{})
	if err != nil {
		d.host.result(resource.ReadWorkArea(), err)
		return
	}
	d.host.report(fmt.Sprintf(resource.PositionFormat(), pos.X, pos.Y, area.Width, area.Height, w.State()))
}
func (d *windowsDemo) openWindow(options gui.WindowOptions, borderBox bool) {
	options.Size = geometry.Size{Width: 440, Height: 300}
	if options.SkipTaskbar || borderBox {
		options.Size = geometry.Size{Width: 500, Height: 380}
	}
	w, err := gui.App.NewWindow(&options)
	if err != nil {
		d.host.result(resource.CreateWindow(), err)
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
		d.host.result(resource.SetTitle(), err)
		return
	}
	if options.Transparent {
		w.SetStyleName("demo-transparent-window")
	}
	box := gui.NewLinearBox(layout.DirectionVertical)
	box.SetSpacing(16)
	box.SetPadding(geometry.UniformInsets(20))
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
		d.host.result(resource.RelativePosition(), w.SetPosition(d.host.app.FindWindow("main"), geometry.Point{X: 240, Y: 160}))
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
		column.SetPadding(geometry.UniformInsets(12))
	} // Keep the subtree inside the rounded corners. Painting a rounded background
	// does not clip children painted later.
	column.AddChild(header)
	// Keep descriptions and close controls reachable with long translations
	// or a resized window; HeaderBar remains outside the scrolling content.
	scroll := gui.NewScrollView()
	scroll.SetDirection(gui.ScrollVertical)
	scroll.SetMainWeight(1)
	scroll.SetChild(box)
	column.AddChild(scroll)
	w.SetWidget(auxWindowContent(column, options.Transparent, borderBox))
	w.ConnectState(func(state gui.WindowState) { d.host.report(fmt.Sprintf(resource.WindowState(), state)) })
	closed := false
	languageChanged := mlang.ConnectChanged(func() {
		if closed {
			return
		}
		if err := w.SetTitle(windowTitle()); err != nil {
			d.host.result(resource.SetTitle(), err)
		}
		for _, item := range labels {
			if closed {
				return
			}
			item.widget.SetText(item.text())
		}
	})
	w.ConnectDestroy(func() {
		closed = true
		languageChanged.Disconnect()
		if d.latest == w {
			d.latest = nil
		}
	})
	// Choose this window's background style and construct its content before
	// showing it, so the initial position is applied before it becomes visible.
	d.latest = w
	d.host.app.Post(func() {
		if closed {
			return
		}
		positionErr := w.SetPosition(d.host.app.FindWindow("main"), geometry.Point{X: 180, Y: 120})
		if err := w.Show(); err != nil {
			w.Destroy()
			d.host.result(resource.ShowWindow(), err)
			return
		}
		if positionErr != nil {
			d.host.result(resource.PositionAfterShow(), positionErr)
		} else {
			d.host.report(resource.WindowShown())
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
		outer.SetPadding(geometry.UniformInsets(16))
		outer.SetCrossAlign(layout.CrossStretch)
		outer.AddChild(panel)
		return outer
	}
	return panel
}

func newWindowsDemo(host *demo) *windowsDemo {
	return &windowsDemo{host: host, inspection: ui.NewTextModel(resource.InspectionSample())}
}

func (d *windowsDemo) showExisting() {
	w := d.latest
	if w == nil {
		w = d.host.app.FindWindow("main")
	}
	if w == nil {
		return
	}
	if err := w.Show(); err != nil {
		d.host.result(resource.ShowExisting(), err)
	} else {
		d.host.report(resource.ExistingShown())
	}
}
