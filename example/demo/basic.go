package main

import (
	"fmt"

	"github.com/golang-gui/goui/example/demo/resource"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/theme/modern"
	"github.com/golang-gui/goui/ui"
	wui "github.com/golang-gui/goui/widgets/ui"
)

func (d *demo) basicPage() ui.View {
	change := func() { d.app.RequestUpdate() }
	return page("basic", resource.Basic(), resource.BasicHint(),
		section(resource.Buttons(),
			wui.Switch(resource.EnableControls()).Checked(d.enabled).OnChange(func(v bool) { d.enabled = v; change() }),
			ui.HWrap(ui.Button(resource.NormalButton()).OnClick(func() { d.report(resource.NormalClicked()) }),
				ui.Button().Content(ui.Label(resource.PrimaryButton()).Style(modern.PrimaryContent)).Style(modern.Primary).OnClick(func() { d.report(resource.PrimaryClicked()) }),
				ui.Button(resource.FlatButton()).Style(modern.Flat).ID("basic-flat").OnClick(func() { d.report(resource.FlatClicked()) }),
				ui.Button().Content(ui.HBox(ui.Icon(d.logo).Size(18), ui.Label(resource.IconButton())).Spacing(8)).OnClick(func() { d.report(resource.IconClicked()) }),
			).Spacing(12).LineSpacing(12).Enabled(d.enabled),
			ui.HWrap(ui.Button(resource.AlwaysDisabled()).Enabled(false),
				ui.Button(resource.FlatButton()).Style(modern.Flat).ID("basic-flat-disabled").Enabled(false)).Spacing(12).LineSpacing(12)),
		section(resource.SemanticStyles(),
			ui.Label(resource.SemanticHint()).WrapMode(gui.WrapWordChar).Style(modern.MutedText),
			ui.HWrap(
				ui.HBox(ui.Icon(d.icons[0]).Size(20).Style(modern.Warning), ui.Label(resource.WarningText()).Style(modern.Warning)).Spacing(8),
				ui.HBox(ui.Icon(d.icons[0]).Size(20).Style(modern.Error), ui.Label(resource.ErrorText()).Style(modern.Error)).Spacing(8),
				ui.HBox(ui.Icon(d.icons[0]).Size(20).Style(modern.Success), ui.Label(resource.SuccessText()).Style(modern.Success)).Spacing(8),
			).ID("basic-semantic-colors").Spacing(24).LineSpacing(12)),
		section(resource.Choices(),
			ui.HWrap(
				wui.CheckButton(resource.Checkbox()).Checked(d.checked).OnChange(func(v wui.CheckState) { d.checked = v == wui.CheckChecked; change() }),
				wui.CheckButton(resource.Mixed()).CheckState(d.mixed).OnChange(func(v wui.CheckState) { d.mixed = v; change() }),
				wui.CheckButton(resource.Toggle()).Appearance(wui.CheckAppearanceButton).Checked(d.toggle).OnChange(func(v wui.CheckState) { d.toggle = v == wui.CheckChecked; change() }),
				wui.Switch(resource.Switch()).Checked(d.switched).OnChange(func(v bool) { d.switched = v; change() }),
			).Spacing(18).LineSpacing(12).Enabled(d.enabled),
			ui.HBox(wui.CheckButton(resource.OptionA()).Group(d.group).Checked(d.radio == 0).OnChange(func(v wui.CheckState) {
				if v == wui.CheckChecked {
					d.radio = 0
					change()
				}
			}),
				wui.CheckButton(resource.OptionB()).Group(d.group).Checked(d.radio == 1).OnChange(func(v wui.CheckState) {
					if v == wui.CheckChecked {
						d.radio = 1
						change()
					}
				})).Spacing(20).Enabled(d.enabled)),
		section(resource.DropDownTitle(),
			wui.DropDown(d.choices).ID("basic-dropdown").Selected(d.drop).OnSelected(func(i int) { d.drop = i; d.report(fmt.Sprintf(resource.SelectedItemFormat(), i+1)) }).OnOpenError(func(e error) { d.result(resource.OpenDropDown(), e) }).Enabled(d.enabled),
			wui.DropDown(d.choices).Selected(d.fancyDrop).Item(func(i int, item wui.DropDownItem) ui.View {
				return ui.HBox(ui.Icon(d.icons[i%len(d.icons)]).Size(22), ui.Label(item.Text)).Spacing(12).Padding(6)
			}).
				OnSelected(func(i int) { d.fancyDrop = i; change() }).OnOpenError(func(e error) { d.result(resource.OpenCustomDropDown(), e) }).Enabled(d.enabled),
			ui.Label(resource.DisabledHint()).Style(modern.MutedText)),
		section(resource.TextLayout(), ui.HBox(ui.Label("Baseline"), ui.TextInput().ID("baseline-input").BindText(&d.baselineInput).MinWidth(240)).CrossAlign(layout.CrossBaseline).Spacing(12),
			ui.Label(resource.LabelHint())),
	)
}
