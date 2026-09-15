package gui

import (
	"fmt"
	"math"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/style"
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
	// Transparent reports the immutable configuration, including before Show.
	Transparent() bool
	// StyleName is explicit; an empty name resolves the "popover" style.
	StyleName() string
	SetStyleName(string)

	Widget() Widget
	SetWidget(Widget)

	Anchor() Widget

	// Position is the body's origin relative to the anchor, excluding shadow (DIP).
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

	// ConnectClosed fires when the popover actually hides — after any Hide /
	// Destroy / release path, at most once per transition. Unlike
	// ConnectDismissRequest ("something asks to close") it reports that it is
	// gone, so a control can restore state.
	ConnectClosed(func()) signal.Handle
}

// PopoverOptions configures the native surface independently of its owner.
type PopoverOptions struct {
	// Transparent requires an alpha surface. Show returns creation failures.
	Transparent bool
}

// NewPopover copies options (nil means opaque). It holds no native resources
// until Show; the anchor need not be mounted yet.
func NewPopover(anchor Widget, options *PopoverOptions) Popover {
	p := &popover{anchor: anchor}
	if options != nil {
		p.transparent = options.Transparent
	}
	return p
}

type popover struct {
	rootBase
	anchor            Widget
	widget            Widget // content
	position          geometry.Point
	owner             Window // resolved from the anchor; only the public Window API is used
	platformPopup     platform.Popup
	styleName         string
	insets            popoverInsets
	requestedSize     geometry.Size
	requestedPosition geometry.Point
	positionValid     bool
	destroyed         bool
	dispatcher        EventDispatcher
	visible           bool
	modal             bool // menu-style: owner window forwards its input here (modeless by default)

	dismissRequest signal.Signal0
	closed         signal.Signal0
	hUnmount       signal.Handle // anchor unmount -> releaseNative
	hWinGone       signal.Handle // owner window destroy -> releaseNative
	hStyle         signal.Handle // application style update -> layout/repaint
}

// --- Root + EventTarget (widget host) ---

func (p *popover) Widget() Widget {
	p.widget = liveRoot(p.widget)
	return p.widget
}

func (p *popover) RequestPaint() error {
	return p.requestPaint()
}

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
func (p *popover) StyleName() string        { return p.styleName }

func (p *popover) SetStyleName(name string) {
	if p.styleName == name {
		return
	}
	p.styleName = name
	invalidateStyleSubtree(p.Widget())
	if p.visible {
		p.measureAndSize()
	}
	p.requestLayout()
}

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
	if p.destroyed {
		return
	}
	if p.widget == widget {
		return
	}
	if widget != nil && widget.base().destroyed {
		return
	}
	if p.widget != nil {
		p.widget.base().detachRoot(p.widget)
	}
	if widget != nil {
		adoptWidget(widget, p)
	}
	p.widget = widget
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

func (p *popover) ConnectClosed(fn func()) signal.Handle {
	return p.closed.Connect(fn)
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
	if p.destroyed {
		return nil
	}
	if p.modal {
		switch e := event.(type) {
		case events.PointerEvent:
			_, border, _ := p.surfaceStyle()
			radius, _ := border.Radius()
			if e.EventType == events.PointerDown && !roundedBodyContains(p.bodyRect(), radius, e.Position) {
				p.RequestDismiss()
				return nil
			}
		case events.KeyEvent:
			if e.EventType == events.KeyDown && e.Key == events.KeyEscape {
				p.RequestDismiss()
				return nil
			}
		}
	}
	return p.dispatcher.DispatchEvent(p, event)
}

// RequestDismiss fires the dismiss-request signal so the controlling code can
// hide/destroy the popover. Part of ModalTarget; also fine to call directly.
func (p *popover) RequestDismiss() { p.dismissRequest.Emit() }

