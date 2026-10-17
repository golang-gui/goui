package gui

import (
	"math"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/style"
)

// BorderBox is a single-child frame with a reserved content inset. Its default
// appearance is invisible; BackgroundColor, BorderColor, BorderWidth and Radius
// come from its own StyleName ("border-box" when unset), not its child's style.
// As the root widget of a WindowChromeNone window it also supplies resize regions
// and cursors through WindowChrome. Nested boxes and popovers only get layout
// and decoration. Use HeaderBar separately for window movement.
type BorderBox struct {
	WidgetBase
	child       Widget
	resizable   bool
	resizeWidth float32
	connection  signal.Handle
}

func NewBorderBox() *BorderBox {
	b := &BorderBox{resizable: true, resizeWidth: 6}
	b.ConnectMount(b.mountChrome)
	b.ConnectUnmount(func() {
		if b.connection != nil {
			b.connection.Disconnect()
			b.connection = nil
		}
	})
	return b
}

func (b *BorderBox) Child() Widget { return b.child }

func (b *BorderBox) SetChild(child Widget) {
	if b.child == child {
		return
	}
	if b.child != nil {
		b.WidgetBase.RemoveChild(b.child)
	}
	b.child = child
	if child != nil {
		b.WidgetBase.AddChild(b, child)
	}
	b.RequestLayout()
}

func (b *BorderBox) Resizable() bool { return b.resizable }

// SetResizable enables automatic resizing in a root WindowChromeNone box.
// It does not remove the reserved inset or resize an arbitrary widget.
func (b *BorderBox) SetResizable(resizable bool) {
	if b.resizable != resizable {
		b.resizable = resizable
		b.RequestPaint()
	}
}

func (b *BorderBox) ResizeWidth() float32 { return b.resizeWidth }

// SetResizeWidth sets the edge hit width and minimum content inset in DIP.
// Zero disables edge hits. Negative and non-finite values become zero.
// Visible border width is a separate style property.
func (b *BorderBox) SetResizeWidth(width float32) {
	width = normalizeLayoutValue(width)
	if b.resizeWidth != width {
		b.resizeWidth = width
		b.RequestLayout()
	}
}

func (b *BorderBox) frameStyle() style.Style {
	name := b.StyleName()
	if name == "" {
		name = "border-box"
	}
	state := style.Normal
	if !IsEnabled(b) {
		state = style.Disabled
	}
	return ResolveWidgetStyle(b, name, style.PartDefault, state)
}

func (b *BorderBox) contentInset() float32 {
	s := b.frameStyle()
	border, _ := s.BorderWidth()
	radius, _ := s.Radius()
	inset := max(b.resizeWidth, normalizeLayoutValue(border))
	if radius = normalizeLayoutValue(radius); radius > 0 {
		// A rectangular child must stay inside the rounded surface. One DIP
		// also separates it from the antialiased edge, without a rounded clip.
		inset = max(inset, radius*(1-1/math.Sqrt2)+1)
	}
	return inset
}

func (b *BorderBox) Measure(c layout.Constraint) layout.Measurement {
	if !b.Visible() {
		return layout.Measurement{}
	}
	c = b.layoutConstraint(c)
	inset := b.contentInset()
	var m layout.Measurement
	if child := b.liveChild(); child != nil && child.Visible() {
		m = measureWidget(child, c.Inset(inset))
	}
	m.Size = c.Clamp(geometry.Size{Width: m.Width + 2*inset, Height: m.Height + 2*inset})
	if m.HasBaseline {
		m.Baseline += min(inset, m.Height/2)
	}
	return m
}

func (b *BorderBox) Arrange(rect geometry.Rectangle) {
	b.rect = rect
	if child := b.liveChild(); child != nil && child.Visible() {
		area := geometry.Rect(0, 0, rect.Width, rect.Height).Inset(b.contentInset())
		m := measureWidget(child, layout.Tight(area.Size))
		child.Arrange(geometry.Rectangle{Pos: area.Pos, Size: m.Size})
	}
}

func (b *BorderBox) Paint(p Painter) {
	paintStyledBox(p, geometry.Rect(0, 0, b.rect.Width, b.rect.Height), b.frameStyle())
}

