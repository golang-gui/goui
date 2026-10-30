package ui

import (
	baseui "github.com/golang-gui/goui/ui"
	"github.com/golang-gui/goui/widgets"
)

// CheckButtonGroupView owns the default exclusivity group for descendant
// CheckButtons, independently of the projected widget tree.
type CheckButtonGroupView struct {
	baseui.ComposeBase[CheckButtonGroupView]
	child baseui.View
}

// CheckButtonGroup supplies a default group without adding a layout widget.
// The nearest scope wins; explicit CheckButton.Group values override it.
func CheckButtonGroup(child baseui.View) *CheckButtonGroupView {
	v := &CheckButtonGroupView{child: child}
	v.Self = v
	return v
}

func (v *CheckButtonGroupView) Build(ctx baseui.BuildContext) baseui.View {
	if ctx.State() == nil {
		ctx.SetState(widgets.NewCheckButtonGroup())
	}
	return v.child
}

// Object returns the group's existing instance. The caller borrows it.
func (*CheckButtonGroupView) Object(ctx baseui.BuildContext) any { return ctx.State() }

// Release needs no native cleanup: members disconnect before the group state
// is released, and CheckButtonGroup itself owns no widgets or external resources.
func (*CheckButtonGroupView) Release(baseui.BuildContext) {}

var _ baseui.ObjectView = (*CheckButtonGroupView)(nil)
