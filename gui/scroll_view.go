package gui

import (
	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/layout"
)

// ScrollView is a container that scrolls its first child vertically. The
// content is measured with unconstrained height so it can exceed the viewport.
type ScrollView struct {
	WidgetBase
	scrollY       float32
	contentWidth  float32
	contentHeight float32
}

func NewScrollView() *ScrollView {
	return &ScrollView{}
}

// AddChild adds a child to the scroll view. The first child is the
// scrollable content.
func (sv *ScrollView) AddChild(child Widget) {
	sv.WidgetBase.AddChild(sv, child)
}

// Content returns the scrollable child, or nil.
func (sv *ScrollView) Content() Widget {
	if len(sv.Children()) == 0 {
		return nil
	}
	return sv.Children()[0]
}

func (sv *ScrollView) Measure(c layout.Constraint) geometry.Size {
	if !sv.Visible() || len(sv.Children()) == 0 {
		return geometry.Size{}
	}
	content := sv.Children()[0]
	contentC := layout.Constraint{
		Min: geometry.Size{},
		Max: geometry.Size{Width: c.Max.Width, Height: 1e7},
	}
	contentSize := content.Measure(contentC)
	sv.contentWidth = contentSize.Width
	sv.contentHeight = contentSize.Height
	return sv.constrain(c, c.Max)
}

func (sv *ScrollView) Arrange(rect geometry.Rectangle) {
	sv.WidgetBase.Arrange(rect)
	sv.arrangeContent()
}

// arrangeContent positions the content at the current scroll offset.
func (sv *ScrollView) arrangeContent() {
	if content := sv.Content(); content != nil {
		content.Arrange(geometry.Rect(0, -sv.scrollY, sv.contentWidth, sv.contentHeight))
	}
}

// SetScrollY sets the vertical scroll offset and re-arranges the content.
// Only RequestPaint is triggered — no layout or recording pass.
func (sv *ScrollView) SetScrollY(y float32) {
	if sv.scrollY == y {
		return
	}
	sv.scrollY = y
	sv.arrangeContent()
	sv.RequestPaint()
}

func (sv *ScrollView) ScrollY() float32 {
	return sv.scrollY
}

func (sv *ScrollView) Paint(p Painter) {
	if !sv.Visible() {
		return
	}
	sv.PaintChildren(p)
}
