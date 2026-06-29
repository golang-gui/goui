package gui

import (
	"slices"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/layout"
)

type Root interface {
	Widget() Widget
	RequestPaint() error
}

type Widget interface {
	base() *WidgetBase

	Parent() Widget
	Root() Root
	Window() Window

	ID() string
	SetID(string)

	Visible() bool
	SetVisible(bool)

	Focusable() bool
	SetFocusable(bool)
	Focused() bool
	ContainsFocus() bool
	ConnectFocused(func(focused bool)) signal.Handle
	ConnectContainsFocus(func(focused bool)) signal.Handle

	Rect() geometry.Rectangle

	EventControllers() []EventController
	AddEventController(EventController)
	RemoveEventController(EventController)

	LayoutManager() layout.LayoutManager
	SetLayoutManager(layout.LayoutManager)

	Measure(available geometry.Size) geometry.Size
	Arrange(rect geometry.Rectangle)

	Paint(p Painter)
	RequestLayout()

	ConnectMount(func()) signal.Handle
	ConnectUnmount(func()) signal.Handle

	Snapshot() WidgetInfo
}

type Container interface {
	Widget

	AddChild(Widget)
	RemoveChild(Widget)
	Children() []Widget
}

type WidgetBase struct {
	id                  string
	hidden              bool
	focusable           bool
	focused             bool
	containsFocus       bool
	rect                geometry.Rectangle
	parentWidget        Widget
	parentRoot          Root
	children            []Widget
	controllers         []EventController
	layoutManager       layout.LayoutManager
	mount               signal.Signal0
	unmount             signal.Signal0
	focusedSignal       signal.Signal1[bool]
	containsFocusSignal signal.Signal1[bool]
	destroyed           bool
}

func (w *WidgetBase) base() *WidgetBase {
	return w
}

func (w *WidgetBase) ID() string {
	return w.id
}

func (w *WidgetBase) SetID(id string) {
	if w.id != id {
		w.id = id
		w.requestSemanticUpdate()
	}
}

func (w *WidgetBase) Visible() bool {
	return !w.hidden
}

func (w *WidgetBase) SetVisible(visible bool) {
	hidden := !visible
	if w.hidden != hidden {
		w.hidden = hidden
		if hidden {
			if win := w.window(); win != nil && win.focusWithinBase(w) {
				win.SetFocusedWidget(nil)
			}
		}
		w.RequestLayout()
		w.requestSemanticUpdate()
	}
}

func (w *WidgetBase) Focusable() bool {
	return w.focusable
}

func (w *WidgetBase) SetFocusable(focusable bool) {
	if w.focusable == focusable {
		return
	}
	w.focusable = focusable
	if !focusable {
		if win := w.window(); win != nil && win.focusIsBase(w) {
			win.SetFocusedWidget(nil)
		}
	}
	w.requestSemanticUpdate()
}

func (w *WidgetBase) Focused() bool {
	return w.focused
}

func (w *WidgetBase) ContainsFocus() bool {
	return w.containsFocus
}

func (w *WidgetBase) ConnectFocused(fn func(bool)) signal.Handle {
	return w.focusedSignal.Connect(fn)
}

func (w *WidgetBase) ConnectContainsFocus(fn func(bool)) signal.Handle {
	return w.containsFocusSignal.Connect(fn)
}

func (w *WidgetBase) Rect() geometry.Rectangle {
	return w.rect
}

func (w *WidgetBase) Parent() Widget {
	return w.parentWidget
}

func (w *WidgetBase) RemoveChild(child Widget) {
	w.removeChild(child)
}

func (w *WidgetBase) Children() []Widget {
	return slices.Clone(w.children)
}

func (w *WidgetBase) Root() Root {
	return w.root()
}

func (w *WidgetBase) Window() Window {
	win, _ := w.root().(Window)
	return win
}

func (w *WidgetBase) EventControllers() []EventController {
	return slices.Clone(w.controllers)
}

func (w *WidgetBase) AddEventController(controller EventController) {
	if controller == nil || slices.Contains(w.controllers, controller) {
		return
	}
	w.controllers = append(w.controllers, controller)
}

func (w *WidgetBase) RemoveEventController(controller EventController) {
	index := slices.Index(w.controllers, controller)
	if index < 0 {
		return
	}
	w.controllers = slices.Delete(w.controllers, index, index+1)
}

