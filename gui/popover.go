package gui

import (
	"fmt"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/graphics"
)

// Popover is a borderless, no-focus floating surface anchored to a Widget, used
// for menus, detached toolbars and tooltips. Whether it behaves as a menu or a
// tooltip is decided by its content Widget; the Popover itself has no dismiss
// policy — the control drives Show/Hide/Destroy.
//
// A Popover belongs to its anchor's window: the native surface is created on
// Show, bound to that window, and released when the anchor unmounts or the
// window is destroyed.
type Popover interface {
	Visible() bool

	Widget() Widget
	SetWidget(Widget)

	Anchor() Widget

	// Position is the offset relative to the anchor's origin (logical coords).
	Position() geometry.Point
	SetPosition(geometry.Point)

	// Modal reports whether this popover is modal relative to its owner window:
	// the window forwards its keyboard here and its own clicks / Esc / focus loss
	// request dismissal (menu-style). Default false — a modeless tooltip/panel
	// that leaves the window's own input alone (any number can coexist). Modal is
	// scoped to the owner window; it does not block other windows.
	Modal() bool
	SetModal(bool)

	Show() error
	Hide()
	Destroy()

	// ConnectDismissRequest fires when the owner window sees an interaction that
	// usually dismisses a menu (outside click / Esc / focus loss). The control
	// decides what to do — typically Hide.
	ConnectDismissRequest(func()) signal.Handle
}

// NewPopover creates a popover anchored to widget. It holds no native resources
// until Show; the anchor need not be mounted yet.
func NewPopover(anchor Widget) Popover {
	return &popover{anchor: anchor}
}

type popover struct {
	anchor        Widget
	widget        Widget // content
	position      geometry.Point
	owner         Window // resolved from the anchor; only the public Window API is used
	platformPopup platform.Popup
	painter       graphics.Painter
	dispatcher    EventDispatcher
	width         float32 // logical
	height        float32
	pixelWidth    float32 // physical
	pixelHeight   float32
	layoutDirty   bool
	paintDirty    bool
	focusedWidget Widget
	visible       bool
	modal         bool // menu-style: owner window forwards its input here (modeless by default)

	dismissRequest signal.Signal0
	hUnmount       signal.Handle // anchor unmount -> releaseNative
	hWinGone       signal.Handle // owner window destroy -> releaseNative
}

// --- Root + EventTarget (widget host) ---

func (p *popover) Widget() Widget {
	p.widget = liveRoot(p.widget)
	return p.widget
}

func (p *popover) RequestPaint() error {
	p.requestPaint()
	return nil
}

func (p *popover) FocusedWidget() Widget { return p.focusedWidget }

func (p *popover) SetFocusedWidget(widget Widget) bool {
	if widget != nil && (widget.Root() != p || !widget.Focusable() || !visibleInTree(widget)) {
		return false
	}
	if p.focusedWidget != widget {
		p.focusedWidget = widget
		p.requestPaint()
	}
	return true
}

// --- Popover API ---

func (p *popover) Visible() bool            { return p.visible }
func (p *popover) Anchor() Widget           { return p.anchor }
func (p *popover) Position() geometry.Point { return p.position }
func (p *popover) Modal() bool              { return p.modal }

func (p *popover) SetModal(v bool) {
	if p.modal == v {
		return
	}
	p.modal = v
	if p.visible {
		if v {
			p.becomeModalTarget()
		} else {
			p.resignModalTarget()
		}
	}
}

func (p *popover) SetWidget(widget Widget) {
	if p.widget == widget {
		return
	}
	if widget != nil && widget.base().destroyed {
		return
	}
	if p.widget != nil {
		p.widget.base().detachRoot(p.widget)
	}
	p.widget = widget
	if widget != nil {
		widget.base().attachRoot(p, widget)
	}
	p.layoutDirty = true
	if p.visible {
		p.measureAndSize()
		p.requestPaint()
	}
}

func (p *popover) SetPosition(pos geometry.Point) {
	p.position = pos
	if p.visible {
		p.reposition()
	}
}

func (p *popover) ConnectDismissRequest(fn func()) signal.Handle {
	return p.dismissRequest.Connect(fn)
}

// becomeModalTarget / resignModalTarget register this popover as its owner
// window's modal input target while it is a visible modal (menu-style). Modeless
// popovers never intercept the window. Both use only the public Window API.
func (p *popover) becomeModalTarget() {
	if p.owner != nil {
		p.owner.SetModalTarget(p)
	}
}

func (p *popover) resignModalTarget() {
	if p.owner != nil {
		p.owner.SetModalTarget(nil)
	}
}

// DispatchEvent routes an event the owner window forwards (keyboard nav, since
// the popover has no native focus) to the popover's content. Part of ModalTarget.
func (p *popover) DispatchEvent(event events.Event) error {
	return p.dispatcher.DispatchEvent(p, event)
}

// RequestDismiss fires the dismiss-request signal so the controlling code can
// hide/destroy the popover. Part of ModalTarget; also fine to call directly.
func (p *popover) RequestDismiss() { p.dismissRequest.Emit() }

func (p *popover) Show() error {
	win, ok := anchorWindow(p.anchor)
	if !ok {
		return fmt.Errorf("popover: anchor is not mounted in a window")
	}
	if p.platformPopup != nil && p.owner != win {
		p.releaseNative() // anchor moved to another window; rebuild for the new one
	}
	if p.platformPopup == nil {
		if err := p.createNative(win); err != nil {
			return err
		}
	} else {
		p.measureAndSize()
	}
	p.reposition()
	p.visible = true
	if p.modal {
		// Only a modal (menu) popover intercepts the window's input; modeless
		// tooltips/panels leave the window's own input untouched.
		p.becomeModalTarget()
	}
	return p.platformPopup.Show()
}

