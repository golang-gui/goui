package gui

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/platform/events"
)

// ConnectError reports input-driven submenu failures at the root presenter.
// Explicit ShowAt failures are returned to their caller instead.
func (pm *PopoverMenu) ConnectError(fn func(error)) signal.Handle { return pm.errors.Connect(fn) }

// Destroy releases this presenter and its descendants, but not host caches.
func (pm *PopoverMenu) Destroy() {
	if pm.destroyed {
		return
	}
	pm.rootMenu().activationRevision++
	pm.destroyed = true
	pm.Hide()
	if pm.child != nil {
		pm.child.Destroy()
		pm.child = nil
	}
	if pm.popover != nil {
		pm.popover.Destroy()
		pm.popover = nil
	}
	pm.model, pm.content, pm.anchor = nil, nil, nil
}

const (
	menuSubmenuOverlap  = 4
	menuHoverOpenDelay  = 250 * time.Millisecond
	menuHoverCloseDelay = 250 * time.Millisecond
	menuTransitionLimit = 500 * time.Millisecond
)

func (pm *PopoverMenu) rootMenu() *PopoverMenu {
	for pm.parent != nil {
		pm = pm.parent
	}
	return pm
}
func (pm *PopoverMenu) nativeHost() *popover {
	if p, ok := pm.popover.(*sharedPopover); ok && p.Visible() {
		return p.slot.popup
	}
	return nil
}
func (pm *PopoverMenu) ancestorsAvailable() bool {
	for p := pm; p.parent != nil; p = p.parent {
		item := p.parent.expanded
		if item == nil || !item.available() || !item.Visible() || !p.parent.contains(item) {
			return false
		}
	}
	return true
}
func (pm *PopoverMenu) cancelHover() {
	pm.hoverEpoch++
	pm.pending = nil
	if pm.hoverTimer != nil {
		pm.hoverTimer.Stop()
	}
	if pm.hoverHandle != nil {
		pm.hoverHandle.Disconnect()
		pm.hoverHandle = nil
	}
}
func (pm *PopoverMenu) cancelAncestorHover() {
	for p := pm; p != nil; p = p.parent {
		p.cancelHover()
		p.protectUntil = time.Time{}
	}
}
func (pm *PopoverMenu) closeChild() {
	pm.cancelHover()
	child := pm.child
	pm.expanded, pm.keyboardChild = nil, false
	pm.protectUntil = time.Time{}
	if child != nil {
		child.Hide()
	}
	if pm.content != nil {
		pm.content.RequestPaint()
		pm.content.requestSemanticUpdate()
	}
}
func (pm *PopoverMenu) endOccurrence() {
	pm.closeChild()
	pm.modelHandles.Disconnect()
	pm.modelHandles = nil
	pm.pointerValid = false
	if pm.content != nil {
		pm.content.disconnectModel()
	}
}
func menuHasEntries(model MenuModel) bool {
	if model == nil {
		return false
	}
	for i := 0; i < model.ItemsCount(); i++ {
		item := model.ItemAt(i)
		if item != nil && item.Visible() && !item.Separator() {
			return true
		}
	}
	return false
}
func (pm *PopoverMenu) rowFor(item *MenuItem) *menuItemRow {
	if pm.content == nil {
		return nil
	}
	for _, w := range pm.content.list.items {
		if r := w.(*menuItemRow); r.mi == item && r.Visible() {
			return r
		}
	}
	return nil
}
func (pm *PopoverMenu) report(err error) {
	if err != nil && !errors.Is(err, ErrPopoverSuperseded) {
		pm.rootMenu().errors.Emit(err)
	}
}
func (pm *PopoverMenu) openChild(item *MenuItem, keyboard bool) {
	if !pm.Visible() || !pm.ancestorsAvailable() || item == nil || !item.available() || !item.Visible() || item.Separator() || !pm.contains(item) || !menuHasEntries(item.Submenu()) {
		return
	}
	if pm.expanded == item && pm.child != nil && pm.child.Visible() {
		pm.cancelHover()
		if keyboard {
			pm.keyboardChild = true
			pm.child.content.moveItem(1, true)
		}
		return
	}
	model := item.Submenu()
	sourceRevision, sourceEpoch := item.revision, pm.epoch
	// Compare entry pointers along this path, never arbitrary MenuModel values.
	// Shared DAG branches remain valid; revisiting an ancestor entry is a cycle.
	for i := 0; i < model.ItemsCount(); i++ {
		entry := model.ItemAt(i)
		if entry == item {
			pm.report(fmt.Errorf("menu: cyclic submenu"))
			return
		}
		for p := pm; p.parent != nil; p = p.parent {
			if entry == p.parent.expanded {
				pm.report(fmt.Errorf("menu: cyclic submenu"))
				return
			}
		}
	}
	pm.closeChild()
	if host := pm.nativeHost(); host != nil && host.layoutDirty {
		if err := host.layoutPopup(); err != nil {
			pm.report(err)
			return
		}
	}
	if !pm.Visible() || pm.epoch != sourceEpoch || pm.expanded != nil || item.revision != sourceRevision || !pm.contains(item) {
		return
	}
	row := pm.rowFor(item)
	if row == nil || !pm.anchorVisible(row) {
		return
	}
	if pm.child == nil {
		pm.child = NewPopoverMenu(row)
		pm.child.parent = pm
		pm.child.SetModal(false)
	}
	child := pm.child
	child.anchor = row
	if child.popover != nil {
		child.popover.SetAnchor(row)
	}
	child.SetMenu(model)
	placement := popoverPlacementRight
	host := pm.nativeHost()
	if host == nil {
		return
	}
	if host.placement == popoverPlacementLeft {
		placement = popoverPlacementLeft
	}
	if pm.parent != nil {
		parent := pm.parent.nativeHost()
		if parent != nil && host.requestedPosition.X < parent.requestedPosition.X {
			placement = popoverPlacementLeft
		}
	}
	// Source row supplies vertical alignment. The parent body supplies horizontal
	// edges: row padding must not accumulate at each cascade level.
	body := host.bodyRect()
	origin := absOrigin(row)
	overlap := min(float32(menuSubmenuOverlap), body.Width/2)
	area := geometry.Rect(body.X-origin.X+overlap, -pm.content.padding, max(0, body.Width-2*overlap), row.Rect().Height)
	// The child's own padding is the same menu style and is measured before Show.
	// Position its first row at the source row's top.
	if child.popover == nil {
		if child.content == nil {
			child.content = newMenuContent(model, child.maxHeight, child.activate)
		}
		child.content.presenter = child
	}
	// show creates the lightweight client, then the normal Popover solver applies
	// the private anchor area, workarea constraints and native size rounding.
	child.ensurePopover()
	child.popover.(*sharedPopover).anchorArea = &area
	epoch := pm.epoch
	if err := child.show(placement, geometry.Point{}); err != nil {
		child.Hide()
		pm.report(err)
		return
	}
	if !pm.Visible() || pm.epoch != epoch || item.revision != sourceRevision || !pm.contains(item) || !item.available() {
		child.Hide()
		return
	}
	pm.expanded, pm.keyboardChild = item, keyboard
	pm.content.RequestPaint()
	pm.content.requestSemanticUpdate()
	pm.protectUntil = time.Time{}
	if keyboard {
		child.content.moveItem(1, true)
	}
}