func (p *popover) Show() error {
	if p.destroyed {
		return fmt.Errorf("popover: destroyed")
	}
	win, ok := anchorWindow(p.anchor)
	if !ok {
		return fmt.Errorf("popover: anchor is not mounted in a window")
	}
	if p.platformPopup != nil && p.owner != win {
		p.releaseNative() // anchor moved to another window; rebuild for the new one
		if p.destroyed {
			return fmt.Errorf("popover: destroyed during owner change")
		}
	}
	// The owner may have moved without changing any owner-local coordinates.
	// Each Show must let the platform resolve them against the current origin.
	p.positionValid = false
	if p.platformPopup == nil {
		if err := p.createNative(win); err != nil {
			return err
		}
	} else {
		p.measureAndSize()
	}
	p.reposition()
	native := p.platformPopup
	if err := native.Show(); err != nil {
		return err
	}
	if p.platformPopup != native {
		return fmt.Errorf("popover: released during Show")
	}
	p.visible = true
	if p.modal {
		// Only a modal (menu) popover intercepts the window's input; modeless
		// tooltips/panels leave the window's own input untouched.
		p.becomeModalTarget()
	}
	return nil
}

func (p *popover) Hide() {
	if !p.visible {
		return
	}
	p.visible = false
	p.resetInput()
	if p.modal {
		p.resignModalTarget()
	}
	if p.platformPopup != nil {
		_ = p.platformPopup.Hide()
	}
	p.closed.Emit()
}

func (p *popover) Destroy() {
	if p.destroyed {
		return
	}
	p.destroyed = true
	p.releaseNative()
	p.widget = nil
}

// Reset controller state as well as dispatch paths. The menu additionally
// resets its presentation state, since Controller.Reset need not emit signals.
func (p *popover) resetInput() {
	p.dispatcher = EventDispatcher{}
	var reset func(Widget)
	reset = func(w Widget) {
		if w == nil || w.base().destroyed {
			return
		}
		for _, c := range w.EventControllers() {
			c.Reset()
		}
		for _, child := range w.Children() {
			reset(child)
		}
	}
	reset(p.Widget())
}

// Distinguishes creation/capability failures from failures of native Show.
// Only PopoverMenu is allowed to retry creation with an opaque configuration.
type popoverCreationError struct{ error }

func (e *popoverCreationError) Unwrap() error { return e.error }

// --- native resource lifecycle (belongs to the window) ---

func (p *popover) createNative(win Window) error {
	if App == nil {
		return fmt.Errorf("popover: application is not created")
	}
	p.owner = win
	if widget := p.Widget(); widget != nil && widget.Root() != p {
		adoptWidget(widget, p)
	}
	// No style connection exists while native resources are absent. Content
	// may have been measured before the application changed its sheet.
	invalidateStyleSubtree(p.Widget())
	p.measureAndSize() // carries the requested content size into native creation
	if p.destroyed {
		return fmt.Errorf("popover: destroyed during measurement")
	}

	// Platform + typography come from the app (global escape hatches); the owner
	// platform window comes from the host's PlatformWindow escape hatch.
	pp, err := App.Platform().NewPopup(win.PlatformWindow(), geometry.Size{Width: p.width, Height: p.height}, p.onEvent, platform.PopupOptions{Transparent: p.transparent})
	if err != nil {
		p.releaseNative()
		return &popoverCreationError{fmt.Errorf("create platform popup: %w", err)}
	}
	painter, err := App.Platform().NewPainter(pp)
	if err != nil {
		pp.Destroy()
		p.releaseNative()
		return &popoverCreationError{fmt.Errorf("create popover painter: %w", err)}
	}
	p.platformPopup = pp
	p.painter = painter

	// Auto-release when the anchor leaves the tree or the window is destroyed —
	// the native surface never outlives its owner window.
	p.hUnmount = p.anchor.ConnectUnmount(p.releaseNative)
	p.hWinGone = win.ConnectDestroy(p.releaseNative)
	if app, ok := App.(*application); ok {
		p.hStyle = app.styleChanged.Connect(func() {
			invalidateStyleSubtree(p.Widget())
			p.measureAndSize()
			p.requestLayout()
		})
	}
	return nil
}

