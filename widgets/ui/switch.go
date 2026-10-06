package ui

import (
	"github.com/golang-gui/goui/core/optional"
	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/gui"
	baseui "github.com/golang-gui/goui/ui"
	"github.com/golang-gui/goui/widgets"
)

// SwitchView declares a two-state switch; input and animation belong to widgets.Switch.
type SwitchView struct {
	baseui.ViewBase[SwitchView]
	child    baseui.View
	checked  bool
	padding  optional.Optional[float32]
	animated optional.Optional[bool]
	onChange func(bool)
}

type switchState struct {
	padding  float32
	animated bool
	onChange func(bool)
	change   signal.Handle
}

// Switch accepts at most one optional label, using its own switch-text style.
func Switch(text ...string) *SwitchView {
	if len(text) > 1 {
		panic("widgets/ui: Switch accepts at most one text argument")
	}
	v := &SwitchView{}
	v.Self = v
	if len(text) == 1 {
		v.Text(text[0])
	}
	return v
}

func (v *SwitchView) Text(text string) *SwitchView {
	return v.Content(baseui.Label(text).Style("switch-text"))
}
func (v *SwitchView) Content(child baseui.View) *SwitchView { v.child = child; return v }
func (v *SwitchView) Child(child baseui.View) *SwitchView   { return v.Content(child) }
func (v *SwitchView) Checked(checked bool) *SwitchView      { v.checked = checked; return v }
func (v *SwitchView) Padding(padding float32) *SwitchView   { v.padding.SetValue(padding); return v }
func (v *SwitchView) Animated(animated bool) *SwitchView    { v.animated.SetValue(animated); return v }
func (v *SwitchView) OnChange(fn func(bool)) *SwitchView    { v.onChange = fn; return v }
func (v *SwitchView) Build() baseui.View                    { return v }

func (v *SwitchView) Mount(ctx baseui.BuildContext) gui.Widget {
	b := widgets.NewSwitch()
	s := &switchState{padding: b.Padding(), animated: b.Animated()}
	s.change = b.ConnectChange(func(checked bool) {
		if s.onChange != nil {
			s.onChange(checked)
		}
	})
	ctx.SetState(s)
	return b
}

func (v *SwitchView) Update(ctx baseui.BuildContext, widget gui.Widget) {
	b := widget.(*widgets.Switch)
	s := ctx.State().(*switchState)
	s.onChange = v.onChange
	b.SetAnimated(v.animated.ValueOr(s.animated))
	b.SetPadding(v.padding.ValueOr(s.padding))
	b.SetChecked(v.checked)
	ctx.UpdateChild(b, v.child)
}

func (v *SwitchView) Unmount(ctx baseui.BuildContext, _ gui.Widget) {
	if s, ok := ctx.State().(*switchState); ok {
		s.change.Disconnect()
	}
}

var _ baseui.WidgetView = (*SwitchView)(nil)
