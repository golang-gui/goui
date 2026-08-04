package gui

import (
	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/style"

	"github.com/goexlib/mathx"
)

// Scrollable is implemented by scrollable content: the content knows
// its total size and can lay out only the visible portion for a given viewport
// and scroll offset.
type Scrollable interface {
	Widget
	// ContentSize returns the content's total size (used for clamping the
	// scroll offset and for the scrollbar proportions).
	ContentSize() geometry.Size
	// LayoutVisible lays out the visible portion of the content. viewport is
	// the visible area size; offset is the scroll offset (the content places
	// its children at -offset).
	LayoutVisible(viewport geometry.Size, offset geometry.Point)
}

// ScrollView is a single-content scrolling container. The content may be a
// Scrollable (virtualized, e.g. ListView) or an ordinary widget tree (viewport
// mode: the tree is arranged at a negative offset and clipped to the viewport).
type ScrollView struct {
	WidgetBase
	content       Widget
	scrollY       float32
	contentWidth  float32 // content size cached by Measure (viewport mode)
	contentHeight float32 // content size cached by Measure (viewport mode)
	lineHeight    float32 // WheelDeltaLine height; 0 = default text line height
	wheel         *WheelEventController
}

const scrollbarWidth = 6
const scrollbarMinThumb = 20

func NewScrollView() *ScrollView {
	sv := new(ScrollView)
	sv.SetMainWeight(1) // vertical viewport: flexes in a linear parent
	sv.wheel = NewWheelEventController()
	sv.wheel.ConnectScroll(func(_ EventContext, e events.WheelEvent) {
		if e.Mode == events.WheelDeltaLine {
			sv.SetScrollY(sv.ScrollY() + e.DeltaY*sv.lineHeightPx())
		} else {
			sv.SetScrollY(sv.ScrollY() + e.DeltaY)
		}
	})
	sv.AddEventController(sv.wheel)
	return sv
}

// SetChild sets the scrollable content. It may be a Scrollable (virtualized
// layout) or an ordinary widget tree (viewport mode). Setting a new content
// replaces the previous one.
func (sv *ScrollView) SetChild(content Widget) {
	if sv.content == content {
		return
	}
	if sv.content != nil {
		sv.WidgetBase.RemoveChild(sv.content)
	}
	sv.content = content
	if content != nil {
		sv.WidgetBase.AddChild(sv, content)
	}
	sv.RequestLayout()
}

// Child returns the scrollable content, or nil.
func (sv *ScrollView) Child() Widget {
	return sv.content
}

// SetLineHeight sets the line height used by WheelDeltaLine wheel events.
// 0 (default) uses the default text line height.
func (sv *ScrollView) SetLineHeight(h float32) {
	if sv.lineHeight == h {
		return
	}
	sv.lineHeight = h
}

func (sv *ScrollView) lineHeightPx() float32 {
	if sv.lineHeight > 0 {
		return sv.lineHeight
	}
	return textLineHeight(defaultFontSize)
}

// SetScrollY sets the vertical scroll offset, rounded to whole logical pixels.
func (sv *ScrollView) SetScrollY(y float32) {
	y = mathx.Round(y)
	if sv.scrollY == y {
		return
	}
	sv.scrollY = y
	sv.clampScroll()
	sv.arrangeContent()
	sv.RequestPaint()
}

func (sv *ScrollView) ScrollY() float32 {
	return sv.scrollY
}

// Scrollable reports whether the content can scroll at all (content taller
// than the viewport).
func (sv *ScrollView) Scrollable() bool {
	return sv.contentHeight > sv.Rect().Height
}

// Snapshot reports the scrollview role and the current scroll state so AI /
// automation can understand position and range. Actions stay implicit: actual
// scrolling is performed by dispatching wheel input.
func (sv *ScrollView) Snapshot() WidgetInfo {
	info := sv.WidgetBase.Snapshot()
	info.Role = RoleScrollView
	info.ScrollY = sv.scrollY
	if max := sv.contentHeight - sv.Rect().Height; max > 0 {
		info.MaxScrollY = max
	}
	return info
}

