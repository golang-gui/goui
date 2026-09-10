package gui

import (
	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/style"
)

// HeaderBar is a single-child, optional window drag region. Its background
// fills its allocation; its child automatically avoids the window's controls.
// It fills bounded available width (subject to size preferences), but uses
// content width when unbounded. Height remains content-driven with a minimum.
// Multiple HeaderBars can share a window. No title or controls are inserted.
type HeaderBar struct {
	WidgetBase
	child       Widget
	padding     float32
	info        ChromeInfo
	connections signal.Handles
	dragRegion  signal.Signal2[geometry.Point, *bool]
	arranged    bool
}

func NewHeaderBar() *HeaderBar {
	h := &HeaderBar{padding: 8}
	h.SetMinSize(geometry.Size{Height: 48})
	h.ConnectMount(h.mountChrome)
	h.ConnectUnmount(func() {
		h.connections.Disconnect()
		h.connections = nil
		h.info = ChromeInfo{}
		h.arranged = false
		h.RequestLayout()
	})
	return h
}

func (h *HeaderBar) Child() Widget { return h.child }

func (h *HeaderBar) SetChild(child Widget) {
	if h.child == child {
		return
	}
	if h.child != nil {
		h.WidgetBase.RemoveChild(h.child)
	}
	h.child = child
	if child != nil {
		h.WidgetBase.AddChild(h, child)
	}
	h.RequestLayout()
}

func (h *HeaderBar) Padding() float32 { return h.padding }
func (h *HeaderBar) SetPadding(padding float32) {
	padding = normalizeLayoutValue(padding)
	if h.padding != padding {
		h.padding = padding
		h.RequestLayout()
	}
}

// ConnectDragRegion queries in HeaderBar-local DIP after conservative automatic
// classification. The hit path through this HeaderBar must have no focusable
// widgets and only Motion/Key controllers (or none) to default to dragging.
// Other controllers, including custom ones, default to client input. Later
// callbacks may override either result. Native resize edges remain platform-owned.
// Queries are read-only and synchronous; never retain drag or dispatch input.
func (h *HeaderBar) ConnectDragRegion(fn func(geometry.Point, *bool)) signal.Handle {
	return h.dragRegion.Connect(fn)
}

func (h *HeaderBar) Measure(constraint layout.Constraint) layout.Measurement {
	if !h.Visible() {
		return layout.Measurement{}
	}
	constraint = h.layoutConstraint(constraint)
	var m layout.Measurement
	if child := h.liveChild(); child != nil && child.Visible() {
		m = measureWidget(child, layout.Loose(constraint.Inset(h.padding).Max))
	}
	childHeight := m.Height
	width := m.Width + 2*h.padding
	if constraint.Max.Width < layout.Inf {
		// Choose the available width within the effective parent/self limits.
		// In an unbounded axis (e.g. a weighted row's initial measure), keep
		// the content basis; the parent will assign a finite width afterwards.
		width = constraint.Max.Width
	}
	m.Size = constraint.Clamp(geometry.Size{
		Width:  width,
		Height: m.Height + 2*h.padding,
	})
	// Header contents are centered vertically.
	if m.HasBaseline {
		inner := geometry.Rect(0, 0, m.Width, m.Height).Inset(h.padding)
		m.Baseline += inner.Y + max(0, (inner.Height-childHeight)/2)
	}
	return m
}

func (h *HeaderBar) Arrange(rect geometry.Rectangle) {
	h.rect = rect
	h.arranged = true
	child := h.liveChild()
	if child == nil || !child.Visible() {
		return
	}
	area := h.contentRect()
	m := measureWidget(child, layout.Constraint{
		Min: geometry.Size{Width: area.Width},
		Max: area.Size,
	})
	child.Arrange(geometry.Rect(area.X, area.Y+(area.Height-m.Height)/2, area.Width, m.Height))
}

// Only this row's content is inset. The stored padding and outer allocation
// stay unchanged. Stale native bounds remain conservative layout reservations.
func (h *HeaderBar) contentRect() geometry.Rectangle {
	bounds := geometry.Rect(0, 0, h.rect.Width, h.rect.Height)
	area := bounds.Inset(h.padding)
	if !h.info.Enabled || emptyRect(h.info.ControlsBounds) {
		return area
	}
	occupied := h.info.ControlsBounds
	own := h.windowRect()
	if emptyRect(own.Intersect(occupied)) {
		return area
	}
	// This is the same side policy as the window-owned controls, not a
	// per-HeaderBar choice. Window coordinates handle split header rows.
	left := h.info.Controls == ChromeControlsNative
	if left {
		x := min(h.rect.Width, max(area.X, occupied.X+occupied.Width-own.X))
		area.Width = max(0, area.X+area.Width-x)
		area.X = x
	} else {
		edge := max(0, min(area.X+area.Width, occupied.X-own.X))
		area.Width = max(0, edge-area.X)
	}
	return area
}

