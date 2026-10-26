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
	tooltip := resource.TooltipText()
	if !d.enabled {
		tooltip = resource.TooltipDisabled()
	}
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
		section(resource.MenuButtons(),
			ui.Label(resource.MenuGroupHint()).WrapMode(gui.WrapWordChar).Style(modern.MutedText),
			ui.MenuButtonGroup("basic-menus", ui.HWrap(
				ui.MenuButton(resource.FileMenu()).ID("basic-menu-file").Group("basic-menus").Style(modern.Flat).Menu(
					ui.MenuItem(resource.OpenPath(), func() { d.report(resource.OpenPath()) }),
					ui.MenuItem(resource.MenuExecuted(), func() { d.report(resource.MenuExecuted()) }),
				).OnOpenError(func(err error) { d.result(resource.FileMenu(), err) }),
				ui.MenuButton(resource.EditMenu()).ID("basic-menu-edit").Group("basic-menus").Style(modern.Flat).Menu(
					ui.MenuItem(resource.Undo(), func() { d.report(resource.Undo()) }),
					ui.MenuItem(resource.Redo(), func() { d.report(resource.Redo()) }),
				).OnOpenError(func(err error) { d.result(resource.EditMenu(), err) }),
				ui.MenuButton(resource.ViewMenu()).ID("basic-menu-view").Group("basic-menus").Style(modern.Flat).Menu(
					ui.MenuItem(resource.MenuExecuted(), func() { d.report(resource.MenuExecuted()) }),
				).OnOpenError(func(err error) { d.result(resource.ViewMenu(), err) }),
				ui.Button(resource.RunCommand()).ID("basic-menu-run").OnClick(func() { d.report(resource.RunCommand()) }),
			).Spacing(8).LineSpacing(8).Enabled(d.enabled))),
		section(resource.Tooltips(),
			ui.Label(resource.TooltipHint()).WrapMode(gui.WrapWordChar).Style(modern.MutedText),
			ui.HWrap(
				ui.Button(resource.TooltipTarget()).ID("tooltip-target").Enabled(d.enabled).Tooltip(tooltip).OnClick(func() { d.report(resource.NormalClicked()) }),
				ui.Button(resource.AlwaysDisabled()).ID("tooltip-disabled").Enabled(false).Tooltip(resource.TooltipDisabled()),
			).Spacing(12).LineSpacing(12)),
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
			wui.CheckButtonGroup("basic-options", ui.HBox(wui.CheckButton(resource.OptionA()).Group("basic-options").Checked(d.radio == 0).OnChange(func(v wui.CheckState) {
				if v == wui.CheckChecked {
					d.radio = 0
					change()
				}
			}),
				wui.CheckButton(resource.OptionB()).Group("basic-options").Checked(d.radio == 1).OnChange(func(v wui.CheckState) {
					if v == wui.CheckChecked {
						d.radio = 1
						change()
					}
				})).Spacing(20).Enabled(d.enabled))),
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
