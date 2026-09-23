package gui

import (
	"errors"
	"fmt"
	"log"
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

	// Position is the requested body origin relative to the anchor, excluding
	// shadow (DIP). Desktop workarea adjustment does not change this request.
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
	anchor                    Widget
	widget                    Widget // content
	position                  geometry.Point
	owner                     Window // resolved from the anchor; only the public Window API is used
	platformPopup             platform.Popup
	styleName                 string
	insets                    popoverInsets
	requestedSize             geometry.Size
	requestedPosition         geometry.Point
	positionValid             bool
	anchorRectangle           bool // MenuButton requests the whole anchor's bounds
	placement                 popoverPlacement
	placementValid            bool
	workAreaUnsupportedLogged bool
	lifecycle                 uint64 // invalidates in-flight native/measurement callbacks
	destroyed                 bool
	visible                   bool
	modal                     bool // menu-style: owner window forwards its input here (modeless by default)

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
	p.anchorRectangle = false
	if p.visible {
		p.measureAndSize()
		p.requestPaint()
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
	showEpoch := p.lifecycle
	// The owner may have moved without changing any owner-local coordinates.
	// Each Show must let the platform resolve them against the current origin.
	p.positionValid = false
	if p.platformPopup == nil {
		if err := p.createNative(win); err != nil {
			return err
		}
	} else {
		p.layoutDirty = true
		if err := p.updateNaturalSize(); err != nil {
			return err
		}
	}
	p.reposition()
	if p.lifecycle != showEpoch || p.destroyed {
		return fmt.Errorf("popover: released while preparing Show")
	}
	native := p.platformPopup
	if native == nil {
		return fmt.Errorf("popover: released before Show")
	}
	epoch := p.lifecycle
	if err := native.Show(); err != nil {
		return err
	}
	if p.platformPopup != native || p.lifecycle != epoch {
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
	p.lifecycle++
	wasVisible := p.visible
	if !wasVisible && p.platformPopup == nil {
		return
	}
	p.visible = false
	p.resetInput()
	if wasVisible && p.modal {
		p.resignModalTarget()
	}
	if p.platformPopup != nil {
		_ = p.platformPopup.Hide()
	}
	if wasVisible {
		p.closed.Emit()
	}
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
	p.cancelInput(GestureHostClosed)
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
	p.layoutDirty = true
	if err := p.updateNaturalSize(); err != nil {
		p.releaseNative()
		return err
	}
	if p.destroyed {
		return fmt.Errorf("popover: destroyed during measurement")
	}

	// Platform + typography come from the app (global escape hatches); the owner
	// platform window comes from the host's PlatformWindow escape hatch.
	epoch := p.lifecycle
	pp, err := App.Platform().NewPopup(win.PlatformWindow(), p.requestedSize, p.onEvent, platform.PopupOptions{Transparent: p.transparent})
	if err != nil {
		p.releaseNative()
		return &popoverCreationError{fmt.Errorf("create platform popup: %w", err)}
	}
	currentOwner, mounted := anchorWindow(p.anchor)
	if p.lifecycle != epoch || p.destroyed || !mounted || currentOwner != win {
		pp.Destroy()
		p.releaseNative()
		return fmt.Errorf("popover: released during creation")
	}
	painter, err := App.Platform().NewPainter(pp)
	if err != nil {
		pp.Destroy()
		p.releaseNative()
		return &popoverCreationError{fmt.Errorf("create popover painter: %w", err)}
	}
	currentOwner, mounted = anchorWindow(p.anchor)
	if p.lifecycle != epoch || p.destroyed || !mounted || currentOwner != win {
		painter.Destroy()
		pp.Destroy()
		p.releaseNative()
		return fmt.Errorf("popover: released during painter creation")
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
	p.lifecycle++
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
	p.placementValid = false
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
	if err := p.updateNaturalSize(); err != nil {
		log.Printf("goui: popup layout: %v", err)
	}
}

func (p *popover) updateNaturalSize() error {
	epoch, widget := p.lifecycle, p.Widget()
	snapshot, err := p.queryPlacement()
	if err != nil {
		return err
	}
	size := geometry.Size{Width: 1, Height: 1}
	if widget != nil {
		size = measureWidget(widget, layout.Unbounded()).Size
	}
	if !finitePopup(size.Width) || !finitePopup(size.Height) {
		return fmt.Errorf("popover: non-finite content size")
	}
	size.Width, size.Height = max(1, size.Width), max(1, size.Height)
	s := p.resolvedStyle()
	shadow, _ := s.Shadow()
	radius, _ := s.Radius()
	insets := popoverInsets{}
	if p.transparent {
		insets = popoverShadowInsets(size, radius, shadow)
	}
	if snapshot.constrained {
		limit, err := snapshot.bodyLimit(insets)
		if err != nil {
			return err
		}
		if size.Width > limit.Width || size.Height > limit.Height {
			c := layout.Loose(limit)
			if widget != nil {
				size = measureWidget(widget, c).Size
			}
			if !finitePopup(size.Width) || !finitePopup(size.Height) {
				return fmt.Errorf("popover: non-finite constrained content size")
			}
			size = c.Clamp(geometry.Size{Width: max(1, size.Width), Height: max(1, size.Height)})
		}
	}
	// Keep conservative natural shadow allocation during shrinking: negative
	// spread may erase the smaller mask, but must not cause a sizing oscillation.
	width, height := size.Width+insets.left+insets.right, size.Height+insets.top+insets.bottom
	if !validPopupRect(geometry.Rect(0, 0, width, height)) {
		return fmt.Errorf("popover: invalid surface size")
	}
	if p.lifecycle != epoch || p.destroyed || p.Widget() != widget {
		return fmt.Errorf("popover: released or content replaced during layout")
	}
	if p.owner != nil {
		if win, ok := anchorWindow(p.anchor); !ok || win != p.owner {
			return fmt.Errorf("popover: anchor moved during layout")
		}
	}
	p.placement, p.placementValid, p.insets = snapshot, true, insets
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
		if p.lifecycle != epoch || p.destroyed {
			return fmt.Errorf("popover: released during resize")
		}
		p.reposition()
	} else {
		// Before native creation these fields carry the requested size into
		// Platform.NewPopup. Creation's SizeEvent replaces them with the actual
		// client size.
		p.width, p.height = width, height
	}
	return nil
}

func (p *popover) queryPlacement() (popoverPlacement, error) {
	origin := absOrigin(p.anchor)
	snapshot := popoverPlacement{anchor: geometry.Rect(origin.X+p.position.X, origin.Y+p.position.Y, 0, 0), rectangle: p.anchorRectangle}
	point := snapshot.anchor.Pos
	if snapshot.rectangle && p.anchor != nil {
		snapshot.anchor = geometry.Rectangle{Pos: origin, Size: p.anchor.Rect().Size}
		point = snapshot.anchor.Center()
	}
	if !finitePopup(point.X) || !finitePopup(point.Y) {
		return snapshot, fmt.Errorf("popover: non-finite anchor position")
	}
	var err error = platform.ErrUnsupported
	if p.owner != nil {
		if desktop, ok := p.owner.PlatformWindow().(platform.DesktopWindow); ok {
			snapshot.workArea, err = desktop.WorkAreaAt(point)
		}
	}
	if errors.Is(err, platform.ErrUnsupported) {
		if !p.workAreaUnsupportedLogged {
			log.Printf("goui: popup work area unsupported; using unconstrained placement")
			p.workAreaUnsupportedLogged = true
		}
		return snapshot, nil
	}
	if err != nil {
		return snapshot, fmt.Errorf("popover work area: %w", err)
	}
	if !validPopupRect(snapshot.workArea) {
		return snapshot, fmt.Errorf("popover work area: invalid rectangle: %w", platform.ErrUnavailable)
	}
	snapshot.constrained = true
	return snapshot, nil
}

func (p *popover) reposition() {
	if p.platformPopup == nil {
		return
	}
	if !p.placementValid {
		return
	}
	pos := p.placement.position(geometry.Size{Width: p.width, Height: p.height}, p.insets)
	if !p.positionValid || pos != p.requestedPosition {
		p.requestedPosition, p.positionValid = pos, true
		p.platformPopup.SetPosition(pos.X, pos.Y)
	}
}

// --- events / paint ---

func (p *popover) onEvent(event platform.Event) {
	if p.destroyed {
		return
	}
	switch e := event.(type) {
	case events.SizeEvent:
		p.width, p.height = e.Width, e.Height
		p.pixelWidth, p.pixelHeight = e.PixelWidth, e.PixelHeight
		p.reposition() // correct native rounding against the completed snapshot
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
		if err := p.updateNaturalSize(); err != nil {
			log.Printf("goui: popup layout: %v", err)
			return // retain the last completed native frame on query failure
		}
		epoch := p.lifecycle
		if p.widget != nil {
			body := p.bodyRect()
			measureWidget(p.widget, layout.Tight(body.Size))
			p.widget.Arrange(body)
		}
		if p.lifecycle != epoch || p.painter == nil || p.destroyed {
			return
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

// One solve uses one owner-local snapshot. No native objects or widgets are
// retained here; rounded SizeEvents can reuse it without another display query.
type popoverPlacement struct {
	anchor      geometry.Rectangle
	rectangle   bool
	workArea    geometry.Rectangle
	constrained bool
}

func finitePopup(v float32) bool { return !math.IsNaN(float64(v)) && !math.IsInf(float64(v), 0) }

func validPopupRect(r geometry.Rectangle) bool {
	return r.Width > 0 && r.Height > 0 && finitePopup(r.X) && finitePopup(r.Y) &&
		finitePopup(r.Width) && finitePopup(r.Height) && finitePopup(r.X+r.Width) && finitePopup(r.Y+r.Height)
}

func (s popoverPlacement) bodyLimit(insets popoverInsets) (geometry.Size, error) {
	size := geometry.Size{Width: s.workArea.Width - insets.left - insets.right, Height: s.workArea.Height - insets.top - insets.bottom}
	if !finitePopup(size.Width) || !finitePopup(size.Height) || size.Width < 1 || size.Height < 1 {
		return geometry.Size{}, fmt.Errorf("popover: shadow leaves no usable body in work area")
	}
	return size, nil
}

// position returns the full surface origin. Alignment is against the body,
// containment against the surface. Oversized actual native sizes anchor at the
// workarea origin rather than generating resize feedback loops.
func (s popoverPlacement) position(surface geometry.Size, insets popoverInsets) geometry.Point {
	a := s.anchor
	body := geometry.Size{Width: max(0, surface.Width-insets.left-insets.right), Height: max(0, surface.Height-insets.top-insets.bottom)}
	preferred := geometry.Point{X: a.X - insets.left, Y: a.Y - insets.top}
	if s.rectangle {
		preferred.Y += a.Height
	}
	if !s.constrained {
		return preferred
	}
	area := s.workArea
	fits := func(p geometry.Point) bool {
		return p.X >= area.X && p.Y >= area.Y && p.X+surface.Width <= area.X+area.Width && p.Y+surface.Height <= area.Y+area.Height
	}
	if s.rectangle {
		for _, p := range [...]geometry.Point{
			preferred,
			{X: a.X + a.Width - body.Width - insets.left, Y: preferred.Y},
			{X: preferred.X, Y: a.Y - body.Height - insets.top},
			{X: a.X + a.Width - body.Width - insets.left, Y: a.Y - body.Height - insets.top},
		} {
			if fits(p) {
				return p
			}
		}
	}
	return geometry.Point{
		X: min(max(preferred.X, area.X), max(area.X, area.X+area.Width-surface.Width)),
		Y: min(max(preferred.Y, area.Y), max(area.Y, area.Y+area.Height-surface.Height)),
	}
}