func (pm *PopoverMenu) anchorVisible(row *menuItemRow) bool {
	if row == nil || row.Root() != pm.nativeHost() || !visibleInTree(row) {
		return false
	}
	viewport := pm.content.sv.viewport.windowRect()
	r := row.windowRect()
	return r.Width > 0 && r.Height > 0 && r.X >= viewport.X && r.Y >= viewport.Y && r.X+r.Width <= viewport.X+viewport.Width+.01 && r.Y+r.Height <= viewport.Y+viewport.Height+.01
}
func (pm *PopoverMenu) checkChildAnchor() {
	if pm.expanded != nil {
		if !pm.anchorVisible(pm.rowFor(pm.expanded)) {
			pm.closeChild()
		} else if child := pm.child.nativeHost(); child != nil {
			child.requestLayout()
		}
	}
}
func (pm *PopoverMenu) modelInvalidated() {
	pm.epoch++
	pm.closeChild()
	if pm.Visible() {
		pm.observeModels()
	}
}
func (pm *PopoverMenu) observeModels() {
	pm.modelHandles.Disconnect()
	pm.modelHandles = nil
	if pm.model == nil {
		return
	}
	// Only directly referenced child availability is observed here. The displayed
	// child observes its own model; unopened descendants allocate no presenters.
	model, epoch := pm.model, pm.epoch
	seen := make(map[*MenuItem]bool)
	for i := 0; i < model.ItemsCount(); i++ {
		item := model.ItemAt(i)
		if item == nil || item.Submenu() == nil || seen[item] {
			continue
		}
		seen[item] = true
		parentItem := item
		h := item.Submenu().ConnectItems(func() {
			if !pm.Visible() {
				return
			}
			if pm.expanded == parentItem && !menuHasEntries(parentItem.Submenu()) {
				pm.closeChild()
			}
			pm.content.RequestPaint()
			pm.content.requestSemanticUpdate()
		})
		if pm.destroyed || pm.epoch != epoch {
			h.Disconnect()
			return
		}
		pm.modelHandles = append(pm.modelHandles, h)
	}
}

