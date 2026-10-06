package ui

import (
	"github.com/golang-gui/goui/core/optional"
	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/gui"
	baseui "github.com/golang-gui/goui/ui"
	"github.com/golang-gui/goui/widgets"
)

type CheckState = widgets.CheckState
type CheckAppearance = widgets.CheckAppearance

const (
	CheckUnchecked           = widgets.CheckUnchecked
	CheckChecked             = widgets.CheckChecked
	CheckMixed               = widgets.CheckMixed
	CheckAppearanceIndicator = widgets.CheckAppearanceIndicator
	CheckAppearanceButton    = widgets.CheckAppearanceButton
)

// CheckButtonView declares selection; it does not implement another state
// machine. Keep a shared *widgets.CheckGroup outside the rebuilding function.
type CheckButtonView struct {
	baseui.ViewBase[CheckButtonView]
	child      baseui.View
	state      CheckState
	group      *widgets.CheckGroup
	appearance CheckAppearance
	disabled   bool
	padding    optional.Optional[float32]
	onChange   func(CheckState)
}

type checkButtonState struct {
	padding  float32
	onChange func(CheckState)
	change   signal.Handle
}

// CheckButton accepts at most one optional label. The label resolves its own
// check-button-text style; custom content never inherits its parent's style.
func CheckButton(text ...string) *CheckButtonView {
	if len(text) > 1 {
		panic("widgets/ui: CheckButton accepts at most one text argument")
	}
	v := &CheckButtonView{}
	v.Self = v
	if len(text) == 1 {
		v.Text(text[0])
	}
	return v
}

func (v *CheckButtonView) Text(text string) *CheckButtonView {
	return v.Content(baseui.Label(text).Style("check-button-text"))
}
func (v *CheckButtonView) Content(child baseui.View) *CheckButtonView { v.child = child; return v }
func (v *CheckButtonView) Child(child baseui.View) *CheckButtonView   { return v.Content(child) }

func (v *CheckButtonView) CheckState(state CheckState) *CheckButtonView {
	v.state = state
	return v
}
func (v *CheckButtonView) Checked(checked bool) *CheckButtonView {
	v.state = CheckUnchecked
	if checked {
		v.state = CheckChecked
	}
	return v
}
func (v *CheckButtonView) Group(group *widgets.CheckGroup) *CheckButtonView {
	v.group = group
	return v
}
func (v *CheckButtonView) Appearance(appearance CheckAppearance) *CheckButtonView {
	v.appearance = appearance
	return v
}
func (v *CheckButtonView) Enabled(enabled bool) *CheckButtonView { v.disabled = !enabled; return v }
func (v *CheckButtonView) Padding(padding float32) *CheckButtonView {
	v.padding.SetValue(padding)
	return v
}
func (v *CheckButtonView) OnChange(fn func(CheckState)) *CheckButtonView {
	v.onChange = fn
	return v
}
func (v *CheckButtonView) Build() baseui.View { return v }

func (v *CheckButtonView) Mount(ctx baseui.BuildContext) gui.Widget {
	b := widgets.NewCheckButton()
	s := &checkButtonState{padding: b.Padding()}
	s.change = b.ConnectChange(func(state CheckState) {
		if s.onChange != nil {
			s.onChange(state)
		}
	})
	ctx.SetState(s)
	return b
}

func (v *CheckButtonView) Update(ctx baseui.BuildContext, widget gui.Widget) {
	b := widget.(*widgets.CheckButton)
	s := ctx.State().(*checkButtonState)
	s.onChange = v.onChange
	b.SetGroup(v.group)
	b.SetCheckState(v.state)
	b.SetAppearance(v.appearance)
	b.SetEnabled(!v.disabled)
	b.SetPadding(v.padding.ValueOr(s.padding))
	ctx.UpdateChild(b, v.child)
}

func (v *CheckButtonView) Unmount(ctx baseui.BuildContext, _ gui.Widget) {
	if s, ok := ctx.State().(*checkButtonState); ok {
		s.change.Disconnect()
	}
}

var _ baseui.WidgetView = (*CheckButtonView)(nil)
