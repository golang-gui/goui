package main

import (
	"fmt"
	"time"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/example/demo/resource"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/gui/textedit"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/theme/modern"
	"github.com/golang-gui/goui/ui"
	wui "github.com/golang-gui/goui/widgets/ui"
	"github.com/rivo/uniseg"
)

// textInputs owns independent form values. Password text is never reported to
// the gallery status, copied into the preview, or included in derived labels.
type textInputs struct {
	username, digits, password, note ui.State[string]
	reveal, brief, star              ui.State[bool]
	usernameFilter, digitsFilter     gui.InputFilter
	passwordFilter, noteFilter       gui.InputFilter
}

func newTextInputs() *textInputs {
	return &textInputs{
		brief:          ui.MakeState(true),
		usernameFilter: textedit.ASCIIUsername(16),
		digitsFilter:   textedit.Chain(textedit.DigitsOnly(), textedit.LimitLength(6)),
		passwordFilter: textedit.Password(128),
		noteFilter:     textedit.LimitLength(80),
	}
}

func (d *textInputs) View() ui.View { return ui.Build(d.view) }

func (d *textInputs) view() ui.View {
	char, delay := '•', time.Duration(0)
	if d.star.Get() {
		char = '*'
	}
	if d.brief.Get() {
		delay = 600 * time.Millisecond
	}
	hint := func(text string) ui.View { return ui.Label(text).WrapMode(gui.WrapWordChar).Style(modern.MutedText) }
	return ui.VBox(
		ui.Label(resource.InputPolicies()).Style("demo-subheading"),
		hint(resource.InputPoliciesHint()),
		ui.Label(resource.Username()),
		ui.TextInput().ID("input-username").BindText(&d.username).InputFilter(d.usernameFilter).Placeholder(resource.UsernamePolicy()),
		hint(resource.UsernamePolicy()),
		ui.Label(resource.DigitsInput()),
		ui.TextInput().ID("input-digits").BindText(&d.digits).InputFilter(d.digitsFilter).Placeholder("012345"),
		hint(resource.DigitsPolicy()),
		ui.Label(resource.PasswordInput()),
		ui.TextInput().ID("input-password").BindText(&d.password).InputFilter(d.passwordFilter).
			Obscured(!d.reveal.Get()).ObscureChar(char).ObscureDelay(delay),
		wui.CheckButton(resource.ShowPassword()).ID("password-reveal").Checked(d.reveal.Get()).OnChange(func(v wui.CheckState) { d.reveal.Set(v == wui.CheckChecked) }),
		wui.CheckButton(resource.BriefReveal()).ID("password-brief").Checked(d.brief.Get()).OnChange(func(v wui.CheckState) { d.brief.Set(v == wui.CheckChecked) }),
		wui.CheckButton(resource.StarMask()).ID("password-star").Checked(d.star.Get()).OnChange(func(v wui.CheckState) { d.star.Set(v == wui.CheckChecked) }),
		hint(resource.PasswordPolicy()),
		ui.Label(resource.LimitedNote()),
		ui.ScrollView(ui.TextView().ID("input-note").BindText(&d.note).InputFilter(d.noteFilter).WrapMode(ui.WrapWordChar)).
			Style("text-view").MinHeight(90).MaxHeight(90),
		ui.Build(func() ui.View {
			return hint(fmt.Sprintf(resource.NoteLength(), uniseg.GraphemeClusterCount(d.note.Get())))
		}),
		ui.Button(resource.ClearInputs()).ID("clear-inputs").OnClick(d.clear),
	).Spacing(8).CrossAlign(layout.CrossStretch).Paddings(geometry.Insets{Right: 8})
}

func (d *textInputs) clear() {
	d.username.Set("")
	d.digits.Set("")
	d.password.Set("")
	d.note.Set("")
}
