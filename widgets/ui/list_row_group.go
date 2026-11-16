package ui

import (
	"github.com/golang-gui/goui/gui"
	baseui "github.com/golang-gui/goui/ui"
	"github.com/golang-gui/goui/widgets"
)

// ListRowGroupView declares a connected column of rows. Layout and the shared
// outline belong to widgets.ListRowGroup; each row keeps its own style, state,
// input and semantics, and the group adds no action of its own.
type ListRowGroupView struct {
	baseui.ViewBase[ListRowGroupView]
	children []baseui.View
}

// ListRowGroup stacks its rows without a gap so they read as one control. Only
// the outer corners stay rounded and neighbours share one border line.
func ListRowGroup(children ...baseui.View) *ListRowGroupView {
	v := &ListRowGroupView{children: listRowGroupViews(children)}
	v.Self = v
	return v
}

// Children replaces the declared rows. Identity follows each row's ID, so a
// reorder keeps every mounted row and its content instead of rebuilding them.
func (v *ListRowGroupView) Children(children ...baseui.View) *ListRowGroupView {
	v.children = listRowGroupViews(children)
	return v
}

func (v *ListRowGroupView) Mount(baseui.BuildContext) gui.Widget {
	return widgets.NewListRowGroup()
}

func (v *ListRowGroupView) Update(ctx baseui.BuildContext, widget gui.Widget) {
	ctx.UpdateChildren(widget.(*widgets.ListRowGroup), v.children)
}

func (*ListRowGroupView) Unmount(baseui.BuildContext, gui.Widget) {}

// A nil entry is an omitted declaration, not an empty row.
func listRowGroupViews(views []baseui.View) []baseui.View {
	if len(views) == 0 {
		return nil
	}
	compacted := make([]baseui.View, 0, len(views))
	for _, view := range views {
		if view != nil {
			compacted = append(compacted, view)
		}
	}
	return compacted
}

var _ baseui.WidgetView = (*ListRowGroupView)(nil)
