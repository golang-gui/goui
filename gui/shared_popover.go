package gui

import (
	"errors"
	"fmt"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/platform"
	"github.com/golang-gui/goui/platform/events"
)

// ErrPopoverSuperseded reports a show request replaced by a later operation.
// It is a canceled request, rather than a native popup failure.
var ErrPopoverSuperseded = errors.New("popover: show request superseded")

// SharedPopover configures one reusable client of its anchor Surface's shared
// GUI host. It is not a Root. Construction and SetWidget do not mount content.
// All methods belong to the GUI thread. Hide and Destroy affect only this client;
// the Surface owns the shared host, painter and native popup until destruction.
type SharedPopover interface {
	Anchor() Widget
	Widget() Widget
	SetWidget(Widget)
	StyleName() string
	SetStyleName(string)
	Placement() PopoverPlacement
	SetPlacement(PopoverPlacement)
	Position() geometry.Point
	SetPosition(geometry.Point)
	Modal() bool
	SetModal(bool)
	AutoDismiss() bool
	SetAutoDismiss(bool)
	Visible() bool
	FocusedWidget() Widget
	SetFocusedWidget(Widget) bool
	DispatchEvent(events.Event) error
	CommitText(string) error
	// Show replaces the current shared popup after closing it. A callback that
	// opens another popup supersedes this request. Repeated Show while visible
	// is a no-op; setters invalidate an existing presentation normally.
	Show() error
	// Hide retains the host and at most one mounted content tree for reuse.
	Hide()
	// Destroy releases this client's bindings and references, never the host.
	Destroy()
	ConnectDismissRequest(func()) signal.Handle
	ConnectClosed(func()) signal.Handle
}

// NewSharedPopover creates an independent client of a window-shared GUI host.
// The anchor may be unmounted until Show. Its owning Surface determines the
// shared slot. Native resources are allocated lazily and reused across clients.
func NewSharedPopover(anchor Widget) SharedPopover { return &sharedPopover{anchor: anchor} }

type sharedPopover struct {
	anchor             Widget
	widget             Widget
	styleName          string
	placement          PopoverPlacement
	position           geometry.Point
	modal, autoDismiss bool
	tooltip            bool
	anchorArea         *geometry.Rectangle
	destroyed          bool
	host               *sharedPopoverHost
	unmount            signal.Handle
	closed             signal.Signal0
	dismissRequest     signal.Signal0
}

