package ui

import (
	"slices"

	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/gui"
)

type DragAction = gui.DragAction

const (
	DragCopy DragAction = gui.DragCopy
	DragMove DragAction = gui.DragMove
	DragLink DragAction = gui.DragLink
)

type DragFormat = gui.DragFormat

const (
	DragFormatText  DragFormat = gui.DragFormatText
	DragFormatFiles DragFormat = gui.DragFormatFiles
	DragFormatURLs  DragFormat = gui.DragFormatURLs
)

func MIMEFormat(mediaType string) DragFormat { return gui.MIMEFormat(mediaType) }
func LocalFormat(name string) DragFormat     { return gui.LocalFormat(name) }

type DragData = gui.DragData
type DragPrepare = gui.DragPrepare
type DragResult = gui.DragResult
type DragMotion = gui.DragMotion
type DropRequest = gui.DropRequest

// DragSourceView declares one optional drag source controller on a widget.
// Rebuilding the descriptor only updates its controller; it does not start a drag.
type DragSourceView struct {
	enabled bool
	actions DragAction
	data    *DragData
	prepare func(*DragPrepare)
	begin   func()
	end     func(DragResult)
}

func DragSource() *DragSourceView { return &DragSourceView{enabled: true, actions: DragCopy} }

// Data supplies the initial data for each Prepare. OnPrepare can add a preview,
// replace Data, or set it to nil to decline this drag. Data is frozen by GUI
// after Prepare; rebuilding the view affects only subsequent drags.
func (v *DragSourceView) Data(data *DragData) *DragSourceView { v.data = data; return v }

func (v *DragSourceView) Enabled(value bool) *DragSourceView              { v.enabled = value; return v }
func (v *DragSourceView) Actions(value DragAction) *DragSourceView        { v.actions = value; return v }
func (v *DragSourceView) OnPrepare(fn func(*DragPrepare)) *DragSourceView { v.prepare = fn; return v }
func (v *DragSourceView) OnBegin(fn func()) *DragSourceView               { v.begin = fn; return v }
func (v *DragSourceView) OnEnd(fn func(DragResult)) *DragSourceView       { v.end = fn; return v }

func (v DragSourceView) prepareRequest(request *DragPrepare) {
	request.Data = v.data
	if v.prepare != nil {
		v.prepare(request)
	}
}

func (b *ViewBase[T]) DragSource(source *DragSourceView) *T {
	b.dragSource = source
	return b.self()
}

// DropTargetView declares one optional drop target. Formats are tried in order.
type DropTargetView struct {
	enabled bool
	actions DragAction
	formats []DragFormat
	enter   func(*DragMotion)
	motion  func(*DragMotion)
	leave   func()
	drop    func(*DropRequest)
	onError func(error)
}

func DropTarget(formats ...DragFormat) *DropTargetView {
	return &DropTargetView{enabled: true, actions: DragCopy, formats: slices.Clone(formats)}
}

func (v *DropTargetView) Enabled(value bool) *DropTargetView       { v.enabled = value; return v }
func (v *DropTargetView) Actions(value DragAction) *DropTargetView { v.actions = value; return v }
func (v *DropTargetView) Formats(formats ...DragFormat) *DropTargetView {
	v.formats = slices.Clone(formats)
	return v
}
func (v *DropTargetView) OnEnter(fn func(*DragMotion)) *DropTargetView  { v.enter = fn; return v }
func (v *DropTargetView) OnMotion(fn func(*DragMotion)) *DropTargetView { v.motion = fn; return v }
func (v *DropTargetView) OnLeave(fn func()) *DropTargetView             { v.leave = fn; return v }
func (v *DropTargetView) OnDrop(fn func(*DropRequest)) *DropTargetView  { v.drop = fn; return v }
func (v *DropTargetView) OnError(fn func(error)) *DropTargetView        { v.onError = fn; return v }

func (b *ViewBase[T]) DropTarget(target *DropTargetView) *T {
	b.dropTarget = target
	return b.self()
}

type dragSourceBinding struct {
	controller *gui.DragSource
	handles    []signal.Handle
	callbacks  DragSourceView
	generation uint64
}

func (b *dragSourceBinding) update(w gui.Widget, view *DragSourceView) {
	if view == nil {
		b.clear(w)
		return
	}
	b.generation++
	generation := b.generation
	if b.controller == nil {
		b.controller = gui.NewDragSource()
		b.callbacks = *view
		b.handles = []signal.Handle{
			b.controller.ConnectPrepare(func(e *gui.DragPrepare) {
				current := b.callbacks
				current.prepareRequest(e)
			}),
			b.controller.ConnectBegin(func() {
				if b.callbacks.begin != nil {
					b.callbacks.begin()
				}
			}),
			b.controller.ConnectEnd(func(e gui.DragResult) {
				if b.callbacks.end != nil {
					b.callbacks.end(e)
				}
			}),
		}
		b.controller.SetActions(view.actions)
		b.controller.SetEnabled(view.enabled)
		w.AddEventController(b.controller)
		return
	}
	b.callbacks = *view
	controller := b.controller
	controller.SetActions(view.actions)
	if b.generation == generation {
		controller.SetEnabled(view.enabled)
	}
}

func (b *dragSourceBinding) clear(w gui.Widget) {
	b.generation++
	controller, handles := b.controller, b.handles
	b.controller = nil
	b.handles = nil
	b.callbacks = DragSourceView{}
	if controller != nil {
		for _, handle := range handles {
			handle.Disconnect()
		}
		w.RemoveEventController(controller)
	}
}

type dropTargetBinding struct {
	controller *gui.DropTarget
	handles    []signal.Handle
	callbacks  DropTargetView
	generation uint64
}

func (b *dropTargetBinding) update(w gui.Widget, view *DropTargetView) {
	if view == nil {
		b.clear(w)
		return
	}
	b.generation++
	generation := b.generation
	if b.controller == nil {
		b.controller = gui.NewDropTarget()
		b.callbacks = *view
		b.handles = []signal.Handle{
			b.controller.ConnectEnter(func(e *gui.DragMotion) {
				if b.callbacks.enter != nil {
					b.callbacks.enter(e)
				}
			}),
			b.controller.ConnectMotion(func(e *gui.DragMotion) {
				if b.callbacks.motion != nil {
					b.callbacks.motion(e)
				}
			}),
			b.controller.ConnectLeave(func() {
				if b.callbacks.leave != nil {
					b.callbacks.leave()
				}
			}),
			b.controller.ConnectDrop(func(e *gui.DropRequest) {
				if b.callbacks.drop != nil {
					b.callbacks.drop(e)
				}
			}),
			b.controller.ConnectError(func(e error) {
				if b.callbacks.onError != nil {
					b.callbacks.onError(e)
				}
			}),
		}
		b.controller.SetFormats(view.formats...)
		b.controller.SetActions(view.actions)
		b.controller.SetEnabled(view.enabled)
		w.AddEventController(b.controller)
		return
	}
	b.callbacks = *view
	controller := b.controller
	controller.SetFormats(view.formats...)
	if b.generation != generation {
		return
	}
	controller.SetActions(view.actions)
	if b.generation == generation {
		controller.SetEnabled(view.enabled)
	}
}

func (b *dropTargetBinding) clear(w gui.Widget) {
	b.generation++
	controller, handles := b.controller, b.handles
	b.controller = nil
	b.handles = nil
	b.callbacks = DropTargetView{}
	if controller != nil {
		for _, handle := range handles {
			handle.Disconnect()
		}
		w.RemoveEventController(controller)
	}
}