func (p *popover) Hide() {
	if !p.visible {
		return
	}
	p.visible = false
	if p.modal {
		p.resignModalTarget()
	}
	if p.platformPopup != nil {
		_ = p.platformPopup.Hide()
	}
}

func (p *popover) Destroy() {
	p.Hide()
	p.releaseNative()
	if p.widget != nil {
		p.widget.base().detachRoot(p.widget)
	}
}

// --- native resource lifecycle (belongs to the window) ---

func (p *popover) createNative(win Window) error {
	if App == nil {
		return fmt.Errorf("popover: application is not created")
	}
	p.owner = win
	p.measureAndSize() // sizes p.width/height from content

	// Platform + typography come from the app (global escape hatches); the owner
	// platform window comes from the host's PlatformWindow escape hatch.
	pp, err := App.Platform().NewPopup(win.PlatformWindow(), p.width, p.height, p.onEvent)
	if err != nil {
		p.owner = nil
		return fmt.Errorf("create platform popup: %w", err)
	}
	painter, err := App.Platform().NewPainter(pp, App.Typography())
	if err != nil {
		pp.Destroy()
		p.owner = nil
		return fmt.Errorf("create popover painter: %w", err)
	}
	p.platformPopup = pp
	p.painter = painter

	// Auto-release when the anchor leaves the tree or the window is destroyed —
	// the native surface never outlives its owner window.
	p.hUnmount = p.anchor.ConnectUnmount(p.releaseNative)
	p.hWinGone = win.ConnectDestroy(p.releaseNative)
	return nil
}

func (p *popover) releaseNative() {
	if p.platformPopup == nil {
		return
	}
	p.visible = false
	if p.hUnmount != nil {
		p.hUnmount.Disconnect()
		p.hUnmount = nil
	}
	if p.hWinGone != nil {
		p.hWinGone.Disconnect()
		p.hWinGone = nil
	}
	if p.modal {
		p.resignModalTarget()
	}
	if p.painter != nil {
		p.painter.Destroy()
		p.painter = nil
	}
	p.platformPopup.Destroy()
	p.platformPopup = nil
	p.owner = nil
}

// measureAndSize sizes the popover to its content (content-driven), using the
// owner window's size as the available constraint, and resizes the native face.
func (p *popover) measureAndSize() {
	if p.widget == nil {
		return
	}
	// Popup is intrinsic: measure with a loose constraint so the popover sizes to
	// its content, independent of the owner window's size.
	const loose = 1 << 14
	size := p.widget.Measure(layout.Loose(geometry.Size{Width: loose, Height: loose}))
	p.width, p.height = size.Width, size.Height
	if p.width < 1 {
		p.width = 1
	}
	if p.height < 1 {
		p.height = 1
	}
	if p.platformPopup != nil {
		p.platformPopup.SetSize(p.width, p.height)
	}
	p.layoutDirty = true
}

func (p *popover) reposition() {
	if p.platformPopup == nil {
		return
	}
	o := absOrigin(p.anchor)
	p.platformPopup.SetPosition(o.X+p.position.X, o.Y+p.position.Y)
}

// --- events / paint (mirrors window; shared host base is a later cleanup) ---

func (p *popover) onEvent(event platform.Event) {
	switch e := event.(type) {
	case events.SizeEvent:
		p.width, p.height = e.Width, e.Height
		p.pixelWidth, p.pixelHeight = e.PixelWidth, e.PixelHeight
		p.requestLayout()
	case events.PaintEvent:
		p.paint()
	default:
		_ = p.dispatcher.DispatchEvent(p, event)
	}
}

func (p *popover) paint() {
	p.widget = liveRoot(p.widget)
	if p.painter == nil || p.widget == nil {
		return
	}
	size := geometry.Size{Width: p.width, Height: p.height}
	if p.layoutDirty {
		p.widget.Measure(layout.Tight(size)) // lay out content at the resolved popover size.
		p.widget.Arrange(geometry.Rect(0, 0, size.Width, size.Height))
		p.layoutDirty = false
	}

	pixelWidth, pixelHeight := p.pixelWidth, p.pixelHeight
	scale := float32(1)
	if p.width > 0 && p.pixelWidth > 0 {
		scale = p.pixelWidth / p.width
	} else {
		pixelWidth, pixelHeight = size.Width, size.Height
	}

	p.painter.Begin(pixelWidth, pixelHeight, scale)
	p.painter.Clear(graphics.RGB(255, 255, 255))
	p.widget.Paint(SubPainter(p.painter, p.widget.Rect()))
	p.painter.End()
	p.paintDirty = false
}

// RequestLayout satisfies Root: schedule a relayout of this host.
func (p *popover) RequestLayout() { p.requestLayout() }

func (p *popover) requestLayout() {
	p.layoutDirty = true
	p.requestPaint()
}

func (p *popover) requestPaint() {
	p.paintDirty = true
	if p.platformPopup != nil {
		_ = p.platformPopup.RequestPaint()
	}
}

// anchorWindow resolves the *window hosting anchor, or false if anchor is not
// mounted in a window.
func anchorWindow(anchor Widget) (Window, bool) {
	if anchor == nil {
		return nil, false
	}
	win := anchor.Window()
	if win == nil {
		return nil, false
	}
	return win, true
}

// absOrigin returns widget's origin in window-content coordinates by summing
// each widget's parent-relative rect origin up the parent chain (the root is
// arranged at (0,0)).
func absOrigin(w Widget) geometry.Point {
	var o geometry.Point
	for cur := w; cur != nil; cur = cur.Parent() {
		r := cur.Rect()
		o.X += r.X
		o.Y += r.Y
	}
	return o
}