// The root is the authorization boundary. Child key input stays on the normal
// EventDispatcher/controller path without weakening public Popover ownership.
func (pm *PopoverMenu) filterEvent(e events.Event) bool {
	if key, ok := e.(events.KeyEvent); ok {
		root := pm.rootMenu()
		host := root.nativeHost()
		if host == nil {
			return true
		}
		owner := surfaceCore(host.owner)
		if owner == nil || owner.destroyed || owner.frameSuspended || owner.modalTarget != nil && owner.modalTarget != host || host.modal && !host.ownsModalInput() {
			return true
		}
		target := root
		for target.keyboardChild && target.child != nil && target.child.Visible() {
			target = target.child
		}
		actual := target.nativeHost()
		if actual != nil && actual.owner == host.owner && modalWidgetAllowed(target.anchor) && target.ancestorsAvailable() {
			_ = actual.dispatcher.DispatchEvent(actual, key)
		}
		return true
	}
	pointer, ok := e.(events.PointerEvent)
	if !ok || pointer.EventType != events.PointerDown && pointer.EventType != events.PointerUp && pointer.EventType != events.PointerMove {
		return false
	}
	host := pm.nativeHost()
	if host == nil {
		return true
	}
	root := pm.rootMenu()
	// An accepted gesture owns move/up before body hit testing.
	if pointer.EventType != events.PointerDown {
		for layer := root; layer != nil && layer.Visible(); layer = layer.child {
			p := layer.nativeHost()
			if p != nil && p.dispatcher.captureTarget != nil {
				if p == host {
					return false
				}
				pointer.Position.X += host.requestedPosition.X - p.requestedPosition.X
				pointer.Position.Y += host.requestedPosition.Y - p.requestedPosition.Y
				_ = p.dispatcher.DispatchEvent(p, pointer)
				return true
			}
		}
	}
	position := geometry.Point{X: pointer.Position.X + host.requestedPosition.X, Y: pointer.Position.Y + host.requestedPosition.Y}
	picked := root.layerAt(position)
	if picked != nil {
		for p := picked.parent; p != nil; p = p.parent {
			p.cancelHover()
		}
		if pointer.EventType == events.PointerDown {
			for p := picked; p.parent != nil; p = p.parent {
				p.parent.keyboardChild = true
			}
			picked.keyboardChild = false
		}
		if picked == pm {
			return false
		}
		p := picked.nativeHost()
		pointer.Position = geometry.Point{X: position.X - p.requestedPosition.X, Y: position.Y - p.requestedPosition.Y}
		_ = p.DispatchEvent(pointer)
		return true
	}
	if pointer.EventType == events.PointerDown {
		owner, modal, epoch := surfaceCore(host.owner), root.modal, root.epoch
		root.Hide()
		if !modal && owner != nil && !owner.destroyed && !root.Visible() && root.epoch == epoch+1 {
			pointer.Position = position
			_ = owner.DispatchEvent(pointer)
		}
		return true
	}
	if pointer.EventType == events.PointerMove {
		root.observeOwnerPointer(events.PointerEvent{EventType: events.PointerMove, Position: position})
	}
	return false
}
func (pm *PopoverMenu) returnToParent() {
	parent := pm.parent
	item, epoch := parent.expanded, parent.epoch
	parent.closeChild()
	if parent.Visible() && parent.epoch == epoch && parent.expanded == nil && parent.contains(item) {
		parent.content.selectItem(item)
	}
}

func (pm *PopoverMenu) keyEvent(ctx EventContext, e events.KeyEvent) bool {
	if e.Modifiers != 0 && !(e.Key == events.KeyTab && e.Modifiers == events.ModifierShift) {
		return false
	}
	handled := true
	switch e.Key {
	case events.KeyArrowDown, events.KeyArrowUp, events.KeyHome, events.KeyEnd:
		pm.closeChild()
		return false
	case events.KeyEnter, events.KeySpace:
		item := pm.content.active
		if item == nil || item.Submenu() == nil {
			return false
		}
		if !e.Repeat {
			pm.openChild(item, true)
		}

	case events.KeyArrowRight:
		item := pm.content.active
		if item == nil || item.Submenu() == nil {
			return false
		}
		if !e.Repeat {
			pm.openChild(item, true)
		}
	case events.KeyArrowLeft:
		if pm.parent == nil {
			return false
		}
		if !e.Repeat {
			pm.returnToParent()
		}
	case events.KeyEscape:
		if !e.Repeat {
			if pm.parent != nil {
				pm.returnToParent()
			} else {
				pm.Hide()
			}
		}
	case events.KeyTab:
		root := pm.rootMenu()
		handled = root.modal
		root.Hide()
	default:
		return false
	}
	if handled {
		e.PreventDefault()
		ctx.StopPropagation()
	}
	return handled
}

