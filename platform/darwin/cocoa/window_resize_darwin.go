package cocoa

import (
	"fmt"

	"github.com/golang-gui/goui/platform/common"
	. "github.com/golang-gui/goui/platform/darwin/frameworks/appkit"
	. "github.com/golang-gui/goui/platform/darwin/frameworks/core_graphics"
	. "github.com/golang-gui/goui/platform/darwin/frameworks/foundation"
	"github.com/golang-gui/goui/platform/events"
)

type windowResize struct {
	frame resizeFrame
	point NSPoint
	edge  common.WindowEdge
}

// BeginResize is a deliberately limited backend adaptation, not a native
// resize session: no cursor policy, edge double-click, or nested event loop.
func (w *Window) BeginResize(edge common.WindowEdge) error {
	if edge > common.WindowEdgeBottomRight {
		return fmt.Errorf("invalid window edge: %d", edge)
	}
	if !w.window.Valid() || movePress.window != w || !movePress.event.Valid() || movePress.motion || w.resize != nil {
		return common.ErrUnavailable
	}
	if w.window.StyleMask()&NSWindowStyleMaskFullScreen != 0 {
		return common.ErrUnavailable
	}
	event := movePress.event
	movePress = nativePress{}
	f := w.window.Frame()
	w.resize = &windowResize{
		frame: resizeFrame{float64(f.Origin.X), float64(f.Origin.Y), float64(f.Size.Width), float64(f.Size.Height)},
		point: w.resizePoint(event), edge: edge,
	}
	w.resizeRelease = true
	w.buttons &^= events.PointerButtonLeftDown
	return nil
}

func (w *Window) resizePoint(event NSEvent) NSPoint {
	return w.window.ConvertRectToScreen(NSRect{Origin: event.LocationInWindow()}).Origin
}

func (w *Window) trackResize() {
	r := w.resize
	if r == nil || !w.window.Valid() {
		return
	}
	if w.window.StyleMask()&NSWindowStyleMaskFullScreen != 0 {
		w.resize = nil
		return
	}
	// Queued event-local coordinates may predate a frame move. Read the current
	// pointer in the current frame, then convert once to stable screen points.
	p := w.window.ConvertRectToScreen(NSRect{Origin: w.window.MouseLocationOutsideOfEventStream()}).Origin
	f := w.window.Frame()
	content := w.window.ContentRectForFrameRect(f)
	minimum := logicalContentSize(w.window, max(1, w.minWidth), max(1, w.minHeight))
	next := resizedFrame(r.frame, r.edge, float64(p.X-r.point.X), float64(p.Y-r.point.Y),
		float64(minimum.Width+f.Size.Width-content.Size.Width), float64(minimum.Height+f.Size.Height-content.Size.Height))
	f.Origin.X, f.Origin.Y = CGFloat(next.x), CGFloat(next.y)
	f.Size.Width, f.Size.Height = CGFloat(next.width), CGFloat(next.height)
	if f == w.window.Frame() {
		return
	}
	// setFrame may synchronously notify resize, paint, or destroy this window.
	native := w.window
	native.Retain()
	defer native.Release()
	native.SetFrameDisplay(f, true)
}

// resizeFrame uses AppKit screen points (Y increases upwards). This math is
// independent of AppKit so all eight directions can be tested on any host.
type resizeFrame struct{ x, y, width, height float64 }

func resizedFrame(start resizeFrame, edge common.WindowEdge, dx, dy, minWidth, minHeight float64) resizeFrame {
	r := start
	left := edge == common.WindowEdgeLeft || edge == common.WindowEdgeTopLeft || edge == common.WindowEdgeBottomLeft
	right := edge == common.WindowEdgeRight || edge == common.WindowEdgeTopRight || edge == common.WindowEdgeBottomRight
	top := edge == common.WindowEdgeTop || edge == common.WindowEdgeTopLeft || edge == common.WindowEdgeTopRight
	bottom := edge == common.WindowEdgeBottom || edge == common.WindowEdgeBottomLeft || edge == common.WindowEdgeBottomRight
	if left {
		r.width = max(minWidth, start.width-dx)
		r.x = start.x + start.width - r.width
	}
	if right {
		r.width = max(minWidth, start.width+dx)
	}
	if bottom {
		r.height = max(minHeight, start.height-dy)
		r.y = start.y + start.height - r.height
	}
	if top {
		r.height = max(minHeight, start.height+dy)
	}
	return r
}
