package ui

import (
	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/core/optional"
	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/gui"
	baseui "github.com/golang-gui/goui/ui"
	"github.com/golang-gui/goui/widgets"
)

// ListRowView declares a settings or action row. Layout, input and semantics
// belong to widgets.ListRow; the two content slots reconcile independently.
type ListRowView struct {
	baseui.ViewBase[ListRowView]
	title, subtitle string
	prefix, suffix  baseui.View
	activatable     bool
	padding         optional.Optional[geometry.Insets]
	spacing         optional.Optional[float32]
	onActivate      func()
}

func ListRow(title string) *ListRowView {
	v := &ListRowView{title: title}
	v.Self = v
	return v
}

func (v *ListRowView) Title(title string) *ListRowView       { v.title = title; return v }
func (v *ListRowView) Subtitle(subtitle string) *ListRowView { v.subtitle = subtitle; return v }
func (v *ListRowView) Prefix(child baseui.View) *ListRowView { v.prefix = child; return v }
func (v *ListRowView) Suffix(child baseui.View) *ListRowView { v.suffix = child; return v }

// Activatable also declares the row's default focus capability. A later
// Focusable(false) keeps activation while opting out of row Tab navigation.
func (v *ListRowView) Activatable(activatable bool) *ListRowView {
	v.activatable = activatable
	v.ViewBase.Focusable(activatable)
	return v
}

func (v *ListRowView) OnActivate(fn func()) *ListRowView { v.onActivate = fn; return v }
func (v *ListRowView) Padding(padding float32) *ListRowView {
	v.padding.SetValue(geometry.UniformInsets(padding))
	return v
}
func (v *ListRowView) Paddings(padding geometry.Insets) *ListRowView {
	v.padding.SetValue(padding)
	return v
}
func (v *ListRowView) Spacing(spacing float32) *ListRowView {
	v.spacing.SetValue(spacing)
	return v
}

type listRowSlot struct {
	row    *widgets.ListRow
	prefix bool
}

func (s *listRowSlot) SetChild(child gui.Widget) {
	if s.prefix {
		s.row.SetPrefix(child)
	} else {
		s.row.SetSuffix(child)
	}
}

type listRowState struct {
	prefix, suffix *listRowSlot
	padding        geometry.Insets
	spacing        float32
	onActivate     func()
	activated      signal.Handle
}

func (v *ListRowView) Mount(ctx baseui.BuildContext) gui.Widget {
	r := widgets.NewListRow(v.title)
	s := &listRowState{prefix: &listRowSlot{row: r, prefix: true}, suffix: &listRowSlot{row: r}, padding: r.Padding(), spacing: r.Spacing()}
	s.activated = r.ConnectActivated(func() {
		if s.onActivate != nil {
			s.onActivate()
		}
	})
	ctx.SetState(s)
	return r
}

func (v *ListRowView) Update(ctx baseui.BuildContext, widget gui.Widget) {
	r, s := widget.(*widgets.ListRow), ctx.State().(*listRowState)
	s.onActivate = v.onActivate
	r.SetTitle(v.title)
	r.SetSubtitle(v.subtitle)
	r.SetPadding(v.padding.ValueOr(s.padding))
	r.SetSpacing(v.spacing.ValueOr(s.spacing))
	r.SetActivatable(v.activatable)
	ctx.UpdateChild(s.prefix, v.prefix)
	ctx.UpdateChild(s.suffix, v.suffix)
}

func (*ListRowView) Unmount(ctx baseui.BuildContext, _ gui.Widget) {
	s := ctx.State().(*listRowState)
	s.activated.Disconnect()
	s.onActivate = nil
}

var _ baseui.WidgetView = (*ListRowView)(nil)
