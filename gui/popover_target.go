package gui

import (
	"fmt"

	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/platform/events"
)

// PopoverTarget is a borrowed input and inspection target for one visible
// popover occurrence. It expires when that occurrence closes, including when
// the same Popover or SharedPopover is shown again. It exposes no host ownership
// or configuration. Surface.Popovers returns only current targets.
type PopoverTarget interface {
	Anchor() Widget
	Widget() Widget
	Visible() bool
	Modal() bool
	StyleName() string
	Placement() PopoverPlacement
	Transparent() bool
	DispatchEvent(events.Event) error
	CommitText(string) error
	ConnectClosed(func()) signal.Handle
}

type popoverTarget struct {
	host   *popover
	anchor Widget
	closed signal.Signal0
}

func (p *popoverTarget) Visible() bool {
	return p.host != nil && p.host.target == p && p.host.visible && !p.host.destroyed
}
func (p *popoverTarget) Anchor() Widget { return p.anchor }
func (p *popoverTarget) Widget() Widget {
	if p.Visible() {
		return p.host.Widget()
	}
	return nil
}
func (p *popoverTarget) Modal() bool                           { return p.Visible() && p.host.modal }
func (p *popoverTarget) ConnectClosed(fn func()) signal.Handle { return p.closed.Connect(fn) }
func (p *popoverTarget) DispatchEvent(e events.Event) error {
	if !p.Visible() {
		return fmt.Errorf("popover: input target expired")
	}
	return p.host.DispatchEvent(e)
}
func (p *popoverTarget) CommitText(s string) error {
	if !p.Visible() {
		return fmt.Errorf("popover: input target expired")
	}
	return p.host.CommitText(s)
}

func (p *popoverTarget) StyleName() string {
	if p.Visible() {
		return p.host.styleName
	}
	return ""
}

func (p *popoverTarget) Placement() PopoverPlacement {
	if p.Visible() {
		return p.host.placement
	}
	return PopoverPlacementPoint
}
func (p *popoverTarget) Transparent() bool { return p.Visible() && p.host.transparent }
