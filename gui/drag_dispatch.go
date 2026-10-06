package gui

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/platform"
	"github.com/golang-gui/goui/platform/dragdrop"
	"github.com/golang-gui/goui/platform/events"
)

type dragHostState struct {
	native     platform.DragDrop
	err        error
	attempted  bool
	updating   bool
	registered []dragdrop.Format
}

func (s *dragHostState) destroy() {
	if s.native != nil {
		s.native.Destroy()
		s.native = nil
	}
	s.err = nil
	s.attempted = false
	s.registered = nil
}

type dragControllerHost interface {
	dragControllersChanged()
}

func notifyDragControllersChanged(root Root) {
	if host, ok := root.(dragControllerHost); ok {
		host.dragControllersChanged()
	}
}

func dragAppOf(root Root) *application {
	if h, ok := root.(interface{ rootState() *rootBase }); ok {
		return h.rootState().app
	}
	return nil
}

func dragControllers(root Widget) (hasController bool, formats []dragdrop.Format, targets []*DropTarget) {
	var walk func(Widget)
	walk = func(w Widget) {
		if w == nil || w.base().destroyed || !IsEnabled(w) {
			return
		}
		for _, controller := range w.EventControllers() {
			switch c := controller.(type) {
			case *DragSource:
				if c.enabled {
					hasController = true
				}
			case *DropTarget:
				if !c.enabled {
					continue
				}
				hasController = true
				targets = append(targets, c)
				for _, format := range c.formats {
					portable := format
					if !validDragFormat(format) {
						continue
					}
					if len(format) >= 6 && string(format[:6]) == "local:" {
						portable = dragdrop.FormatLocalMarker
					}
					if !slices.Contains(formats, portable) {
						formats = append(formats, portable)
					}
				}
			case *DragMotionEventController:
				if !c.passive {
					hasController = true
				}
			}
		}
		for _, child := range w.Children() {
			walk(child)
		}
	}
	walk(root)
	return
}

func updateDragHost(s *dragHostState, app *application, surface platform.Surface, root Widget) {
	if app == nil || app.platform == nil || surface == nil {
		return
	}
	if s.updating {
		return
	}
	s.updating = true
	defer func() { s.updating = false }()
	needed, formats, targets := dragControllers(root)
	if !needed && s.native == nil {
		return
	}
	previousErr := s.err
	if s.native == nil && needed && !s.attempted {
		s.attempted = true
		s.native, s.err = app.platform.NewDragDrop(surface)
	}
	if s.native != nil && !slices.Equal(s.registered, formats) {
		s.err = s.native.SetFormats(formats)
		if s.err == nil {
			s.registered = slices.Clone(formats)
		}
	}
	if s.err != nil && needed && (previousErr == nil || previousErr.Error() != s.err.Error()) {
		for _, target := range targets {
			if target.owner != nil && !target.owner.base().destroyed && target.owner.Root() != nil {
				target.error.Emit(s.err)
			}
		}
	}
}

func (w *window) dragControllersChanged() {
	if !w.destroyed {
		if w.drag.native == nil && w.drag.err != nil {
			w.drag.attempted = false
		}
		updateDragHost(&w.drag, w.app, w.surface, w.Widget())
	}
}

func (p *popover) dragControllersChanged() {
	if p.destroyed {
		return
	}
	if p.drag.native == nil && p.drag.err != nil {
		p.drag.attempted = false
	}
	app := dragAppOf(p)
	updateDragHost(&p.drag, app, p.surface, p.Widget())
}

func dragCapability(root Root) (platform.DragDrop, error) {
	if h, ok := root.(interface{ rootState() *rootBase }); ok {
		base := h.rootState()
		// Manual handoff need not register a second pointer controller just to
		// create the native capability. This is still owned by the same root.
		if base.drag.native == nil && base.app != nil && base.app.platform != nil && base.surface != nil {
			base.drag.attempted = true
			base.drag.native, base.drag.err = base.app.platform.NewDragDrop(base.surface)
		}
		updateDragHost(&base.drag, base.app, base.surface, root.Widget())
		if base.drag.native != nil {
			return base.drag.native, nil
		}
		if base.drag.err != nil {
			return nil, base.drag.err
		}
	}
	return nil, errors.New("gui: drag capability unavailable")
}

