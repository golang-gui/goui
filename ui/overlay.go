package ui

import (
	"slices"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/gui"
)

// OverlayItemView describes a floating child, its placement and optional modal
// input boundary in an Overlay.
// Like MenuItemView, it is not a View and creates no wrapper widget. Visibility,
// style and size preferences belong to the child View, not this descriptor.
type OverlayItemView struct {
	child      View
	fill       bool
	modal      bool
	onPosition func(available, size geometry.Size, position *geometry.Point)
}

// OverlayItem creates a floating child that keeps its measured size by default.
func OverlayItem(child View) *OverlayItemView {
	return &OverlayItemView{child: child}
}

// Fill forces the child to fill both available axes. It does not imply modality.
func (v *OverlayItemView) Fill(fill bool) *OverlayItemView {
	v.fill = fill
	return v
}

// Modal restricts the owning Root's content input to this child's subtree while
// it is mounted and visible. It defaults to false and does not imply Fill, draw
// a backdrop or handle dismissal. The item owns its ModalScope; do not attach
// another scope directly to the same child. Rebuilds reuse the existing scope.
// False or omission releases only this item's scope without changing child visibility.
func (v *OverlayItemView) Modal(modal bool) *OverlayItemView {
	v.modal = modal
	return v
}

// OnPosition replaces the positioning callback. It runs during GUI Arrange,
// after measurement, with Overlay-local DIP coordinates and an initial position
// of zero. nil restores default positioning. Do not retain position, mutate the
// tree or request a synchronous rebuild/layout from this callback.
func (v *OverlayItemView) OnPosition(fn func(available, size geometry.Size, position *geometry.Point)) *OverlayItemView {
	v.onPosition = fn
	return v
}

// OverlayView binds gui.Overlay. Only the main child contributes to its desired
// size. Modal items delegate their input policy to gui.ModalScope.
type OverlayView struct {
	ViewBase[OverlayView]
	child View
	items []*OverlayItemView
}

// Overlay hosts an optional main child and floating items in back-to-front order.
// Nil items and children building to nil are omitted. Items reconcile by child ID
// and concrete View type, or position when un-IDed, not descriptor pointer/Name.
// Keep persistent layers
// declared and change their child's Visible modifier to preserve their state.
func Overlay(child View, items ...*OverlayItemView) *OverlayView {
	v := &OverlayView{child: child, items: slices.Clone(items)}
	v.Self = v
	return v
}

func (v *OverlayView) Child(child View) *OverlayView {
	v.child = child
	return v
}

// Overlays replaces, rather than appends to, the floating item declarations.
func (v *OverlayView) Overlays(items ...*OverlayItemView) *OverlayView {
	v.items = slices.Clone(items)
	return v
}

func (v *OverlayView) Mount(ctx BuildContext) gui.Widget {
	overlay := gui.NewOverlay()
	ctx.SetState(&overlayChildren{overlay: overlay, items: make(map[gui.Widget]*overlayItemState)})
	return overlay
}

func (v *OverlayView) Update(ctx BuildContext, widget gui.Widget) {
	overlay := widget.(*gui.Overlay)
	ctx.UpdateChild(overlay, v.child)
	floating := ctx.State().(*overlayChildren)
	children := make([]View, len(v.items))
	for i, item := range v.items {
		if item != nil {
			children[i] = item.child
		}
	}
	ctx.UpdateChildren(floating, children, func(i int, child gui.Widget) {
		if child != nil {
			item := v.items[i]
			floating.items[child].onPosition = item.onPosition
			overlay.SetOverlayFill(child, item.fill)
			floating.items[child].setModal(child, item.modal)
		}
	})
	// Callback identity cannot be compared; captured state may have changed.
	overlay.RequestLayout()
}

func (v *OverlayView) Unmount(ctx BuildContext, _ gui.Widget) {
	s := ctx.State().(*overlayChildren)
	for _, item := range s.items {
		item.disconnect()
	}
	s.items = nil
}

// overlayChildren adapts floating slots to the ordinary UI mounting interface.
// It is not a Widget and knows nothing about reconciler nodes or Root.
type overlayChildren struct {
	overlay *gui.Overlay
	items   map[gui.Widget]*overlayItemState
}

func (s *overlayChildren) AddChild(child gui.Widget) {
	s.overlay.AddOverlay(child)
	entry := new(overlayItemState)
	s.items[child] = entry
	entry.position = s.overlay.ConnectOverlayPosition(child,
		func(available, size geometry.Size, position *geometry.Point) {
			if entry.onPosition != nil {
				entry.onPosition(available, size, position)
			}
		})
}

func (s *overlayChildren) RemoveChild(child gui.Widget) {
	if entry := s.items[child]; entry != nil {
		entry.disconnect()
		delete(s.items, child)
	}
	s.overlay.RemoveOverlay(child)
}

func (s *overlayChildren) MoveChildBefore(child, sibling gui.Widget) {
	s.overlay.MoveChildBefore(child, sibling)
}

type overlayItemState struct {
	position   signal.Handle
	onPosition func(available, size geometry.Size, position *geometry.Point)
	scope      *gui.ModalScope
}

func (s *overlayItemState) setModal(child gui.Widget, modal bool) {
	if modal {
		if s.scope == nil {
			s.scope = gui.NewModalScope(child)
		}
	} else if s.scope != nil {
		scope := s.scope
		s.scope = nil
		scope.Destroy()
	}
}

func (s *overlayItemState) disconnect() {
	if s.position != nil {
		s.position.Disconnect()
		s.position = nil
	}
	s.onPosition = nil
	s.setModal(nil, false)
}
