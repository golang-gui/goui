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
	rowInsetLeft        float32
	rowInsetRight       float32
	rowStyle            func(widgets.TreeRow, T) string
	rowContentDepth     func(widgets.TreeRow, T) (int, bool)
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

// RowStyle names the outer row style, including selected, current and content
// parts. Empty names or an omitted callback restore "tree-item". Content
// Widgets keep their own styles. The callback follows normal row binding.
func (v *TreeViewView[T]) RowStyle(fn func(widgets.TreeRow, T) string) *TreeViewView[T] {
	v.rowStyle = fn
	return v
}

// RowContentDepth overrides only the content part's paint depth. Return
// (depth, true) to override, including zero; false or omission uses row.Depth.
// Content layout, full-row background and keyboard focus geometry are unchanged.
func (v *TreeViewView[T]) RowContentDepth(fn func(widgets.TreeRow, T) (int, bool)) *TreeViewView[T] {
	v.rowContentDepth = fn
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

// RowInsets declares fixed left and right viewport margins in DIP. Omission
// restores zero; row backgrounds, focus, hit bounds and content are inset together.
func (v *TreeViewView[T]) RowInsets(left, right float32) *TreeViewView[T] {
	v.rowInsetLeft, v.rowInsetRight = left, right
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

// OnActivate handles row-body double clicks and Enter for both branches and leaves.
// Expansion is unchanged unless the handler updates it explicitly.
func (v *TreeViewView[T]) OnActivate(fn func(string)) *TreeViewView[T] { v.onActivate = fn; return v }
func (v *TreeViewView[T]) ContextMenu(fn func(string) []*baseui.MenuItemView) *TreeViewView[T] {
	v.contextMenu = fn
	return v
}
func (v *TreeViewView[T]) OnContextMenuError(fn func(error)) *TreeViewView[T] {
	v.onContextMenuError = fn
	return v
}

type treeState[T any] struct {
	ctx             baseui.BuildContext
	model           widgets.TreeData[T]
	item            func(widgets.TreeRow, T) baseui.View
	rowStyle        func(widgets.TreeRow, T) string
	rowContentDepth func(widgets.TreeRow, T) (int, bool)
	version         uint64
	handles         signal.Handles
	onSelection     func([]string)
	onCurrent       func(string)
	onExpanded      func(string, bool)
	onActivate      func(string)
	contextMenu     func(string) []*baseui.MenuItemView
	onError         func(error)
	tree            *widgets.TreeView
}

func (v *TreeViewView[T]) Mount(ctx baseui.BuildContext) gui.Widget {
	tree := widgets.NewTreeView()
	state := &treeState[T]{ctx: ctx, tree: tree}
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
				*result = baseui.MenuFor(ctx, state.contextMenu(id)...)
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
	tree.SetRowStyle(func(row widgets.TreeRow) string {
		fn, model, version := state.rowStyle, state.model, state.version
		if state.ctx != nil && model != nil && fn != nil {
			value := model.Item(row.ID)
			if state.ctx != nil && state.version == version && !tree.Destroyed() {
				return fn(row, value)
			}
		}
		return ""
	})
	tree.SetRowContentDepth(func(row widgets.TreeRow) (int, bool) {
		fn, model, version := state.rowContentDepth, state.model, state.version
		if state.ctx != nil && model != nil && fn != nil {
			value := model.Item(row.ID)
			if state.ctx != nil && state.version == version && !tree.Destroyed() {
				return fn(row, value)
			}
		}
		return 0, false
	})
	return tree
}
func (v *TreeViewView[T]) Update(ctx baseui.BuildContext, widget gui.Widget) {
	tree, state := widget.(*widgets.TreeView), ctx.State().(*treeState[T])
	state.version++
	state.model, state.item = v.model, v.item
	state.rowStyle, state.rowContentDepth = v.rowStyle, v.rowContentDepth
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
	tree.SetRowInsets(v.rowInsetLeft, v.rowInsetRight)
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
func (*TreeViewView[T]) Unmount(ctx baseui.BuildContext, widget gui.Widget) {
	state := ctx.State().(*treeState[T])
	state.version++
	state.handles.Disconnect()
	state.ctx, state.model, state.item = nil, nil, nil
	state.onSelection, state.onCurrent, state.onExpanded, state.onActivate = nil, nil, nil, nil
	state.contextMenu, state.onError = nil, nil
	state.rowStyle, state.rowContentDepth = nil, nil
	tree := widget.(*widgets.TreeView)
	tree.SetRowStyle(nil)
	tree.SetRowContentDepth(nil)
}

type treeContent struct {
	*widgets.TreeExpander
	box *gui.LinearBox
	id  string
}

func (s *treeState[T]) Setup() gui.Widget {
	c := &treeContent{TreeExpander: widgets.NewTreeExpander(), box: gui.NewLinearBox(layout.DirectionVertical)}
	c.SetChild(c.box)
	c.ConnectToggle(func() {
		if s.ctx != nil && c.id != "" && !s.tree.Destroyed() {
			s.tree.SetExpanded(c.id, !s.tree.Expanded(c.id))
		}
	})
	return c
}
func (s *treeState[T]) Bind(row widgets.TreeRow, widget gui.Widget) {
	ctx := s.ctx
	if ctx == nil {
		return
	}
	var view baseui.View
	if s.item != nil && s.model != nil {
		view = s.item(row, s.model.Item(row.ID))
	}
	c := widget.(*treeContent)
	c.id = row.ID
	c.SetDepth(row.Depth)
	c.SetIndentation(s.tree.Indentation())
	c.SetExpandable(row.Expandable)
	c.SetExpanded(row.Expanded)
	ctx.UpdateChildren(c.box, []baseui.View{view})
}
func (s *treeState[T]) Unbind(_ widgets.TreeRow, widget gui.Widget) {
	if s.ctx != nil {
		c := widget.(*treeContent)
		c.id = ""
		c.SetExpandable(false)
		s.ctx.UpdateChildren(c.box, nil)
	}
}
