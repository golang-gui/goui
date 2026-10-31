package main

import (
	"fmt"
	"time"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/example/demo/resource"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/mlang"
	"github.com/golang-gui/goui/theme/modern"
	"github.com/golang-gui/goui/ui"
)

// overlaysDemo owns this feature for the lifetime of the gallery.
// host is borrowed for shared assets and application operations.
type overlaysDemo struct {
	host       *demo
	panel      ui.State[bool]
	mask       ui.State[bool]
	toast      ui.State[string]
	toastTimer *ui.Timer
	popup      gui.Popover
}

func (d *overlaysDemo) View() ui.View { return ui.Build(d.view) }

func (d *overlaysDemo) view() ui.View {
	long := make([]*ui.MenuItemView, 45)
	for i := range long {
		long[i] = ui.MenuItem(fmt.Sprintf(resource.LongMenuItem(), i+1), func() { d.host.report(fmt.Sprintf(resource.MenuSelected(), i+1)) })
	}
	base := page("overlay-content", resource.Overlays(), resource.OverlaysHint(),
		section("PopoverMenu",
			ui.HWrap(ui.MenuButton(resource.ActionsMenu()).Menu(ui.MenuItem(resource.Execute(), func() { d.host.report(resource.MenuExecuted()) }), ui.MenuItem(resource.DisabledMenuItem(), nil).Enabled(false), ui.MenuSeparator(), ui.MenuItem(resource.ShowToast(), func() { d.showToast(resource.MenuToast()) })),
				ui.MenuButton(resource.LongMenu()).Menu(long...),
				ui.Button(resource.Popover()).ID("popover-anchor").OnClick(d.openPopover)).Spacing(12).LineSpacing(8)),
		section("Overlay",
			ui.HWrap(ui.Button(resource.SearchPanel()).ID("open-panel").OnClick(func() { d.panel.Set(!d.panel.Get()) }),
				ui.Button(resource.MaskDialog()).ID("open-dialog").OnClick(func() {
					if d.popup != nil {
						d.popup.Hide()
					}
					d.mask.Set(true)
					d.host.app.RequestUpdate()
				}),
				ui.Button(resource.ToastButton()).ID("open-toast").OnClick(func() { d.showToast(resource.ToastHint()) })).Spacing(12).LineSpacing(8),
			ui.TextInput().BindText(&d.host.input), ui.Button(resource.BackgroundAction()).OnClick(func() { d.host.report(resource.BackgroundClicked()) }),
			ui.Label(resource.MaskHint())),
		section(resource.ScrollableBackground(), ui.VBox(chipsCopy(25)...).Spacing(12)),
	)
	panel := surface(ui.VBox(ui.Label(resource.Search()).Style("demo-subheading"), ui.TextInput().BindText(&d.host.input), ui.Button(resource.ClosePanel()).OnClick(func() { d.panel.Set(false) })).Padding(16).Spacing(12).CrossAlign(layout.CrossStretch)).Style(modern.Card).MinWidth(300).MaxWidth(330).Visible(d.panel.Get())
	dialog := surface(ui.VBox(ui.Label(resource.ConfirmQuestion()).Style("demo-subheading"), ui.Label(resource.ConfirmHint()),
		ui.HBox(ui.Button(resource.Cancel()).OnClick(func() { d.mask.Set(false) }), ui.Button().Content(ui.Label(resource.Confirm()).Style(modern.PrimaryContent)).Style(modern.Primary).OnClick(func() { d.mask.Set(false); d.showToast(resource.Confirmed()) })).Spacing(12).MainAlign(layout.MainEnd)).Padding(24).Spacing(16).CrossAlign(layout.CrossStretch)).Style(modern.Card).MaxWidth(450)
	mask := ui.Overlay(surface(nil).Style("demo-mask"), ui.OverlayItem(dialog).OnPosition(center)).Visible(d.mask.Get())
	toast := surface(ui.HBox(ui.Label(d.toast.Get())).Padding(12)).Style(modern.Card).Visible(d.toast.Get() != "").MinHeight(48).MinWidth(280)
	return ui.Overlay(base,
		ui.OverlayItem(panel).OnPosition(func(a, s geometry.Size, p *geometry.Point) { *p = geometry.Point{X: max(0, a.Width-s.Width-20), Y: 20} }),
		ui.OverlayItem(mask).Fill(true),
		ui.OverlayItem(toast).OnPosition(func(a, s geometry.Size, p *geometry.Point) {
			*p = geometry.Point{X: max(0, (a.Width-s.Width)/2), Y: max(0, a.Height-s.Height-20)}
		}),
	).ID("page-overlays").MainWeight(1)
}
func (d *overlaysDemo) openPopover() {
	if d.popup != nil {
		d.popup.Destroy()
		d.popup = nil
	}
	anchor := d.host.app.FindWidget("main", "popover-anchor")
	if anchor == nil {
		return
	}
	p := gui.NewPopover(anchor, nil)
	box := gui.NewLinearBox(layout.DirectionVertical)
	box.SetPadding(geometry.UniformInsets(16))
	box.SetSpacing(12)
	title := gui.NewLabel(resource.Popover())
	description := gui.NewLabel(resource.PopoverHint())
	closeLabel := gui.NewLabel(resource.Close())
	box.AddChild(title)
	box.AddChild(description)
	button := gui.NewButton()
	button.SetChild(closeLabel)
	button.ConnectClicked(func() { p.Hide() })
	box.AddChild(button)
	p.SetWidget(box)
	p.SetPlacement(gui.PopoverPlacementBottom)
	p.ConnectDismissRequest(p.Hide)
	languageChanged := mlang.ConnectChanged(func() {
		title.SetText(resource.Popover())
		description.SetText(resource.PopoverHint())
		closeLabel.SetText(resource.Close())
	})
	// This example creates a new popover for each open. Closed/failed surfaces
	// must not keep a global language listener or touch destroyed labels.
	p.ConnectClosed(languageChanged.Disconnect)
	d.popup = p
	err := p.Show()
	if err != nil {
		languageChanged.Disconnect()
	}
	d.host.result(resource.ShowPopover(), err)
}

func (d *overlaysDemo) showToast(text string) {
	d.toast.Set(text)
	if d.toastTimer != nil {
		d.toastTimer.Stop()
	}
	d.toastTimer = d.host.app.TimeoutFunc(3*time.Second, func() { d.toast.Set(""); d.toastTimer = nil })
}

func newOverlaysDemo(host *demo) *overlaysDemo {
	return &overlaysDemo{host: host}
}