func (r *menuItemRow) submenuWidth() float32 {
	if r.columns != nil {
		return r.columns.submenu
	}
	if r.mi != nil && r.mi.Submenu() != nil {
		return menuIndicatorSize + menuIndicatorGap
	}
	return 0
}

// Seed before queued crossing events: opening a surface under a stationary
// pointer may update selection, but must not count as movement to a branch.
func (pm *PopoverMenu) seedPointer() {
	host := pm.nativeHost()
	if host == nil {
		return
	}
	var point geometry.Point
	if pm.parent != nil && pm.parent.pointerValid {
		parent := pm.parent.nativeHost()
		if parent == nil {
			return
		}
		origin := absOrigin(pm.parent.content)
		point = geometry.Point{X: pm.parent.pointer.X + origin.X + parent.requestedPosition.X, Y: pm.parent.pointer.Y + origin.Y + parent.requestedPosition.Y}
	} else if owner := surfaceCore(host.owner); owner != nil && owner.cursorInside {
		point = owner.cursorPosition
	} else {
		return
	}
	origin := absOrigin(pm.content)
	pm.pointer = geometry.Point{X: point.X - host.requestedPosition.X - origin.X, Y: point.Y - host.requestedPosition.Y - origin.Y}
	pm.pointerValid = true
}

// layerAt uses body geometry, including round corners, in native owner DIP.
// Shadows never keep an unrelated branch active.
func (pm *PopoverMenu) layerAt(position geometry.Point) *PopoverMenu {
	var picked *PopoverMenu
	for layer := pm.rootMenu(); layer != nil && layer.Visible(); layer = layer.child {
		host := layer.nativeHost()
		if host == nil {
			break
		}
		point := geometry.Point{X: position.X - host.requestedPosition.X, Y: position.Y - host.requestedPosition.Y}
		_, border, _ := host.surfaceStyle()
		radius, _ := border.CornerRadius()
		if roundedBodyContains(host.bodyRect(), radius, point) {
			picked = layer
		}
	}
	return picked
}

func (pm *PopoverMenu) observeOwnerPointer(event events.PointerEvent) {
	if !pm.Visible() || pm.layerAt(event.Position) != nil {
		return
	}
	for layer := pm; layer != nil && layer.Visible(); layer = layer.child {
		if host := layer.nativeHost(); host != nil && host.dispatcher.captureTarget != nil {
			return
		}
	}
	epoch := pm.epoch
	owner := surfaceCore(pm.nativeHost().owner)
	shown := owner.popoversShown
	for layer := pm; layer != nil && layer.Visible(); layer = layer.child {
		layer.content.selectItem(nil)
		layer.pointerValid = false
		layer.cancelHover()
		layer.protectUntil = time.Time{}
		host := layer.nativeHost()
		local := event
		local.EventType = events.PointerLeave
		local.Position = geometry.Point{X: event.Position.X - host.requestedPosition.X, Y: event.Position.Y - host.requestedPosition.Y}
		_ = host.dispatcher.DispatchEvent(host, local)
		// Crossing observers may destroy or replace the displayed chain.
		if !pm.Visible() || pm.epoch != epoch || owner.popoversShown != shown || layer.nativeHost() != host {
			return
		}
	}
}

// Leave may only carry the last known position. Cancel local intent without
// inferring a destination or closing the path that the user can return to.
func (pm *PopoverMenu) pointerLeave() {
	pm.cancelHover()
	pm.protectUntil = time.Time{}
}