// guiDragSession owns one local source across all application hosts. It is
// installed before native Begin, whose callbacks may synchronously reenter GUI.
type guiDragSession struct {
	app           *application
	id            uint64
	source        *DragSource
	widget        Widget
	host          Root
	data          *DragData
	native        platform.DragDrop
	began         bool
	ending        bool
	delivering    bool
	pendingCancel bool
	committed     DragAction // synchronous local Drop accepted by the application
	localDrop     bool       // observed delivery, not inferred from hover/Action
	pendingEnd    *DragResult
	unmount       signal.Handle
}

func (a *application) startDrag(host Root, widget Widget, source *DragSource, data *DragData, preview DragPreview, actions DragAction) error {
	if a.dragSession != nil {
		return fmt.Errorf("gui: another drag session is active")
	}
	native, err := dragCapability(host)
	if err != nil {
		return err
	}
	a.nextDragID++
	if a.nextDragID == 0 {
		a.nextDragID++
	}
	run := &guiDragSession{app: a, id: a.nextDragID, source: source, widget: widget,
		host: host, data: data, native: native}
	a.dragSession = run
	source.run = run
	run.unmount = widget.ConnectUnmount(run.cancel)
	err = native.Begin(run.id, &data.portable, actions, dragdrop.Preview{
		Image: preview.Image, Scale: preview.Scale, Hotspot: preview.Hotspot,
	}, dragdrop.Feedback{NeutralOutsideTargets: source.feedback.NeutralOutsideTargets, DisableReturnAnimation: source.feedback.DisableReturnAnimation})
	if err != nil {
		if a.dragSession != run {
			// Begin may synchronously deliver the terminal result. That result
			// has already notified the source and must not be emitted again.
			return nil
		}
		a.dragSession = nil
		source.run = nil
		run.unmount.Disconnect()
		run.data = nil
	}
	return err
}

func (s *guiDragSession) cancel() {
	if s == nil || s.app.dragSession != s || s.ending || s.committed != 0 {
		return
	}
	if s.delivering {
		s.pendingCancel = true
		return
	}
	s.native.Cancel()
}

func (s *guiDragSession) end(result DragResult) {
	if s == nil || s.app.dragSession != s || s.ending {
		return
	}
	if s.delivering {
		// Native callbacks can reenter while a Drop handler unmounts its source.
		// Wait for that synchronous handler to decide whether it committed.
		if s.pendingEnd == nil {
			s.pendingEnd = &result
		}
		return
	}
	if s.committed != 0 {
		result.Action, result.Canceled, result.Err = s.committed, false, nil
	}
	result.LocalDrop = s.localDrop
	s.ending = true
	s.app.dragSession = nil
	s.source.run = nil
	if s.unmount != nil {
		s.unmount.Disconnect()
		s.unmount = nil
	}
	s.source.dragging = false
	if s.widget != nil && !s.widget.base().destroyed {
		s.widget.base().requestSemanticUpdate()
	}
	s.source.end.Emit(result)
	s.data = nil
}

func (a *application) dispatchDragSourceEvent(event events.DragSourceEvent) {
	run := a.dragSession
	if run == nil || run.id != event.ID || run.ending {
		return
	}
	switch event.EventType {
	case events.DragSourceBegin:
		if run.began {
			return
		}
		run.began = true
		run.source.dragging = true
		if run.widget != nil && !run.widget.base().destroyed {
			run.widget.base().requestSemanticUpdate()
		}
		run.source.begin.Emit()
	case events.DragSourceEnd:
		r := event.Result
		run.end(DragResult{Action: r.Action, Canceled: r.Canceled, Err: r.Err,
			Position: r.Position, PositionValid: r.PositionValid, LocalTarget: r.LocalTarget})
	}
}

