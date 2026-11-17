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

// PopoverPlacement specifies the preferred position of a popover's body.
// Work area constraints may adjust the actual position without changing it.
type PopoverPlacement uint8

const (
	// PopoverPlacementPoint uses Position as the body's preferred top-left
	// corner, in anchor-local DIP. It is the default placement.
	PopoverPlacementPoint PopoverPlacement = iota
	// PopoverPlacementBottom prefers the bottom-left of the complete anchor
	// rectangle. It may align right, flip above, or slide into the work area.
	PopoverPlacementBottom
)

// Popover is a borderless, no-focus floating surface anchored to a Widget, used
// for menus, detached toolbars and tooltips. Modal controls owner input
// exclusivity; AutoDismiss enables standard dismissal requests independently.
// The control handles those requests and drives Show/Hide/Destroy.
//
// A Popover belongs to its anchor's Surface: the native surface is created on
// Show, retained across anchor unmounts, and released when its current owner
// Surface is destroyed or a later Show resolves another Surface. NewSharedPopover provides a separate window-shared popup API.
type Popover interface {
	Root

	Visible() bool
	// Transparent reports the immutable native configuration.
	Transparent() bool
	// StyleName is explicit; an empty name resolves the "popover" style.
	StyleName() string
	SetStyleName(string)

	SetWidget(Widget)

	Anchor() Widget
	// SetAnchor ends the current occurrence and configures the next Show.
	// It retains native resources until Show resolves the next owner.
	SetAnchor(Widget)

	// Placement reports the requested strategy, not the actual placement.
	Placement() PopoverPlacement
	// SetPlacement preserves Position. Unknown values use Point placement.
	// A visible popover updates on its next layout without being reopened.
	SetPlacement(PopoverPlacement)

	// Position is the requested body origin relative to the anchor, excluding
	// shadow (DIP). Desktop workarea adjustment does not change this request.
	// Only Point placement uses it; other strategies preserve but ignore it.
	Position() geometry.Point
	// SetPosition preserves Placement and schedules layout only in Point mode.
	SetPosition(geometry.Point)

	// Modal reports whether this popover is modal relative to its owner window:
	// the window forwards keyboard input here and blocks owner pointer input.
	// AutoDismiss separately controls dismissal requests. Default false; modeless
	// surfaces receive keys only through explicit DispatchEvent calls. Modal is
	// scoped to the owner window; it does not block other windows.
	Modal() bool
	SetModal(bool)
	// AutoDismiss enables dismissal requests on outside presses, Escape and
	// owner deactivation. The default is false, independently of Modal.
	// The anchor is inside the region; modeless outside presses continue to
	// their original owner target unless a callback changes that input context.
	AutoDismiss() bool
	SetAutoDismiss(bool)
	// DispatchEvent uses this surface's normal input path and local DIP.
	// Native keyboard input reaches modal popovers through their owner window.
	// Call on the GUI thread. Visible modeless popovers accept explicit key
	// forwarding; modal popovers must own their window's modal input.
	DispatchEvent(events.Event) error
	// CommitText submits confirmed UTF-8 text to this popover's editable logical
	// focus. It requires a visible popover holding its owner's modal input.
	// Like Window.CommitText, it does not change focus or simulate native IME;
	// native preedit/candidate support remains a separate capability.
	CommitText(text string) error

	// Show resolves current anchor geometry and work area using the configured
	// Placement and Position. It does not change either request, even on failure.
	// Geometry is prepared while hidden and stale presentation is invalidated
	// before showing, for both independent and Surface-cached popovers.
	Show() error
	Hide()
	Destroy()

	// ConnectDismissRequest fires for outside presses, Escape or owner
	// deactivation while AutoDismiss is enabled. The control decides what
	// to do, typically Hide.
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
	p := &popover{anchor: anchor, rootBase: rootBase{frameSuspended: true}}
	if options != nil {
		p.transparent = options.Transparent
	}
	return p
}

