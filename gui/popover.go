package gui

import (
	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/platform/events"
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
// Show, bound to that host, and released when the anchor unmounts or the
// host is destroyed. NewSharedPopover provides a separate window-shared popup API.
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
	// SharedPopover separately prepares observed geometry and invalidates old
	// presentations; ordinary Popover retains its independent lifetime.
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
	p := &popover{popoverHost: popoverHost{anchor: anchor, rootBase: rootBase{frameSuspended: true}}}
	p.bind()
	if options != nil {
		p.transparent = options.Transparent
	}
	return p
}

type popover struct{ popoverHost }

func (p *popover) bind() *popoverHost {
	p.host = p
	p.nativeReady = func() {
		p.hUnmount = p.anchor.ConnectUnmount(p.releaseNative)
		p.hWinGone = p.owner.ConnectDestroy(p.releaseNative)
	}
	return &p.popoverHost
}
func (p *popover) SetWidget(w Widget)                 { p.bind().setWidget(w) }
func (p *popover) SetStyleName(v string)              { p.bind().setStyleName(v) }
func (p *popover) SetModal(v bool)                    { p.bind().setModal(v) }
func (p *popover) SetAutoDismiss(v bool)              { p.bind().setAutoDismiss(v) }
func (p *popover) SetPlacement(v PopoverPlacement)    { p.bind().setPlacement(v) }
func (p *popover) SetPosition(v geometry.Point)       { p.bind().setPosition(v) }
func (p *popover) Show() error                        { return p.bind().show(false) }
func (p *popover) Hide()                              { p.bind().hide() }
func (p *popover) Destroy()                           { p.bind().destroy() }
func (p *popover) DispatchEvent(e events.Event) error { return p.bind().DispatchEvent(e) }
func (p *popover) SetFocusedWidget(w Widget) bool     { return p.bind().SetFocusedWidget(w) }
func (p *popover) CommitText(s string) error          { return p.bind().CommitText(s) }
