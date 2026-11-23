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

// SharedPopover configures one reusable client of its anchor presentation host's cache.
// It is not a Root and owns no native resources. All methods use the GUI thread.
type SharedPopover interface {
	Anchor() Widget
	// SetAnchor ends this occurrence and configures the next Show.
	// Moving between Surfaces selects their separate cached popovers.
	SetAnchor(Widget)
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
	// Show replaces the current shared client. A later operation supersedes it.
	Show() error
	// Hide retains the host's popover and at most one mounted content tree.
	Hide()
	// Destroy releases this client, never the host's cached popover.
	Destroy()
	ConnectDismissRequest(func()) signal.Handle
	ConnectClosed(func()) signal.Handle
}

// NewSharedPopover creates a lightweight, independent client. The anchor may be
// unmounted until Show, which selects its current presentation host's shared cache.
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
	slot               *sharedPopoverSlot
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
	return !p.destroyed && p.slot != nil && !p.slot.owner.destroyed && p.slot.current == p && p.slot.popup.Visible()
}
func (p *sharedPopover) ConnectClosed(fn func()) signal.Handle { return p.closed.Connect(fn) }
func (p *sharedPopover) ConnectDismissRequest(fn func()) signal.Handle {
	return p.dismissRequest.Connect(fn)
}
func (p *sharedPopover) SetAnchor(anchor Widget) {
	if p.destroyed || p.anchor == anchor {
		return
	}
	if p.unmount != nil {
		p.unmount.Disconnect()
		p.unmount = nil
	}
	p.anchor, p.anchorArea = anchor, nil
	if slot := p.slot; slot != nil {
		slot.evict(p)
	}
}
func (p *sharedPopover) SetWidget(w Widget) {
	if p.destroyed || p.widget == w || w != nil && w.base().destroyed {
		return
	}
	p.widget = w
	if s := p.slot; s != nil {
		if p.Visible() {
			previous := s.transitioning
			s.transitioning = true
			defer func() { s.transitioning = previous; s.owner.finishDestroy() }()
			s.popup.SetWidget(w)
		} else {
			s.evict(p)
		}
	}
}
func (p *sharedPopover) SetStyleName(v string) {
	if p.destroyed {
		return
	}
	p.styleName = v
	if p.Visible() {
		p.slot.popup.SetStyleName(v)
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
		p.slot.popup.SetPlacement(v)
	}
}
func (p *sharedPopover) SetPosition(v geometry.Point) {
	if p.destroyed {
		return
	}
	p.position = v
	if p.Visible() {
		p.slot.popup.SetPosition(v)
	}
}
func (p *sharedPopover) SetModal(v bool) {
	if p.destroyed {
		return
	}
	p.modal = v
	if p.Visible() {
		p.slot.popup.SetModal(v)
	}
}
func (p *sharedPopover) SetAutoDismiss(v bool) {
	if p.destroyed {
		return
	}
	p.autoDismiss = v
	if p.Visible() {
		p.slot.popup.SetAutoDismiss(v)
	}
}
func (p *sharedPopover) FocusedWidget() Widget {
	if p.Visible() {
		return p.slot.popup.FocusedWidget()
	}
	return nil
}
func (p *sharedPopover) SetFocusedWidget(w Widget) bool {
	return p.Visible() && p.slot.popup.SetFocusedWidget(w)
}
func (p *sharedPopover) DispatchEvent(e events.Event) error {
	if !p.Visible() {
		return fmt.Errorf("shared popover: input target is unavailable")
	}
	return p.slot.popup.DispatchEvent(e)
}
func (p *sharedPopover) CommitText(s string) error {
	if !p.Visible() {
		return fmt.Errorf("shared popover: input target is unavailable")
	}
	return p.slot.popup.CommitText(s)
}
func (p *sharedPopover) Hide() {
	if s := p.slot; s != nil {
		s.closeClient(p)
	}
}
func (p *sharedPopover) Destroy() {
	if p.destroyed {
		return
	}
	p.destroyed = true
	if s := p.slot; s != nil {
		s.evict(p)
	}
	p.widget = nil
}

// sharedPopoverSlot arbitrates clients; its ordinary popover is the only Root.
type sharedPopoverSlot struct {
	owner                 *surfaceBase
	host                  *rootBase
	popup                 *popover
	current, retained     *sharedPopover
	revision              uint64
	transitioning, closed bool
}

