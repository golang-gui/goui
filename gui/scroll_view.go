package gui

import (
	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/events"

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
//
// ScrollView is a Bin: SetChild/Child manage the content slot. The two
// ScrollBars and the content viewport are internal structural children; they
// are not exposed through Child/SetChild.
type ScrollView struct {
	WidgetBase
	content       Widget
	viewport      *scrollViewport
	scrollY       float32
	scrollX       float32 // horizontal scroll offset
	contentWidth  float32 // content size cached by Measure (viewport mode)
	contentHeight float32 // content size cached by Measure (viewport mode)
	lineHeight    float32 // WheelDeltaLine height; 0 = default text line height
	wheel         *WheelEventController
	vbar          *ScrollBar // vertical scrollbar (right column)
	hbar          *ScrollBar // horizontal scrollbar (bottom row)
}

// scrollViewport gives scrolling content a real structural clipping boundary.
// It deliberately has no visual behavior; the GUI traversal paints its child
// automatically and intersects that child's clip with the viewport bounds.
type scrollViewport struct {
	WidgetBase
}

const scrollbarWidth = 8
const scrollbarRadius = scrollbarWidth / 2
const scrollbarMinThumb = 20

func NewScrollView() *ScrollView {
	sv := new(ScrollView)
	sv.SetMainWeight(1) // vertical viewport: flexes in a linear parent

	sv.viewport = new(scrollViewport)
	sv.vbar = NewScrollBar(layout.DirectionVertical)
	sv.hbar = NewScrollBar(layout.DirectionHorizontal)
	sv.vbar.ConnectChange(func(v float32) { sv.SetScrollY(v) })
	sv.hbar.ConnectChange(func(v float32) { sv.SetScrollX(v) })

	// Attach structural children in paint order: content viewport first, then
	// scrollbars above it. setParent bypasses the Bin guard because SetChild /
	// Child describe only the public content slot.
	sv.viewport.base().setParent(sv.viewport, sv)
	sv.vbar.base().setParent(sv.vbar, sv)
	sv.hbar.base().setParent(sv.hbar, sv)

	sv.wheel = NewWheelEventController()
	sv.wheel.ConnectScroll(func(_ EventContext, e events.WheelEvent) {
		dx, dy := e.DeltaX, e.DeltaY
		if dy != 0 && e.Modifiers&events.ModifierShift != 0 {
			dx, dy = dy, 0
		}
		sv.SetScrollX(sv.ScrollX() + sv.wheelDelta(dx, e.Mode))
		sv.SetScrollY(sv.ScrollY() + sv.wheelDelta(dy, e.Mode))
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
		sv.viewport.WidgetBase.RemoveChild(sv.content)
	}
	sv.content = content
	if content != nil {
		sv.viewport.WidgetBase.AddChild(sv.viewport, content)
	}
	sv.RequestLayout()
}

// Child returns the scrollable content, or nil.
func (sv *ScrollView) Child() Widget {
	return sv.content
}

// SetStyleName pushes the style name down to both scrollbars so ScrollView and
// ScrollBar theme as a single unit.
func (sv *ScrollView) SetStyleName(name string) {
	sv.WidgetBase.SetStyleName(name)
	sv.vbar.SetStyleName(name)
	sv.hbar.SetStyleName(name)
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

func (sv *ScrollView) wheelDelta(delta float32, mode events.WheelDeltaMode) float32 {
	if mode == events.WheelDeltaLine {
		return delta * sv.lineHeightPx()
	}
	return delta
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
	sv.syncBars()
	sv.RequestPaint()
}

func (sv *ScrollView) ScrollY() float32 {
	return sv.scrollY
}

// SetScrollX sets the horizontal scroll offset, rounded to whole logical pixels.
func (sv *ScrollView) SetScrollX(x float32) {
	x = mathx.Round(x)
	if sv.scrollX == x {
		return
	}
	sv.scrollX = x
	sv.clampScroll()
	sv.arrangeContent()
	sv.syncBars()
	sv.RequestPaint()
}

func (sv *ScrollView) ScrollX() float32 {
	return sv.scrollX
}

// Scrollable reports whether the content can scroll vertically (content taller
// than the content area).
func (sv *ScrollView) Scrollable() bool {
	return sv.contentHeight > sv.contentAreaSize().Height
}

// vScrollable reports whether the vertical scrollbar should be shown.
func (sv *ScrollView) vScrollable() bool {
	return sv.content != nil && sv.contentHeight > sv.Rect().Height
}

// hScrollable reports whether the horizontal scrollbar should be shown.
func (sv *ScrollView) hScrollable() bool {
	return sv.content != nil && sv.contentWidth > sv.Rect().Width
}

// contentAreaSize returns the content area dimensions: the full viewport minus
// the space occupied by visible scrollbars.
func (sv *ScrollView) contentAreaSize() geometry.Size {
	r := sv.Rect()
	w := r.Width
	h := r.Height
	if sv.hScrollable() {
		h -= scrollbarWidth
	}
	if sv.vScrollable() {
		w -= scrollbarWidth
	}
	return geometry.Size{Width: w, Height: h}
}

// contentAreaRect returns the content area in ScrollView-local coordinates.
func (sv *ScrollView) contentAreaRect() geometry.Rectangle {
	return geometry.Rect(0, 0, sv.contentAreaSize().Width, sv.contentAreaSize().Height)
}

func (sv *ScrollView) Snapshot() WidgetInfo {
	info := sv.WidgetBase.Snapshot()
	info.Role = RoleScrollView
	info.ScrollY = sv.scrollY
	info.ScrollX = sv.scrollX
	if max := sv.contentHeight - sv.contentAreaSize().Height; max > 0 {
		info.MaxScrollY = max
	}
	if max := sv.contentWidth - sv.contentAreaSize().Width; max > 0 {
		info.MaxScrollX = max
	}
	// Filter out the scrollbars: the snapshot exposes only the content child
	// so automation sees the real content tree, not structural bars.
	info.Children = nil
	if sv.content != nil {
		info.Children = append(info.Children, sv.content.Snapshot())
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
		// A Scrollable fills its viewport's cross axis: the viewport is always
		// sized from the parent constraint, not from ContentSize().Width. The
		// reported content width drives the horizontal scrollbar (hScrollable),
		// never the size of the viewport itself — feeding a content or
		// bar-influenced width back here would shrink the viewport each layout.
		return sv.constrain(c, geometry.Size{
			Width:  sv.viewportWidth(c),
			Height: sv.viewportHeight(c),
		})
	}
	contentC := layout.Constraint{
		Min: geometry.Size{},
		Max: geometry.Size{Width: layout.Inf, Height: layout.Inf},
	}
	contentSize := sv.content.Measure(contentC)
	sv.contentWidth, sv.contentHeight = contentSize.Width, contentSize.Height
	return sv.constrain(c, geometry.Size{
		Width:  contentSize.Width,
		Height: sv.viewportHeight(c),
	})
}

func (sv *ScrollView) viewportWidth(c layout.Constraint) float32 {
	if c.Max.Width >= layout.Inf {
		return 0
	}
	return c.Max.Width
}

func (sv *ScrollView) viewportHeight(c layout.Constraint) float32 {
	if c.Max.Height >= layout.Inf {
		return 0
	}
	return c.Max.Height
}

func (sv *ScrollView) Arrange(rect geometry.Rectangle) {
	sv.WidgetBase.Arrange(rect)
	sv.clampScroll()
	sv.arrangeContent()
	if sv.refreshContentSize() {
		// A virtualized content (e.g. ListView) reports its real extent only
		// after its items are measured during LayoutVisible above, so Measure's
		// cached value is stale on the first pass. Re-clamp and re-layout so
		// the content area and scrollbars reflect the true extent.
		sv.clampScroll()
		sv.arrangeContent()
	}
	sv.viewport.Arrange(sv.contentAreaRect())
	sv.layoutScrollBars(rect)
	sv.syncBars()
}

// refreshContentSize re-reads a Scrollable content's extent after layout and
// returns whether the cached extent changed. Virtualized content measures its
// items lazily (during LayoutVisible), so this catches the first-pass growth.
func (sv *ScrollView) refreshContentSize() bool {
	sc, ok := sv.content.(Scrollable)
	if !ok {
		return false
	}
	size := sc.ContentSize()
	if size.Width == sv.contentWidth && size.Height == sv.contentHeight {
		return false
	}
	sv.contentWidth, sv.contentHeight = size.Width, size.Height
	return true
}

func (sv *ScrollView) arrangeContent() {
	if sv.content == nil {
		return
	}
	area := sv.contentAreaSize()
	if sc, ok := sv.content.(Scrollable); ok {
		sv.content.Arrange(geometry.Rect(0, 0, area.Width, area.Height))
		sc.LayoutVisible(area, geometry.Point{X: sv.scrollX, Y: sv.scrollY})
	} else {
		sv.content.Arrange(geometry.Rect(-sv.scrollX, -sv.scrollY, sv.contentWidth, sv.contentHeight))
	}
}

// layoutScrollBars positions the scrollbars and toggles their visibility
// based on scrollability. A hidden bar is arranged to a zero-size rect.
func (sv *ScrollView) layoutScrollBars(rect geometry.Rectangle) {
	barW := float32(scrollbarWidth)
	hScroll := sv.hScrollable()
	vScroll := sv.vScrollable()

	if vScroll {
		sv.vbar.SetVisible(true)
		sv.vbar.Arrange(geometry.Rect(rect.Width-barW, 0, barW, rect.Height-boolHeight(hScroll, barW)))
	} else {
		sv.vbar.SetVisible(false)
		sv.vbar.Arrange(geometry.Rect(0, 0, 0, 0))
	}

	if hScroll {
		sv.hbar.SetVisible(true)
		sv.hbar.Arrange(geometry.Rect(0, rect.Height-barW, rect.Width-boolWidth(vScroll, barW), barW))
	} else {
		sv.hbar.SetVisible(false)
		sv.hbar.Arrange(geometry.Rect(0, 0, 0, 0))
	}
}

func boolHeight(b bool, barW float32) float32 {
	if b {
		return barW
	}
	return 0
}

func boolWidth(b bool, barW float32) float32 {
	if b {
		return barW
	}
	return 0
}

// syncBars pushes the current scroll state into the scrollbars so their thumbs
// reflect the actual offset and range.
func (sv *ScrollView) syncBars() {
	area := sv.contentAreaSize()
	maxY := sv.contentHeight - area.Height
	if maxY < 0 {
		maxY = 0
	}
	maxX := sv.contentWidth - area.Width
	if maxX < 0 {
		maxX = 0
	}
	sv.vbar.SetMax(maxY)
	sv.vbar.SetValue(sv.scrollY)
	sv.hbar.SetMax(maxX)
	sv.hbar.SetValue(sv.scrollX)
}

// clampScroll keeps the scroll offsets within [0, content - contentArea], with
// whole-pixel boundaries so the content never lands on fractional coordinates.
func (sv *ScrollView) clampScroll() {
	area := sv.contentAreaSize()
	maxY := mathx.Floor(sv.contentHeight - area.Height)
	if maxY < 0 {
		maxY = 0
	}
	if sv.scrollY > maxY {
		sv.scrollY = maxY
	}
	if sv.scrollY < 0 {
		sv.scrollY = 0
	}
	maxX := mathx.Floor(sv.contentWidth - area.Width)
	if maxX < 0 {
		maxX = 0
	}
	if sv.scrollX > maxX {
		sv.scrollX = maxX
	}
	if sv.scrollX < 0 {
		sv.scrollX = 0
	}
}
