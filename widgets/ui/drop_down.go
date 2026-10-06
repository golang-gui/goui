package ui

import (
	"github.com/golang-gui/goui/core/bits"
	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/internal/identity"
	"github.com/golang-gui/goui/layout"
	baseui "github.com/golang-gui/goui/ui"
	"github.com/golang-gui/goui/widgets"
)

type DropDownItem = widgets.DropDownItem

// DropDownView declares a non-editable single choice. Omitted Selected retains
// user state when the same model pointer survives rebuilding. Item and
// SelectedItem build real, independently coordinated widget subtrees.
type DropDownView struct {
	baseui.ViewBase[DropDownView]
	model              gui.ListData[DropDownItem]
	selected           int
	placeholder        string
	padding, maxHeight float32
	fields             bits.Bitmap[uint8]
	item, selectedItem func(int, DropDownItem) baseui.View
	onSelected         func(int)
	onOpenError        func(error)
}

const (
	dropDownSelected = iota
	dropDownPadding
	dropDownMaxHeight
)

// DropDown accepts a typed option model. Reuse a non-nil model pointer to avoid
// reinstalling it. Value models are reinstalled; typed nils are not supported.
func DropDown(model gui.ListData[DropDownItem]) *DropDownView {
	v := &DropDownView{model: model}
	v.Self = v
	return v
}
func (v *DropDownView) Model(model gui.ListData[DropDownItem]) *DropDownView {
	v.model = model
	return v
}
func (v *DropDownView) Selected(index int) *DropDownView {
	v.selected = index
	v.fields.Set(dropDownSelected, true)
	return v
}
func (v *DropDownView) Placeholder(text string) *DropDownView { v.placeholder = text; return v }
func (v *DropDownView) Padding(value float32) *DropDownView {
	v.padding = value
	v.fields.Set(dropDownPadding, true)
	return v
}
func (v *DropDownView) PopupMaxHeight(value float32) *DropDownView {
	v.maxHeight = value
	v.fields.Set(dropDownMaxHeight, true)
	return v
}
func (v *DropDownView) Item(builder func(int, DropDownItem) baseui.View) *DropDownView {
	v.item = builder
	return v
}
func (v *DropDownView) SelectedItem(builder func(int, DropDownItem) baseui.View) *DropDownView {
	v.selectedItem = builder
	return v
}
func (v *DropDownView) OnSelected(fn func(int)) *DropDownView    { v.onSelected = fn; return v }
func (v *DropDownView) OnOpenError(fn func(error)) *DropDownView { v.onOpenError = fn; return v }
func (v *DropDownView) Build() baseui.View                       { return v }

type dropDownState struct {
	ctx                baseui.BuildContext
	widget             *widgets.DropDown
	model              gui.ListData[DropDownItem]
	item, selectedItem func(int, DropDownItem) baseui.View
	onSelected         func(int)
	onOpenError        func(error)
	handles            signal.Handles
	padding, maxHeight float32
}

func (v *DropDownView) Mount(ctx baseui.BuildContext) gui.Widget {
	d := widgets.NewDropDown()
	s := &dropDownState{ctx: ctx, widget: d, padding: d.Padding(), maxHeight: d.PopupMaxHeight()}
	s.handles = signal.Handles{
		d.ConnectSelected(func(index int) {
			if s.onSelected != nil {
				s.onSelected(index)
			}
		}),
		d.ConnectOpenError(func(err error) {
			if s.onOpenError != nil {
				s.onOpenError(err)
			}
		}),
	}
	ctx.SetState(s)
	d.SetDelegate(&dropDownUIDelegate{state: s})
	d.SetSelectedDelegate(&dropDownUIDelegate{state: s, selected: true})
	return d
}

func (v *DropDownView) Update(ctx baseui.BuildContext, widget gui.Widget) {
	d, s := widget.(*widgets.DropDown), ctx.State().(*dropDownState)
	s.model, s.item, s.selectedItem = v.model, v.item, v.selectedItem
	s.onSelected, s.onOpenError = v.onSelected, v.onOpenError
	d.SetPlaceholder(v.placeholder)
	padding, maxHeight := s.padding, s.maxHeight
	if v.fields.Check(dropDownPadding) {
		padding = v.padding
	}
	if v.fields.Check(dropDownMaxHeight) {
		maxHeight = v.maxHeight
	}
	d.SetPadding(padding)
	d.SetPopupMaxHeight(maxHeight)
	if !identity.SamePointer(d.Model(), v.model) {
		d.SetModel(v.model)
	}
	if v.fields.Check(dropDownSelected) {
		d.SetSelected(v.selected)
	}
	d.Refresh()
}

func (v *DropDownView) Unmount(ctx baseui.BuildContext, _ gui.Widget) {
	if s, ok := ctx.State().(*dropDownState); ok {
		s.handles.Disconnect()
		s.ctx, s.model, s.item, s.selectedItem, s.onSelected, s.onOpenError = nil, nil, nil, nil, nil, nil
	}
}

type dropDownUIDelegate struct {
	state    *dropDownState
	selected bool
}

func (*dropDownUIDelegate) Setup() gui.Widget { return gui.NewLinearBox(layout.DirectionHorizontal) }
func (d *dropDownUIDelegate) Bind(index int, widget gui.Widget) {
	s := d.state
	if s.ctx == nil || s.model == nil || index < 0 || index >= s.model.ItemsCount() {
		return
	}
	item := s.model.ItemAt(index)
	builder := s.item
	if d.selected && s.selectedItem != nil {
		builder = s.selectedItem
	}
	var content baseui.View
	if builder != nil {
		content = builder(index, item)
	} else {
		name, disabled := "drop-down-item-text", item.Disabled
		if d.selected {
			name, disabled = "drop-down-text", false
		}
		if disabled {
			name += "-disabled"
		}
		content = baseui.Label(item.Text).Style(name)
	}
	s.ctx.UpdateChildren(widget.(*gui.LinearBox), []baseui.View{content})
}
func (d *dropDownUIDelegate) Unbind(_ int, widget gui.Widget) {
	if d.state.ctx != nil {
		d.state.ctx.UpdateChildren(widget.(*gui.LinearBox), nil)
	}
}

var _ baseui.WidgetView = (*DropDownView)(nil)
