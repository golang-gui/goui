package gui

import (
	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/style"
)

const scrollbarWidth float32 = 12
const scrollbarThumbWidth float32 = 8
const scrollbarMinThumb float32 = 20

// ScrollBarPolicy controls the visibility and occupancy of a built-in bar.
// It does not enable or disable scrolling along that axis.
type ScrollBarPolicy uint8

const (
	ScrollBarAuto   ScrollBarPolicy = iota // Show and reserve space when needed.
	ScrollBarAlways                        // Always reserve and show the bar.
	ScrollBarHidden                        // Scroll without a built-in bar or gap.
)

// ScrollBarOptions configures built-in scrollbar geometry in DIP. A zero value
// selects Auto, 12 DIP thickness, an 8 DIP thumb, and no extra spacing.
// Colors, borders and radii remain style values.
type ScrollBarOptions struct {
	Policy         ScrollBarPolicy
	Thickness      float32
	ThumbThickness float32
	Spacing        float32
}

func (o ScrollBarOptions) normalized() ScrollBarOptions {
	if o.Policy > ScrollBarHidden {
		o.Policy = ScrollBarAuto
	}
	o.Thickness = scrollBarThickness(o.Thickness, scrollbarWidth)
	o.ThumbThickness = scrollBarThickness(o.ThumbThickness, scrollbarThumbWidth)
	o.Spacing = normalizeLayoutValue(o.Spacing)
	return o
}

func scrollBarThickness(value, fallback float32) float32 {
	if value = normalizeLayoutValue(value); value == 0 {
		return fallback
	}
	return value
}

// space reserves the bar before its optional gap, including tiny allocations.
func (o ScrollBarOptions) space(available float32) (total, thickness float32) {
	thickness = min(max(0, available), o.Thickness)
	return thickness + min(max(0, available-thickness), o.Spacing), thickness
}

// ScrollBar is an independent, public scrollbar control. It occupies its own
// layout rect (it does not float over content), participates in hit-testing,
// and mounts a DragEventController for thumb dragging and track jumping.
//
// The bar's axis is expressed with the same layout.Direction the layout
// package uses (e.g. HScrollBar == layout.DirectionHorizontal), so the bar
// vocabulary matches the boxes that place it.
//
// ScrollView and ScrollBar are one unit in the style system: both resolve
// under the same style name (default "scroll-view") and the bar uses two
// style parts — "trough" (the track) and "thumb" (the handle) — with
// Normal/Hovered/Pressed states on the thumb.
type ScrollBar struct {
	WidgetBase
	orientation  layout.Direction
	thickness    float32
	thumbWidth   float32
	value        float32 // current scroll offset (driven by the owning ScrollView)
	max          float32 // scrollable range (max <= 0 means not scrollable)
	grabOffset   float32 // press-to-thumb-start offset (keeps the thumb from jumping to the pointer center)
	thumbHovered bool
	change       signal.Signal1[float32] // requested new value; the consumer clamps/rounds
	drag         *DragEventController    // mounted on this widget (no start predicate: the bar rect IS the trough)
	motion       *MotionEventController  // thumb hover state
}

// NewScrollBar creates a scrollbar running along the given layout.Direction.
func NewScrollBar(ori layout.Direction) *ScrollBar {
	b := &ScrollBar{orientation: ori, thickness: scrollbarWidth, thumbWidth: scrollbarThumbWidth}

	b.drag = NewDragEventController()
	b.drag.ConnectBegin(b.onDragBegin)
	b.drag.ConnectUpdate(b.onDragUpdate)
	b.drag.ConnectEnd(b.onDragEnd)
	b.drag.ConnectCanceled(func(_ GestureCancelReason) {
		b.finishDrag()
	})
	b.AddEventController(b.drag)

	b.motion = NewMotionEventController()
	b.motion.ConnectMotion(func(_ EventContext, mi MotionInfo) {
		b.setThumbHovered(b.max > 0 && containsPoint(b.thumbHitRect(), mi.Position))
	})
	b.motion.ConnectHover(func(hovered bool) {
		if !hovered {
			b.setThumbHovered(false)
		}
	})
	b.AddEventController(b.motion)

	return b
}

func (b *ScrollBar) Orientation() layout.Direction { return b.orientation }

func (b *ScrollBar) SetOrientation(ori layout.Direction) {
	if b.orientation == ori {
		return
	}
	cancelGestureController(b.Root(), b.drag)
	if b.Destroyed() {
		return
	}
	b.orientation = ori
	b.RequestLayout()
}