func (w *WidgetBase) LayoutManager() layout.LayoutManager {
	return w.layoutManager
}

func (w *WidgetBase) SetLayoutManager(l layout.LayoutManager) {
	w.layoutManager = l
	w.RequestLayout()
}

func (w *WidgetBase) Measure(available geometry.Size) geometry.Size {
	if w.hidden {
		return geometry.Size{}
	}
	if w.layoutManager != nil {
		return w.layoutManager.Measure(w.visibleChildren(), available)
	}
	return geometry.Size{}
}

func (w *WidgetBase) Arrange(rect geometry.Rectangle) {
	w.rect = rect
	if w.layoutManager != nil {
		w.layoutManager.Arrange(w.visibleChildren(), geometry.Rect(0, 0, rect.Width, rect.Height))
	}
}

func (w *WidgetBase) Paint(p Painter) {
	w.PaintChildren(p)
}

func (w *WidgetBase) PaintChildren(p Painter) {
	for _, child := range w.children {
		if child.Visible() {
			child.Paint(SubPainter(p, child.Rect()))
		}
	}
}

func (w *WidgetBase) Snapshot() WidgetInfo {
	info := WidgetInfo{
		ID:            w.ID(),
		Role:          RoleWidget,
		Bounds:        w.windowRect(),
		Visible:       w.Visible(),
		Enabled:       true,
		Focusable:     w.Focusable(),
		Focused:       w.Focused(),
		ContainsFocus: w.ContainsFocus(),
	}
	for _, child := range w.children {
		info.Children = append(info.Children, child.Snapshot())
	}
	return info
}

func (w *WidgetBase) windowRect() geometry.Rectangle {
	rect := w.rect
	for parent := w.parentWidget; parent != nil; parent = parent.Parent() {
		parentRect := parent.Rect()
		rect.X += parentRect.X
		rect.Y += parentRect.Y
	}
	return rect
}

func (w *WidgetBase) RequestLayout() {
	if win := w.window(); win != nil {
		win.requestLayout()
	}
}

func (w *WidgetBase) ConnectMount(fn func()) signal.Handle {
	return w.mount.Connect(fn)
}

func (w *WidgetBase) ConnectUnmount(fn func()) signal.Handle {
	return w.unmount.Connect(fn)
}

func (w *WidgetBase) requestSemanticUpdate() {
	// Reserved for the inspector and automation layers.
}

func (w *WidgetBase) AddChild(parent, child Widget) {
	if parent == nil || parent.base() != w {
		return
	}
	if child == nil {
		return
	}
	child.base().setParent(child, parent)
}

func (w *WidgetBase) removeChild(child Widget) {
	if child == nil || child.base().parentWidget == nil || child.base().parentWidget.base() != w {
		return
	}
	child.base().setParent(child, nil)
}

func (w *WidgetBase) setParent(child, parent Widget) {
	if child == nil || child.base() != w {
		return
	}
	if child == nil || child == parent {
		return
	}
	if parent != nil && parent.base().isDescendant(parent, child) {
		return
	}

	childBase := child.base()
	if childBase.destroyed {
		return
	}
	if childBase.parentWidget == parent && childBase.parentRoot == nil {
		return
	}
	if parent != nil && parent.base().destroyed {
		return
	}

	oldRoot := child.Root()
	var newRoot Root
	if parent != nil {
		newRoot = parent.Root()
	}
	rootChanged := oldRoot != newRoot

	if rootChanged && oldRoot != nil {
		w.emitUnmountSubtree(child)
		if win, ok := oldRoot.(*window); ok && win.focusWithinWidget(child) {
			win.SetFocusedWidget(nil)
		}
	}
	w.detach(child)

	if parent != nil {
		w.attachChild(parent, child)
	}
	if !rootChanged && oldRoot != nil {
		if win, ok := oldRoot.(*window); ok && win.focusWithinWidget(child) {
			win.SetFocusedWidget(win.FocusedWidget())
		}
	}
	if rootChanged && newRoot != nil {
		w.emitMountSubtree(child)
	}
}

func (w *WidgetBase) root() Root {
	if w == nil {
		return nil
	}
	if w.parentRoot != nil {
		return w.parentRoot
	}
	if w.parentWidget != nil {
		return w.parentWidget.Root()
	}
	return nil
}