func (p *sharedPopover) Anchor() Widget              { return p.anchor }
func (p *sharedPopover) Widget() Widget              { return liveRoot(p.widget) }
func (p *sharedPopover) StyleName() string           { return p.styleName }
func (p *sharedPopover) Placement() PopoverPlacement { return p.placement }
func (p *sharedPopover) Position() geometry.Point    { return p.position }
func (p *sharedPopover) Modal() bool                 { return p.modal }
func (p *sharedPopover) AutoDismiss() bool           { return p.autoDismiss }
func (p *sharedPopover) Visible() bool {
	return !p.destroyed && p.host != nil && p.host.active == p && p.host.visible
}
func (p *sharedPopover) ConnectClosed(fn func()) signal.Handle { return p.closed.Connect(fn) }
func (p *sharedPopover) ConnectDismissRequest(fn func()) signal.Handle {
	return p.dismissRequest.Connect(fn)
}
func (p *sharedPopover) SetWidget(w Widget) {
	if p.destroyed || p.widget == w || w != nil && w.base().destroyed {
		return
	}
	p.widget = w
	if h := p.host; h != nil {
		if p.Visible() {
			wasBusy := h.busy
			h.busy = true
			h.setWidget(w)
			h.busy = wasBusy
			h.surfaceOwner.finishDestroy()
		} else {
			h.evict(p)
		}
	}
}
func (p *sharedPopover) SetStyleName(v string) {
	if p.destroyed {
		return
	}
	p.styleName = v
	if p.Visible() {
		p.host.setStyleName(v)
	}
}
func (p *sharedPopover) SetPlacement(v PopoverPlacement) {
	if p.destroyed {
		return
	}
	if v != PopoverPlacementBottom {
		v = PopoverPlacementPoint
	}
	p.placement = v
	if p.Visible() {
		p.host.setPlacement(v)
	}
}
func (p *sharedPopover) SetPosition(v geometry.Point) {
	if p.destroyed {
		return
	}
	p.position = v
	if p.Visible() {
		p.host.setPosition(v)
	}
}
func (p *sharedPopover) SetModal(v bool) {
	if p.destroyed {
		return
	}
	p.modal = v
	if p.Visible() {
		p.host.setModal(v)
	}
}
func (p *sharedPopover) SetAutoDismiss(v bool) {
	if p.destroyed {
		return
	}
	p.autoDismiss = v
	if p.Visible() {
		p.host.setAutoDismiss(v)
	}
}
func (p *sharedPopover) FocusedWidget() Widget {
	if p.Visible() {
		return p.host.FocusedWidget()
	}
	return nil
}
func (p *sharedPopover) SetFocusedWidget(w Widget) bool {
	return p.Visible() && p.host.SetFocusedWidget(w)
}
func (p *sharedPopover) DispatchEvent(e events.Event) error {
	if !p.Visible() {
		return fmt.Errorf("shared popover: input target is unavailable")
	}
	return p.host.DispatchEvent(e)
}
func (p *sharedPopover) CommitText(s string) error {
	if !p.Visible() {
		return fmt.Errorf("shared popover: input target is unavailable")
	}
	return p.host.CommitText(s)
}
func (p *sharedPopover) Hide() {
	if h := p.host; h != nil {
		h.closeClient(p)
	}
}
func (p *sharedPopover) Destroy() {
	if p.destroyed {
		return
	}
	p.destroyed = true
	if h := p.host; h != nil {
		h.evict(p)
	}
	p.widget = nil
}

// sharedPopoverHost is the sole live GUI root for all clients of one Surface.
// popoverHost supplies mechanics only; arbitration stays here.
type sharedPopoverHost struct {
	popoverHost
	surfaceOwner   *surfaceBase
	active, cached *sharedPopover
	epoch          uint64
	busy           bool
	closed         bool
}

func newSharedPopoverHost(owner *surfaceBase) *sharedPopoverHost {
	h := &sharedPopoverHost{surfaceOwner: owner, popoverHost: popoverHost{rootBase: rootBase{app: owner.app, frameSuspended: true}}}
	h.host = h
	h.dismissRequest.Connect(func() {
		if p := h.active; p != nil && p.Visible() {
			p.dismissRequest.Emit()
		}
	})

	h.onClosing = func() func() {
		if p := h.active; p != nil {
			h.active = nil
			h.epoch++
			return p.closed.Emit
		}
		return nil
	}

	return h
}

