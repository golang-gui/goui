package ui

import (
	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/core/optional"
	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/gui"
	baseui "github.com/golang-gui/goui/ui"
	"github.com/golang-gui/goui/widgets"
)

// DialogView declares card content, backdrop appearance and close requests.
// Place it in baseui.OverlayItem(...).Fill(true).Modal(true) for modal use.
type DialogView struct {
	baseui.ViewBase[DialogView]
	child                           baseui.View
	padding                         optional.Optional[geometry.Insets]
	backdropCaption, closeOnOutside bool
	onCloseRequest                  func()
}

// Dialog declares a dialog with optional content; it adds no native window.
func Dialog(child ...baseui.View) *DialogView {
	if len(child) > 1 {
		panic("widgets/ui: Dialog accepts at most one child")
	}
	v := new(DialogView)
	v.Self = v
	if len(child) != 0 {
		v.child = child[0]
	}
	return v
}
func (v *DialogView) Child(child baseui.View) *DialogView { v.child = child; return v }
func (v *DialogView) Padding(padding float32) *DialogView {
	return v.Paddings(geometry.UniformInsets(padding))
}
func (v *DialogView) Paddings(padding geometry.Insets) *DialogView {
	v.padding.SetValue(padding)
	return v
}
func (v *DialogView) BackdropCaption(caption bool) *DialogView { v.backdropCaption = caption; return v }
func (v *DialogView) CloseOnOutside(close bool) *DialogView    { v.closeOnOutside = close; return v }
func (v *DialogView) OnCloseRequest(fn func()) *DialogView     { v.onCloseRequest = fn; return v }

type dialogState struct {
	padding        geometry.Insets
	close          signal.Handle
	onCloseRequest func()
}

func (v *DialogView) Mount(ctx baseui.BuildContext) gui.Widget {
	d := widgets.NewDialog()
	s := &dialogState{padding: d.Padding()}
	s.close = d.ConnectCloseRequest(func() {
		if s.onCloseRequest != nil {
			s.onCloseRequest()
		}
	})
	ctx.SetState(s)
	return d
}
func (v *DialogView) Update(ctx baseui.BuildContext, widget gui.Widget) {
	d, s := widget.(*widgets.Dialog), ctx.State().(*dialogState)
	s.onCloseRequest = v.onCloseRequest
	d.SetPadding(v.padding.ValueOr(s.padding))
	d.SetBackdropCaption(v.backdropCaption)
	d.SetCloseOnOutside(v.closeOnOutside)
	ctx.UpdateChild(d, v.child)
}
func (*DialogView) Unmount(ctx baseui.BuildContext, _ gui.Widget) {
	if s, ok := ctx.State().(*dialogState); ok {
		s.close.Disconnect()
		s.onCloseRequest = nil
	}
}

var _ baseui.WidgetView = (*DialogView)(nil)
