package gui

import (
	"errors"
	"time"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/gui/textformat"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/style"
)

const (
	tooltipDelay     = 700 * time.Millisecond
	tooltipTolerance = 4
)

// tooltipState belongs to one window, including its visible popovers. Widgets
// carry only text; one application timer and one transient surface suffice.
type tooltipState struct {
	owner    *window
	host     EventTarget
	target   Widget
	blocked  Widget
	keyboard bool
	position geometry.Point
	text     string
	bounds   geometry.Rectangle
	epoch    uint64
	timer    *Timer
	unmount  signal.Handle
	popup    *popover
}

func tooltipsFor(host EventTarget) *tooltipState {
	var owner *window
	switch h := host.(type) {
	case *window:
		owner = h
	case *popover:
		owner, _ = h.owner.(*window)
	}
	if owner == nil {
		return nil
	}
	owner.tooltip.owner = owner
	return &owner.tooltip
}

func (t *tooltipState) eligible(host EventTarget) bool {
	if t.owner == nil || t.owner.destroyed || !t.owner.focused || t.owner.frameSuspended {
		return false
	}
	if host == t.owner {
		return t.owner.modalTarget == nil
	}
	p, ok := host.(*popover)
	return ok && p != t.popup && !p.destroyed && p.visible &&
		(t.owner.modalTarget == nil || t.owner.modalTarget == p)
}

func tooltipAncestor(widget Widget) Widget {
	for w := widget; w != nil; w = w.Parent() {
		if w.Tooltip() != "" {
			return w
		}
	}
	return nil
}

func (t *tooltipState) candidate(host EventTarget, keyboard bool, pos geometry.Point) Widget {
	if !t.eligible(host) {
		return nil
	}
	root := host.(interface{ rootState() *rootBase }).rootState()
	if keyboard {
		if !root.focusVisible {
			return nil
		}
		return tooltipAncestor(host.FocusedWidget())
	}
	return tooltipAncestor(root.dispatcher.pick(host.Widget(), pos))
}

func (t *tooltipState) consider(host EventTarget, keyboard bool, pos geometry.Point) {
	next := t.candidate(host, keyboard, pos)
	if next != t.blocked {
		t.blocked = nil
	}
	if next == nil || next == t.blocked {
		if t.target != nil {
			t.dismiss(false)
		}
		return
	}
	if next == t.target && host == t.host && keyboard == t.keyboard && next.Tooltip() == t.text {
		if t.popup != nil || keyboard || !gestureMoved(t.position, pos, tooltipTolerance) {
			return
		}
	}
	t.dismiss(false)
	if !t.eligible(host) || next.base().destroyed || !visibleInTree(next) {
		return
	}
	t.host, t.target, t.keyboard, t.position, t.text = host, next, keyboard, pos, next.Tooltip()
	t.bounds = tooltipBounds(next)
	if emptyRect(t.bounds) {
		t.dismiss(false)
		return
	}
	t.unmount = next.ConnectUnmount(func() { t.dismiss(false) })
	if t.owner.app == nil || t.owner.app.timers == nil {
		t.dismiss(false)
		return
	}
	if t.timer == nil {
		t.timer = t.owner.app.NewTimer()
		t.timer.ConnectTimeout(t.show)
	}
	if err := t.timer.StartOnce(tooltipDelay); err != nil {
		t.dismiss(true)
	}
}

func (t *tooltipState) current() bool {
	if t.target == nil || !t.eligible(t.host) || t.target.base().destroyed ||
		!visibleInTree(t.target) || t.target.Tooltip() != t.text {
		return false
	}
	root, ok := t.target.Root().(EventTarget)
	if !ok || root != t.host {
		return false
	}
	if t.keyboard {
		return t.candidate(t.host, true, geometry.Point{}) == t.target
	}
	d := &t.host.(interface{ rootState() *rootBase }).rootState().dispatcher
	return d.pointerInside && t.candidate(t.host, false, d.pointerEvent.Position) == t.target &&
		d.pointerEvent.Buttons == 0
}