func (w *WidgetBase) window() *window {
	win, _ := w.root().(*window)
	return win
}

func (w *WidgetBase) attachRoot(root Root, child Widget) {
	if child == nil || child.base() != w {
		return
	}
	w.parentWidget = nil
	w.parentRoot = root
	w.emitMountSubtree(child)
}

func (w *WidgetBase) detachRoot(child Widget) {
	if child == nil || child.base() != w {
		return
	}
	oldRoot := child.Root()
	if oldRoot != nil {
		w.emitUnmountSubtree(child)
		if win, ok := oldRoot.(*window); ok && win.focusWithinWidget(child) {
			win.SetFocusedWidget(nil)
		}
	}
	w.detach(child)
}

func (w *WidgetBase) attachChild(parent, child Widget) {
	if child == nil || child.base() != w {
		return
	}
	parentBase := parent.base()
	w.parentWidget = parent
	w.parentRoot = nil
	parentBase.children = append(parentBase.children, child)
	parent.RequestLayout()
	parentBase.requestSemanticUpdate()
}

func (w *WidgetBase) detach(child Widget) {
	if child == nil || child.base() != w {
		return
	}
	if w.parentWidget != nil {
		parent := w.parentWidget
		parentBase := parent.base()
		index := slices.Index(parentBase.children, child)
		if index >= 0 {
			parentBase.children = slices.Delete(parentBase.children, index, index+1)
		}
		w.parentWidget = nil
		parent.RequestLayout()
		parentBase.requestSemanticUpdate()
		return
	}
	if w.parentRoot != nil {
		root := w.parentRoot
		w.parentRoot = nil
		if win, ok := root.(*window); ok && win.root == child {
			win.root = nil
			win.requestLayout()
		}
	}
}

func (w *WidgetBase) destroy(widget Widget) {
	if widget == nil || widget.base() != w || w.destroyed {
		return
	}

	oldRoot := widget.Root()
	if oldRoot != nil {
		w.emitUnmountSubtree(widget)
		if win, ok := oldRoot.(*window); ok && win.focusWithinWidget(widget) {
			win.SetFocusedWidget(nil)
		}
	}
	w.detach(widget)

	w.destroyed = true
	for _, child := range slices.Clone(w.children) {
		child.base().destroy(child)
	}

	w.children = nil
	w.controllers = nil
	w.layoutManager = nil
}

func (w *WidgetBase) emitMountSubtree(widget Widget) {
	if widget == nil || widget.base() != w {
		return
	}
	w.mount.Emit()
	for _, child := range slices.Clone(w.children) {
		child.base().emitMountSubtree(child)
	}
}

func (w *WidgetBase) emitUnmountSubtree(widget Widget) {
	if widget == nil || widget.base() != w {
		return
	}
	for _, child := range slices.Clone(w.children) {
		child.base().emitUnmountSubtree(child)
	}
	w.unmount.Emit()
}

func (w *WidgetBase) isDescendant(widget, ancestor Widget) bool {
	for widget != nil {
		if widget == ancestor {
			return true
		}
		widget = widget.Parent()
	}
	return false
}

func (w *WidgetBase) setFocused(focused bool) {
	if w.focused == focused {
		return
	}
	w.focused = focused
	w.focusedSignal.Emit(focused)
	w.requestSemanticUpdate()
}

func (w *WidgetBase) setContainsFocus(containsFocus bool) {
	if w.containsFocus == containsFocus {
		return
	}
	w.containsFocus = containsFocus
	w.containsFocusSignal.Emit(containsFocus)
	w.requestSemanticUpdate()
}

func (w *WidgetBase) handleCrossing(ctx CrossingContext) {
	if ctx.Type() != CrossingFocus {
		return
	}
	switch ctx.Mode() {
	case CrossingTarget:
		w.setFocused(ctx.Direction() == CrossingEnter)
	case CrossingContains:
		w.setContainsFocus(ctx.Direction() == CrossingEnter)
	}
}

func (w *WidgetBase) visibleChildren() []layout.Child {
	children := make([]layout.Child, 0, len(w.children))
	for _, child := range w.children {
		if child.Visible() {
			children = append(children, child)
		}
	}
	return children
}
