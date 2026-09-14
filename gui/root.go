package gui

import (
	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/style"
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
	transparent   bool // immutable native surface configuration, not its style
	painter       graphics.Painter
	width         float32 // logical (DIP)
	height        float32 // logical (DIP)
	pixelWidth    float32 // physical (backing) pixels
	pixelHeight   float32 // physical (backing) pixels
	layoutDirty   bool
	paintDirty    bool
	focusedWidget Widget
}

func (b *rootBase) Transparent() bool { return b.transparent }

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
func (b *rootBase) paintFrame(content Widget, background style.Style) {
	if b.painter == nil {
		return
	}

	b.paintDirty = false
	b.layoutFrame(content)
	b.drawFrame(content, background)
}

// layoutFrame is shared by windows and popovers. Windows seed controls bounds
// first, query the row height afterwards, and may apply one reservation pass.
func (b *rootBase) layoutFrame(content Widget) {
	if content == nil {
		b.layoutDirty = false
		return
	}
	size := geometry.Size{Width: b.width, Height: b.height}
	if b.layoutDirty {
		b.layoutDirty = false
		measureWidget(content, layout.Tight(size)) // hosts are extrinsic: content fills them
		content.Arrange(geometry.Rect(0, 0, size.Width, size.Height))
	}
}

func (b *rootBase) drawFrame(content Widget, background style.Style, decorations ...Widget) {
	b.drawDecoratedFrame(content, background, style.Style{}, 0, decorations...)
}

func (b *rootBase) frameScale() float32 {
	if b.width > 0 && b.pixelWidth > 0 {
		return b.pixelWidth / b.width
	}
	return 1
}

func (b *rootBase) drawDecoratedFrame(content Widget, background, border style.Style, inset float32, decorations ...Widget) {
	if b.painter == nil {
		return
	}
	size := geometry.Size{Width: b.width, Height: b.height}
	// Begin takes the physical (backing) pixel size; scale = physical / logical.
	pixelWidth, pixelHeight := b.pixelWidth, b.pixelHeight
	scale := b.frameScale()
	if b.width <= 0 || b.pixelWidth <= 0 {
		pixelWidth, pixelHeight = size.Width, size.Height
	}

	b.painter.Begin(pixelWidth, pixelHeight, scale)
	defer b.painter.End()
	// Initialize storage independently of the visible body background. Opaque
	// surfaces use black for uncovered pixels; alpha surfaces use transparent zero.
	clearColor := graphics.Color{}
	if !b.transparent {
		clearColor.A = 1
	}
	b.painter.Clear(clearColor)
	bounds := geometry.Rect(0, 0, size.Width, size.Height)
	guiPainter := newPainter(b.painter, bounds)
	guiPainter.applyState()
	// Only the host background reaches the rounded perimeter. Widgets retain
	// their full allocations but cannot paint over this window-owned area.
	if bg, ok := background.BackgroundColor(); ok && bg != nil {
		radius, _ := border.Radius()
		radius = min(normalizeLayoutValue(radius), max(0, min(size.Width, size.Height)/2))
		if radius > 0 {
			guiPainter.FillRoundRect(bounds, radius, graphics.ColorOf(bg))
		} else {
			guiPainter.FillRect(bounds, graphics.ColorOf(bg))
		}
	}
	safe := geometry.Rect(inset, inset, max(0, size.Width-2*inset), max(0, size.Height-2*inset))
	guiPainter.state.scopeClip, guiPainter.state.clip = safe, safe
	guiPainter.applyState()
	paintWidget(content, guiPainter)
	for _, decoration := range decorations {
		paintWidget(decoration, guiPainter)
	}
	guiPainter.state.scopeClip, guiPainter.state.clip = bounds, bounds
	guiPainter.applyState()
	paintStyledBorder(guiPainter, bounds, border)
}
