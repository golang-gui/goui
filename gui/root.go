package gui

import (
	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/graphics"
)

// Root is the host a widget lives in — a window or a popover. Widgets reach it
// via Root() and depend only on this interface, never on the concrete host type.
type Root interface {
	Widget() Widget
	RequestPaint() error
	RequestLayout()
}

// paintRequester is the native surface capability a host notifies to schedule
// a repaint (platform.Window and platform.Popup both provide it).
type paintRequester interface {
	RequestPaint() error
}

// rootBase is the frame state shared by every widget host (window, popover):
// painter, logical/physical size, dirty flags and the focused widget. Hosts
// embed it and let the promoted methods drive their frame; platform-specific
// parts (the platform window/popup handle) stay in the embedding struct and
// are passed in.
type rootBase struct {
	painter       graphics.Painter
	width         float32 // logical (DIP)
	height        float32 // logical (DIP)
	pixelWidth    float32 // physical (backing) pixels
	pixelHeight   float32 // physical (backing) pixels
	layoutDirty   bool
	paintDirty    bool
	focusedWidget Widget
}

// FocusedWidget returns the widget holding keyboard focus, or nil.
func (b *rootBase) FocusedWidget() Widget { return b.focusedWidget }

// adoptWidget migrates widget into newRoot: it emits the unmount notification
// on the old root, clears the old root's focus if the widget subtree holds it,
// detaches the widget from its previous parent chain and mounts it under
// newRoot. window.SetWidget and popover.SetWidget share this migration
// semantics.
func adoptWidget(widget Widget, newRoot Root) {
	oldRoot := widget.Root()
	if oldRoot != nil {
		widget.base().emitUnmountSubtree(widget)
		if h, ok := oldRoot.(EventTarget); ok && focusWithin(h, widget.base()) {
			h.SetFocusedWidget(nil)
		}
	}
	widget.base().detach(widget)
	widget.base().attachRoot(newRoot, widget)
}

// requestLayout schedules a relayout of the next frame.
func (b *rootBase) requestLayout(platform paintRequester) {
	b.layoutDirty = true
	b.requestPaint(platform)
}

// requestPaint schedules a repaint of the next frame and notifies the native
// surface.
func (b *rootBase) requestPaint(platform paintRequester) error {
	b.paintDirty = true
	if platform == nil {
		return nil
	}
	return platform.RequestPaint()
}

// paintFrame runs one layout + paint frame for the host's content. The dirty
// flags are consumed *before* the work: layout and painting may issue new
// requests (e.g. a virtualized ListView measures its items during Arrange and
// requests a relayout), and those must survive to schedule the next frame
// instead of being cleared here.
func (b *rootBase) paintFrame(content Widget) {
	if b.painter == nil || content == nil {
		return
	}

	b.paintDirty = false

	size := geometry.Size{Width: b.width, Height: b.height}
	if b.layoutDirty {
		b.layoutDirty = false
		content.Measure(layout.Tight(size)) // hosts are extrinsic: content fills them
		content.Arrange(geometry.Rect(0, 0, size.Width, size.Height))
	}

	// Begin takes the physical (backing) pixel size; scale = physical / logical.
	pixelWidth, pixelHeight := b.pixelWidth, b.pixelHeight
	scale := float32(1)
	if b.width > 0 && b.pixelWidth > 0 {
		scale = b.pixelWidth / b.width
	} else {
		pixelWidth, pixelHeight = size.Width, size.Height
	}

	b.painter.Begin(pixelWidth, pixelHeight, scale)
	defer b.painter.End()
	b.painter.Clear(graphics.RGB(255, 255, 255))
	guiPainter := newPainter(b.painter, geometry.Rect(0, 0, size.Width, size.Height))
	paintWidget(content, guiPainter)
}
