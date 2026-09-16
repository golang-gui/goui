package ui

import (
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
)

// ListViewView is the declarative wrapper of gui.ListView. The item closure
// builds a declarative View tree per row (SwiftUI List style) and receives
// the typed item data (ListData[T]), so the builder never sees `any`.
type ListViewView[T any] struct {
	ViewBase[ListViewView[T]]
	model   gui.ListData[T]
	builder func(index int, data T) View
}

// ListView creates a virtualized list driven by model. item receives the
// index and the model's typed data at that index and returns the row's
// declarative View tree. The list must be placed inside a ui.ScrollView.
//
// The model parameter is gui.ListData[T]: a string model cannot be passed to
// a ListView[int] — the type mismatch is a compile error.
func ListView[T any](model gui.ListData[T], item func(index int, data T) View) *ListViewView[T] {
	v := &ListViewView[T]{model: model, builder: item}
	v.Self = v
	return v
}

// SliceList adapts a plain slice into a ListData[T] (gui.NewSliceListModel),
// the one-liner companion of ListView for static data.
func SliceList[T any](slice []T) gui.ListData[T] {
	return gui.NewSliceListModel(slice)
}

// Model replaces the data model (reference-equal no-op at the gui level).
func (v *ListViewView[T]) Model(model gui.ListData[T]) *ListViewView[T] {
	v.model = model
	return v
}

// Item replaces the row builder.
func (v *ListViewView[T]) Item(item func(index int, data T) View) *ListViewView[T] {
	v.builder = item
	return v
}

func (v *ListViewView[T]) Build() View {
	return v
}

func (v *ListViewView[T]) Mount(ctx BuildContext) gui.Widget {
	lv := gui.NewListView()
	lv.SetModel(v.model) // ListData[T] embeds ListModel, so this always holds
	lv.SetDelegate(newListItemDelegate(v, ctx))
	return lv
}

func (v *ListViewView[T]) Update(ctx BuildContext, widget gui.Widget) {
	lv := widget.(*gui.ListView)
	lv.SetModel(v.model) // idempotent: same instance is a no-op
	if d, ok := lv.Delegate().(*uiItemDelegate[T]); ok {
		// Reuse the delegate (a new one would reload the list on every update);
		// refresh its data so future Bind calls see the new model/builder.
		d.model = v.model
		d.builder = v.builder
	} else {
		lv.SetDelegate(newListItemDelegate(v, ctx))
	}
}

func (v *ListViewView[T]) Unmount(ctx BuildContext, widget gui.Widget) {
	lv := widget.(*gui.ListView)
	if d, ok := lv.Delegate().(*uiItemDelegate[T]); ok {
		// Root has already released registered row targets. In the window
		// destruction path their GUI trees deliberately remain intact.
		d.ctx = nil
		d.builder = nil
		d.model = nil
	}
}

// uiItemDelegate bridges the declarative row builder to the imperative
// gui.ListItemDelegate contract:
//
//	Setup  → an empty shell (LinearBox) that can host and measure a row
//	Bind   → coordinate the row View into the shell (mount or update node)
//	Unbind → release the row node and detach it from the shell
//
// Rows scroll out of view and are released through the same mounting-target
// API as ordinary containers. The model is gui.ListData[T]: Bind fetches the typed item
// and hands it straight to the builder.
type uiItemDelegate[T any] struct {
	model   gui.ListData[T]
	builder func(index int, data T) View
	ctx     BuildContext
}

func newListItemDelegate[T any](v *ListViewView[T], ctx BuildContext) *uiItemDelegate[T] {
	return &uiItemDelegate[T]{
		model:   v.model,
		builder: v.builder,
		ctx:     ctx,
	}
}

func (d *uiItemDelegate[T]) Setup() gui.Widget {
	return gui.NewLinearBox(layout.DirectionVertical)
}

func (d *uiItemDelegate[T]) Bind(index int, w gui.Widget) {
	if d.ctx == nil {
		return
	}
	shell, ok := w.(*gui.LinearBox)
	if !ok {
		return
	}
	var content View
	if d.builder != nil {
		content = d.builder(index, d.model.ItemAt(index))
	}
	d.ctx.UpdateChildren(shell, []View{content})
}

func (d *uiItemDelegate[T]) Unbind(index int, w gui.Widget) {
	if shell, ok := w.(Container); ok && d.ctx != nil {
		d.ctx.UpdateChildren(shell, nil)
	}
}
