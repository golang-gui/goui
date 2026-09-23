package gui

import (
	"slices"
	"time"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform/events"
)

// GestureController opts an EventController into pointer-sequence arbitration.
// It recognizes input in HandleEvent and joins the current press with JoinGesture.
type GestureController interface {
	EventController
	GestureAccepted(EventContext)
	GestureCanceled(GestureCancelReason)
}

type GestureCancelReason uint8

const (
	GestureRejected GestureCancelReason = iota
	GestureLost
	GestureInterrupted
	GestureDetached
	GestureHostClosed
)

// GestureParticipation belongs to one controller and one pointer press. Its
// methods are GUI-thread operations. A stale participation has no effect.
type GestureParticipation struct {
	sequence   *gestureSequence
	controller GestureController
	widget     Widget
	depth      int
	order      int
	state      gestureState
	delivered  uint64
}

// JoinGesture enrolls a controller only while its PointerDown is being
// dispatched. The same controller may join a sequence only once.
func JoinGesture(ctx EventContext) *GestureParticipation {
	c, ok := ctx.(*eventContext)
	if !ok || c.dispatcher == nil || c.controller == nil {
		return nil
	}
	g, ok := c.controller.(GestureController)
	if !ok || c.event == nil {
		return nil
	}
	e, ok := c.event.(events.PointerEvent)
	if !ok || e.EventType != events.PointerDown {
		return nil
	}
	s := c.dispatcher.gesture
	if s == nil || !s.active || s.button != e.Button || s.host != c.host {
		return nil
	}
	for _, p := range s.members {
		if p.controller == g {
			return p
		}
	}
	depth := -1
	order := len(s.members)
	if c.current != nil {
		depth = slices.Index(s.path, c.current)
		if depth < 0 {
			return nil
		}
		order = slices.Index(c.current.EventControllers(), c.controller)
		if order < 0 {
			return nil
		}
	}
	p := &GestureParticipation{sequence: s, controller: g, widget: c.current,
		depth: depth, order: order, delivered: s.serial}
	s.members = append(s.members, p)
	return p
}

// Claim requests ownership. Resolution occurs after all participants have
// processed the current native event, before DispatchEvent returns.
func (p *GestureParticipation) Claim() {
	if p != nil && p.state == gesturePending && p.sequence.active && p.sequence.delivering {
		p.state = gestureClaimed
	}
}

// Reject leaves the sequence. It also cancels an already accepted gesture.
func (p *GestureParticipation) Reject() {
	if p != nil && p.state != gestureEnded && p.sequence.active {
		p.sequence.cancelMember(p, GestureRejected)
	}
}

const gestureClickInterval = 500 * time.Millisecond
const gestureDefaultDistance float32 = 4

type gestureState uint8

const (
	gesturePending gestureState = iota
	gestureClaimed
	gestureWon
	gestureEnded
)

type gestureSequence struct {
	dispatcher      *EventDispatcher
	host            EventTarget
	button          events.PointerButton
	buttonsObserved bool
	path            []Widget
	members         []*GestureParticipation
	winner          *GestureParticipation
	event           events.PointerEvent
	serial          uint64
	active          bool
	delivering      bool
}

func (s *gestureSequence) valid(p *GestureParticipation) bool {
	if !s.active || s.dispatcher.gesture != s || p.state == gestureEnded {
		return false
	}
	if p.widget == nil {
		return s.dispatcher.hostController == p.controller
	}
	if p.widget.base().destroyed || !visibleInTree(p.widget) ||
		!slices.Contains(p.widget.EventControllers(), EventController(p.controller)) {
		return false
	}
	root := s.dispatcher.treeRoot(s.host.Widget(), p.widget)
	return len(widgetPath(root, p.widget)) != 0
}

func (s *gestureSequence) cancelMember(p *GestureParticipation, reason GestureCancelReason) {
	if p.state == gestureEnded {
		return
	}
	p.state = gestureEnded
	if s.winner == p {
		s.winner = nil
		s.dispatcher.captureTarget = nil
	}
	p.controller.GestureCanceled(reason)
}

func (s *gestureSequence) cancel(reason GestureCancelReason) {
	if s == nil || !s.active {
		return
	}
	s.active = false
	if s.dispatcher.gesture == s {
		s.dispatcher.gesture = nil
		s.dispatcher.captureTarget = nil
	}
	for _, p := range slices.Clone(s.members) {
		s.cancelMember(p, reason)
	}
}

func (s *gestureSequence) revalidate() {
	if s == nil || !s.active {
		return
	}
	for _, p := range slices.Clone(s.members) {
		if p.state != gestureEnded && !s.valid(p) {
			won := s.winner == p
			s.cancelMember(p, GestureDetached)
			if won {
				s.cancel(GestureDetached)
				return
			}
		}
	}
	for _, p := range s.members {
		if p.state != gestureEnded {
			return
		}
	}
	s.cancel(GestureRejected)
}

