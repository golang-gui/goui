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
		d.releaseAll()
	}
}

// uiItemDelegate bridges the declarative row builder to the imperative
// gui.ListItemDelegate contract:
//
//	Setup  → an empty shell (LinearBox) that can host and measure a row
//	Bind   → coordinate the row View into the shell (mount or update node)
//	Unbind → release the row node and detach it from the shell
//
// Rows scroll out of view and are released; the node map stays bounded by the
// reuse pool size. The model is gui.ListData[T]: Bind fetches the typed item
// and hands it straight to the builder.
type uiItemDelegate[T any] struct {
	model   gui.ListData[T]
	builder func(index int, data T) View
	root    *root
	nodes   map[gui.Widget]*node // shell widget → coordinated row node
}

func newListItemDelegate[T any](v *ListViewView[T], ctx BuildContext) *uiItemDelegate[T] {
	bc, _ := ctx.(*buildContext)
	return &uiItemDelegate[T]{
		model:   v.model,
		builder: v.builder,
		root:    bc.root,
		nodes:   make(map[gui.Widget]*node),
	}
}

func (d *uiItemDelegate[T]) Setup() gui.Widget {
	return gui.NewLinearBox(layout.DirectionVertical)
}

func (d *uiItemDelegate[T]) Bind(index int, w gui.Widget) {
	if d.builder == nil || d.root == nil {
		return
	}
	shell, ok := w.(*gui.LinearBox)
	if !ok {
		return
	}
	item := d.root.updateNode(d.nodes[w], d.builder(index, d.model.ItemAt(index)))
	if item == nil || item.widget == nil {
		return
	}
	d.nodes[w] = item
	shell.AddChild(item.widget)
}

func (d *uiItemDelegate[T]) Unbind(index int, w gui.Widget) {
	if n := d.nodes[w]; n != nil {
		if c, ok := w.(gui.Container); ok && n.widget != nil {
			c.RemoveChild(n.widget)
		}
		d.root.release(n, true)
		delete(d.nodes, w)
	}
}

// releaseAll releases every remaining row node (list teardown).
func (d *uiItemDelegate[T]) releaseAll() {
	for w, n := range d.nodes {
		if c, ok := w.(gui.Container); ok && n.widget != nil {
			c.RemoveChild(n.widget)
		}
		d.root.release(n, true)
		delete(d.nodes, w)
	}
}