// Thickness returns the preferred cross-axis size in DIP.
func (b *ScrollBar) Thickness() float32 { return b.thickness }

// SetThickness sets the preferred cross-axis size. Non-positive or non-finite
// values restore 12 DIP. Widget size constraints and parent limits still apply.
func (b *ScrollBar) SetThickness(value float32) {
	value = scrollBarThickness(value, scrollbarWidth)
	if b.Destroyed() || b.thickness == value {
		return
	}
	cancelGestureController(b.Root(), b.drag)
	if !b.Destroyed() {
		b.thickness = value
		b.RequestLayout()
	}
}

// ThumbThickness returns the preferred painted thumb thickness in DIP.
func (b *ScrollBar) ThumbThickness() float32 { return b.thumbWidth }

// SetThumbThickness centers the thumb inside the track, clamped to its actual
// width. Non-positive or non-finite values restore 8 DIP. The full track width
// remains available for dragging and track clicks.
func (b *ScrollBar) SetThumbThickness(value float32) {
	value = scrollBarThickness(value, scrollbarThumbWidth)
	if b.Destroyed() || b.thumbWidth == value {
		return
	}
	cancelGestureController(b.Root(), b.drag)
	if !b.Destroyed() {
		b.thumbWidth = value
		b.RequestPaint()
	}
}

// SetValue sets the current scroll offset. The owning ScrollView calls this
// after clamping/rounding to sync the bar's visual.
func (b *ScrollBar) SetValue(v float32) {
	if b.value == v {
		return
	}
	b.value = v
	b.RequestPaint()
}

// SetMax sets the scrollable range. max <= 0 means the axis is not scrollable;
// a visible bar then paints a full thumb without producing value requests.
func (b *ScrollBar) SetMax(m float32) {
	if b.max == m {
		return
	}
	b.max = m
	if m <= 0 {
		b.setThumbHovered(false)
	}
	b.RequestPaint()
}

// ConnectChange subscribes to value requests produced by thumb dragging or
// track jumping. The receiver is responsible for clamping/rounding the value
// and calling SetValue to sync the bar back.
func (b *ScrollBar) ConnectChange(fn func(value float32)) signal.Handle {
	return b.change.Connect(fn)
}

func (b *ScrollBar) Measure(c layout.Constraint) layout.Measurement {
	if !b.Visible() {
		return layout.Measurement{}
	}
	if b.orientation == layout.DirectionVertical {
		return layout.Measured(b.constrain(c, geometry.Size{Width: b.thickness}))
	}
	return layout.Measured(b.constrain(c, geometry.Size{Height: b.thickness}))
}

func (b *ScrollBar) Arrange(rect geometry.Rectangle) {
	if b.Rect().Size != rect.Size {
		cancelGestureController(b.Root(), b.drag)
	}
	if !b.Destroyed() {
		b.WidgetBase.Arrange(rect)
	}
}

func (b *ScrollBar) Paint(p Painter) {
	if !b.Visible() {
		return
	}
	r := geometry.Rect(0, 0, b.Rect().Width, b.Rect().Height)
	PaintStyledBox(p, r, b.troughStyle())
	PaintStyledBox(p, b.thumbRect(), b.thumbStyle())
}

func (b *ScrollBar) Snapshot() WidgetInfo {
	info := b.WidgetBase.Snapshot()
	info.Role = RoleScrollBar
	return info
}

// --- style ---

// ScrollView and ScrollBar share one style name so they theme as a unit. The
// bar's default name is "scroll-view"; an explicit StyleName (pushed down by
// ScrollView) overrides it for both.
func (b *ScrollBar) styleName() string {
	if name := b.StyleName(); name != "" {
		return name
	}
	return styleNameScrollView
}

func (b *ScrollBar) troughStyle() style.Style {
	return ResolveWidgetStyle(b, b.styleName(), stylePartTrough, b.State())
}

func (b *ScrollBar) thumbStyle() style.Style {
	return ResolveWidgetStyle(b, b.styleName(), stylePartThumb, b.thumbState())
}

func (b *ScrollBar) thumbState() style.State {
	state := b.State() &^ style.Hovered
	if b.thumbHovered && !state.Has(style.Disabled) {
		state |= style.Hovered
	}
	return state
}

func (b *ScrollBar) setThumbHovered(v bool) {
	if b.thumbHovered == v {
		return
	}
	b.thumbHovered = v
	b.RequestPaint()
}

// --- geometry (widget-local) ---

