package events

import (
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform/dragdrop"
)

func TestDragOfferEventResponseDoesNotChangeFacts(t *testing.T) {
	answer := dragdrop.Action(0)
	event := DragOfferEvent{
		EventType:   DragMotion,
		Position:    geometry.Point{X: 12, Y: 23},
		Actions:     dragdrop.Copy | dragdrop.Move,
		Suggested:   dragdrop.Copy,
		ActionReply: &answer,
	}
	var generic Event = event
	if generic.Type() != DragMotion {
		t.Fatalf("event type %v", generic.Type())
	}
	copy := event
	*copy.ActionReply = dragdrop.Move
	if answer != dragdrop.Move || event.Position != (geometry.Point{X: 12, Y: 23}) || event.Actions != dragdrop.Copy|dragdrop.Move {
		t.Fatal("response lost or native facts changed")
	}
}