func (t *tooltipState) show() {
	if !t.current() {
		t.dismiss(false)
		return
	}
	epoch, target := t.epoch, t.target
	for _, transparent := range []bool{true, false} {
		p := NewPopover(target, &PopoverOptions{Transparent: transparent}).(*popover)
		p.SetStyleName("tooltip")
		p.SetPlacement(PopoverPlacementBottom)
		area := t.bounds
		origin := target.base().windowRect().Pos
		area.X -= origin.X
		area.Y -= origin.Y
		p.anchorArea = &area
		p.SetWidget(newTooltipContent(target, t.text))
		t.popup = p
		p.ConnectClosed(func() {
			if t.popup == p {
				t.dismiss(true)
			}
		})
		err := p.Show()
		if t.epoch != epoch || t.popup != p || !t.current() {
			p.Destroy()
			return
		}
		if err == nil {
			return
		}
		t.popup = nil
		p.Destroy()
		var creation *popoverCreationError
		if !transparent || !errors.As(err, &creation) || !errors.Is(err, platform.ErrUnsupported) {
			break
		}
	}
	t.dismiss(true)
}

func (t *tooltipState) dismiss(block bool) {
	t.epoch++
	if block && t.target != nil {
		t.blocked = t.target
	} else if !block {
		t.blocked = nil
	}
	if t.timer != nil {
		t.timer.Stop()
	}
	if t.unmount != nil {
		t.unmount.Disconnect()
		t.unmount = nil
	}
	p := t.popup
	t.popup, t.host, t.target, t.text = nil, nil, nil, ""
	if p != nil {
		p.Destroy()
	}
}

func (t *tooltipState) dismissOnEvent(host EventTarget, event events.Event) {
	switch e := event.(type) {
	case events.PointerEvent:
		if e.EventType == events.PointerDown || e.Buttons != 0 {
			t.dismiss(true)
		}
	case events.KeyEvent:
		if e.EventType == events.KeyDown {
			t.dismiss(true)
		}
	case events.WheelEvent, events.DragOfferEvent, events.DragSourceEvent:
		t.dismiss(true)
	case events.SizeEvent:
		if host == t.host || host == t.owner {
			t.dismiss(true)
		}
	case events.FocusEvent:
		if !e.Focused && (host == t.host || host == t.owner) {
			t.dismiss(false)
		}
	case events.StateEvent:
		if (host == t.host || host == t.owner) && (e.State == WindowStateHidden || e.State == WindowStateMinimized) {
			t.dismiss(false)
		}
	}
}

func (t *tooltipState) afterEvent(host EventTarget, event events.Event, pointerMoved bool) {
	// Native popup creation can synchronously report its own initial Hidden
	// state. Presentation-surface events must not cancel the anchor's session.
	if !t.eligible(host) {
		return
	}
	switch e := event.(type) {
	case events.PointerEvent:
		switch e.EventType {
		case events.PointerEnter, events.PointerMove:
			// Native popup presentation can re-report an unchanged cursor. Only
			// actual pointer movement changes a keyboard-triggered session.
			if t.keyboard && t.target != nil && !pointerMoved {
				return
			}
			if e.Buttons == 0 {
				t.consider(host, false, e.Position)
			}
		case events.PointerLeave:
			if t.host == host && !t.keyboard || t.blocked != nil && t.blocked.Root() == host.(Root) {
				t.dismiss(false)
			}
		}
	case events.FocusEvent:
		if e.Focused && host.(interface{ rootState() *rootBase }).rootState().focusVisible {
			t.consider(host, true, geometry.Point{})
		} else if t.host == host && t.keyboard {
			t.dismiss(false)
		}
	case events.KeyEvent:
		if e.EventType == events.KeyDown && e.Key == events.KeyTab {
			t.consider(host, true, geometry.Point{})
		}
	}
}

