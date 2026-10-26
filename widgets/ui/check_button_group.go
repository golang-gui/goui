package ui

import (
	baseui "github.com/golang-gui/goui/ui"
	"github.com/golang-gui/goui/widgets"
)

// CheckButtonGroupView owns a nonvisual exclusivity group. Its ID names the group
// for descendant CheckButtons, independently of the projected widget tree.
type CheckButtonGroupView struct {
	baseui.ComposeBase[CheckButtonGroupView]
	child baseui.View
}

// CheckButtonGroup declares an explicit group around one content view. Equal IDs in
// separate declaration branches are independent; the nearest ancestor wins.
func CheckButtonGroup(id string, child baseui.View) *CheckButtonGroupView {
	if id == "" {
		panic("widgets/ui: CheckButtonGroup requires a nonempty ID")
	}
	v := &CheckButtonGroupView{child: child}
	v.Self = v
	return v.ID(id)
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