type dragTargetState struct {
	offer   dragdrop.Offer
	target  *DropTarget
	format  DragFormat
	action  DragAction
	point   geometry.Point // host-client DIP, locked at Drop
	reading bool
}

func (b *rootBase) dispatchDragOffer(host EventTarget, e events.DragOfferEvent) {
	if e.Offer == nil {
		return
	}
	switch e.EventType {
	case events.DragLeave:
		if b.dragMotion.event.Offer != nil && b.dragMotion.event.Offer.ID() == e.Offer.ID() {
			b.clearDragMotion()
		}
		if b.dragTarget.offer != nil && b.dragTarget.offer.ID() == e.Offer.ID() && !b.dragTarget.reading {
			b.leaveDragTarget()
		}
	case events.DragEnter, events.DragMotion:
		b.updateDragMotion(host, e)
		action := b.negotiateDragTarget(host, e)
		if e.ActionReply != nil {
			*e.ActionReply = action
		}
	case events.DragDrop:
		b.clearDragMotion()
		// Record before negotiation, Read, or callbacks: rejection and native
		// synchronous End must not erase the fact that a local Drop occurred.
		if app := dragAppOf(host.(Root)); app != nil && app.dragSession != nil && app.dragSession.id == e.Offer.SourceID() {
			app.dragSession.localDrop = true
		}
		action := b.negotiateDragTarget(host, e)
		if action == 0 || b.dragTarget.target == nil {
			b.leaveDragTarget()
			_ = e.Offer.Finish(0)
			return
		}
		b.dragTarget.point, b.dragTarget.reading = e.Position, true
		format := b.dragTarget.format
		if strings.HasPrefix(string(format), "local:") {
			app := dragAppOf(host.(Root))
			if app == nil || app.dragSession == nil || app.dragSession.id != e.Offer.SourceID() {
				b.finishDragDataError(nil)
				return
			}
			value := app.dragSession.data.selected(format)
			if receiver, ok := host.(interface{ DispatchEvent(events.Event) error }); ok {
				_ = receiver.DispatchEvent(events.DragDataEvent{OfferID: e.Offer.ID(), Format: format,
					Data: &value.portable})
			} else {
				b.finishDragDataError(nil)
			}
			return
		}
		if err := e.Offer.Read(format); err != nil {
			b.finishDragDataError(err)
		}
	}
}

func (b *rootBase) negotiateDragTarget(host EventTarget, e events.DragOfferEvent) DragAction {
	root := host.Widget()
	pathTarget := b.dispatcher.pick(root, e.Position)
	if pathTarget != nil && !IsEnabled(pathTarget) {
		b.leaveDragTarget()
		return 0
	}
	path := widgetPath(b.dispatcher.treeRoot(root, pathTarget), pathTarget)
	if len(path) == 0 {
		b.leaveDragTarget()
		return 0
	}
	formats := e.Offer.Formats()
	app := dragAppOf(host.(Root))
	if app != nil && app.dragSession != nil && app.dragSession.id == e.Offer.SourceID() {
		for _, format := range app.dragSession.data.formats() {
			if strings.HasPrefix(string(format), "local:") {
				formats = append(formats, format)
			}
		}
	}
	for i := len(path) - 1; i >= 0; i-- {
		w := path[i]
		for _, controller := range w.EventControllers() {
			t, ok := controller.(*DropTarget)
			if !ok || !IsEnabled(w) || !t.enabled || t.owner != w || t.actions == 0 || !t.actions.ValidSet() {
				continue
			}
			allowed := t.actions & e.Actions
			if allowed == 0 || e.Forced && allowed&e.Suggested == 0 {
				continue
			}
			var chosen DragFormat
			for _, format := range t.formats {
				if slices.Contains(formats, format) {
					chosen = format
					break
				}
			}
			if chosen == "" {
				continue
			}
			if b.dragTarget.offer != nil && (b.dragTarget.offer.ID() != e.Offer.ID() || b.dragTarget.target != t || b.dragTarget.format != chosen) {
				b.leaveDragTarget()
			}
			initial := b.dragTarget.target != t
			action := preferredDragAction(allowed, e.Suggested)
			request := &DragMotion{Position: widgetLocalPoint(w, e.Position), Format: chosen, Allowed: allowed, Action: action}
			if app != nil && app.dragSession != nil && app.dragSession.id == e.Offer.SourceID() && strings.HasPrefix(string(chosen), "local:") {
				request.local = app.dragSession.data
			}
			if initial {
				t.active = true
				w.base().requestSemanticUpdate()
				b.dragTarget = dragTargetState{offer: e.Offer, target: t, format: chosen}
				t.enter.Emit(request)
			} else {
				t.motion.Emit(request)
			}
			request.local = nil
			if t.owner != w || w.base().destroyed || !IsEnabled(w) || w.Root() != host.(Root) || !t.enabled ||
				!request.Action.ValidResult() || request.Action&allowed == 0 {
				b.leaveDragTarget()
				continue
			}
			b.dragTarget.action = request.Action
			return request.Action
		}
	}
	b.leaveDragTarget()
	return 0
}