func (h *HeaderBar) Paint(p Painter) {
	name := h.StyleName()
	if name == "" {
		name = "header-bar"
	}
	paintStyledBox(p, geometry.Rect(0, 0, h.rect.Width, h.rect.Height),
		ResolveStyle(name, style.PartDefault, style.Normal))
}

func (h *HeaderBar) Snapshot() WidgetInfo {
	info := h.WidgetBase.Snapshot()
	info.Role = RoleBox
	return info
}

func (h *HeaderBar) mountChrome() {
	h.arranged = false
	h.connections.Disconnect()
	h.connections = nil
	if win := h.Window(); win != nil {
		h.connections = signal.Handles{
			win.Chrome().ConnectInfo(func(info ChromeInfo) {
				if h.info != info {
					previous := h.contentRect()
					h.info = info
					// Position-only changes within the same row usually do
					// not change its reservation or intrinsic measurement.
					if !h.arranged || previous != h.contentRect() {
						h.RequestLayout()
					}
				}
			}),
			win.Chrome().ConnectQueryRegion(h.queryRegion),
			win.Chrome().ConnectQueryControls(h.queryControls),
		}
	}
}

// Only the top segment covering the group's outer anchor supplies height.
// The horizontal reservation is known before layout; vertical position is
// deliberately irrelevant to choosing the segment.
func (h *HeaderBar) queryControls(result *ChromeControls) {
	win := h.Window()
	if win == nil || !h.arranged || !visibleInTree(h) || !h.info.Enabled {
		return
	}
	own, controls := h.windowRect(), h.info.ControlsBounds
	if own.Y != 0 || emptyRect(own) || controls.Width <= 0 {
		return
	}
	x := controls.X
	if h.info.Controls == ChromeControlsNative {
		x += min(0.5, controls.Width/2)
	} else {
		x += controls.Width - min(0.5, controls.Width/2)
	}
	point := geometry.Point{X: x, Y: min(0.5, own.Height/2)}
	if !containsPoint(own, point) {
		return
	}
	target := hitTest(win.Widget(), point)
	if target == nil || !target.base().isDescendant(target, h) {
		return
	}
	result.Height = own.Height
}

func (h *HeaderBar) queryRegion(p geometry.Point, result *ChromeRegion) {
	win := h.Window()
	if win == nil || !visibleInTree(h) {
		return
	}
	if containsPoint(h.info.ControlsBounds, p) {
		return
	}
	target := hitTest(win.Widget(), p)
	if target == nil || !target.base().isDescendant(target, h) {
		return
	}
	local := widgetLocalPoint(h, p)
	// Padding separates content from the background, not input from dragging.
	// Only the platform knows the actual native resize borders and their DPI.
	if !containsPoint(geometry.Rect(0, 0, h.rect.Width, h.rect.Height), local) {
		return
	}
	drag := h.defaultDragRegion(target)
	h.dragRegion.Emit(local, &drag)
	if drag {
		*result = ChromeRegionDrag
	} else {
		*result = ChromeRegionClient
	}
}

// Inspect only the live hit path, including the HeaderBar itself. This is a
// local policy, not a prediction that a controller will consume an event.
// In particular, a passive label inside a button must not bypass its parent.
func (h *HeaderBar) defaultDragRegion(target Widget) bool {
	for widget := target; widget != nil; widget = widget.Parent() {
		if widget.Focusable() {
			return false
		}
		for _, controller := range widget.base().controllers {
			switch controller.(type) {
			case nil, *MotionEventController, *KeyEventController:
				// Known observation/keyboard controllers do not block dragging.
			default:
				// Click, Drag, Wheel and unknown controllers keep client input.
				return false
			}
		}
		if widget == h {
			return true
		}
	}
	return false
}

func (h *HeaderBar) liveChild() Widget {
	if h.child != nil && !h.child.base().destroyed && h.child.Parent() == h {
		return h.child
	}
	return nil
}
