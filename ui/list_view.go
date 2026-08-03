package ui

import (
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
)

// ListViewView is the declarative wrapper of gui.ListView. The item closure
// builds a declarative View tree per row (SwiftUI List style); the framework
// bridges it to gui.ListItemDelegate through a shell widget: Setup creates an
// empty shell, Bind coordinates the item View into the shell. Item state is
// released when the row scrolls out of view (Flutter-like); see
// doc/DesignScroll.md §4.
type ListViewView struct {
	ViewBase[ListViewView]
	model   gui.ListModel
	builder func(index int, data any) View
}

// ListView creates a virtualized list driven by model. item receives the
// index and the model's data at that index and returns the row's declarative
// View tree. The list must be placed inside a ui.ScrollView.
func ListView(model gui.ListModel, item func(index int, data any) View) *ListViewView {
	v := &ListViewView{model: model, builder: item}
	v.Self = v
	return v
}

// Model replaces the data model (reference-equal no-op).
func (v *ListViewView) Model(model gui.ListModel) *ListViewView {
	v.model = model
	return v
}

// Item replaces the row builder.
func (v *ListViewView) Item(item func(index int, data any) View) *ListViewView {
	v.builder = item
	return v
}

func (v *ListViewView) Build() View {
	return v
}

func (v *ListViewView) Mount(ctx BuildContext) gui.Widget {
	lv := gui.NewListView()
	lv.SetModel(v.model)
	lv.SetDelegate(newListItemDelegate(v, ctx))
	return lv
}

func (v *ListViewView) Update(ctx BuildContext, widget gui.Widget) {
	lv := widget.(*gui.ListView)
	lv.SetModel(v.model) // idempotent: same instance is a no-op
	if d, ok := lv.Delegate().(*uiItemDelegate); ok {
		// Reuse the delegate (a new one would reload the list on every update);
		// refresh its data so future Bind calls see the new model/builder.
		d.model = v.model
		d.builder = v.builder
	} else {
		lv.SetDelegate(newListItemDelegate(v, ctx))
	}
}

func (v *ListViewView) Unmount(ctx BuildContext, widget gui.Widget) {
	lv := widget.(*gui.ListView)
	if d, ok := lv.Delegate().(*uiItemDelegate); ok {
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
// reuse pool size.
type uiItemDelegate struct {
	model   gui.ListModel
	builder func(index int, data any) View
	root    *root
	nodes   map[gui.Widget]*node // shell widget → coordinated row node
}

func newListItemDelegate(v *ListViewView, ctx BuildContext) *uiItemDelegate {
	bc, _ := ctx.(*buildContext)
	return &uiItemDelegate{
		model:   v.model,
		builder: v.builder,
		root:    bc.root,
		nodes:   make(map[gui.Widget]*node),
	}
}

func (d *uiItemDelegate) Setup() gui.Widget {
	return gui.NewLinearBox(layout.DirectionVertical)
}

func (d *uiItemDelegate) Bind(index int, w gui.Widget) {
	if d.builder == nil || d.root == nil {
		return
	}
	var data any
	if d.model != nil {
		data = d.model.ItemAt(index)
	}
	shell, ok := w.(*gui.LinearBox)
	if !ok {
		return
	}
	item := d.root.updateNode(d.nodes[w], d.builder(index, data))
	if item == nil || item.widget == nil {
		return
	}
	d.nodes[w] = item
	shell.AddChild(item.widget)
}

func (d *uiItemDelegate) Unbind(index int, w gui.Widget) {
	if n := d.nodes[w]; n != nil {
		if c, ok := w.(gui.Container); ok && n.widget != nil {
			c.RemoveChild(n.widget)
		}
		d.root.release(n, true)
		delete(d.nodes, w)
	}
}

// releaseAll releases every remaining row node (list teardown).
func (d *uiItemDelegate) releaseAll() {
	for w, n := range d.nodes {
		if c, ok := w.(gui.Container); ok && n.widget != nil {
			c.RemoveChild(n.widget)
		}
		d.root.release(n, true)
		delete(d.nodes, w)
	}
}
