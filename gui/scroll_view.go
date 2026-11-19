package gui

import (
	"math"

	"github.com/goexlib/mathx"
	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/style"
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
// Measure follows the child's requested height within the parent's constraints
// and its own MinSize/MaxSize. MainWeight controls remaining-space allocation.
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
	contentWidth   float32 // committed content extent at the allocated viewport
	contentHeight  float32 // Measure must not overwrite the current scroll geometry
	lineHeight     float32 // WheelDeltaLine height; 0 = default text line height
	wheel          *WheelEventController
	vbar           *ScrollBar // vertical scrollbar (right column)
	hbar           *ScrollBar // horizontal scrollbar (bottom row)
	hOptions       ScrollBarOptions
	vOptions       ScrollBarOptions
	geometry       ScrollGeometry
	revealHandle   signal.Handle
	revealRect     geometry.Rectangle
	revealPending  bool
	scrollIntent   uint64 // invalidates requests across external content callbacks
	layoutDepth    int
	revealing      bool
	dragAutoScroll bool
	dragMotion     *DragMotionEventController
	frameInset     float32
}

// scrollViewport gives scrolling content a real structural clipping boundary.
// It deliberately has no visual behavior; the GUI traversal paints its child
// automatically and intersects that child's clip with the viewport bounds.
type scrollViewport struct {
	WidgetBase
}

// ScrollGeometry describes the last completed allocation in ScrollView-local
// DIP. Hidden bars and an absent junction have empty rectangles. Extra spacing
// is outside the bar rectangles and Corner. Measure does not change this value.
type ScrollGeometry struct {
	Viewport   geometry.Rectangle
	HScrollBar geometry.Rectangle
	VScrollBar geometry.Rectangle
	Corner     geometry.Rectangle
}

// ScrollGeometry returns the last completed allocation without forcing layout.
func (sv *ScrollView) ScrollGeometry() ScrollGeometry { return sv.geometry }

// HScrollBarOptions returns the normalized horizontal bar configuration.
func (sv *ScrollView) HScrollBarOptions() ScrollBarOptions { return sv.hOptions }

// SetHScrollBarOptions configures horizontal occupancy and thumb geometry.
// Hiding the bar leaves horizontal scrolling available when Direction allows it.
func (sv *ScrollView) SetHScrollBarOptions(options ScrollBarOptions) {
	sv.setScrollBarOptions(sv.hbar, &sv.hOptions, options)
}

// VScrollBarOptions returns the normalized vertical bar configuration.
func (sv *ScrollView) VScrollBarOptions() ScrollBarOptions { return sv.vOptions }

// SetVScrollBarOptions configures vertical occupancy and thumb geometry.
// Hiding the bar leaves vertical scrolling available when Direction allows it.
func (sv *ScrollView) SetVScrollBarOptions(options ScrollBarOptions) {
	sv.setScrollBarOptions(sv.vbar, &sv.vOptions, options)
}

func (sv *ScrollView) setScrollBarOptions(bar *ScrollBar, current *ScrollBarOptions, options ScrollBarOptions) {
	options = options.normalized()
	if sv.Destroyed() || *current == options {
		return
	}
	cancelGestureController(sv.Root(), bar.drag)
	if sv.Destroyed() {
		return
	}
	*current = options
	bar.SetThickness(options.Thickness)
	bar.SetThumbThickness(options.ThumbThickness)
	sv.RequestLayout()
}