func (p *sharedPopover) Show() error {
	if p.destroyed {
		return fmt.Errorf("shared popover: destroyed")
	}
	if p.Visible() {
		return nil
	}
	owner, mounted := anchorSurface(p.anchor)
	if !mounted || !modalWidgetAllowed(p.anchor) {
		return fmt.Errorf("shared popover: anchor is unavailable")
	}
	core := surfaceCore(owner)
	if core == nil || core.destroyed {
		return fmt.Errorf("shared popover: owner is unavailable")
	}
	if core.shared == nil {
		core.shared = newSharedPopoverHost(core)
	}
	h := core.shared
	if h.closed || h.busy || h.hiding || h.releasing || h.framePainting || h.paintDepth != 0 || h.showing {
		return fmt.Errorf("shared popover: host transition in progress")
	}
	if popoverCore(p.anchor.Root()) == &h.popoverHost {
		return fmt.Errorf("shared popover: anchor belongs to the replaced shared content")
	}
	if p.tooltip && h.active != nil && !h.active.tooltip {
		return ErrPopoverSuperseded
	}
	if !p.tooltip {
		core.tooltip.dismiss(false)
	}
	if previous := h.active; previous != nil {
		epoch := h.epoch
		h.closeClient(previous)
		if h.epoch != epoch+1 || h.active != nil {
			return ErrPopoverSuperseded
		}
	}
	if old := p.host; old != nil && old != h {
		old.evict(p)
	}
	if h.closed || core.destroyed || p.destroyed {
		return ErrPopoverSuperseded
	}
	if h.cached != p {
		h.evict(h.cached)
	}
	ownerNow, valid := anchorSurface(p.anchor)
	if !valid || ownerNow != owner || h.closed || core.destroyed || p.destroyed {
		return ErrPopoverSuperseded
	}
	h.busy = true
	defer func() { h.busy = false; core.finishDestroy() }()
	h.active, h.cached, p.host = p, p, h
	h.epoch++
	epoch := h.epoch
	current := func() bool { return !p.destroyed && !h.closed && !core.destroyed && h.active == p && h.epoch == epoch }
	h.owner = owner
	h.anchor = p.anchor
	h.anchorArea = p.anchorArea
	h.styleName, h.position, h.placement = p.styleName, p.position, p.placement
	h.modal, h.autoDismiss = p.modal, p.autoDismiss
	if p.unmount == nil {
		p.unmount = p.anchor.ConnectUnmount(func() {
			if p.host == h {
				h.evict(p)
			}
		})
	}
	h.setWidget(p.Widget())
	if !current() {
		return ErrPopoverSuperseded
	}
	invalidateStyleSubtree(h.Widget())
	h.layoutDirty = true
	h.requestedSize = geometry.Size{}
	// A shared host chooses its native transparency once; unsupported alpha may
	// fall back only during creation, never for layout or ordinary Show errors.
	if h.platformPopup == nil {
		h.transparent = true
	}
	err := h.show(true)
	var creation *popoverCreationError
	if err != nil && current() && h.transparent && errors.As(err, &creation) &&
		(errors.Is(err, platform.ErrUnsupported) || errors.Is(err, platform.ErrUnavailable)) {
		h.owner = owner
		h.transparent = false
		err = h.show(true)
	}
	if err != nil {
		h.closeClient(p)
		if !current() && (errors.Is(err, ErrPopoverSuperseded) || p.destroyed || core.destroyed) {
			return ErrPopoverSuperseded
		}
		return err
	}
	if !current() || !p.Visible() {
		return ErrPopoverSuperseded
	}
	return nil
}

func (h *sharedPopoverHost) closeClient(p *sharedPopover) {
	defer h.surfaceOwner.finishDestroy()
	if p == nil || h.active != p {
		return
	}
	wasVisible := h.visible
	h.active = nil
	h.epoch++
	h.hide()
	if wasVisible {
		p.closed.Emit()
	}
}

func (h *sharedPopoverHost) evict(p *sharedPopover) {
	defer h.surfaceOwner.finishDestroy()
	if p == nil || h.cached != p {
		return
	}
	h.closeClient(p)
	// Closed may have installed a later client, including this same object.
	if h.cached != p || h.active != nil {
		return
	}
	h.busy = true
	h.cached = nil
	p.host = nil
	if p.unmount != nil {
		p.unmount.Disconnect()
		p.unmount = nil
	}
	h.setWidget(nil)
	h.anchor = nil
	h.anchorArea = nil
	h.busy = false
}

func (h *sharedPopoverHost) painting() bool { return h.framePainting || h.paintDepth != 0 }
func (h *sharedPopoverHost) destroy() {
	if h.closed {
		return
	}
	h.closed = true
	h.evict(h.cached)
	h.popoverHost.destroy()
}

func (h *sharedPopoverHost) DispatchEvent(e events.Event) error {
	switch e.(type) {
	case events.DestroyEvent, events.DragSourceEvent:
	default:
		if !h.visible {
			return fmt.Errorf("shared popover: input target is unavailable")
		}
	}
	return h.popoverHost.DispatchEvent(e)
}

// Retained hidden content remains mounted, but cannot acquire logical focus.
func (h *sharedPopoverHost) SetFocusedWidget(w Widget) bool {
	if w != nil && (!h.visible || h.active == nil) {
		return false
	}
	return h.popoverHost.SetFocusedWidget(w)
}