func (b *BorderBox) Snapshot() WidgetInfo {
	info := b.WidgetBase.Snapshot()
	info.Role = RoleBox
	return info
}

func (b *BorderBox) liveChild() Widget {
	if b.child != nil && !b.child.base().destroyed && b.child.Parent() == b {
		return b.child
	}
	return nil
}

func (b *BorderBox) mountChrome() {
	if b.connection != nil {
		b.connection.Disconnect()
		b.connection = nil
	}
	if win := b.Window(); win != nil {
		// SetWidget mounts before assigning the root slot. Qualify at query
		// time instead of assuming root ownership during this notification.
		b.connection = win.Chrome().ConnectQueryRegion(b.queryRegion)
	}
}

func (b *BorderBox) resizeWindow() *window {
	w, ok := b.Window().(*window)
	if !ok || b.destroyed || w.destroyed || w.Widget() != b || !b.resizable ||
		!visibleInTree(b) || !IsEnabled(b) || w.chrome == nil || !w.chrome.info.Enabled ||
		w.chrome.info.Mode != WindowChromeNone || w.State() != WindowStateNormal {
		return nil
	}
	return w
}

func (b *BorderBox) queryRegion(point geometry.Point, result *ChromeRegion) {
	if b.resizeWindow() == nil {
		return
	}
	if region := b.edgeRegion(widgetLocalPoint(b, point)); region != ChromeRegionDefault {
		*result = region
	}
}

// Cursor uses the final ordered Chrome query, just like Integrated's frame.
// No cached hover geometry, extra controller or independent resize gesture is
// needed. Explicit SetCursor remains an override; native hit queries stay read-only.
func (b *BorderBox) Cursor() Cursor {
	if cursor := b.WidgetBase.Cursor(); cursor != nil && cursor != CursorDefault {
		return cursor
	}
	if w := b.resizeWindow(); w != nil && w.cursorInside && !w.layoutDirty && !w.layingOut && w.chrome != nil {
		switch w.chrome.queryRegion(w.cursorPosition) {
		case ChromeRegionLeft:
			return CursorResizeLeft
		case ChromeRegionRight:
			return CursorResizeRight
		case ChromeRegionTop:
			return CursorResizeTop
		case ChromeRegionBottom:
			return CursorResizeBottom
		case ChromeRegionTopLeft:
			return CursorResizeTopLeft
		case ChromeRegionTopRight:
			return CursorResizeTopRight
		case ChromeRegionBottomLeft:
			return CursorResizeBottomLeft
		case ChromeRegionBottomRight:
			return CursorResizeBottomRight
		}
	}
	return CursorDefault
}

func (b *BorderBox) edgeRegion(p geometry.Point) ChromeRegion {
	w, h := b.rect.Width, b.rect.Height
	if b.resizeWidth <= 0 || !containsPoint(geometry.Rect(0, 0, w, h), p) {
		return ChromeRegionDefault
	}
	// Nearest-edge selection resolves overlapping strips in very small boxes.
	left := p.X < min(b.resizeWidth, w/2)
	right := !left && p.X >= max(w-b.resizeWidth, w/2)
	top := p.Y < min(b.resizeWidth, h/2)
	bottom := !top && p.Y >= max(h-b.resizeWidth, h/2)
	if !left && !right && !top && !bottom {
		return ChromeRegionDefault
	}
	xCorner, yCorner := min(float32(16), w/2), min(float32(16), h/2)
	switch {
	case (left && p.Y < yCorner) || (top && p.X < xCorner):
		return ChromeRegionTopLeft
	case (right && p.Y < yCorner) || (top && p.X >= w-xCorner):
		return ChromeRegionTopRight
	case (left && p.Y >= h-yCorner) || (bottom && p.X < xCorner):
		return ChromeRegionBottomLeft
	case (right && p.Y >= h-yCorner) || (bottom && p.X >= w-xCorner):
		return ChromeRegionBottomRight
	case left:
		return ChromeRegionLeft
	case right:
		return ChromeRegionRight
	case top:
		return ChromeRegionTop
	default:
		return ChromeRegionBottom
	}
}

var _ Bin = (*BorderBox)(nil)
