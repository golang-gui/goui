package ui

import (
	baseui "github.com/golang-gui/goui/ui"
	"github.com/golang-gui/goui/widgets"
)

// CheckGroupView owns a nonvisual exclusivity group. Its ID names the group
// for descendant CheckButtons, independently of the projected widget tree.
type CheckGroupView struct {
	baseui.ComposeBase[CheckGroupView]
	child baseui.View
}

// CheckGroup declares an explicit group around one content view. Equal IDs in
// separate declaration branches are independent; the nearest ancestor wins.
func CheckGroup(id string, child baseui.View) *CheckGroupView {
	if id == "" {
		panic("widgets/ui: CheckGroup requires a nonempty ID")
	}
	v := &CheckGroupView{child: child}
	v.Self = v
	return v.ID(id)
}

func (v *CheckGroupView) Build(ctx baseui.BuildContext) baseui.View {
	if ctx.State() == nil {
		ctx.SetState(widgets.NewCheckGroup())
	}
	return v.child
}

// Object returns the group's existing instance. The caller borrows it.
func (*CheckGroupView) Object(ctx baseui.BuildContext) any { return ctx.State() }

// Release needs no native cleanup: members disconnect before the group state
// is released, and CheckGroup itself owns no widgets or external resources.
func (*CheckGroupView) Release(baseui.BuildContext) {}

var _ baseui.ObjectView = (*CheckGroupView)(nil)