// NewScrollView creates a viewport with MainWeight 1 so it shares a linear
// parent's remaining space. SetMainWeight(0) opts out of that allocation.
func NewScrollView() *ScrollView {
	sv := &ScrollView{dragAutoScroll: true}
	sv.hOptions, sv.vOptions = (ScrollBarOptions{}).normalized(), (ScrollBarOptions{}).normalized()
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
	sv.wheel.ConnectScroll(func(ctx EventContext, e WheelInfo) {
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
	sv.ConnectContainsFocus(func(bool) { sv.RequestPaint() })
	return sv
}

// StyleChanged reserves a stable inner rectangle for every focus state. A
// thicker focus border must not change wrapping or the scroll range.
func (sv *ScrollView) StyleChanged() {
	sv.frameInset = 0
	for _, state := range []style.State{style.Normal, style.Focused, style.FocusVisible, style.Disabled} {
		s := sv.frameStyle(state)
		border, _ := s.BorderWidth()
		radius, _ := s.CornerRadius()
		border = normalizeLayoutValue(border)
		maximum := radius.Max()
		inset := border
		if maximum > 0 {
			// Keep rectangular children inside the rounded border, including an
			// antialiasing margin, without requiring a rounded clipping API.
			inset += max(0, maximum-border)*(1-1/math.Sqrt2) + 1
		}
		sv.frameInset = max(sv.frameInset, inset)
	}
}

func (sv *ScrollView) frameStyle(state style.State) style.Style {
	name := sv.StyleName()
	if name == "" {
		name = styleNameScrollView
	}
	return ResolveWidgetStyle(sv, name, "frame", state)
}

func (sv *ScrollView) contentInset() float32 {
	ensureWidgetStyle(sv)
	return sv.frameInset
}

func (sv *ScrollView) innerRect() geometry.Rectangle {
	return (geometry.Rectangle{Size: sv.Rect().Size}).Inset(sv.contentInset())
}

// Paint draws the shared frame behind the viewport, both bars and their corner.
// Select an explicit StyleName to give the scroll container an editor frame.
func (sv *ScrollView) Paint(p Painter) {
	state := sv.State()
	if !IsEnabled(sv) {
		state = style.Disabled
	} else if sv.FocusVisible() {
		state |= style.FocusVisible
	} else if sv.Focused() || sv.ContainsFocus() {
		state |= style.Focused
		if host, ok := sv.Root().(EventTarget); ok {
			if focused := host.FocusedWidget(); focused != nil && focused.FocusVisible() {
				state |= style.FocusVisible
			}
		}
	}
	s := sv.frameStyle(state)
	r := geometry.Rectangle{Size: sv.Rect().Size}
	if bg, ok := s.BackgroundColor(); ok && bg != nil && graphics.ColorOf(bg).A > 0 {
		PaintStyledBox(p, r, s)
	} else {
		PaintStyledBorder(p, r, s)
	}
	if corner := sv.geometry.Corner; corner.Width > 0 && corner.Height > 0 {
		if ink, ok := sv.vbar.troughStyle().BackgroundColor(); ok && ink != nil {
			p.FillRect(corner, graphics.ColorOf(ink))
		}
	}
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
	sv.cancelReveal()
}

func (sv *ScrollView) cancelReveal() {
	sv.scrollIntent++
	sv.revealPending = false
}

// ScrollIntoView minimally scrolls to show rect in unscrolled content-local
// coordinates. For a rectangle larger than the viewport its leading edge is
// preferred. Invalid rectangles are ignored. During layout only the latest
// request is retained and applied after the content extent has been refreshed.
// A later valid SetScrollX/Y cancels an older request, even at the same offset.
// Large coordinate conversions round toward keeping the target visible.
func (sv *ScrollView) ScrollIntoView(rect geometry.Rectangle) {
	if sv.destroyed || sv.content == nil || rect.Width < 0 || rect.Height < 0 {
		return
	}
	for _, v := range []float32{rect.X, rect.Y, rect.Width, rect.Height, rect.X + rect.Width, rect.Y + rect.Height} {
		if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
			return
		}
	}
	sv.scrollIntent++
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
	defer func() {
		sv.revealing = false
		// A callback may replace this request. Never recurse into user layout.
		if sv.revealPending && !sv.destroyed {
			sv.RequestLayout()
		}
	}()
	intent, rect := sv.scrollIntent, sv.revealRect
	extentChanged := sv.refreshContentSize()
	if sv.destroyed || sv.content == nil || intent != sv.scrollIntent {
		return
	}
	area = sv.contentAreaSize()
	sv.revealPending = false
	oldX, oldY := sv.scrollX, sv.scrollY
	sv.scrollX = revealOffset(oldX, area.Width, rect.X, rect.Width)
	sv.scrollY = revealOffset(oldY, area.Height, rect.Y, rect.Height)
	sv.clampScroll()
	if extentChanged || oldX != sv.scrollX || oldY != sv.scrollY {
		sv.Arrange(sv.Rect())
		sv.RequestPaint()
	}
}

func revealOffset(offset, viewport, start, size float32) float32 {
	first, last := float64(start), float64(start)+float64(size)
	if first < float64(offset) {
		return floorScrollCoordinate(math.Floor(first))
	}
	if last > float64(offset)+float64(viewport) {
		return ceilScrollCoordinate(math.Ceil(min(first, last-float64(viewport))))
	}
	return offset
}

// Directed conversion preserves containment when a document coordinate's
// float32 spacing exceeds one DIP. Exactly representable values stay unchanged.
func ceilScrollCoordinate(value float64) float32 {
	n := float32(value)
	if float64(n) < value {
		n = math.Nextafter32(n, float32(math.Inf(1)))
	}
	return n
}
func floorScrollCoordinate(value float64) float32 {
	n := float32(value)
	if float64(n) > value {
		n = math.Nextafter32(n, float32(math.Inf(-1)))
	}
	return n
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
// A valid request cancels any older pending ScrollIntoView, even at the same offset.
func (sv *ScrollView) SetScrollY(y float32) {
	if sv.destroyed || !sv.canScrollY() || math.IsNaN(float64(y)) || math.IsInf(float64(y), 0) {
		return
	}
	sv.cancelReveal()
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
// A valid request cancels any older pending ScrollIntoView, even at the same offset.
func (sv *ScrollView) SetScrollX(x float32) {
	if sv.destroyed || !sv.canScrollX() || math.IsNaN(float64(x)) || math.IsInf(float64(x), 0) {
		return
	}
	sv.cancelReveal()
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

func (sv *ScrollView) scrollGeometry() (area geometry.Size, horizontal, vertical bool) {
	return sv.scrollGeometryFor(sv.innerRect().Size, geometry.Size{Width: sv.contentWidth, Height: sv.contentHeight})
}

// Resolve both bars from their policies, independently of their old visibility.
// Each pass can only add an Auto bar, so three evaluations settle both axes.
// Measure uses local content metrics; it must not publish a new scroll range.
func (sv *ScrollView) scrollGeometryFor(size, content geometry.Size) (area geometry.Size, horizontal, vertical bool) {
	horizontal = sv.canScrollX() && sv.hOptions.Policy == ScrollBarAlways
	vertical = sv.canScrollY() && sv.vOptions.Policy == ScrollBarAlways
	for range 3 {
		area = sv.scrollArea(size, horizontal, vertical)
		h := horizontal || sv.canScrollX() && sv.hOptions.Policy == ScrollBarAuto && content.Width > area.Width
		v := vertical || sv.canScrollY() && sv.vOptions.Policy == ScrollBarAuto && content.Height > area.Height
		if h == horizontal && v == vertical {
			return
		}
		horizontal, vertical = h, v
	}
	area = sv.scrollArea(size, horizontal, vertical)
	return
}

func (sv *ScrollView) scrollArea(size geometry.Size, horizontal, vertical bool) geometry.Size {
	if horizontal {
		space, _ := sv.hOptions.space(size.Height)
		if size.Height < layout.Inf {
			size.Height -= space
		}
	}
	if vertical {
		space, _ := sv.vOptions.space(size.Width)
		if size.Width < layout.Inf {
			size.Width -= space
		}
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
		extent.Width = scrollExtent(sv.contentWidth, area.Width)
	}
	if sv.canScrollY() {
		extent.Height = scrollExtent(sv.contentHeight, area.Height)
	}
	return extent
}

// scrollExtent contains the far edge on the whole-DIP scroll lattice. Directed
// narrowing to float32 also preserves containment at large coordinates.
// Editors which own or constrain an offset use the same range as ScrollView.
func scrollExtent(content, viewport float32) float32 {
	return ceilScrollCoordinate(math.Ceil(max(0, float64(content)-float64(viewport))))
}

// contentAreaRect returns the content area in ScrollView-local coordinates.
func (sv *ScrollView) contentAreaRect() geometry.Rectangle {
	return geometry.Rectangle{Pos: sv.innerRect().Pos, Size: sv.contentAreaSize()}
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
	inset := sv.contentInset()
	inner := sv.layoutConstraint(c).Inset(inset)
	if sc, ok := sv.content.(Scrollable); ok {
		if !ensureWidgetStyle(sc) {
			return layout.Measurement{}
		}
		// Measure determines the requested viewport; ContentSize remains the
		// independent (possibly virtualized) scrolling extent. Give the content
		// the actual cross width and a loose height so short content can hug.
		childConstraint := inner
		childConstraint.Min.Height = 0
		if childConstraint.Max.Width < layout.Inf {
			childConstraint.Min.Width = childConstraint.Max.Width
		}
		measured := measureWidget(sc, childConstraint)
		if sv.Destroyed() || sv.content != sc || sc.base().destroyed {
			return layout.Measurement{}
		}
		size := sc.ContentSize()
		// A Scrollable fills its viewport's cross axis: the viewport is always
		// sized from the parent constraint, not from ContentSize().Width. The
		// reported content width drives the horizontal scrollbar (hScrollable),
		// never the size of the viewport itself — feeding a content or
		// bar-influenced width back here would shrink the viewport each layout.
		width := sv.viewportWidth(inner)
		if childConstraint.Max.Width >= layout.Inf {
			width = measured.Width
		}
		height := measured.Height
		_, horizontal, vertical := sv.scrollGeometryFor(inner.Max, size)
		if horizontal {
			space, _ := sv.hOptions.space(inner.Max.Height)
			height += space
		}
		if vertical && inner.Max.Width >= layout.Inf {
			space, _ := sv.vOptions.space(inner.Max.Width)
			width += space
		}
		measured.Size = sv.constrain(c, geometry.Size{Width: width + 2*inset, Height: height + 2*inset})
		if measured.HasBaseline {
			measured.Baseline += min(inset, measured.Height/2)
		}
		return measured
	}
	contentSize := sv.measureContent(inner.Max)
	if sv.direction != ScrollBoth {
		// A reserved bar changes wrapping on the non-scrolling axis. Measure
		// that axis at the same width/height Arrange will give the content.
		area, _, _ := sv.scrollGeometryFor(inner.Max, contentSize)
		if area != inner.Max {
			contentSize = sv.measureContent(area)
		}
	}
	size := contentSize
	if sv.direction == ScrollVertical {
		if inner.Max.Width < layout.Inf {
			size.Width = sv.viewportWidth(inner)
		}
	}
	if sv.direction == ScrollHorizontal {
		size.Width, size.Height = sv.viewportWidth(inner), contentSize.Height
	}
	_, horizontal, vertical := sv.scrollGeometryFor(inner.Max, contentSize)
	if horizontal {
		space, _ := sv.hOptions.space(inner.Max.Height)
		size.Height += space
	}
	if vertical && sv.vOptions.Policy == ScrollBarAlways {
		space, _ := sv.vOptions.space(inner.Max.Width)
		size.Width += space
	}
	size.Width += 2 * inset
	size.Height += 2 * inset
	return layout.Measured(sv.constrain(c, size))
}

// measureContent releases scrolling axes, keeping a finite non-scrolling
// axis tight. With an unbounded cross axis there is no invented viewport size.
// Measurement queries do not publish new geometry for an existing allocation.
func (sv *ScrollView) measureContent(viewport geometry.Size) geometry.Size {
	c := layout.Unbounded()
	if sv.direction == ScrollVertical && viewport.Width < layout.Inf {
		c.Min.Width, c.Max.Width = max(0, viewport.Width), max(0, viewport.Width)
	}
	if sv.direction == ScrollHorizontal && viewport.Height < layout.Inf {
		c.Min.Height, c.Max.Height = max(0, viewport.Height), max(0, viewport.Height)
	}
	return measureWidget(sv.content, c).Constrain(c).Size
}

func (sv *ScrollView) viewportWidth(c layout.Constraint) float32 {
	if c.Max.Width >= layout.Inf {
		return 0
	}
	return c.Max.Width
}

func (sv *ScrollView) Arrange(rect geometry.Rectangle) {
	sv.layoutDepth++
	defer sv.finishContentLayout()
	sv.WidgetBase.Arrange(rect)
	inner := sv.innerRect()
	if sc, virtual := sv.content.(Scrollable); virtual {
		if ensureWidgetStyle(sc) {
			// Seed first-pass growth, but retain the allocated extent until
			// LayoutVisible refreshes it. Editors can temporarily report a small
			// estimate while replacing text or deferring width-sensitive work.
			// Dropping the old extent here would remove a bar for one pass and
			// lay out the retained text at the wrong width.
			size := sc.ContentSize()
			sv.contentWidth = max(sv.contentWidth, size.Width)
			sv.contentHeight = max(sv.contentHeight, size.Height)
		}
	} else if sv.content != nil {
		// Begin at the full cross extent on each allocation, then remeasure at
		// the bar-reduced extent. Width-sensitive forms wrap at their actual
		// viewport width, including after a resize or content shrink.
		size := sv.measureContent(inner.Size)
		sv.contentWidth, sv.contentHeight = size.Width, size.Height
		if sv.direction != ScrollBoth {
			area := sv.contentAreaSize()
			if area != inner.Size {
				size = sv.measureContent(area)
				sv.contentWidth, sv.contentHeight = size.Width, size.Height
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
	sv.layoutScrollBars(inner)
	sv.syncBars()
}

// refreshContentSize publishes a Scrollable content's extent for layout or a
// reveal request. LayoutVisible may measure items lazily, so layout also calls
// this after arranging to catch first-pass growth.
func (sv *ScrollView) refreshContentSize() bool {
	sc, ok := sv.content.(Scrollable)
	if !ok {
		return false
	}
	if !ensureWidgetStyle(sc) {
		return false
	}
	size := sc.ContentSize()
	if sv.destroyed || sv.content != sc || sc.base().destroyed {
		return false
	}
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
	area, hScroll, vScroll := sv.scrollGeometry()
	g := ScrollGeometry{Viewport: geometry.Rectangle{Pos: rect.Pos, Size: area}}

	if vScroll {
		_, thickness := sv.vOptions.space(rect.Width)
		g.VScrollBar = geometry.Rect(rect.X+rect.Width-thickness, rect.Y, thickness, area.Height)
		sv.vbar.SetVisible(true)
		sv.vbar.Arrange(g.VScrollBar)
	} else {
		sv.vbar.SetVisible(false)
		sv.vbar.Arrange(geometry.Rect(0, 0, 0, 0))
	}

	if hScroll {
		_, thickness := sv.hOptions.space(rect.Height)
		g.HScrollBar = geometry.Rect(rect.X, rect.Y+rect.Height-thickness, area.Width, thickness)
		sv.hbar.SetVisible(true)
		sv.hbar.Arrange(g.HScrollBar)
	} else {
		sv.hbar.SetVisible(false)
		sv.hbar.Arrange(geometry.Rect(0, 0, 0, 0))
	}
	if hScroll && vScroll {
		g.Corner = geometry.Rect(g.VScrollBar.X, g.HScrollBar.Y, g.VScrollBar.Width, g.HScrollBar.Height)
	}
	if !sv.Destroyed() {
		sv.geometry = g
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

// clampScroll keeps offsets within the outward, representable scroll range.
// Whole-DIP boundaries keep ordinary content off fractional coordinates.
func (sv *ScrollView) clampScroll() {
	extent := sv.scrollRange()
	maxY := extent.Height
	if sv.scrollY > maxY {
		sv.scrollY = maxY
	}
	if sv.scrollY < 0 {
		sv.scrollY = 0
	}
	maxX := extent.Width
	if sv.scrollX > maxX {
		sv.scrollX = maxX
	}
	if sv.scrollX < 0 {
		sv.scrollX = 0
	}
}