func (t *tooltipState) refresh(layoutChanged bool) {
	if t.target == nil {
		return
	}
	if !t.current() {
		t.dismiss(false)
		return
	}
	bounds := tooltipBounds(t.target)
	if emptyRect(bounds) {
		t.dismiss(false)
		return
	}
	if t.popup != nil && (bounds != t.bounds || layoutChanged) {
		t.bounds = bounds
		area := bounds
		origin := t.target.base().windowRect().Pos
		area.X -= origin.X
		area.Y -= origin.Y
		t.popup.anchorArea = &area
		invalidateStyleSubtree(t.popup.Widget())
		t.popup.RequestLayout()
	} else {
		t.bounds = bounds
	}
}

func tooltipChanged(root Root, widget Widget) {
	host, ok := root.(EventTarget)
	if !ok || widget == nil {
		return
	}
	t := tooltipsFor(host)
	if t == nil || t.target != widget && t.blocked != widget {
		return
	}
	keyboard, pos := t.keyboard, t.position
	t.dismiss(false)
	t.consider(host, keyboard, pos)
}

func cancelTooltipSubtree(root Root, subtree Widget) {
	host, ok := root.(EventTarget)
	if !ok || subtree == nil {
		return
	}
	if t := tooltipsFor(host); t != nil &&
		(t.target != nil && t.target.base().isDescendant(t.target, subtree) ||
			t.blocked != nil && t.blocked.base().isDescendant(t.blocked, subtree)) {
		t.dismiss(false)
	}
}

func cancelTooltipHost(host EventTarget) {
	if t := tooltipsFor(host); t != nil && t.host == host {
		t.dismiss(false)
	}
}

// Match ordinary picking: each ancestor clips its descendants, including
// viewport containers. The result is in the target Root's logical coordinates.
func tooltipBounds(target Widget) geometry.Rectangle {
	if target == nil || !visibleInTree(target) {
		return geometry.Rectangle{}
	}
	r := target.base().windowRect()
	for p := target.Parent(); p != nil; p = p.Parent() {
		r = intersectTooltipRect(r, p.base().windowRect())
	}
	if root, ok := target.Root().(interface{ rootState() *rootBase }); ok {
		b := root.rootState()
		r = intersectTooltipRect(r, geometry.Rect(0, 0, b.width, b.height))
	}
	return r
}

func intersectTooltipRect(a, b geometry.Rectangle) geometry.Rectangle {
	x, y := max(a.X, b.X), max(a.Y, b.Y)
	return geometry.Rect(x, y, max(0, min(a.X+a.Width, b.X+b.Width)-x), max(0, min(a.Y+a.Height, b.Y+b.Height)-y))
}

// This private sheet explicitly borrows the anchor's style environment. Public
// popovers keep their normal application-wide style unless given a StyleScope.
type tooltipSheet struct{ anchor Widget }

func (s tooltipSheet) Resolve(sel style.Sel) style.Style {
	return ResolveWidgetStyle(s.anchor, sel.Name, sel.Part, sel.State)
}

type tooltipContent struct {
	WidgetBase
	label *Label
}

func newTooltipContent(anchor Widget, text string) Widget {
	b := &tooltipContent{label: NewLabel(textformat.Literal(text))}
	l := layout.NewLinearLayout(layout.DirectionVertical)
	l.Padding = geometry.UniformInsets(8)
	b.SetLayoutManager(l)
	b.SetMaxSize(geometry.Size{Width: 360})
	label := b.label
	label.SetStyleName("tooltip-text")
	label.SetWrapMode(WrapWordChar)
	b.WidgetBase.AddChild(b, label)
	scope := NewStyleScope()
	scope.SetStyleSheet(tooltipSheet{anchor})
	scope.SetChild(b)
	return scope
}

func (b *tooltipContent) Snapshot() WidgetInfo {
	info := b.WidgetBase.Snapshot()
	info.Role, info.Text = RoleTooltip, b.label.PlainText()
	return info
}