// popover is the GUI root and resource owner for one floating surface.
type popover struct {
	mutationDepth uint32
	clearing      bool
	paintDepth    uint32
	target        *popoverTarget
	hiding        bool
	rootBase
	anchor                    Widget
	widget                    Widget // content
	position                  geometry.Point
	placement                 PopoverPlacement
	anchorArea                *geometry.Rectangle // optional anchor-local area for internal presentations
	owner                     Surface             // resolved from the anchor; native creation uses its platform Window
	platformPopup             platform.Popup
	styleName                 string
	insets                    popoverInsets
	requestedSize             geometry.Size
	requestedPosition         geometry.Point
	positionValid             bool
	placementSnapshot         popoverPlacementSnapshot
	placementSnapshotValid    bool
	workAreaUnsupportedLogged bool
	lifecycle                 uint64 // invalidates in-flight native/measurement callbacks
	releasing                 bool
	releasePending            bool // native termination may reenter a frame
	showing                   bool
	preparing                 bool // defer native paint and event-driven positioning during preparation
	destroyed                 bool
	visible                   bool
	modal                     bool
	autoDismiss               bool // requests closing independently of modal input ownership

	dismissRequest signal.Signal0
	closed         signal.Signal0
	hUnmount       signal.Handle // anchor unmount -> Hide
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
	ensureModal(p)
	if widget != nil && ((!p.visible && p.lifecycle != 0) || p.destroyed || widget.Root() != p || !focusEligible(widget)) {
		return false
	}
	p.focusedWidget = widget
	p.dispatcher.updateFocus(p.Widget(), widget)
	p.requestPaint()
	return true
}

func (p *popover) CommitText(text string) error {
	return commitText(p, text)
}

func (p *popover) ownsModalInput() bool {
	owner := surfaceCore(p.owner)
	ok := owner != nil
	return ok && !owner.destroyed && !owner.frameSuspended && p.modal && owner.modalTarget == p
}

// --- Popover API ---

func (p *popover) Visible() bool               { return !p.destroyed && p.visible }
func (p *popover) Anchor() Widget              { return p.anchor }
func (p *popover) Position() geometry.Point    { return p.position }
func (p *popover) Placement() PopoverPlacement { return p.placement }
func (p *popover) Modal() bool                 { return p.modal }
func (p *popover) AutoDismiss() bool           { return p.autoDismiss }
func (p *popover) SetAutoDismiss(v bool)       { p.autoDismiss = v }
func (p *popover) StyleName() string           { return p.styleName }

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
	if p.destroyed || p.widget == widget || widget != nil && widget.base().destroyed {
		return
	}
	owner := surfaceCore(p.owner)
	p.mutationDepth++
	defer func() {
		p.mutationDepth--
		p.finishRelease()
		if owner != nil {
			owner.finishDestroy()
		}
	}()
	previous := p.widget
	p.widget = widget
	if previous != nil {
		previous.base().detachRoot(previous)
	}
	if p.destroyed || p.widget != widget || owner != nil && owner.destroyed {
		return
	}
	if widget != nil {
		adoptWidget(widget, p)
	}
	if p.destroyed || p.widget != widget || owner != nil && owner.destroyed {
		return
	}
	modalChanged(p)
	ensureModal(p)
	p.dragControllersChanged()
	p.layoutDirty = true
	if p.visible {
		p.measureAndSize()
		p.requestPaint()
	}
}

func (p *popover) SetPlacement(placement PopoverPlacement) {
	if placement != PopoverPlacementBottom {
		placement = PopoverPlacementPoint
	}
	if p.destroyed || p.placement == placement {
		return
	}
	p.placement = placement
	p.layoutDirty = true
	if p.visible {
		p.requestLayout()
	}
}