func preferredDragAction(allowed, suggested DragAction) DragAction {
	if suggested.ValidResult() && suggested != 0 && suggested&allowed != 0 {
		return suggested
	}
	for _, action := range []DragAction{DragCopy, DragMove, DragLink} {
		if allowed&action != 0 {
			return action
		}
	}
	return 0
}

func (b *rootBase) dispatchDragData(host EventTarget, e events.DragDataEvent) {
	state := b.dragTarget
	if !state.reading || state.offer == nil || state.offer.ID() != e.OfferID || state.format != e.Format {
		return
	}
	if e.Err != nil || e.Data == nil {
		b.finishDragDataError(e.Err)
		return
	}
	var data *DragData
	if strings.HasPrefix(string(state.format), "local:") {
		app := dragAppOf(host.(Root))
		if app == nil || app.dragSession == nil || app.dragSession.id != state.offer.SourceID() ||
			!app.dragSession.data.contains(state.format) {
			b.finishDragDataError(nil)
			return
		}
		data = app.dragSession.data.selected(state.format)
	} else {
		data = &DragData{portable: *e.Data.Clone()}
	}
	target := state.target
	if target == nil || target.owner == nil || target.owner.base().destroyed ||
		target.owner.Root() != host.(Root) || !IsEnabled(target.owner) || !target.enabled || !slices.Contains(target.formats, state.format) ||
		target.actions&state.action == 0 {
		b.finishDragDataError(nil)
		return
	}
	w := target.owner
	request := &DropRequest{Position: widgetLocalPoint(w, state.point), Format: state.format,
		Action: state.action, Data: data}
	app := dragAppOf(host.(Root))
	var local *guiDragSession
	if app != nil && app.dragSession != nil && app.dragSession.id == state.offer.SourceID() {
		local = app.dragSession
		local.delivering = true
	}
	target.drop.Emit(request)
	action := DragAction(0)
	if request.Accepted {
		action = state.action
	}
	if local != nil {
		local.committed = action
		// Keep delivering set during Leave and Finish: both can call back
		// synchronously, and End must see the already committed action.
	}
	b.leaveDragTarget()
	_ = state.offer.Finish(action)
	if local != nil {
		local.delivering = false
		if result := local.pendingEnd; result != nil {
			local.pendingEnd = nil
			local.end(*result)
		} else if local.pendingCancel && action == 0 {
			local.cancel()
		}
	}
}

func (b *rootBase) finishDragDataError(err error) {
	state := b.dragTarget
	if state.target != nil && err != nil {
		state.target.error.Emit(err)
	}
	b.leaveDragTarget()
	if state.offer != nil {
		_ = state.offer.Finish(0)
	}
}

func (b *rootBase) leaveDragTarget() {
	target := b.dragTarget.target
	b.dragTarget = dragTargetState{}
	if target != nil && target.active {
		target.active = false
		if target.owner != nil && !target.owner.base().destroyed {
			target.owner.base().requestSemanticUpdate()
		}
		target.leave.Emit()
	}
}