func (s *gestureSequence) resolve() {
	if !s.active || s.winner != nil {
		return
	}
	var winner *GestureParticipation
	for _, p := range s.members {
		if p.state != gestureClaimed {
			continue
		}
		if !s.valid(p) {
			s.cancelMember(p, GestureDetached)
			continue
		}
		if winner == nil || p.depth > winner.depth || p.depth == winner.depth && p.order < winner.order {
			winner = p
		}
	}
	if winner == nil {
		return
	}
	winner.state = gestureWon
	s.winner = winner
	for _, p := range slices.Clone(s.members) {
		if p != winner && p.state != gestureEnded {
			s.cancelMember(p, GestureLost)
		}
	}
	if !s.valid(winner) {
		s.cancelMember(winner, GestureDetached)
		return
	}
	ctx := &eventContext{event: s.event, current: winner.widget, host: s.host}
	winner.controller.GestureAccepted(ctx)
	if s.active && s.dispatcher.gesture == s && s.winner == winner && !s.valid(winner) {
		s.cancelMember(winner, GestureDetached)
	}
	if s.active && s.winner == winner {
		s.dispatcher.captureTarget = winner.widget
	}
}

// takeOver finishes GUI routing before a synchronous native move or drag.
func (s *gestureSequence) takeOver() {
	if s == nil || !s.active {
		return
	}
	s.active = false
	if s.dispatcher.gesture == s {
		s.dispatcher.gesture = nil
		s.dispatcher.captureTarget = nil
		s.dispatcher.suppressUp = s.button
	}
	for _, p := range s.members {
		p.state = gestureEnded
	}
}

func (d *EventDispatcher) finishGesture(s *gestureSequence, e events.PointerEvent) {
	if s == nil || d.gesture != s || !s.active {
		return
	}
	s.delivering = false
	for _, p := range slices.Clone(s.members) {
		if p.state == gestureEnded || !s.valid(p) {
			s.cancelMember(p, GestureDetached)
		}
	}
	if d.gesture != s || !s.active {
		return
	}
	s.resolve()
	if e.EventType == events.PointerDown && len(s.members) == 0 {
		s.active = false
		d.gesture = nil
		return
	}
	if e.EventType == events.PointerUp && e.Button == s.button {
		s.active = false
		d.gesture = nil
		d.captureTarget = nil
		for _, p := range s.members {
			if p.state != gestureEnded {
				p.state = gestureEnded
				if p != s.winner {
					p.controller.GestureCanceled(GestureRejected)
				}
			}
		}
	}
}

func (d *EventDispatcher) deliverGestureRemainder(s *gestureSequence) {
	if s == nil || d.gesture != s || !s.active {
		return
	}
	for _, p := range slices.Clone(s.members) {
		if p.state == gestureEnded || p.delivered == s.serial {
			continue
		}
		if !s.valid(p) {
			s.cancelMember(p, GestureDetached)
			continue
		}
		p.delivered = s.serial
		ctx := &eventContext{event: s.event, current: p.widget, host: s.host, dispatcher: d, controller: p.controller}
		p.controller.HandleEvent(ctx)
		if d.gesture != s || !s.active {
			return
		}
	}
}

func pointerButtonMask(button events.PointerButton) events.PointerButtons {
	switch button {
	case events.PointerButtonLeft:
		return events.PointerButtonLeftDown
	case events.PointerButtonRight:
		return events.PointerButtonRightDown
	case events.PointerButtonMiddle:
		return events.PointerButtonMiddleDown
	case events.PointerButtonBack:
		return events.PointerButtonBackDown
	case events.PointerButtonForward:
		return events.PointerButtonForwardDown
	}
	return 0
}

func gestureMoved(a, b geometry.Point, threshold float32) bool {
	dx, dy := a.X-b.X, a.Y-b.Y
	return dx*dx+dy*dy > threshold*threshold
}

func cancelGestureSubtree(root Root, widget Widget) {
	host, ok := root.(interface{ rootState() *rootBase })
	if !ok || widget == nil {
		return
	}
	s := host.rootState().dispatcher.gesture
	if s == nil {
		return
	}
	for _, p := range slices.Clone(s.members) {
		if p.state != gestureEnded && p.widget != nil && p.widget.base().isDescendant(p.widget, widget) {
			won := s.winner == p
			s.cancelMember(p, GestureDetached)
			if won {
				s.cancel(GestureDetached)
				return
			}
		}
	}
	s.revalidate()
}

func cancelGestureController(root Root, controller EventController) {
	host, ok := root.(interface{ rootState() *rootBase })
	if !ok {
		return
	}
	s := host.rootState().dispatcher.gesture
	if s == nil {
		return
	}
	for _, p := range slices.Clone(s.members) {
		if p.state != gestureEnded && EventController(p.controller) == controller {
			won := s.winner == p
			s.cancelMember(p, GestureDetached)
			if won {
				s.cancel(GestureDetached)
				return
			}
		}
	}
	s.revalidate()
}