func newSharedPopoverSlot(owner *surfaceBase, host *rootBase, anchor Widget) *sharedPopoverSlot {
	s := &sharedPopoverSlot{owner: owner, host: host, popup: NewPopover(anchor, &PopoverOptions{Transparent: true}).(*popover)}
	s.popup.app = owner.app
	// Register before the cached root can be borrowed. Internal identity is
	// cleared before any client or occurrence-level closing callback can reenter.
	s.popup.ConnectClosed(func() {
		p := s.current
		s.current = nil
		s.revision++
		if p != nil {
			p.closed.Emit()
		}
	})
	s.popup.ConnectDismissRequest(func() {
		if p := s.current; p != nil && p.Visible() {
			p.dismissRequest.Emit()
		}
	})
	return s
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
	host := p.anchor.Root().(interface{ rootState() *rootBase }).rootState()
	if parent, ok := p.anchor.Root().(*popover); ok && (!parent.visible || parent.destroyed || parent.hiding) {
		return fmt.Errorf("shared popover: presentation host is unavailable")
	}
	if host.shared != nil && host.shared.owner != core {
		host.shared.destroy()
		host.shared = nil
	}
	if host.shared == nil {
		host.shared = newSharedPopoverSlot(core, host, p.anchor)
	}
	s := host.shared
	if s.closed || s.transitioning || s.popup.transitioning() {
		return fmt.Errorf("shared popover: host transition in progress")
	}
	if p.anchor.Root() == s.popup {
		return fmt.Errorf("shared popover: anchor belongs to the replaced shared content")
	}
	if p.tooltip && s.current != nil && !s.current.tooltip {
		return ErrPopoverSuperseded
	}
	if previous := s.current; previous != nil {
		revision := s.revision
		s.closeClient(previous)
		if s.revision != revision+1 || s.current != nil {
			return ErrPopoverSuperseded
		}
	}
	revision := s.revision
	if !p.tooltip {
		core.tooltip.dismiss(false)
	}
	if old := p.slot; old != nil && old != s {
		old.evict(p)
	}
	if s.retained != p {
		s.evict(s.retained)
	}
	ownerNow, valid := anchorSurface(p.anchor)
	if !valid || ownerNow != owner || s.closed || core.destroyed || p.destroyed || s.revision != revision || s.current != nil {
		return ErrPopoverSuperseded
	}
	s.transitioning = true
	defer func() { s.transitioning = false; core.finishDestroy() }()
	s.current, s.retained, p.slot = p, p, s
	s.revision++
	revision = s.revision
	current := func() bool {
		return !p.destroyed && !s.closed && !core.destroyed && s.current == p && s.revision == revision
	}
	if p.unmount == nil {
		p.unmount = p.anchor.ConnectUnmount(func() {
			if p.slot == s {
				s.evict(p)
			}
		})
	}
	popup := s.popup
	popup.SetAnchor(p.anchor)
	popup.anchorArea = p.anchorArea
	popup.SetStyleName(p.styleName)
	popup.SetPosition(p.position)
	popup.SetPlacement(p.placement)
	popup.SetModal(p.modal)
	popup.SetAutoDismiss(p.autoDismiss)
	popup.SetWidget(p.Widget())
	if !current() {
		return ErrPopoverSuperseded
	}
	invalidateStyleSubtree(popup.Widget())
	popup.layoutDirty = true
	popup.requestedSize = geometry.Size{}
	err := popup.Show()
	var creation *popoverCreationError
	if err != nil && current() && popup.transparent && errors.As(err, &creation) &&
		(errors.Is(err, platform.ErrUnsupported) || errors.Is(err, platform.ErrUnavailable)) {
		// Native transparency becomes fixed when a configuration succeeds.
		popup.transparent = false
		err = popup.Show()
	}
	if err != nil {
		s.closeClient(p)
		if errors.Is(err, ErrPopoverSuperseded) || p.destroyed || core.destroyed {
			return ErrPopoverSuperseded
		}
		return err
	}
	if !current() || !p.Visible() {
		return ErrPopoverSuperseded
	}
	return nil
}

func (s *sharedPopoverSlot) closeClient(p *sharedPopover) {
	defer s.owner.finishDestroy()
	if p == nil || s.current != p {
		return
	}
	if !s.popup.visible {
		s.current = nil
		s.revision++
	}
	s.popup.Hide()
}
func (s *sharedPopoverSlot) evict(p *sharedPopover) {
	defer s.owner.finishDestroy()
	if p == nil || s.retained != p {
		return
	}
	s.closeClient(p)
	if s.retained != p || s.current != nil {
		return
	}
	previous := s.transitioning
	s.transitioning = true
	defer func() { s.transitioning = previous }()
	s.retained, p.slot = nil, nil
	if p.unmount != nil {
		p.unmount.Disconnect()
		p.unmount = nil
	}
	s.popup.SetWidget(nil)
	s.popup.SetAnchor(nil)
}
func (s *sharedPopoverSlot) destroy() {
	if s.closed {
		return
	}
	s.closed = true
	s.evict(s.retained)
	s.popup.Destroy()
}
