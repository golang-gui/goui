package gui

import (
	"errors"
	"fmt"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/graphics"
)

// transientPopup owns physical resources; popover owns each presentation's
// widget tree, input, style and frame state. Only the owner destroys the cache.
type transientPopup struct {
	owner       *surfaceBase
	app         *application
	native      platform.Popup
	painter     graphics.Painter
	active      *popover
	size        events.SizeEvent
	hasSize     bool
	transparent bool
	busy        bool
	closed      bool
	nativeEpoch uint64
	epoch       uint64
}

func (h *transientPopup) painting() bool { return h.active != nil && h.active.framePainting }

func (h *transientPopup) onEvent(event events.Event) {
	// Source completion belongs to the application session, even after its
	// logical presenter has closed or a different presenter owns the surface.
	if e, ok := event.(events.DragSourceEvent); ok {
		if h.app != nil {
			h.app.dispatchDragSourceEvent(e)
		}
		return
	}
	if h.closed {
		return
	}
	if e, ok := event.(events.SizeEvent); ok {
		h.size, h.hasSize = e, true
	}
	if p := h.active; p != nil && !p.destroyed {
		p.onEvent(event)
	}
}

func (p *popover) acquireTransient(win Surface) error {
	owner := surfaceCore(win)
	if owner == nil || owner.destroyed {
		return fmt.Errorf("popover: shared surface requires a live owner")
	}
	if owner.transient == nil {
		owner.transient = &transientPopup{owner: owner, app: p.app}
	}
	h := owner.transient
	if h.busy || h.closed || h.painting() {
		return fmt.Errorf("popover: shared surface transition in progress")
	}
	if p.tooltipPresentation && h.active != nil && !h.active.tooltipPresentation {
		return fmt.Errorf("popover: tooltip cannot replace an active transient")
	}
	if !p.tooltipPresentation {
		owner.tooltip.dismiss(false)
	}
	if previous := h.active; previous != nil && previous != p {
		epoch := h.epoch
		previous.releaseNative()
		// Closed callbacks may open a third presenter, even open and close it.
		// A later request wins; never overwrite that callback's presentation.
		if h.epoch != epoch+1 || h.active != nil {
			return fmt.Errorf("popover: shared presentation changed while closing")
		}
	}
	currentOwner, mounted := anchorSurface(p.anchor)
	if h.closed || owner.destroyed || p.destroyed || !mounted || currentOwner != win {
		return fmt.Errorf("popover: released while acquiring shared surface")
	}
	h.busy = true
	defer func() { h.busy = false; owner.finishDestroy() }()
	h.active, p.transientHost = p, h
	h.epoch++
	epoch := p.lifecycle
	current := func() bool {
		ownerNow, mounted := anchorSurface(p.anchor)
		return !h.closed && !owner.destroyed && !p.destroyed && p.lifecycle == epoch && h.active == p && mounted && ownerNow == win
	}
	fail := func(err error) error {
		p.releaseNative()
		return err
	}
	if h.native == nil {
		for _, transparent := range []bool{true, false} {
			p.transparent = transparent
			if err := p.updateNaturalSize(); err != nil {
				return fail(err)
			}
			h.nativeEpoch++
			nativeEpoch := h.nativeEpoch
			handler := func(event events.Event) {
				if _, source := event.(events.DragSourceEvent); source || nativeEpoch == h.nativeEpoch {
					h.onEvent(event)
				}
			}
			native, err := App.Platform().NewPopup(win.PlatformWindow(), p.requestedSize, handler, platform.PopupOptions{Transparent: transparent})
			if err != nil {
				if transparent && current() && (errors.Is(err, platform.ErrUnsupported) || errors.Is(err, platform.ErrUnavailable)) {
					continue
				}
				return fail(&popoverCreationError{fmt.Errorf("create shared popup: %w", err)})
			}
			if !current() {
				h.nativeEpoch++
				native.Destroy()
				return fail(fmt.Errorf("popover: released during shared creation"))
			}
			painter, err := App.Platform().NewPainter(native)
			if err != nil {
				h.nativeEpoch++
				native.Destroy()
				if transparent && current() && (errors.Is(err, platform.ErrUnsupported) || errors.Is(err, platform.ErrUnavailable)) {
					continue
				}
				return fail(&popoverCreationError{fmt.Errorf("create shared painter: %w", err)})
			}
			if !current() {
				painter.Destroy()
				h.nativeEpoch++
				native.Destroy()
				return fail(fmt.Errorf("popover: released during shared painter creation"))
			}
			h.native, h.painter, h.transparent = native, painter, transparent
			break
		}
	}
	p.transparent = h.transparent
	p.platformPopup, p.surface, p.painter = h.native, h.native, h.painter
	if h.hasSize {
		p.width, p.height = h.size.Width, h.size.Height
		p.pixelWidth, p.pixelHeight = h.size.PixelWidth, h.size.PixelHeight
	}
	// The same physical dimensions need not emit another SizeEvent. Seed the
	// last actual geometry, then force this presenter's requested size through.
	p.requestedSize = geometry.Size{}
	if err := p.updateNaturalSize(); err != nil || !current() {
		if err == nil {
			err = fmt.Errorf("popover: released during shared resize")
		}
		return fail(err)
	}
	p.hUnmount = p.anchor.ConnectUnmount(p.releaseNative)
	if app, ok := App.(*application); ok {
		p.hStyle = app.styleChanged.Connect(func() {
			invalidateStyleSubtree(p.Widget())
			p.measureAndSize()
			p.requestLayout()
		})
	}
	p.dragControllersChanged()
	return nil
}

func (p *popover) releaseTransient() {
	if p.releasing {
		return
	}
	if p.framePainting {
		p.releasePending = true
		p.suspendFrames()
		return
	}
	p.releasePending = false
	p.releasing = true
	p.suspendFrames()
	p.frameRequester = nil
	p.lifecycle++
	p.surfaceEpoch++
	wasVisible := p.visible
	p.visible = false
	h := p.transientHost
	// Hide before running callbacks, and relinquish all borrowed pointers before
	// another presenter can acquire them. Destroy never releases the cache here.
	if h != nil && h.active == p {
		h.busy = true
		if h.native != nil {
			_ = h.native.Hide()
		}
		h.active = nil
		h.epoch++
	}
	cancelTooltipHost(p)
	if owner := surfaceCore(p.owner); owner != nil {
		owner.removePopover(p)
	}
	p.platformPopup, p.surface, p.painter = nil, nil, nil
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
	p.owner, p.transientHost = nil, nil
	p.positionValid, p.placementSnapshotValid = false, false
	p.requestedSize = geometry.Size{}
	if p.widget != nil && p.widget.Root() == p {
		p.widget.base().detachRoot(p.widget)
	}
	if p.destroyed {
		p.widget = nil
	}
	p.releasing = false
	if h != nil {
		h.busy = false
	}
	if wasVisible {
		p.closed.Emit()
	}
	if h != nil {
		h.owner.finishDestroy()
	}
}

func (h *transientPopup) destroy() {
	if h.closed {
		return
	}
	h.closed = true
	h.nativeEpoch++
	if p := h.active; p != nil {
		p.releaseNative()
	}
	if h.painter != nil {
		h.painter.Destroy()
		h.painter = nil
	}
	if h.native != nil {
		h.native.Destroy()
		h.native = nil
	}
}