func (pm *PopoverMenu) pointerMotion(position geometry.Point, motion bool) {
	previous, valid := pm.pointer, pm.pointerValid
	pm.pointer, pm.pointerValid = position, true
	pm.content.selectPointer(position)
	item := pm.content.active
	if motion && valid && previous == position {
		return
	}
	for parent := pm.parent; parent != nil; parent = parent.parent {
		parent.cancelHover()
		parent.protectUntil = time.Time{}
		parent.content.selectItem(nil)
		parent.keyboardChild = true
	}
	for child := pm.child; child != nil && child.Visible(); child = child.child {
		child.cancelHover()
		child.protectUntil = time.Time{}
		child.content.selectItem(nil)
		child.pointerValid = false
	}
	pm.keyboardChild = false
	if !motion {
		pm.cancelAncestorHover()
		if item != pm.expanded {
			pm.closeChild()
		}
		return
	}
	if item == pm.expanded {
		pm.cancelHover()
		pm.protectUntil = time.Time{}
		return
	}
	delay := menuHoverOpenDelay
	if pm.expanded != nil {
		if valid && pm.preserveTransition(previous, position, item) {
			return
		}
		if item == nil || item.Submenu() == nil {
			delay = menuHoverCloseDelay
		}
		if !pm.protectUntil.IsZero() {
			remaining := pm.protectUntil.Sub(pm.hoverNow())
			pm.protectUntil = time.Time{}
			pm.cancelHover()
			if remaining <= 0 {
				pm.finishHover(item)
				return
			}
			delay = min(delay, remaining)
		}
	}
	if pm.expanded == nil && (item == nil || item.Submenu() == nil) {
		pm.cancelHover()
		return
	}
	pm.scheduleHover(item, delay)
}

func (pm *PopoverMenu) hoverNow() time.Time {
	if host := pm.nativeHost(); host != nil && host.app != nil && host.app.timers != nil {
		return host.app.timers.now()
	}
	return time.Now()
}
func (pm *PopoverMenu) preserveTransition(previous, current geometry.Point, item *MenuItem) bool {
	if pm.child == nil || !pm.child.Visible() || !pm.movingTowardChild(previous, current) {
		return false
	}
	now := pm.hoverNow()
	if pm.protectUntil.IsZero() {
		row := pm.rowFor(pm.expanded)
		origin := absOrigin(pm.content)
		if row == nil || !containsPoint(row.windowRect(), geometry.Point{X: previous.X + origin.X, Y: previous.Y + origin.Y}) {
			return false
		}
		pm.protectUntil = now.Add(menuTransitionLimit)
	}
	remaining := pm.protectUntil.Sub(now)
	if remaining <= 0 {
		return false
	}
	pm.scheduleHover(item, remaining)
	return true
}

// Only the displayed branch waits for intent; selection follows input now.
// Changing rows cannot extend a protected trajectory's original deadline.
func (pm *PopoverMenu) scheduleHover(item *MenuItem, delay time.Duration) {
	if pm.pending == item && pm.hoverTimer != nil && pm.hoverTimer.Active() {
		return
	}
	pm.cancelHover()
	host := pm.nativeHost()
	if host == nil || host.app == nil {
		return
	}
	if pm.hoverTimer == nil {
		pm.hoverTimer = host.app.NewTimer()
	}
	pm.pending = item
	epoch, occurrence := pm.hoverEpoch, pm.epoch
	pm.hoverHandle = pm.hoverTimer.ConnectTimeout(func() {
		if pm.destroyed || !pm.Visible() || pm.hoverEpoch != epoch || pm.epoch != occurrence {
			return
		}
		pm.cancelHover()
		pm.protectUntil = time.Time{}
		pm.finishHover(item)
	})
	pm.report(pm.hoverTimer.StartOnce(delay))
}

func (pm *PopoverMenu) finishHover(item *MenuItem) {
	if item != nil && item.available() && menuHasEntries(item.Submenu()) {
		pm.openChild(item, false)
	} else {
		pm.closeChild()
	}
}
func (pm *PopoverMenu) movingTowardChild(previous, current geometry.Point) bool {
	host, child := pm.nativeHost(), pm.child.nativeHost()
	if host == nil || child == nil {
		return false
	}
	body := child.bodyRect()
	left := child.requestedPosition.X + body.X - host.requestedPosition.X
	right := left + body.Width
	top := child.requestedPosition.Y + body.Y - host.requestedPosition.Y
	bottom := top + body.Height
	edge := left
	if right <= previous.X {
		edge = right
		if current.X >= previous.X {
			return false
		}
	} else if current.X <= previous.X {
		return false
	}
	dx := edge - previous.X
	if dx == 0 {
		return false
	}
	t := (current.X - previous.X) / dx
	if t < 0 || t > 1 {
		return false
	}
	low, high := previous.Y+(top-previous.Y)*t, previous.Y+(bottom-previous.Y)*t
	return current.Y >= low && current.Y <= high
}
