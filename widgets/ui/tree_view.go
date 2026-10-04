package ui

import (
	"slices"

	"github.com/golang-gui/goui/core/bits"
	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/internal/identity"
	"github.com/golang-gui/goui/layout"
	baseui "github.com/golang-gui/goui/ui"
	"github.com/golang-gui/goui/widgets"
)

// TreeViewView is a thin model-and-row-builder binding. Omitted selection,
// current and expansion declarations preserve user state across rebuilds when
// the same model pointer is retained.
type TreeViewView[T any] struct {
	baseui.ViewBase[TreeViewView[T]]
	model               widgets.TreeData[T]
	item                func(widgets.TreeRow, T) baseui.View
	mode                widgets.SelectionMode
	selection, expanded []string
	current             string
	indentation         float32
	fields              bits.Bitmap[uint8]
	onSelection         func([]string)
	onCurrent           func(string)
	onExpanded          func(string, bool)
	onActivate          func(string)
	contextMenu         func(string) []*baseui.MenuItemView
	onContextMenuError  func(error)
}

const (
	treeSelection = iota
	treeCurrent
	treeExpanded
	treeIndentation
)

// TreeView declares a tree. Reuse a non-nil model pointer to preserve expansion,
// selection and Current across rebuilds. Non-pointer values are reinstalled on
// every update; their contents are never compared. A nil interface clears the
// model; typed nil pointers are not supported.
func TreeView[T any](model widgets.TreeData[T], item func(widgets.TreeRow, T) baseui.View) *TreeViewView[T] {
	v := &TreeViewView[T]{model: model, item: item}
	v.Self = v
	return v
}

// Model declares the model; only an unchanged non-nil pointer skips installation.
func (v *TreeViewView[T]) Model(model widgets.TreeData[T]) *TreeViewView[T] {
	v.model = model
	return v
}
func (v *TreeViewView[T]) Item(item func(widgets.TreeRow, T) baseui.View) *TreeViewView[T] {
	v.item = item
	return v
}
func (v *TreeViewView[T]) SelectionMode(mode widgets.SelectionMode) *TreeViewView[T] {
	v.mode = mode
	return v
}
func (v *TreeViewView[T]) Selection(ids []string) *TreeViewView[T] {
	v.selection = slices.Clone(ids)
	v.fields.Set(treeSelection, true)
	return v
}
func (v *TreeViewView[T]) Current(id string) *TreeViewView[T] {
	v.current = id
	v.fields.Set(treeCurrent, true)
	return v
}
func (v *TreeViewView[T]) Expanded(ids []string) *TreeViewView[T] {
	v.expanded = slices.Clone(ids)
	v.fields.Set(treeExpanded, true)
	return v
}
func (v *TreeViewView[T]) Indentation(value float32) *TreeViewView[T] {
	v.indentation = value
	v.fields.Set(treeIndentation, true)
	return v
}
func (v *TreeViewView[T]) OnSelection(fn func([]string)) *TreeViewView[T] {
	v.onSelection = fn
	return v
}
func (v *TreeViewView[T]) OnCurrent(fn func(string)) *TreeViewView[T] { v.onCurrent = fn; return v }
func (v *TreeViewView[T]) OnExpanded(fn func(string, bool)) *TreeViewView[T] {
	v.onExpanded = fn
	return v
}
func (v *TreeViewView[T]) OnActivate(fn func(string)) *TreeViewView[T] { v.onActivate = fn; return v }
func (v *TreeViewView[T]) ContextMenu(fn func(string) []*baseui.MenuItemView) *TreeViewView[T] {
	v.contextMenu = fn
	return v
}
func (v *TreeViewView[T]) OnContextMenuError(fn func(error)) *TreeViewView[T] {
	v.onContextMenuError = fn
	return v
}
func (v *TreeViewView[T]) Build() baseui.View { return v }

type treeState[T any] struct {
	ctx         baseui.BuildContext
	model       widgets.TreeData[T]
	item        func(widgets.TreeRow, T) baseui.View
	handles     signal.Handles
	onSelection func([]string)
	onCurrent   func(string)
	onExpanded  func(string, bool)
	onActivate  func(string)
	contextMenu func(string) []*baseui.MenuItemView
	onError     func(error)
}

func (v *TreeViewView[T]) Mount(ctx baseui.BuildContext) gui.Widget {
	tree := widgets.NewTreeView()
	state := &treeState[T]{ctx: ctx}
	state.handles = signal.Handles{
		tree.ConnectSelection(func(ids []string) {
			if state.onSelection != nil {
				state.onSelection(ids)
			}
		}),
		tree.ConnectCurrent(func(id string) {
			if state.onCurrent != nil {
				state.onCurrent(id)
			}
		}),
		tree.ConnectExpanded(func(id string, expanded bool) {
			if state.onExpanded != nil {
				state.onExpanded(id, expanded)
			}
		}),
		tree.ConnectActivate(func(id string) {
			if state.onActivate != nil {
				state.onActivate(id)
			}
		}),
		tree.ConnectContextMenu(func(id string, result *gui.MenuModel) {
			if state.contextMenu != nil {
				*result = baseui.Menu(state.contextMenu(id)...)
			}
		}),
		tree.ConnectContextMenuError(func(err error) {
			if state.onError != nil {
				state.onError(err)
			}
		}),
	}
	ctx.SetState(state)
	tree.SetDelegate(state)
	return tree
}
func (v *TreeViewView[T]) Update(ctx baseui.BuildContext, widget gui.Widget) {
	tree, state := widget.(*widgets.TreeView), ctx.State().(*treeState[T])
	state.model, state.item = v.model, v.item
	state.onSelection, state.onCurrent = v.onSelection, v.onCurrent
	state.onExpanded, state.onActivate = v.onExpanded, v.onActivate
	state.contextMenu, state.onError = v.contextMenu, v.onContextMenuError
	state.handles.Block()
	defer state.handles.Unblock()
	if !identity.SamePointer(tree.Model(), v.model) {
		tree.SetModel(v.model)
	}
	tree.SetSelectionMode(v.mode)
	indentation := float32(16)
	if v.fields.Check(treeIndentation) {
		indentation = v.indentation
	}
	tree.SetIndentation(indentation)
	if v.fields.Check(treeExpanded) {
		tree.SetExpandedIDs(v.expanded)
	}
	if v.fields.Check(treeSelection) {
		tree.SetSelection(v.selection)
	}
	if v.fields.Check(treeCurrent) {
		tree.SetCurrent(v.current)
	}
	tree.Refresh()
}
func (*TreeViewView[T]) Unmount(ctx baseui.BuildContext, _ gui.Widget) {
	state := ctx.State().(*treeState[T])
	state.handles.Disconnect()
	state.ctx, state.model, state.item = nil, nil, nil
	state.onSelection, state.onCurrent, state.onExpanded, state.onActivate = nil, nil, nil, nil
	state.contextMenu, state.onError = nil, nil
}
func (*treeState[T]) Setup() gui.Widget { return gui.NewLinearBox(layout.DirectionVertical) }
func (s *treeState[T]) Bind(row widgets.TreeRow, widget gui.Widget) {
	ctx := s.ctx
	if ctx == nil {
		return
	}
	var view baseui.View
	if s.item != nil && s.model != nil {
		view = s.item(row, s.model.Item(row.ID))
	}
	ctx.UpdateChildren(widget.(*gui.LinearBox), []baseui.View{view})
}
func (s *treeState[T]) Unbind(_ widgets.TreeRow, widget gui.Widget) {
	if s.ctx != nil {
		s.ctx.UpdateChildren(widget.(*gui.LinearBox), nil)
	}
}
