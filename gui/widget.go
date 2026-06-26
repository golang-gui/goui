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

	ID() string
	SetID(string)

	Visible() bool
	SetVisible(bool)

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

type WidgetBase struct {
	id            string
	hidden        bool
	rect          geometry.Rectangle
	parentWidget  Widget
	parentRoot    Root
	children      []Widget
	controllers   []EventController
	layoutManager layout.LayoutManager
	mount         signal.Signal0
	unmount       signal.Signal0
	destroyed     bool
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
		w.RequestLayout()
		w.requestSemanticUpdate()
	}
}

func (w *WidgetBase) Rect() geometry.Rectangle {
	return w.rect
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
		ID:      w.ID(),
		Role:    RoleWidget,
		Bounds:  w.windowRect(),
		Visible: w.Visible(),
		Enabled: true,
	}
	for _, child := range w.children {
		info.Children = append(info.Children, child.Snapshot())
	}
	return info
}

func (w *WidgetBase) windowRect() geometry.Rectangle {
	rect := w.rect
	for parent := w.parentWidget; parent != nil; parent = Parent(parent) {
		parentRect := parent.Rect()
		rect.X += parentRect.X
		rect.Y += parentRect.Y
	}
	return rect
}

func (w *WidgetBase) RequestLayout() {
	if win := getWindowFromBase(w); win != nil {
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

func baseOf(widget Widget) *WidgetBase {
	if widget == nil {
		return nil
	}
	return widget.base()
}

func Parent(widget Widget) Widget {
	if base := baseOf(widget); base != nil {
		return base.parentWidget
	}
	return nil
}

func GetRoot(widget Widget) Root {
	for widget != nil {
		base := baseOf(widget)
		if base == nil {
			return nil
		}
		if base.parentRoot != nil {
			return base.parentRoot
		}
		widget = base.parentWidget
	}
	return nil
}

func GetWindow(widget Widget) Window {
	root := GetRoot(widget)
	if root == nil {
		return nil
	}
	win, _ := root.(Window)
	return win
}

func GetChildren(widget Widget) []Widget {
	if base := baseOf(widget); base != nil {
		return slices.Clone(base.children)
	}
	return nil
}

func SetParent(child, parent Widget) {
	if child == nil || child == parent {
		return
	}
	if parent != nil && isDescendant(parent, child) {
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

	oldRoot := GetRoot(child)
	newRoot := GetRoot(parent)
	rootChanged := oldRoot != newRoot

	detachWidget(child)
	if rootChanged && oldRoot != nil {
		emitUnmountSubtree(child)
	}

	if parent != nil {
		attachChild(parent, child)
	}
	if rootChanged && newRoot != nil {
		emitMountSubtree(child)
	}
}

func AddChild(parent, child Widget) {
	if parent == nil {
		return
	}
	SetParent(child, parent)
}

func RemoveChild(parent, child Widget) {
	if parent == nil || child == nil || Parent(child) != parent {
		return
	}
	SetParent(child, nil)
}

func rootOfBase(base *WidgetBase) Root {
	if base == nil {
		return nil
	}
	if base.parentRoot != nil {
		return base.parentRoot
	}
	return GetRoot(base.parentWidget)
}

func getWindowFromBase(base *WidgetBase) *window {
	win, _ := rootOfBase(base).(*window)
	return win
}

func attachRoot(root Root, child Widget) {
	base := child.base()
	base.parentWidget = nil
	base.parentRoot = root
	emitMountSubtree(child)
}

func detachRoot(child Widget) {
	oldRoot := GetRoot(child)
	detachWidget(child)
	if oldRoot != nil {
		emitUnmountSubtree(child)
	}
}

func attachChild(parent, child Widget) {
	parentBase := parent.base()
	childBase := child.base()
	childBase.parentWidget = parent
	childBase.parentRoot = nil
	parentBase.children = append(parentBase.children, child)
	parent.RequestLayout()
	parentBase.requestSemanticUpdate()
}

func detachWidget(child Widget) {
	base := child.base()
	if base.parentWidget != nil {
		parent := base.parentWidget
		parentBase := parent.base()
		index := slices.Index(parentBase.children, child)
		if index >= 0 {
			parentBase.children = slices.Delete(parentBase.children, index, index+1)
		}
		base.parentWidget = nil
		parent.RequestLayout()
		parentBase.requestSemanticUpdate()
		return
	}
	if base.parentRoot != nil {
		root := base.parentRoot
		base.parentRoot = nil
		if win, ok := root.(*window); ok && win.root == child {
			win.root = nil
			win.requestLayout()
		}
	}
}

func destroyWidget(widget Widget) {
	base := widget.base()
	if base.destroyed {
		return
	}

	oldRoot := GetRoot(widget)
	detachWidget(widget)
	if oldRoot != nil {
		emitUnmountSubtree(widget)
	}

	base.destroyed = true
	for _, child := range slices.Clone(base.children) {
		destroyWidget(child)
	}

	base.children = nil
	base.controllers = nil
	base.layoutManager = nil
}

func emitMountSubtree(widget Widget) {
	base := widget.base()
	base.mount.Emit()
	for _, child := range slices.Clone(base.children) {
		emitMountSubtree(child)
	}
}

func emitUnmountSubtree(widget Widget) {
	base := widget.base()
	for _, child := range slices.Clone(base.children) {
		emitUnmountSubtree(child)
	}
	base.unmount.Emit()
}

func isDescendant(widget, ancestor Widget) bool {
	for widget != nil {
		if widget == ancestor {
			return true
		}
		widget = Parent(widget)
	}
	return false
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
