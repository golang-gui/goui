package ui

import "github.com/golang-gui/goui/gui"

// MenuButtonGroupView owns a nonvisual menu group for descendant declarations.
type MenuButtonGroupView struct {
	ComposeBase[MenuButtonGroupView]
	child View
}

// MenuButtonGroup declares a group with explicit ancestor scope. Members use
// Group(id); ordinary buttons in the content remain independent.
func MenuButtonGroup(id string, child View) *MenuButtonGroupView {
	if id == "" {
		panic("ui: MenuButtonGroup requires a nonempty ID")
	}
	v := &MenuButtonGroupView{child: child}
	v.Self = v
	return v.ID(id)
}
func (v *MenuButtonGroupView) Build(ctx BuildContext) View {
	if ctx.State() == nil {
		ctx.SetState(gui.NewMenuButtonGroup())
	}
	return v.child
}
func (*MenuButtonGroupView) Object(ctx BuildContext) any { return ctx.State() }

// Release has no native resources. Descendants disconnect before this state
// is released, and the group holds no member or surface references.
func (*MenuButtonGroupView) Release(BuildContext) {}

var _ ObjectView = (*MenuButtonGroupView)(nil)