func (p *popover) releaseNative() {
	wasVisible := p.visible
	p.visible = false
	p.resetInput()
	if p.hUnmount != nil {
		p.hUnmount.Disconnect()
		p.hUnmount = nil
	}
	if p.hWinGone != nil {
		p.hWinGone.Disconnect()
		p.hWinGone = nil
	}
	if p.hStyle != nil {
		p.hStyle.Disconnect()
		p.hStyle = nil
	}
	if wasVisible && p.modal {
		p.resignModalTarget()
	}
	if p.painter != nil {
		p.painter.Destroy()
		p.painter = nil
	}
	native := p.platformPopup
	p.platformPopup = nil
	p.owner = nil
	p.positionValid = false
	p.requestedSize = geometry.Size{}
	if p.widget != nil && p.widget.Root() == p {
		p.widget.base().detachRoot(p.widget)
	}
	if native != nil {
		native.Destroy()
	}
	if wasVisible {
		p.closed.Emit()
	}
}

// measureAndSize requests an intrinsic content size plus shadow allocation.
func (p *popover) measureAndSize() {
	p.layoutDirty = true
	p.updateNaturalSize()
}

func (p *popover) updateNaturalSize() {
	if p.widget == nil {
		return
	}
	// Popup is intrinsic: measure with a loose constraint so the popover sizes to
	// its content, independent of the owner window's size.
	const loose = 1 << 14
	size := measureWidget(p.widget, layout.Loose(geometry.Size{Width: loose, Height: loose})).Size
	s := p.resolvedStyle()
	shadow, _ := s.Shadow()
	radius, _ := s.Radius()
	p.insets = popoverInsets{}
	if p.transparent {
		p.insets = popoverShadowInsets(size, radius, shadow)
	}
	width, height := size.Width, size.Height
	if width < 1 {
		width = 1
	}
	if height < 1 {
		height = 1
	}
	width += p.insets.left + p.insets.right
	height += p.insets.top + p.insets.bottom
	desired := geometry.Size{Width: width, Height: height}
	changed := p.requestedSize != desired
	p.requestedSize = desired // before SetSize: it may synchronously send SizeEvent
	if p.platformPopup != nil {
		// The platform's SizeEvent is authoritative: native surfaces have an
		// integer physical size, so their logical size can differ slightly from
		// the requested DIP size after DPI conversion. Keep the current actual
		// size until that event arrives. In particular, resizing to the same
		// physical dimensions may produce no event at all.
		if changed {
			p.platformPopup.SetSize(width, height)
		}
		p.reposition()
	} else {
		// Before native creation these fields carry the requested size into
		// Platform.NewPopup. Creation's SizeEvent replaces them with the actual
		// client size.
		p.width, p.height = width, height
	}
}

func (p *popover) reposition() {
	if p.platformPopup == nil {
		return
	}
	o := absOrigin(p.anchor)
	pos := geometry.Point{X: o.X + p.position.X - p.insets.left, Y: o.Y + p.position.Y - p.insets.top}
	if !p.positionValid || pos != p.requestedPosition {
		p.requestedPosition, p.positionValid = pos, true
		p.platformPopup.SetPosition(pos.X, pos.Y)
	}
}

// --- events / paint ---

func (p *popover) onEvent(event platform.Event) {
	switch e := event.(type) {
	case events.SizeEvent:
		p.width, p.height = e.Width, e.Height
		p.pixelWidth, p.pixelHeight = e.PixelWidth, e.PixelHeight
		p.requestLayout()
	case events.PaintEvent:
		p.paint()
	default:
		_ = p.DispatchEvent(event)
	}
}

func (p *popover) paint() {
	if p.painter == nil || p.destroyed {
		return
	}
	p.widget = liveRoot(p.widget)
	p.paintDirty = false
	if p.layoutDirty {
		p.layoutDirty = false
		p.updateNaturalSize()
		if p.widget != nil {
			body := p.bodyRect()
			measureWidget(p.widget, layout.Tight(body.Size))
			p.widget.Arrange(body)
		}
	}
	background, border, shadow := p.surfaceStyle()
	radius, _ := border.Radius()
	width, _ := border.BorderWidth()
	body := p.bodyRect()
	inset := popoverSafeInset(radius, width)
	safe := geometry.Rect(body.X+inset, body.Y+inset, max(0, body.Width-2*inset), max(0, body.Height-2*inset))
	p.drawSurfaceFrame(p.widget, background, border, body, safe, shadow)
}

// RequestLayout satisfies Root: schedule a relayout of this host.
func (p *popover) RequestLayout() { p.requestLayout() }

