package gui

import (
	"slices"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/layout"
)

type Widget interface {
	Base() *WidgetBase

	ID() string
	SetID(string)

	Visible() bool
	SetVisible(bool)

	Rect() geometry.Rectangle
	Parent() Widget
	Window() Window

	Children() []Widget
	AddChild(Widget)
	RemoveChild(Widget)

	EventControllers() []EventController
	AddEventController(EventController)
	RemoveEventController(EventController)

	LayoutManager() layout.LayoutManager
	SetLayoutManager(layout.LayoutManager)

	Measure(available geometry.Size) geometry.Size
	Arrange(rect geometry.Rectangle)
	Paint(p Painter)
	RequestLayout()

	Snapshot() WidgetInfo
}

type WidgetBase struct {
	owner         Widget
	id            string
	visible       bool
	rect          geometry.Rectangle
	parent        Widget
	window        *window
	children      []Widget
	controllers   []EventController
	layoutManager layout.LayoutManager
}

func (w *WidgetBase) Init(owner Widget) {
	w.owner = owner
	w.visible = true
}

func (w *WidgetBase) Base() *WidgetBase {
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
	return w.visible
}

func (w *WidgetBase) SetVisible(visible bool) {
	if w.visible != visible {
		w.visible = visible
		w.RequestLayout()
		w.requestSemanticUpdate()
	}
}

func (w *WidgetBase) Rect() geometry.Rectangle {
	return w.rect
}

func (w *WidgetBase) Parent() Widget {
	return w.parent
}

func (w *WidgetBase) Window() Window {
	if w.window == nil {
		return nil
	}
	return w.window
}

func (w *WidgetBase) Children() []Widget {
	return slices.Clone(w.children)
}

func (w *WidgetBase) AddChild(child Widget) {
	if child == nil || child == w.self() {
		return
	}
	childBase := child.Base()
	if childBase.parent == w.self() {
		return
	}
	if childBase.parent != nil {
		childBase.parent.RemoveChild(child)
	}
	childBase.parent = w.self()
	childBase.setWindow(w.window)
	w.children = append(w.children, child)
	w.RequestLayout()
	w.requestSemanticUpdate()
}

func (w *WidgetBase) RemoveChild(child Widget) {
	if child == nil {
		return
	}
	index := slices.Index(w.children, child)
	if index < 0 {
		return
	}
	w.children = slices.Delete(w.children, index, index+1)
	childBase := child.Base()
	childBase.parent = nil
	childBase.setWindow(nil)
	w.RequestLayout()
	w.requestSemanticUpdate()
}

func (w *WidgetBase) EventControllers() []EventController {
	return slices.Clone(w.controllers)
}

func (w *WidgetBase) AddEventController(controller EventController) {
	if controller == nil || slices.Contains(w.controllers, controller) {
		return
	}
	w.controllers = append(w.controllers, controller)
	controller.SetWidget(w.self())
}

func (w *WidgetBase) RemoveEventController(controller EventController) {
	index := slices.Index(w.controllers, controller)
	if index < 0 {
		return
	}
	w.controllers = slices.Delete(w.controllers, index, index+1)
	controller.SetWidget(nil)
}

func (w *WidgetBase) LayoutManager() layout.LayoutManager {
	return w.layoutManager
}

func (w *WidgetBase) SetLayoutManager(l layout.LayoutManager) {
	w.layoutManager = l
	w.RequestLayout()
}

func (w *WidgetBase) Measure(available geometry.Size) geometry.Size {
	if !w.visible {
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
		Bounds:  w.Rect(),
		Visible: w.Visible(),
		Enabled: true,
	}
	for _, child := range w.children {
		info.Children = append(info.Children, child.Snapshot())
	}
	return info
}

func (w *WidgetBase) RequestLayout() {
	if w.window != nil {
		w.window.requestLayout()
	}
}

func (w *WidgetBase) requestSemanticUpdate() {
	// Reserved for the inspector and automation layers.
}

func (w *WidgetBase) self() Widget {
	if w.owner != nil {
		return w.owner
	}
	return w
}

func (w *WidgetBase) setWindow(win *window) {
	if w.window == win {
		return
	}
	w.window = win
	for _, controller := range w.controllers {
		controller.SetWidget(w.self())
	}
	for _, child := range w.children {
		child.Base().setWindow(win)
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
