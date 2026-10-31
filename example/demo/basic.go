package main

import (
	"fmt"

	"github.com/golang-gui/goui/example/demo/resource"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/theme/modern"
	"github.com/golang-gui/goui/ui"
	"github.com/golang-gui/goui/widgets"
	wui "github.com/golang-gui/goui/widgets/ui"
)

// basicDemo owns this feature for the lifetime of the gallery.
// host is borrowed for shared assets and application operations.
type basicDemo struct {
	host          *demo
	enabled       ui.State[bool]
	checked       ui.State[bool]
	switched      ui.State[bool]
	toggle        ui.State[bool]
	mixed         ui.State[wui.CheckState]
	radio         ui.State[int]
	drop          ui.State[int]
	fancyDrop     ui.State[int]
	choices       *gui.SliceListModel[widgets.DropDownItem]
	baselineInput sampleInput
}

func (d *basicDemo) View() ui.View { return ui.Build(d.view) }

func (d *basicDemo) view() ui.View {
	tooltip := resource.TooltipText()
	if !d.enabled.Get() {
		tooltip = resource.TooltipDisabled()
	}
	return page("basic", resource.Basic(), resource.BasicHint(),
		section(resource.Buttons(),
			wui.Switch(resource.EnableControls()).Checked(d.enabled.Get()).OnChange(func(v bool) { d.enabled.Set(v) }),
			ui.HWrap(ui.Button(resource.NormalButton()).OnClick(func() { d.host.report(resource.NormalClicked()) }),
				ui.Button().Content(ui.Label(resource.PrimaryButton()).Style(modern.PrimaryContent)).Style(modern.Primary).OnClick(func() { d.host.report(resource.PrimaryClicked()) }),
				ui.Button(resource.FlatButton()).Style(modern.Flat).ID("basic-flat").OnClick(func() { d.host.report(resource.FlatClicked()) }),
				ui.Button().Content(ui.HBox(ui.Icon(d.host.logo).Size(18), ui.Label(resource.IconButton())).Spacing(8)).OnClick(func() { d.host.report(resource.IconClicked()) }),
			).Spacing(12).LineSpacing(12).Enabled(d.enabled.Get()),
			ui.HWrap(ui.Button(resource.AlwaysDisabled()).Enabled(false),
				ui.Button(resource.FlatButton()).Style(modern.Flat).ID("basic-flat-disabled").Enabled(false)).Spacing(12).LineSpacing(12),
			ui.ButtonGroup(
				ui.Button(resource.Undo()).OnClick(func() { d.host.report(resource.Undo()) }),
				ui.Button(resource.Redo()).OnClick(func() { d.host.report(resource.Redo()) }),
				ui.MenuButton(resource.FileMenu()).Menu(ui.MenuItem(resource.OpenPath(), func() { d.host.report(resource.OpenPath()) })).OnOpenError(func(err error) { d.host.result(resource.FileMenu(), err) }),
			).Enabled(d.enabled.Get())),
		section(resource.MenuButtons(),
			ui.Label(resource.MenuGroupHint()).WrapMode(gui.WrapWordChar).Style(modern.MutedText),
			ui.MenuButtonGroup(ui.HWrap(
				ui.MenuButton(resource.FileMenu()).ID("basic-menu-file").Style(modern.Flat).Menu(
					ui.MenuItem(resource.OpenPath(), func() { d.host.report(resource.OpenPath()) }),
					ui.MenuItem(resource.MenuExecuted(), func() { d.host.report(resource.MenuExecuted()) }),
				).OnOpenError(func(err error) { d.host.result(resource.FileMenu(), err) }),
				ui.MenuButton(resource.EditMenu()).ID("basic-menu-edit").Style(modern.Flat).Menu(
					ui.MenuItem(resource.Undo(), func() { d.host.report(resource.Undo()) }),
					ui.MenuItem(resource.Redo(), func() { d.host.report(resource.Redo()) }),
				).OnOpenError(func(err error) { d.host.result(resource.EditMenu(), err) }),
				ui.MenuButton(resource.ViewMenu()).ID("basic-menu-view").Style(modern.Flat).Menu(
					ui.MenuItem(resource.MenuExecuted(), func() { d.host.report(resource.MenuExecuted()) }),
				).OnOpenError(func(err error) { d.host.result(resource.ViewMenu(), err) }),
				ui.Button(resource.RunCommand()).ID("basic-menu-run").OnClick(func() { d.host.report(resource.RunCommand()) }),
			).Spacing(8).LineSpacing(8).Enabled(d.enabled.Get()))),
		section(resource.Tooltips(),
			ui.Label(resource.TooltipHint()).WrapMode(gui.WrapWordChar).Style(modern.MutedText),
			ui.HWrap(
				ui.Button(resource.TooltipTarget()).ID("tooltip-target").Enabled(d.enabled.Get()).Tooltip(tooltip).OnClick(func() { d.host.report(resource.NormalClicked()) }),
				ui.Button(resource.AlwaysDisabled()).ID("tooltip-disabled").Enabled(false).Tooltip(resource.TooltipDisabled()),
			).Spacing(12).LineSpacing(12)),
		section(resource.SemanticStyles(),
			ui.Label(resource.SemanticHint()).WrapMode(gui.WrapWordChar).Style(modern.MutedText),
			ui.HWrap(
				ui.HBox(ui.Icon(d.host.icons[0]).Size(20).Style(modern.Warning), ui.Label(resource.WarningText()).Style(modern.Warning)).Spacing(8),
				ui.HBox(ui.Icon(d.host.icons[0]).Size(20).Style(modern.Error), ui.Label(resource.ErrorText()).Style(modern.Error)).Spacing(8),
				ui.HBox(ui.Icon(d.host.icons[0]).Size(20).Style(modern.Success), ui.Label(resource.SuccessText()).Style(modern.Success)).Spacing(8),
			).ID("basic-semantic-colors").Spacing(24).LineSpacing(12)),
		section(resource.Choices(),
			ui.HWrap(
				wui.CheckButton(resource.Checkbox()).Checked(d.checked.Get()).OnChange(func(v wui.CheckState) { d.checked.Set(v == wui.CheckChecked) }),
				wui.CheckButton(resource.Mixed()).CheckState(d.mixed.Get()).OnChange(func(v wui.CheckState) { d.mixed.Set(v) }),
				wui.CheckButton(resource.Toggle()).Appearance(wui.CheckAppearanceButton).Checked(d.toggle.Get()).OnChange(func(v wui.CheckState) { d.toggle.Set(v == wui.CheckChecked) }),
				wui.Switch(resource.Switch()).Checked(d.switched.Get()).OnChange(func(v bool) { d.switched.Set(v) }),
			).Spacing(18).LineSpacing(12).Enabled(d.enabled.Get()),
			wui.CheckButtonGroup(ui.HBox(wui.CheckButton(resource.OptionA()).Checked(d.radio.Get() == 0).OnChange(func(v wui.CheckState) {
				if v == wui.CheckChecked {
					d.radio.Set(0)
				}
			}),
				wui.CheckButton(resource.OptionB()).Checked(d.radio.Get() == 1).OnChange(func(v wui.CheckState) {
					if v == wui.CheckChecked {
						d.radio.Set(1)
					}
				})).Spacing(20).Enabled(d.enabled.Get()))),
		section(resource.DropDownTitle(),
			wui.DropDown(d.choices).ID("basic-dropdown").Selected(d.drop.Get()).OnSelected(func(i int) { d.drop.Set(i); d.host.report(fmt.Sprintf(resource.SelectedItemFormat(), i+1)) }).OnOpenError(func(e error) { d.host.result(resource.OpenDropDown(), e) }).Enabled(d.enabled.Get()),
			wui.DropDown(d.choices).Selected(d.fancyDrop.Get()).Item(func(i int, item wui.DropDownItem) ui.View {
				return ui.HBox(ui.Icon(d.host.icons[i%len(d.host.icons)]).Size(22), ui.Label(item.Text)).Spacing(12).Padding(6)
			}).
				OnSelected(func(i int) { d.fancyDrop.Set(i) }).OnOpenError(func(e error) { d.host.result(resource.OpenCustomDropDown(), e) }).Enabled(d.enabled.Get()),
			ui.Label(resource.DisabledHint()).Style(modern.MutedText)),
		section(resource.TextLayout(), ui.HBox(ui.Label("Baseline"), ui.TextInput().ID("baseline-input").BindText(&d.baselineInput).MinWidth(240)).CrossAlign(layout.CrossBaseline).Spacing(12),
			ui.Label(resource.LabelHint()),
			ui.Label(resource.FormattedLabel()).ID("formatted-label").Selectable(true).WrapMode(gui.WrapWordChar).OnCopyError(func(err error) { d.host.result(resource.CopyText(), err) }),
			ui.Label(resource.LabelSelectHint()).WrapMode(gui.WrapWordChar).Style(modern.MutedText),
			ui.TextInput().ID("binding-input").BindText(&d.host.input),
			ui.Label().ID("binding-preview").BindText(&d.host.input).Selectable(true).WrapMode(gui.WrapWordChar),
		),
	)
}

func newBasicDemo(host *demo) *basicDemo {
	d := &basicDemo{host: host, enabled: ui.MakeState(true), mixed: ui.MakeState(wui.CheckMixed), baselineInput: newSampleInput(resource.BaselineSample())}
	d.choices = gui.NewSliceListModel([]widgets.DropDownItem{{Text: resource.DefaultOption()}, {Text: resource.CustomOption()}, {Text: resource.DisabledOption(), Disabled: true}, {Text: resource.LastOption()}})
	return d
}