func (p *popover) requestLayout() {
	p.rootBase.requestLayout(p.platformPopup)
}

func (p *popover) requestPaint() error {
	return p.rootBase.requestPaint(p.platformPopup)
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

// Only conservative surface allocation lives here; the graphics backend owns
// the shadow mask. Keep its 3-sigma extent in sync with BoxShadow semantics.
type popoverInsets struct{ left, top, right, bottom float32 }

func popoverShadowInsets(size geometry.Size, radius float32, s style.Shadow) popoverInsets {
	if s.Color == nil || size.Width <= 0 || size.Height <= 0 {
		return popoverInsets{}
	}
	_, _, _, alpha := s.Color.RGBA()
	if alpha == 0 {
		return popoverInsets{}
	}
	for _, v := range []float32{size.Width, size.Height, radius, s.Offset.X, s.Offset.Y, s.BlurRadius, s.SpreadRadius, size.Width + 2*s.SpreadRadius, size.Height + 2*s.SpreadRadius, radius + s.SpreadRadius} {
		if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
			return popoverInsets{}
		}
	}
	if size.Width+2*s.SpreadRadius <= 0 || size.Height+2*s.SpreadRadius <= 0 {
		return popoverInsets{}
	}
	blur := float32(0)
	if s.BlurRadius > 0 {
		blur = max(.5, s.BlurRadius)
	}
	extent := 1.5*blur + s.SpreadRadius
	pad := func(v float32) float32 { return float32(math.Ceil(float64(max(0, v) + 1))) }
	insets := popoverInsets{pad(extent - s.Offset.X), pad(extent - s.Offset.Y), pad(extent + s.Offset.X), pad(extent + s.Offset.Y)}
	if math.IsInf(float64(size.Width+insets.left+insets.right), 0) || math.IsInf(float64(size.Height+insets.top+insets.bottom), 0) {
		return popoverInsets{}
	}
	return insets
}

func popoverSafeInset(radius, border float32) float32 {
	inset := normalizeLayoutValue(border)
	if radius > 0 {
		inset = max(inset, radius*(1-1/math.Sqrt2)+1)
	}
	return inset
}

func (p *popover) resolvedStyle() style.Style {
	name := p.styleName
	if name == "" {
		name = styleNamePopover
	}
	return ResolveStyle(name, style.PartDefault, style.Normal)
}

// surfaceStyle changes effective geometry, never the application's style sheet.
func (p *popover) surfaceStyle() (style.Style, style.Style, graphics.BoxShadow) {
	s := p.resolvedStyle()
	radius, _ := s.Radius()
	radius = min(normalizeLayoutValue(radius), max(0, min(p.bodyRect().Width, p.bodyRect().Height)/2))
	if !p.transparent {
		radius = 0
	}
	width, _ := s.BorderWidth()
	color, _ := s.BorderColor()
	border := style.Name("").Radius(radius).BorderWidth(normalizeLayoutValue(width)).BorderColor(color).Style
	var shadow graphics.BoxShadow
	if p.transparent && p.insets != (popoverInsets{}) {
		v, _ := s.Shadow()
		shadow = graphics.BoxShadow{Color: graphics.ColorOf(v.Color), Offset: v.Offset, BlurRadius: v.BlurRadius, SpreadRadius: v.SpreadRadius}
	}
	return s, border, shadow
}

func (p *popover) bodyRect() geometry.Rectangle {
	i := p.insets
	return geometry.Rect(i.left, i.top, max(0, p.width-i.left-i.right), max(0, p.height-i.top-i.bottom))
}

func roundedBodyContains(rect geometry.Rectangle, radius float32, point geometry.Point) bool {
	if point.X < rect.X || point.Y < rect.Y || point.X >= rect.X+rect.Width || point.Y >= rect.Y+rect.Height {
		return false
	}
	radius = min(normalizeLayoutValue(radius), min(rect.Width, rect.Height)/2)
	x := point.X - min(max(point.X, rect.X+radius), rect.X+rect.Width-radius)
	y := point.Y - min(max(point.Y, rect.Y+radius), rect.Y+rect.Height-radius)
	return x*x+y*y <= radius*radius
}
