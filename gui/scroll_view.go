package gui

import (
	"math"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/core/signal"
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

// ScrollIntoViewRequester is optional scrolling-content behavior. Requests use
// content-local DIP coordinates before subtracting the scroll offset. A
// ScrollView connects to its content automatically; the content does not need
// to know its host or write the host's scroll state.
type ScrollIntoViewRequester interface {
	ConnectScrollIntoView(func(geometry.Rectangle)) signal.Handle
}

// ScrollDirection selects the axes available for scrolling. Single-axis modes
// measure ordinary content at the viewport's cross-axis size; Scrollable
// content retains responsibility for its own layout and wrapping policy.
type ScrollDirection uint8

const (
	// ScrollBoth preserves ordinary content's natural width and height.
	ScrollBoth ScrollDirection = iota
	// ScrollVertical fits ordinary content to the viewport width.
	ScrollVertical
	// ScrollHorizontal fits ordinary content to the viewport height.
	ScrollHorizontal
)

// ScrollView is a single-content scrolling container. The content may be a
// Scrollable (virtualized, e.g. ListView) or an ordinary widget tree (viewport
// mode: the tree is arranged at a negative offset and clipped to the viewport).
//
// ScrollView is a Bin: SetChild/Child manage the content slot. The two
// ScrollBars and the content viewport are internal structural children; they
// are not exposed through Child/SetChild.
type ScrollView struct {
	WidgetBase
	content        Widget
	viewport       *scrollViewport
	direction      ScrollDirection
	scrollY        float32
	scrollX        float32 // horizontal scroll offset
	contentWidth   float32 // measured content or Scrollable extent
	contentHeight  float32 // refreshed at the actual viewport during Arrange
	lineHeight     float32 // WheelDeltaLine height; 0 = default text line height
	wheel          *WheelEventController
	vbar           *ScrollBar // vertical scrollbar (right column)
	hbar           *ScrollBar // horizontal scrollbar (bottom row)
	revealHandle   signal.Handle
	revealRect     geometry.Rectangle
	revealPending  bool
	layoutDepth    int
	revealing      bool
	dragAutoScroll bool
	dragMotion     *DragMotionEventController
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

// NewScrollView creates a viewport with MainWeight 1 so it shares a linear
// parent's remaining space. SetMainWeight(0) opts out of that allocation.
func NewScrollView() *ScrollView {
	sv := &ScrollView{dragAutoScroll: true}
	sv.SetMainWeight(1)

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
	sv.wheel.ConnectScroll(func(ctx EventContext, e events.WheelEvent) {
		dx, dy := e.DeltaX, e.DeltaY
		if dy != 0 && e.Modifiers&events.ModifierShift != 0 {
			dx, dy = dy, 0
		}
		extent := sv.scrollRange()
		if (dx == 0 || extent.Width <= 0) && (dy == 0 || extent.Height <= 0) {
			return
		}
		// Own the whole event whenever a requested axis has a scroll range,
		// even at its boundary or when rounding leaves the offset unchanged.
		ctx.StopPropagation()
		sv.SetScrollX(sv.ScrollX() + sv.wheelDelta(dx, e.Mode))
		sv.SetScrollY(sv.ScrollY() + sv.wheelDelta(dy, e.Mode))
	})
	sv.AddEventController(sv.wheel)
	sv.dragMotion = NewDragMotionEventController()
	sv.dragMotion.passive = true
	sv.AddEventController(sv.dragMotion)
	sv.ConnectMount(sv.connectReveal)
	sv.ConnectUnmount(sv.disconnectReveal)
	return sv
}

// Direction returns the scrolling axes, initially ScrollBoth.
func (sv *ScrollView) Direction() ScrollDirection { return sv.direction }

// SetDirection selects the scrolling axes and requests layout. Offsets on a
// disabled axis are reset immediately. Invalid values select ScrollBoth.
func (sv *ScrollView) SetDirection(direction ScrollDirection) {
	if direction > ScrollHorizontal {
		direction = ScrollBoth
	}
	if sv.direction == direction {
		return
	}
	sv.direction = direction
	if !sv.canScrollX() {
		sv.scrollX = 0
	}
	if !sv.canScrollY() {
		sv.scrollY = 0
	}
	sv.RequestLayout()
}

func (sv *ScrollView) canScrollX() bool { return sv.direction != ScrollVertical }
func (sv *ScrollView) canScrollY() bool { return sv.direction != ScrollHorizontal }

// DragAutoScroll reports whether native drag-and-drop scrolls at viewport edges.
func (sv *ScrollView) DragAutoScroll() bool { return sv.dragAutoScroll }
func (sv *ScrollView) SetDragAutoScroll(enabled bool) {
	if sv.dragAutoScroll == enabled {
		return
	}
	sv.dragAutoScroll = enabled
	if h, ok := sv.Root().(interface{ rootState() *rootBase }); ok {
		h.rootState().syncDragScroll()
	}
}

// SetChild sets the scrollable content. It may be a Scrollable (virtualized
// layout) or an ordinary widget tree (viewport mode). Setting a new content
// replaces the previous one.
func (sv *ScrollView) SetChild(content Widget) {
	if sv.content == content {
		return
	}
	sv.disconnectReveal()
	if sv.content != nil {
		sv.viewport.WidgetBase.RemoveChild(sv.content)
	}
	sv.content = content
	if content == nil {
		sv.contentWidth, sv.contentHeight = 0, 0
		sv.scrollX, sv.scrollY = 0, 0
	}
	if content != nil {
		sv.viewport.WidgetBase.AddChild(sv.viewport, content)
	}
	sv.connectReveal()
	sv.RequestLayout()
}

func (sv *ScrollView) connectReveal() {
	if sv.revealHandle != nil || sv.destroyed {
		return
	}
	content := sv.content
	if requester, ok := content.(ScrollIntoViewRequester); ok {
		sv.revealHandle = requester.ConnectScrollIntoView(func(rect geometry.Rectangle) {
			if !sv.destroyed && sv.content == content {
				sv.ScrollIntoView(rect)
			}
		})
	}
}

func (sv *ScrollView) disconnectReveal() {
	if sv.revealHandle != nil {
		sv.revealHandle.Disconnect()
		sv.revealHandle = nil
	}
	sv.revealPending = false
}

// ScrollIntoView minimally scrolls to show rect in unscrolled content-local
// coordinates. For a rectangle larger than the viewport its leading edge is
// preferred. Invalid rectangles are ignored. During layout only the latest
// request is retained and applied after the content extent has been refreshed.
func (sv *ScrollView) ScrollIntoView(rect geometry.Rectangle) {
	if sv.destroyed || sv.content == nil || rect.Width < 0 || rect.Height < 0 {
		return
	}
	for _, v := range []float32{rect.X, rect.Y, rect.Width, rect.Height, rect.X + rect.Width, rect.Y + rect.Height} {
		if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
			return
		}
	}
	sv.revealRect, sv.revealPending = rect, true
	sv.flushReveal()
}

func (sv *ScrollView) flushReveal() {
	if !sv.revealPending || sv.layoutDepth != 0 || sv.revealing || sv.destroyed || sv.content == nil {
		return
	}
	area := sv.contentAreaSize()
	if area.Width <= 0 || area.Height <= 0 {
		return // retain until the first allocation
	}
	sv.revealing = true
	defer func() { sv.revealing = false }()
	extentChanged := sv.refreshContentSize()
	area = sv.contentAreaSize()
	rect := sv.revealRect
	sv.revealPending = false
	oldX, oldY := sv.scrollX, sv.scrollY
	sv.scrollX = revealOffset(oldX, area.Width, rect.X, rect.Width)
	sv.scrollY = revealOffset(oldY, area.Height, rect.Y, rect.Height)
	sv.clampScroll()
	if extentChanged || oldX != sv.scrollX || oldY != sv.scrollY {
		sv.Arrange(sv.Rect())
		sv.RequestPaint()
	}
	// A layout may revise its request after measuring new content. Process it
	// on the next layout, not recursively on this stack.
	if sv.revealPending {
		sv.RequestLayout()
	}
}

func revealOffset(offset, viewport, start, size float32) float32 {
	if start < offset {
		return float32(math.Floor(float64(start)))
	}
	if start+size > offset+viewport {
		return float32(math.Ceil(float64(min(start, start+size-viewport))))
	}
	return offset
}

func (sv *ScrollView) finishContentLayout() {
	sv.layoutDepth--
	sv.flushReveal()
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
	if !sv.canScrollY() {
		return
	}
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
	if !sv.canScrollX() {
		return
	}
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
	return sv.canScrollY() && sv.contentHeight > sv.contentAreaSize().Height
}

// vScrollable reports whether the vertical scrollbar should be shown.
func (sv *ScrollView) vScrollable() bool {
	_, _, vertical := sv.scrollGeometry()
	return vertical
}

// hScrollable reports whether the horizontal scrollbar should be shown.
func (sv *ScrollView) hScrollable() bool {
	_, horizontal, _ := sv.scrollGeometry()
	return horizontal
}

// scrollGeometry resolves the two bars together, starting without bars so a
// previous allocation cannot keep unnecessary bars alive. Each step can only
// add a bar; two axes need at most three evaluations to settle their occupancy.
func (sv *ScrollView) scrollGeometry() (area geometry.Size, horizontal, vertical bool) {
	size := sv.Rect().Size
	for range 3 {
		area = scrollArea(size, horizontal, vertical)
		h := horizontal || sv.content != nil && sv.canScrollX() && sv.contentWidth > area.Width
		v := vertical || sv.content != nil && sv.canScrollY() && sv.contentHeight > area.Height
		if h == horizontal && v == vertical {
			return
		}
		horizontal, vertical = h, v
	}
	area = scrollArea(size, horizontal, vertical)
	return
}

func scrollArea(size geometry.Size, horizontal, vertical bool) geometry.Size {
	if horizontal {
		size.Height -= scrollbarWidth
	}
	if vertical {
		size.Width -= scrollbarWidth
	}
	size.Width = max(0, size.Width)
	size.Height = max(0, size.Height)
	return size
}

// contentAreaSize returns the content area dimensions: the full viewport minus
// the space occupied by visible scrollbars.
func (sv *ScrollView) contentAreaSize() geometry.Size {
	area, _, _ := sv.scrollGeometry()
	return area
}

func (sv *ScrollView) scrollRange() geometry.Size {
	area := sv.contentAreaSize()
	var extent geometry.Size
	if sv.canScrollX() {
		extent.Width = max(0, sv.contentWidth-area.Width)
	}
	if sv.canScrollY() {
		extent.Height = max(0, sv.contentHeight-area.Height)
	}
	return extent
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
	extent := sv.scrollRange()
	info.MaxScrollY, info.MaxScrollX = extent.Height, extent.Width
	// Filter out the scrollbars: the snapshot exposes only the content child
	// so automation sees the real content tree, not structural bars.
	info.Children = nil
	if sv.content != nil {
		info.Children = append(info.Children, sv.content.Snapshot())
	}
	return info
}

func (sv *ScrollView) Measure(c layout.Constraint) layout.Measurement {
	if !sv.Visible() || sv.content == nil {
		return layout.Measurement{}
	}
	if sc, ok := sv.content.(Scrollable); ok {
		if !ensureWidgetStyle(sc) {
			return layout.Measurement{}
		}
		size := sc.ContentSize()
		sv.contentWidth, sv.contentHeight = size.Width, size.Height
		// A Scrollable fills its viewport's cross axis: the viewport is always
		// sized from the parent constraint, not from ContentSize().Width. The
		// reported content width drives the horizontal scrollbar (hScrollable),
		// never the size of the viewport itself — feeding a content or
		// bar-influenced width back here would shrink the viewport each layout.
		return layout.Measured(sv.constrain(c, geometry.Size{
			Width:  sv.viewportWidth(c),
			Height: sv.viewportHeight(c),
		}))
	}
	contentSize := sv.measureContent(sv.layoutConstraint(c).Max)
	size := geometry.Size{Width: contentSize.Width, Height: sv.viewportHeight(c)}
	if sv.direction == ScrollHorizontal {
		size.Width, size.Height = sv.viewportWidth(c), contentSize.Height
	}
	return layout.Measured(sv.constrain(c, size))
}

// measureContent releases scrolling axes, keeping a finite non-scrolling
// axis tight. With an unbounded cross axis there is no invented viewport size.
func (sv *ScrollView) measureContent(viewport geometry.Size) geometry.Size {
	c := layout.Unbounded()
	if sv.direction == ScrollVertical && viewport.Width < layout.Inf {
		c.Min.Width, c.Max.Width = max(0, viewport.Width), max(0, viewport.Width)
	}
	if sv.direction == ScrollHorizontal && viewport.Height < layout.Inf {
		c.Min.Height, c.Max.Height = max(0, viewport.Height), max(0, viewport.Height)
	}
	size := measureWidget(sv.content, c).Constrain(c).Size
	sv.contentWidth, sv.contentHeight = size.Width, size.Height
	return size
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
	sv.layoutDepth++
	defer sv.finishContentLayout()
	sv.WidgetBase.Arrange(rect)
	if _, virtual := sv.content.(Scrollable); sv.content != nil && !virtual {
		// Begin at the full cross extent on each allocation, then remeasure at
		// the bar-reduced extent. Width-sensitive forms wrap at their actual
		// viewport width, including after a resize or content shrink.
		sv.measureContent(rect.Size)
		if sv.direction != ScrollBoth {
			area := sv.contentAreaSize()
			if area != rect.Size {
				sv.measureContent(area)
			}
		}
	}
	sv.clampScroll()
	for range 3 {
		area, x, y := sv.contentAreaSize(), sv.scrollX, sv.scrollY
		sv.arrangeContent()
		if !sv.refreshContentSize() {
			break
		}
		sv.clampScroll()
		if area == sv.contentAreaSize() && x == sv.scrollX && y == sv.scrollY {
			break
		}
		if sv.destroyed {
			return
		}
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
	if !ensureWidgetStyle(sc) {
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
	sv.layoutDepth++
	defer sv.finishContentLayout()
	if !ensureWidgetStyle(sv.content) {
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
	area, hScroll, vScroll := sv.scrollGeometry()

	if vScroll {
		sv.vbar.SetVisible(true)
		sv.vbar.Arrange(geometry.Rect(max(0, rect.Width-barW), 0, min(barW, max(0, rect.Width)), area.Height))
	} else {
		sv.vbar.SetVisible(false)
		sv.vbar.Arrange(geometry.Rect(0, 0, 0, 0))
	}

	if hScroll {
		sv.hbar.SetVisible(true)
		sv.hbar.Arrange(geometry.Rect(0, max(0, rect.Height-barW), area.Width, min(barW, max(0, rect.Height))))
	} else {
		sv.hbar.SetVisible(false)
		sv.hbar.Arrange(geometry.Rect(0, 0, 0, 0))
	}
}

// syncBars pushes the current scroll state into the scrollbars so their thumbs
// reflect the actual offset and range.
func (sv *ScrollView) syncBars() {
	extent := sv.scrollRange()
	sv.vbar.SetMax(extent.Height)
	sv.vbar.SetValue(sv.scrollY)
	sv.hbar.SetMax(extent.Width)
	sv.hbar.SetValue(sv.scrollX)
}

// clampScroll keeps the scroll offsets within [0, content - contentArea], with
// whole-pixel boundaries so the content never lands on fractional coordinates.
func (sv *ScrollView) clampScroll() {
	extent := sv.scrollRange()
	maxY := mathx.Floor(extent.Height)
	if sv.scrollY > maxY {
		sv.scrollY = maxY
	}
	if sv.scrollY < 0 {
		sv.scrollY = 0
	}
	maxX := mathx.Floor(extent.Width)
	if sv.scrollX > maxX {
		sv.scrollX = maxX
	}
	if sv.scrollX < 0 {
		sv.scrollX = 0
	}
}
