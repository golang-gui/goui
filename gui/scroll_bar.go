package gui

import (
	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/style"
)

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
	b := &ScrollBar{orientation: ori}

	b.drag = NewDragEventController()
	b.drag.ConnectBegin(b.onDragBegin)
	b.drag.ConnectUpdate(b.onDragUpdate)
	b.drag.ConnectEnd(b.onDragEnd)
	b.AddEventController(b.drag)

	b.motion = NewMotionEventController()
	b.motion.ConnectMotion(func(mi MotionInfo) {
		b.setThumbHovered(containsPoint(b.thumbRect(), mi.Position))
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
	b.orientation = ori
	b.RequestLayout()
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

// SetMax sets the scrollable range. max <= 0 means the axis is not scrollable
// (the owning ScrollView hides the bar in that case).
func (b *ScrollBar) SetMax(m float32) {
	if b.max == m {
		return
	}
	b.max = m
	b.RequestPaint()
}

// ConnectChange subscribes to value requests produced by thumb dragging or
// track jumping. The receiver is responsible for clamping/rounding the value
// and calling SetValue to sync the bar back.
func (b *ScrollBar) ConnectChange(fn func(value float32)) signal.Handle {
	return b.change.Connect(fn)
}

func (b *ScrollBar) Measure(c layout.Constraint) geometry.Size {
	if !b.Visible() {
		return geometry.Size{}
	}
	if b.orientation == layout.DirectionVertical {
		return c.Clamp(geometry.Size{Width: scrollbarWidth})
	}
	return c.Clamp(geometry.Size{Height: scrollbarWidth})
}

func (b *ScrollBar) Paint(p Painter) {
	if !b.Visible() || b.max <= 0 {
		return
	}
	r := geometry.Rect(0, 0, b.Rect().Width, b.Rect().Height)
	paintStyledBox(p, r, b.troughStyle())
	paintStyledBox(p, b.thumbRect(), b.thumbStyle())
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
	return ResolveStyle(b.styleName(), stylePartTrough, style.Normal)
}

func (b *ScrollBar) thumbStyle() style.Style {
	return ResolveStyle(b.styleName(), stylePartThumb, b.thumbState())
}

func (b *ScrollBar) thumbState() style.State {
	if b.drag != nil && b.drag.Dragging() {
		return style.Pressed
	}
	if b.thumbHovered {
		return style.Hovered
	}
	return style.Normal
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

func (b *ScrollBar) onDragBegin(pos geometry.Point, _ events.Modifiers) {
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
	b.emitValueFromPos(pos)
	b.RequestPaint()
}

func (b *ScrollBar) onDragUpdate(pos geometry.Point, _ events.Modifiers) {
	b.emitValueFromPos(pos)
}

func (b *ScrollBar) onDragEnd(_ geometry.Point, _ events.Modifiers) {
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
	b.change.Emit(start * b.max / (ml - t))
}