func (sv *ScrollView) Measure(c layout.Constraint) geometry.Size {
	if !sv.Visible() || sv.content == nil {
		return geometry.Size{}
	}
	if sc, ok := sv.content.(Scrollable); ok {
		size := sc.ContentSize()
		sv.contentWidth, sv.contentHeight = size.Width, size.Height
		// A bounded height is the viewport height, not the content's intrinsic
		// scroll extent. An unbounded height has no intrinsic viewport size.
		return sv.constrain(c, geometry.Size{
			Width:  size.Width,
			Height: sv.viewportHeight(c),
		})
	}
	contentC := layout.Constraint{
		Min: geometry.Size{},
		Max: geometry.Size{Width: c.Max.Width, Height: layout.Inf},
	}
	contentSize := sv.content.Measure(contentC)
	sv.contentWidth, sv.contentHeight = contentSize.Width, contentSize.Height
	return sv.constrain(c, geometry.Size{
		Width:  contentSize.Width,
		Height: sv.viewportHeight(c),
	})
}

func (sv *ScrollView) viewportHeight(c layout.Constraint) float32 {
	if c.Max.Height >= layout.Inf {
		return 0
	}
	return c.Max.Height
}

func (sv *ScrollView) Arrange(rect geometry.Rectangle) {
	sv.WidgetBase.Arrange(rect)
	if sv.content == nil {
		return
	}
	sv.clampScroll()
	viewport := geometry.Size{Width: rect.Width, Height: rect.Height}
	if sc, ok := sv.content.(Scrollable); ok {
		// The content itself fills the viewport (its own rect must be set for
		// the SubPainter clip to be non-empty); items are laid out by
		// LayoutVisible at -offset.
		sv.content.Arrange(geometry.Rect(0, 0, viewport.Width, viewport.Height))
		sc.LayoutVisible(viewport, geometry.Point{X: 0, Y: sv.scrollY})
	} else {
		sv.content.Arrange(geometry.Rect(0, -sv.scrollY, sv.contentWidth, sv.contentHeight))
	}
}

func (sv *ScrollView) arrangeContent() {
	if sv.content == nil {
		return
	}
	viewport := geometry.Size{Width: sv.Rect().Width, Height: sv.Rect().Height}
	if sc, ok := sv.content.(Scrollable); ok {
		sv.content.Arrange(geometry.Rect(0, 0, viewport.Width, viewport.Height))
		sc.LayoutVisible(viewport, geometry.Point{X: 0, Y: sv.scrollY})
	} else {
		sv.content.Arrange(geometry.Rect(0, -sv.scrollY, sv.contentWidth, sv.contentHeight))
	}
}

// clampScroll keeps the scroll offset within [0, content - viewport], with
// whole-pixel boundaries so the content never lands on fractional coordinates.
func (sv *ScrollView) clampScroll() {
	maxY := mathx.Floor(sv.contentHeight - sv.Rect().Height)
	if maxY < 0 {
		maxY = 0
	}
	if sv.scrollY > maxY {
		sv.scrollY = maxY
	}
	if sv.scrollY < 0 {
		sv.scrollY = 0
	}
}

func (sv *ScrollView) Paint(p Painter) {
	if !sv.Visible() {
		return
	}
	sv.PaintChildren(p)
	sv.paintScrollbar(p)
}

// paintScrollbar draws the in-built scrollbar thumb. It is pure drawing: not
// a widget, no layout participation, no events (no dragging yet).
func (sv *ScrollView) paintScrollbar(p Painter) {
	viewportH := sv.Rect().Height
	if sv.contentHeight <= viewportH {
		return // content fits: no scrollbar
	}
	contentH := sv.contentHeight
	thumbH := viewportH * viewportH / contentH
	if thumbH < scrollbarMinThumb {
		thumbH = scrollbarMinThumb
	}
	thumbY := (viewportH - thumbH) * sv.scrollY / (contentH - viewportH)

	x := sv.Rect().Width - scrollbarWidth - 2
	foreground, _ := sv.resolvedStyle().ForegroundColor()
	thumb := graphics.ColorOf(foreground)
	thumb.A = 140

	// Track: subtle full-height bar.
	trackColor := graphics.Color{R: 0, G: 0, B: 0, A: 24}
	p.FillRect(geometry.Rect(x, 0, scrollbarWidth, viewportH), trackColor)
	// Thumb: rounded rect.
	p.FillRoundRect(geometry.Rect(x, thumbY, scrollbarWidth, thumbH), scrollbarWidth/2, thumb)
}

// resolvedStyle resolves the scrollbar colors from the application style
// sheet (styleNameScrollView), falling back to the built-in default.
func (sv *ScrollView) resolvedStyle() style.Style {
	name := sv.StyleName()
	if name == "" {
		name = styleNameScrollView
	}
	return ResolveStyle(name, style.PartDefault, style.Normal)
}
