package events

import (
	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform/dragdrop"
)

// DragSourceEvent reports native session start or its one terminal result.
// ID is the nonzero identifier supplied to DragDrop.Begin.
type DragSourceEvent struct {
	EventType EventType
	ID        uint64
	Result    dragdrop.Result // meaningful only for DragSourceEnd
}

func (e DragSourceEvent) Type() EventType { return e.EventType }
func (e DragSourceEvent) isEvent()        {}

// DragOfferEvent reports the native target-side interaction. Position is in
// the receiving surface's client DIP. ActionReply is meaningful for Enter and
// Motion only; the reply slot is valid only during synchronous dispatch.
// Suggested is one native recommendation, not an action capability mask.
// Forced is true only when the backend can observe an explicit user action
// request that must not silently fall back to another action.
type DragOfferEvent struct {
	EventType   EventType
	Position    geometry.Point
	Offer       dragdrop.Offer
	Actions     dragdrop.Action
	Suggested   dragdrop.Action
	Forced      bool
	Modifiers   Modifiers
	ActionReply *dragdrop.Action
}

func (e DragOfferEvent) Type() EventType { return e.EventType }
func (e DragOfferEvent) isEvent()        {}

// DragDataEvent completes one requested offer read. Data contains only the
// selected portable representation; Err means no Drop should be delivered.
type DragDataEvent struct {
	OfferID uint64
	Format  dragdrop.Format
	Data    *dragdrop.Data
	Err     error
}

func (e DragDataEvent) Type() EventType { return DragData }
func (e DragDataEvent) isEvent()        {}