func (p *popover) SetPosition(pos geometry.Point) {
	if p.destroyed || p.position == pos {
		return
	}
	p.position = pos
	if p.placement == PopoverPlacementPoint {
		p.layoutDirty = true
		if p.visible {
			p.requestLayout()
		}
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
// popovers never intercept the window. Resignation checks current ownership
// so an old surface cannot clear a replacement modal target.
func (p *popover) becomeModalTarget() {
	if p.owner != nil {
		p.owner.SetModalTarget(p)
	}
}

func (p *popover) resignModalTarget() {
	if owner := surfaceCore(p.owner); owner != nil {
		if owner.modalTarget == p {
			owner.SetModalTarget(nil)
		}
	} else if p.owner != nil {
		p.owner.SetModalTarget(nil)
	}
}

// DispatchEvent routes an event the owner window forwards (keyboard nav, since
// the popover has no native focus) to the popover's content. Part of ModalTarget.
func (p *popover) DispatchEvent(event events.Event) error {
	if _, ok := event.(events.DestroyEvent); ok {
		p.releaseNative()
		return nil
	}
	if e, ok := event.(events.DragSourceEvent); ok {
		if app := dragAppOf(p); app != nil {
			app.dispatchDragSourceEvent(e)
		}
		return nil
	}
	if p.destroyed {
		return nil
	}
	if !p.visible && (p.anchor != nil || p.lifecycle != 0) {
		return fmt.Errorf("popover: input target is unavailable")
	}
	ensureModal(p)
	if p.destroyed {
		return nil
	}
	if p.anchor != nil && !modalWidgetAllowed(p.anchor) {
		modalPreventDefault(event)
		return nil
	}
	if _, key := event.(events.KeyEvent); key {
		// A never-presented, unanchored popover is also a valid offscreen GUI
		// host. Presented or anchored surfaces must still be visible.
		standalone := p.anchor == nil && p.owner == nil && p.lifecycle == 0
		if !p.visible && !standalone {
			return fmt.Errorf("popover: keyboard input requires a visible surface")
		}
		if p.modal && p.owner != nil && !p.ownsModalInput() {
			return fmt.Errorf("popover: does not own modal keyboard input")
		}
		if owner := surfaceCore(p.owner); owner != nil && (owner.destroyed || owner.frameSuspended || owner.modalTarget != nil && owner.modalTarget != p) {
			return fmt.Errorf("popover: owner keyboard input is unavailable")
		}
	}
	if e, ok := event.(events.DragOfferEvent); ok {
		p.dispatchDragOffer(p, e)
		return nil
	}
	if e, ok := event.(events.DragDataEvent); ok {
		p.dispatchDragData(p, e)
		return nil
	}
	if p.autoDismiss {
		switch e := event.(type) {
		case events.PointerEvent:
			_, border, _ := p.surfaceStyle()
			radius, _ := border.CornerRadius()
			if e.EventType == events.PointerDown && !roundedBodyContains(p.bodyRect(), radius, e.Position) {
				setFocusVisible(p, false)
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

// RequestDismiss asks the controller to close an active AutoDismiss surface.
// Explicit Hide and Destroy do not depend on this policy.
func (p *popover) RequestDismiss() {
	if p.autoDismiss && p.visible && !p.destroyed {
		p.dismissRequest.Emit()
	}
}

func (p *popover) Show() error {
	if p.destroyed {
		return fmt.Errorf("popover: destroyed")
	}
	if p.visible {
		return nil
	}
	if p.anchor != nil && !modalWidgetAllowed(p.anchor) {
		return fmt.Errorf("popover: anchor is outside the active input context")
	}
	if p.releasing || p.showing || p.framePainting || p.paintDepth != 0 || p.mutationDepth != 0 {
		return fmt.Errorf("popover: resource transition in progress")
	}
	p.showing = true
	p.preparing = true
	var showOwner *surfaceBase
	defer func() {
		p.showing, p.preparing = false, false
		if showOwner != nil {
			showOwner.finishDestroy()
		}
	}()
	if p.destroyed {
		return fmt.Errorf("popover: destroyed")
	}
	win, ok := anchorSurface(p.anchor)
	if !ok {
		return fmt.Errorf("popover: anchor is not mounted in a Surface")
	}
	showOwner = surfaceCore(win)
	if owner := showOwner; owner != nil && owner.destroyed {
		return fmt.Errorf("popover: owner is destroyed")
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
	if err := p.prepareShow(); err != nil {
		return err
	}

	p.preparing = false
	p.reposition()
	if p.lifecycle != showEpoch || p.destroyed {
		return fmt.Errorf("popover: released while preparing Show")
	}
	native := p.platformPopup
	if native == nil {
		return fmt.Errorf("popover: released before Show")
	}
	if owner := surfaceCore(p.owner); owner != nil && owner.destroyed {
		return ErrPopoverSuperseded
	}
	if reset, ok := p.painter.(graphics.PresentationInvalidator); ok {
		if err := reset.InvalidatePresentation(); err != nil {
			return err
		}
	}
	p.clearing = true
	clearError := native.Repaint()
	p.clearing = false
	if clearError != nil {
		return clearError
	}
	if p.destroyed || p.lifecycle != showEpoch || p.platformPopup != native || surfaceCore(p.owner) != nil && surfaceCore(p.owner).destroyed {
		return ErrPopoverSuperseded
	}

	epoch := p.lifecycle
	if err := native.Show(); err != nil {
		return err
	}
	if p.platformPopup != native || p.lifecycle != epoch {
		return fmt.Errorf("popover: released during Show")
	}
	if !p.visible {
		p.target = &popoverTarget{host: p, anchor: p.anchor}
	}
	p.visible = true
	if owner := surfaceCore(p.owner); owner != nil {
		owner.addPopover(p)
	}
	if p.modal {
		// Modal surfaces exclusively receive owner input. Modeless surfaces
		// keep normal owner routing and use explicit keyboard forwarding.
		p.becomeModalTarget()
	}
	if p.destroyed || p.platformPopup != native || p.lifecycle != epoch || !p.visible || p.modal && !p.ownsModalInput() {
		p.Hide()
		return fmt.Errorf("popover: input ownership changed while showing")
	}
	// Commit visibility and modal ownership before allowing frame callbacks.
	// Draw through the native paint context immediately: posting the first
	// frame would expose the previous borrower's retained surface meanwhile.
	p.frameSuspended = false
	p.frameRequester = native
	if err := native.Repaint(); err != nil {
		p.Hide()
		return fmt.Errorf("popover first frame: %w", err)
	}
	if p.destroyed || p.platformPopup != native || p.lifecycle != epoch || !p.visible {
		return fmt.Errorf("%w: released during first frame", ErrPopoverSuperseded)
	}

	return nil
}

func (p *popover) Hide() {
	p.hiding = true
	cleanupOwner := surfaceCore(p.owner)
	defer func() {
		p.hiding = false
		if cleanupOwner != nil {
			cleanupOwner.finishDestroy()
		}
	}()
	cancelTooltipHost(p)
	p.suspendFrames()
	p.lifecycle++
	wasVisible := p.visible
	native, owner := p.platformPopup, p.owner
	if !wasVisible && p.platformPopup == nil {
		return
	}
	target := p.target
	p.target = nil
	p.visible = false
	if owner := surfaceCore(p.owner); owner != nil {
		owner.removePopover(p)
	}
	p.resetInput()
	// Focus callbacks may reopen this same surface or replace its resources.
	// Complete the old notification without hiding that new presentation.
	if p.visible || p.owner != owner || p.platformPopup != native {
		if wasVisible {
			p.emitClosed(target)
		}
		return
	}
	if wasVisible && p.modal {
		p.resignModalTarget()
	}
	if p.platformPopup != nil {
		_ = p.platformPopup.Hide()
	}
	if wasVisible {
		p.emitClosed(target)
	}
}

func (p *popover) Destroy() {
	if p.destroyed {
		return
	}
	p.destroyed = true
	p.destroyFrames()
	p.releaseNative()
}

// Reset controller state as well as dispatch paths. The menu additionally
// resets its presentation state, since Controller.Reset need not emit signals.
func (p *popover) resetInput() {
	epoch, root := p.lifecycle, p.Widget()
	// Callbacks may reopen or replace this surface during any cleanup step.
	current := func() bool {
		return p.lifecycle == epoch && !p.visible && p.Widget() == root
	}
	p.cancelInput(GestureHostClosed)
	if !current() {
		return
	}
	// Deliver leaves while the old hit path is still available. Resetting
	// controllers alone does not clear Widget's pointer-crossing state.
	p.dispatcher.clearHover(p.dispatcher.pointerEvent)
	if !current() {
		return
	}
	setFocusVisible(p, false)
	if !current() {
		return
	}
	p.SetFocusedWidget(nil)
	if !current() {
		return
	}
	p.dispatcher = EventDispatcher{}
	var reset func(Widget)
	reset = func(w Widget) {
		if w == nil || w.base().destroyed || !current() || w.Root() != p {
			return
		}
		for _, c := range w.EventControllers() {
			c.Reset()
			if w.base().destroyed || !current() || w.Root() != p {
				return
			}
		}
		for _, child := range w.Children() {
			reset(child)
		}
	}
	reset(root)
}

// Distinguishes creation/capability failures from failures of native Show.
// Framework-owned menus and tooltips may retry unsupported alpha creation
// with an opaque configuration; native Show failures are not retried.
type popoverCreationError struct{ error }

func (e *popoverCreationError) Unwrap() error { return e.error }

// --- native resource lifecycle (belongs to the window) ---

func (p *popover) createNative(win Surface) error {
	if App == nil {
		return fmt.Errorf("popover: application is not created")
	}
	if host, ok := win.(interface{ rootState() *rootBase }); ok {
		p.app = host.rootState().app
	} else if app, ok := App.(*application); ok {
		p.app = app
	}
	p.owner = win
	if owner := surfaceCore(win); owner != nil {
		owner.ownPopover(p)
	}
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
	p.surfaceEpoch++
	surfaceEpoch := p.surfaceEpoch
	nativeApp := p.app
	pp, err := App.Platform().NewPopup(win.PlatformWindow(), p.requestedSize, func(event events.Event) {
		// A source result belongs to the application session even if its popup
		// has since been released. Other callbacks must match this surface.
		if e, ok := event.(events.DragSourceEvent); ok {
			if nativeApp != nil {
				nativeApp.dispatchDragSourceEvent(e)
			}
			return
		}
		if p.surfaceEpoch == surfaceEpoch && !p.destroyed {
			p.onEvent(event)
		}
	}, platform.PopupOptions{Transparent: p.transparent})
	if err != nil {
		p.releaseNative()
		return &popoverCreationError{fmt.Errorf("create platform popup: %w", err)}
	}
	currentOwner, mounted := anchorSurface(p.anchor)
	if p.lifecycle != epoch || p.destroyed || !mounted || currentOwner != win || surfaceCore(win) != nil && surfaceCore(win).destroyed {
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
	currentOwner, mounted = anchorSurface(p.anchor)
	if p.lifecycle != epoch || p.destroyed || !mounted || currentOwner != win || surfaceCore(win) != nil && surfaceCore(win).destroyed {
		painter.Destroy()
		pp.Destroy()
		p.releaseNative()
		return fmt.Errorf("popover: released during painter creation")
	}
	p.platformPopup = pp
	p.surface = pp
	p.painter = painter
	p.dragControllersChanged()

	// Keep the anchor close policy separate from Surface resource ownership.
	p.watchAnchor()
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
	if p.releasing {
		return
	}
	if p.framePainting || p.paintDepth != 0 || p.mutationDepth != 0 {
		p.releasePending = true
		p.suspendFrames()
		return
	}
	p.releasePending = false
	p.releasing = true
	owner := surfaceCore(p.owner)
	defer func() {
		p.releasing = false
		if owner != nil {
			owner.finishDestroy()
		}
	}()
	cancelTooltipHost(p)
	p.suspendFrames()
	p.frameRequester = nil
	p.lifecycle++
	p.surfaceEpoch++
	p.surface = nil
	wasVisible := p.visible
	target := p.target
	p.target = nil
	p.visible = false
	if owner := surfaceCore(p.owner); owner != nil {
		owner.removePopover(p)
	}
	p.resetInput()
	p.drag.destroy()
	if p.hUnmount != nil {
		p.hUnmount.Disconnect()
		p.hUnmount = nil
	}
	if p.hStyle != nil {
		p.hStyle.Disconnect()
		p.hStyle = nil
	}
	if wasVisible && p.modal {
		p.resignModalTarget()
	}
	if p.widget != nil && p.widget.Root() == p {
		p.widget.base().detachRoot(p.widget)
	}
	if p.painter != nil {
		p.painter.Destroy()
		p.painter = nil
	}
	native := p.platformPopup
	p.platformPopup = nil
	if owner := surfaceCore(p.owner); owner != nil {
		owner.disownPopover(p)
	}
	p.owner = nil
	p.positionValid = false
	p.placementSnapshotValid = false
	p.requestedSize = geometry.Size{}
	if native != nil {
		native.Destroy()
	}
	if p.destroyed {
		p.widget = nil
	}
	p.releasing = false
	if wasVisible {
		p.emitClosed(target)
	}
}

func (p *popover) finishRelease() {
	owner := surfaceCore(p.owner)
	defer func() {
		if owner != nil {
			owner.finishDestroy()
		}
	}()
	if p.releasePending && !p.framePainting && p.paintDepth == 0 && p.mutationDepth == 0 {
		p.releaseNative()
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
	placement, position := p.placement, p.position
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
	radius, _ := s.CornerRadius()
	insets := popoverInsets{}
	if p.transparent {
		insets = popoverShadowInsets(size, radius.Max(), shadow)
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
	if !p.modal && placement == PopoverPlacementBottom {
		insets = snapshot.trimAnchorShadow(size, insets)
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
	if p.placement != placement || (placement == PopoverPlacementPoint && p.position != position) {
		return fmt.Errorf("popover: placement changed during layout")
	}
	if p.owner != nil {
		if win, ok := anchorSurface(p.anchor); !ok || win != p.owner {
			return fmt.Errorf("popover: anchor moved during layout")
		}
	}
	p.placementSnapshot, p.placementSnapshotValid, p.insets = snapshot, true, insets
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
		if p.placement != placement || (placement == PopoverPlacementPoint && p.position != position) {
			return fmt.Errorf("popover: placement changed during resize")
		}
		if changed && !p.preparing {
			if err := p.refreshClientSize(); err != nil {
				return err
			}
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

func (p *popover) queryPlacement() (popoverPlacementSnapshot, error) {
	origin := absOrigin(p.anchor)
	if p.anchor != nil {
		if parent, ok := p.anchor.Root().(*popover); ok {
			origin.X += parent.requestedPosition.X
			origin.Y += parent.requestedPosition.Y
		}
	}
	snapshot := popoverPlacementSnapshot{anchor: geometry.Rect(origin.X+p.position.X, origin.Y+p.position.Y, 0, 0), placement: p.placement}
	point := snapshot.anchor.Pos
	if snapshot.placement == PopoverPlacementBottom && p.anchor != nil {
		snapshot.anchor = geometry.Rectangle{Pos: origin, Size: p.anchor.Rect().Size}
		if p.anchorArea != nil {
			snapshot.anchor = *p.anchorArea
			snapshot.anchor.X += origin.X
			snapshot.anchor.Y += origin.Y
		}
		point = snapshot.anchor.Center()
	}
	if !finitePopup(point.X) || !finitePopup(point.Y) {
		return snapshot, fmt.Errorf("popover: non-finite anchor position")
	}
	var err error = platform.ErrUnsupported
	if p.owner != nil {
		if desktop, ok := p.owner.PlatformWindow().(platform.WindowManagement); ok {
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
	if p.platformPopup == nil || p.preparing {
		return
	}
	p.applyPosition()
}

func (p *popover) applyPosition() {
	if !p.placementSnapshotValid {
		return
	}
	pos := p.placementSnapshot.position(geometry.Size{Width: p.width, Height: p.height}, p.insets)
	if !p.positionValid || pos != p.requestedPosition {
		p.requestedPosition, p.positionValid = pos, true
		p.platformPopup.SetPosition(pos.X, pos.Y)
	}
}

// --- events / paint ---

func (p *popover) onEvent(event platform.Event) {
	if _, paint := event.(events.PaintEvent); paint {
		p.paintDepth++
		owner := surfaceCore(p.owner)
		defer func() {
			p.paintDepth--
			p.finishRelease()
			if owner != nil {
				owner.finishDestroy()
			}
		}()
	}
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
	if p.clearing && p.painter != nil {
		backend := p.painter
		backend.Begin(p.pixelWidth, p.pixelHeight, p.frameScale())
		defer backend.End()
		clear := graphics.Color{}
		if !p.transparent {
			clear.A = 1
		}
		backend.Clear(clear)
		return
	}
	defer p.finishRelease()
	if p.painter == nil || p.destroyed || p.preparing {
		return
	}
	if !p.beginFrame() {
		return
	}
	defer p.endFrame()
	p.widget = liveRoot(p.widget)
	laidOut := p.layoutDirty
	if laidOut && !p.layoutContent() {
		return
	}
	if laidOut {
		p.refreshDragMotion(p)
	}
	if p.destroyed {
		return
	}
	if tips := tooltipsFor(p); tips != nil && tips.host == p {
		tips.refresh(laidOut)
		if p.destroyed {
			return
		}
	}
	background, border, shadow := p.surfaceStyle()
	radius, _ := border.CornerRadius()
	width, _ := border.BorderWidth()
	body := p.bodyRect()
	inset := popoverSafeInset(radius.Max(), width)
	safe := geometry.Rect(body.X+inset, body.Y+inset, max(0, body.Width-2*inset), max(0, body.Height-2*inset))
	p.drawSurfaceFrame(p.widget, background, border, body, safe, shadow, nil)
}

// layoutContent preserves the last completed native frame on a failed query.
func (p *popover) layoutContent() bool {
	if err := p.layoutPopup(); err != nil {
		log.Printf("goui: popup layout: %v", err)
		return false // retain the last completed native frame on query failure
	}
	return true
}

func (p *popover) layoutPopup() error {
	observation := p.observeLayout()
	defer observation.end()
	p.layoutDirty = false
	if err := p.updateNaturalSize(); err != nil {
		return err
	}
	if p.preparing {
		if err := p.refreshClientSize(); err != nil {
			return err
		}
	}
	epoch := p.lifecycle
	if p.widget != nil {
		body := p.bodyRect()
		measureWidget(p.widget, layout.Tight(body.Size))
		p.widget.Arrange(body)
	}
	if p.lifecycle != epoch || p.painter == nil || p.destroyed {
		return fmt.Errorf("popover: released during arrangement")
	}
	return nil
}

// RequestLayout satisfies Root: schedule a relayout of this host.
func (p *popover) RequestLayout() { p.requestLayout() }

func (p *popover) requestLayout() {
	p.rootBase.requestLayout(p.platformPopup)
}

func (p *popover) requestPaint() error {
	return p.rootBase.requestPaint(p.platformPopup)
}

// anchorSurface resolves the presentation host, following visible Popover owners.
func anchorSurface(anchor Widget) (Surface, bool) {
	seen := make(map[Root]bool)
	for anchor != nil {
		root := anchor.Root()
		if seen[root] {
			return nil, false
		}
		seen[root] = true
		if win, ok := anchor.Root().(Surface); ok {
			return win, true
		}
		parent, ok := anchor.Root().(*popover)
		if !ok || parent.destroyed || !parent.visible {
			return nil, false
		}
		anchor = parent.anchor
	}
	return nil, false
}

// absOrigin returns a widget's origin in its Root by summing parent-relative
// origins. queryPlacement separately converts popup-local anchors to their
// native owner window.
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
	return ResolveWidgetStyle(p.Widget(), name, style.PartDefault, style.Normal)
}

// surfaceStyle changes effective geometry, never the application's style sheet.
func (p *popover) surfaceStyle() (style.Style, style.Style, graphics.BoxShadow) {
	s := p.resolvedStyle()
	radius, _ := s.CornerRadius()
	radius = radius.Normalize(p.bodyRect().Size)
	if !p.transparent {
		radius = geometry.CornerRadius{}
	}
	width, _ := s.BorderWidth()
	color, _ := s.BorderColor()
	border := style.Name("").CornerRadius(radius).BorderWidth(normalizeLayoutValue(width)).BorderColor(color).BorderSides(styledBorderSides(s)).Style
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

func roundedBodyContains(rect geometry.Rectangle, radius geometry.CornerRadius, point geometry.Point) bool {
	return radius.Contains(rect, point)
}

// One solve uses one owner-local snapshot. No native objects or widgets are
// retained here; rounded SizeEvents can reuse it without another display query.
type popoverPlacementSnapshot struct {
	anchor      geometry.Rectangle
	placement   PopoverPlacement
	workArea    geometry.Rectangle
	constrained bool
	anchorSide  int8 // resolved side for modeless surfaces that exclude anchor shadow
}

func finitePopup(v float32) bool { return !math.IsNaN(float64(v)) && !math.IsInf(float64(v), 0) }

func validPopupRect(r geometry.Rectangle) bool {
	return r.Width > 0 && r.Height > 0 && finitePopup(r.X) && finitePopup(r.Y) &&
		finitePopup(r.Width) && finitePopup(r.Height) && finitePopup(r.X+r.Width) && finitePopup(r.Y+r.Height)
}

func (s popoverPlacementSnapshot) bodyLimit(insets popoverInsets) (geometry.Size, error) {
	size := geometry.Size{Width: s.workArea.Width - insets.left - insets.right, Height: s.workArea.Height - insets.top - insets.bottom}
	if !finitePopup(size.Width) || !finitePopup(size.Height) || size.Width < 1 || size.Height < 1 {
		return geometry.Size{}, fmt.Errorf("popover: shadow leaves no usable body in work area")
	}
	return size, nil
}

// trimAnchorShadow keeps an adjacent modeless surface out of the anchor's
// input area. Transparent shadow pixels still belong to the native window:
// allocate only the outward-facing vertical shadow, without moving the body.
// Oversized surfaces that must overlap the anchor keep normal workarea sliding.
func (s *popoverPlacementSnapshot) trimAnchorShadow(body geometry.Size, insets popoverInsets) popoverInsets {
	if !s.constrained || s.anchor.Y+s.anchor.Height+body.Height+insets.bottom <= s.workArea.Y+s.workArea.Height {
		s.anchorSide = 1
		insets.top = 0
	} else if s.anchor.Y-body.Height-insets.top >= s.workArea.Y {
		s.anchorSide = -1
		insets.bottom = 0
	}
	return insets
}

// position returns the full surface origin. Alignment is against the body,
// containment against the surface. Oversized actual native sizes anchor at the
// workarea origin rather than generating resize feedback loops.
func (s popoverPlacementSnapshot) position(surface geometry.Size, insets popoverInsets) geometry.Point {
	a := s.anchor
	body := geometry.Size{Width: max(0, surface.Width-insets.left-insets.right), Height: max(0, surface.Height-insets.top-insets.bottom)}
	preferred := geometry.Point{X: a.X - insets.left, Y: a.Y - insets.top}
	if s.placement == PopoverPlacementBottom {
		preferred.Y += a.Height
	}
	if s.anchorSide < 0 {
		preferred.Y = a.Y - body.Height - insets.top
	}
	if !s.constrained {
		return preferred
	}
	area := s.workArea
	fits := func(p geometry.Point) bool {
		return p.X >= area.X && p.Y >= area.Y && p.X+surface.Width <= area.X+area.Width && p.Y+surface.Height <= area.Y+area.Height
	}
	if s.anchorSide != 0 {
		// The side is fixed for this measurement, including native rounding.
		// Horizontal alignment can still slide into the workarea.
		for _, candidate := range [...]geometry.Point{preferred, {X: a.X + a.Width - body.Width - insets.left, Y: preferred.Y}} {
			if fits(candidate) {
				return candidate
			}
		}
		return geometry.Point{
			X: min(max(preferred.X, area.X), max(area.X, area.X+area.Width-surface.Width)),
			Y: min(max(preferred.Y, area.Y), max(area.Y, area.Y+area.Height-surface.Height)),
		}
	}
	if s.placement == PopoverPlacementBottom {
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

func (p *popover) prepareShow() error {
	for range 8 {
		if err := p.layoutPopup(); err != nil {
			return err
		}
		if p.layoutDirty {
			continue
		}
		// Moving a reused popup to another display can change its native scale.
		// Finish that hidden move, observe the new target, and reapply the DIP
		// size at the new scale before considering the presentation prepared.
		logical := geometry.Size{Width: p.width, Height: p.height}
		pixels := geometry.Size{Width: p.pixelWidth, Height: p.pixelHeight}
		p.applyPosition()
		if err := p.refreshClientSize(); err != nil {
			return err
		}
		if logical != (geometry.Size{Width: p.width, Height: p.height}) || pixels != (geometry.Size{Width: p.pixelWidth, Height: p.pixelHeight}) {
			p.layoutDirty = true
			if pixels.Width/logical.Width != p.frameScale() {
				p.requestedSize = geometry.Size{}
			}
		}
		if !p.layoutDirty {
			return nil
		}
	}
	return fmt.Errorf("popover: layout did not settle before Show")
}

func (p *popover) refreshClientSize() error {
	native, epoch := p.platformPopup, p.lifecycle
	if native == nil {
		return fmt.Errorf("popover: no native surface for size query")
	}
	logical, pixels, err := native.ClientSize()
	if err != nil {
		return fmt.Errorf("popover client size: %w", err)
	}
	if p.destroyed || p.lifecycle != epoch || p.platformPopup != native {
		return fmt.Errorf("popover: released during size query")
	}
	if !validPopupRect(geometry.Rectangle{Size: logical}) || !validPopupRect(geometry.Rectangle{Size: pixels}) {
		return fmt.Errorf("popover: invalid native client size")
	}
	p.width, p.height = logical.Width, logical.Height
	p.pixelWidth, p.pixelHeight = pixels.Width, pixels.Height
	return nil
}

func (p *popover) emitClosed(target *popoverTarget) {
	p.hiding = false
	p.closed.Emit()
	if target != nil {
		target.closed.Emit()
	}
}

// SetAnchor configures a new occurrence without allocating native resources.
func (p *popover) SetAnchor(anchor Widget) {
	if p.destroyed || p.anchor == anchor {
		return
	}
	if p.hUnmount != nil {
		p.hUnmount.Disconnect()
		p.hUnmount = nil
	}
	p.anchor = anchor
	p.anchorArea = nil
	p.positionValid = false
	p.placementSnapshotValid = false
	p.layoutDirty = true
	p.watchAnchor()
	p.Hide()
}

func (p *popover) watchAnchor() {
	if p.hUnmount == nil && p.anchor != nil && !p.destroyed {
		p.hUnmount = p.anchor.ConnectUnmount(p.Hide)
	}
}

func (p *popover) transitioning() bool {
	return p.showing || p.hiding || p.releasing || p.framePainting || p.paintDepth != 0 || p.mutationDepth != 0
}
