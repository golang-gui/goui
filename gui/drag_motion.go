package gui

import (
	"math"
	"slices"
	"time"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/platform/events"
)

// DragMotionEventController observes native drag-and-drop over its Widget
// subtree, independently of DropTarget acceptance. Positions are local DIP.
// It cannot read data, accept drops or affect the negotiated action.
// Observed offers use the formats registered by the root's DropTargets; adding
// an observer does not advertise additional native data formats.
type DragMotionEventController struct {
	EventControllerBase
	owner         Widget
	active        bool
	passive       bool // built-in scrolling does not request native registration
	enter, motion signal.Signal2[geometry.Point, uint64]
	leave         signal.Signal1[uint64]
	generation    uint64
}

func NewDragMotionEventController() *DragMotionEventController {
	return &DragMotionEventController{EventControllerBase: NewEventControllerBase(PhaseBubble)}
}
func (c *DragMotionEventController) ConnectEnter(fn func(geometry.Point)) signal.Handle {
	return c.connectPoint(&c.enter, fn)
}
func (c *DragMotionEventController) ConnectMotion(fn func(geometry.Point)) signal.Handle {
	return c.connectPoint(&c.motion, fn)
}
func (c *DragMotionEventController) connectPoint(s *signal.Signal2[geometry.Point, uint64], fn func(geometry.Point)) signal.Handle {
	return s.Connect(func(p geometry.Point, generation uint64) {
		if generation == c.generation && c.owner != nil && !c.owner.base().destroyed && IsEnabled(c.owner) {
			fn(p)
		}
	})
}
func (c *DragMotionEventController) ConnectLeave(fn func()) signal.Handle {
	return c.leave.Connect(func(generation uint64) {
		if generation == c.generation {
			fn()
		}
	})
}
func (c *DragMotionEventController) Reset() {
	c.generation++
	if c.active {
		c.active = false
		c.leave.Emit(c.generation)
	}
}
func (c *DragMotionEventController) setWidget(w Widget) {
	if c.owner != w {
		c.owner = w
		c.Reset()
	}
}

// dragMotionState retains an offer only between Enter/Motion and Leave/Drop.
// The synchronous native ActionReply pointer is never retained. One root-owned
// timer arbitrates nested ScrollViews; observers do not own native resources.
type dragMotionState struct {
	event      events.DragOfferEvent
	host       EventTarget
	path       []Widget
	observers  []*DragMotionEventController
	timer      *Timer
	last       time.Time
	x, y       *ScrollView
	fraction   geometry.Point
	updating   bool
	generation uint64
}

func (b *rootBase) clearDragMotion() {
	s := &b.dragMotion
	s.generation++
	s.event = events.DragOfferEvent{}
	s.host = nil
	s.path = nil
	s.x = nil
	s.y = nil
	s.fraction = geometry.Point{}
	if s.timer != nil {
		s.timer.Stop()
	}
	old := s.observers
	s.observers = nil
	for _, c := range old {
		c.Reset()
	}
}

func (b *rootBase) clearDisabledDragMotion() {
	for _, widget := range b.dragMotion.path {
		if !IsEnabled(widget) {
			b.clearDragMotion()
			return
		}
	}
}

func (b *rootBase) updateDragMotion(host EventTarget, e events.DragOfferEvent) {
	s := &b.dragMotion
	if s.updating {
		return
	}
	s.updating = true
	defer func() { s.updating = false }()
	if s.event.Offer != nil && s.event.Offer.ID() != e.Offer.ID() {
		b.clearDragMotion()
	}
	e.ActionReply = nil
	s.event, s.host = e, host
	generation := s.generation
	root := host.Widget()
	if root == nil || root.base().destroyed {
		b.clearDragMotion()
		return
	}
	target := b.dispatcher.pick(root, e.Position)
	if target != nil && !IsEnabled(target) {
		b.clearDragMotion()
		return
	}
	path := widgetPath(b.dispatcher.treeRoot(root, target), target)
	var observers []*DragMotionEventController
	for _, w := range path {
		for _, controller := range w.EventControllers() {
			if c, ok := controller.(*DragMotionEventController); ok && c.owner == w {
				observers = append(observers, c)
			}
		}
	}
	old := s.observers
	s.path, s.observers = path, observers
	for i := len(old) - 1; i >= 0; i-- {
		if !slices.Contains(observers, old[i]) {
			old[i].Reset()
		}
	}
	if generation != s.generation {
		return
	}
	for _, c := range observers {
		owner := c.owner
		if owner == nil || owner.base().destroyed || !IsEnabled(owner) || owner.Root() != root.Root() {
			continue
		}
		point := widgetLocalPoint(owner, e.Position)
		if !c.active {
			c.active = true
			c.enter.Emit(point, c.generation)
		} else {
			c.motion.Emit(point, c.generation)
		}
		if generation != s.generation || root.base().destroyed {
			return
		}
	}
	b.syncDragScroll()
}