// mainLen is the scrollable length of the bar (height for vertical, width for
// horizontal).
func (b *ScrollBar) mainLen() float32 {
	if b.orientation == layout.DirectionVertical {
		return b.Rect().Height
	}
	return b.Rect().Width
}

func (b *ScrollBar) mainCoord(pos geometry.Point) float32 {
	if b.orientation == layout.DirectionVertical {
		return pos.Y
	}
	return pos.X
}

// thumbLen is the thumb length along the main axis. It reflects the visible
// fraction: viewport² / content, with a minimum so very long content still
// keeps the thumb grabbable.
func (b *ScrollBar) thumbLen() float32 {
	ml := b.mainLen()
	content := b.max + ml // content = scrollable range + viewport
	if content <= 0 {
		return ml
	}
	t := ml * ml / content
	if t < scrollbarMinThumb {
		t = scrollbarMinThumb
	}
	if t > ml {
		t = ml
	}
	return t
}

// thumbStart is the thumb's position along the main axis.
func (b *ScrollBar) thumbStart() float32 {
	if b.max <= 0 {
		return 0
	}
	ml := b.mainLen()
	t := b.thumbLen()
	return (ml - t) * b.value / b.max
}

func (b *ScrollBar) thumbRect() geometry.Rectangle {
	r := b.thumbHitRect()
	if b.orientation == layout.DirectionVertical {
		width := min(r.Width, b.thumbWidth)
		return geometry.Rect((r.Width-width)/2, r.Y, width, r.Height)
	}
	height := min(r.Height, b.thumbWidth)
	return geometry.Rect(r.X, (r.Height-height)/2, r.Width, height)
}

func (b *ScrollBar) thumbHitRect() geometry.Rectangle {
	t := b.thumbLen()
	start := b.thumbStart()
	if b.orientation == layout.DirectionVertical {
		return geometry.Rect(0, start, b.Rect().Width, t)
	}
	return geometry.Rect(start, 0, t, b.Rect().Height)
}

// pointOnThumb reports whether pos (widget-local) falls within the current
// thumb rectangle along the main axis.
func (b *ScrollBar) pointOnThumb(pos geometry.Point) bool {
	m := b.mainCoord(pos)
	start := b.thumbStart()
	t := b.thumbLen()
	return m >= start && m < start+t
}

// --- drag interaction (jump-then-drag) ---

func (b *ScrollBar) onDragBegin(_ EventContext, inputInfo DragInfo) {
	pos := inputInfo.Position
	if b.max <= 0 {
		return
	}
	if b.pointOnThumb(pos) {
		// Press on thumb: grab at the press point so the thumb does not jump.
		b.grabOffset = b.mainCoord(pos) - b.thumbStart()
	} else {
		// Press on trough (not thumb): jump so the thumb centers on the press
		// point, then continue dragging from there.
		b.grabOffset = b.thumbLen() / 2
	}
	b.SetState(style.Pressed, true)
	b.emitValueFromPos(pos)
	b.RequestPaint()
}

func (b *ScrollBar) onDragUpdate(_ EventContext, inputInfo DragInfo) {
	pos := inputInfo.Position
	b.emitValueFromPos(pos)
}

func (b *ScrollBar) onDragEnd(_ EventContext, _ DragInfo) {
	b.finishDrag()
}

func (b *ScrollBar) finishDrag() {
	b.SetState(style.Pressed, false)
	b.grabOffset = 0
	// The thumb state (Pressed -> Normal/Hovered) changed, but no value change
	// follows a release, so nothing else repaints. Without this the thumb stays
	// visually pressed until the pointer happens to trigger another repaint.
	b.RequestPaint()
}

// emitValueFromPos computes the scroll value implied by the pointer position
// (using grabOffset so the thumb follows the pointer without centering) and
// emits it as a change request. The consumer clamps/rounds.
func (b *ScrollBar) emitValueFromPos(pos geometry.Point) {
	if b.max <= 0 {
		return
	}
	ml := b.mainLen()
	t := b.thumbLen()
	if ml <= t {
		return // thumb fills the trough; no room to scroll
	}
	start := b.mainCoord(pos) - b.grabOffset
	if start < 0 {
		start = 0
	}
	if start > ml-t {
		start = ml - t
	}
	// Normalize before scaling and keep the ratio in double precision. The
	// far edge must request Max exactly even for billion-DIP content.
	b.change.Emit(float32(float64(start) / float64(ml-t) * float64(b.max)))
}