// refreshDragMotion reevaluates a stationary pointer after layout/scroll. It
// uses the last still-live offer, never dispatches synthetic platform input.
func (b *rootBase) refreshDragMotion(host EventTarget) {
	s := &b.dragMotion
	if s.event.Offer == nil || s.updating || s.host != host || b.dragTarget.reading {
		return
	}
	e := s.event
	generation := s.generation
	b.updateDragMotion(host, e)
	if s.event.Offer != nil && s.event.Offer.ID() == e.Offer.ID() && generation == s.generation && !s.updating {
		b.negotiateDragTarget(host, e)
	}
}

func dragScrollVelocity(position, extent float32) float32 {
	if extent <= 0 || position < 0 || position >= extent {
		return 0
	}
	zone := min(float32(32), extent/2)
	if position < zone {
		return -480 * (zone - position) / zone
	}
	if position > extent-zone {
		return 480 * (position - (extent - zone)) / zone
	}
	return 0
}

func (b *rootBase) dragScrollAxes() (x, y *ScrollView, vx, vy float32) {
	s := &b.dragMotion
	for i := len(s.path) - 1; i >= 0; i-- {
		sv, ok := s.path[i].(*ScrollView)
		if !ok || !sv.dragAutoScroll || sv.destroyed || !IsEnabled(sv) || sv.Root() == nil {
			continue
		}
		p := widgetLocalPoint(sv, s.event.Position)
		area := sv.contentAreaRect()
		if !containsPoint(area, p) {
			continue
		}
		dx, dy := dragScrollVelocity(p.X-area.X, area.Width), dragScrollVelocity(p.Y-area.Y, area.Height)
		if x == nil && (dx < 0 && sv.scrollX > 0 || dx > 0 && sv.scrollX < float32(math.Floor(float64(max(0, sv.contentWidth-area.Width))))) {
			x, vx = sv, dx
		}
		if y == nil && (dy < 0 && sv.scrollY > 0 || dy > 0 && sv.scrollY < float32(math.Floor(float64(max(0, sv.contentHeight-area.Height))))) {
			y, vy = sv, dy
		}
	}
	return
}

func (b *rootBase) syncDragScroll() {
	s := &b.dragMotion
	x, y, _, _ := b.dragScrollAxes()
	if x != s.x {
		s.fraction.X = 0
	}
	if y != s.y {
		s.fraction.Y = 0
	}
	s.x, s.y = x, y
	if x == nil && y == nil {
		if s.timer != nil {
			s.timer.Stop()
		}
		return
	}
	if b.app == nil || b.app.timers == nil {
		return
	}
	if s.timer == nil {
		s.timer = b.app.NewTimer()
		s.timer.ConnectTimeout(func() { b.advanceDragScroll(time.Now()) })
	}
	if !s.timer.Active() {
		s.last = time.Now()
		_ = s.timer.Start(16 * time.Millisecond)
	}
}

func (b *rootBase) advanceDragScroll(now time.Time) {
	s := &b.dragMotion
	if s.event.Offer == nil || s.host == nil {
		b.clearDragMotion()
		return
	}
	x, y, vx, vy := b.dragScrollAxes()
	seconds := float32(min(now.Sub(s.last), 100*time.Millisecond).Seconds())
	s.last = now
	if seconds <= 0 {
		return
	}
	if x != s.x {
		s.fraction.X = 0
	}
	if y != s.y {
		s.fraction.Y = 0
	}
	s.x, s.y = x, y
	if x != nil {
		s.fraction.X += vx * seconds
		old := x.ScrollX()
		x.SetScrollX(old + s.fraction.X)
		s.fraction.X -= x.ScrollX() - old
	}
	if s.event.Offer == nil {
		return
	}
	if y != nil && !y.destroyed {
		s.fraction.Y += vy * seconds
		old := y.ScrollY()
		y.SetScrollY(old + s.fraction.Y)
		s.fraction.Y -= y.ScrollY() - old
	}
	if s.host != nil {
		b.refreshDragMotion(s.host)
	}
	b.syncDragScroll()
}
