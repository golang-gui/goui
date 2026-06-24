package events

// Event is a read-only platform notification. Only event types declared by
// this package can implement it.
type Event interface {
	Type() EventType
	isEvent()
}

type EventType int

const (
	CloseRequest EventType = iota
	Size
	Paint
	Scale
	PointerEnter
	PointerLeave
	PointerMove
	PointerDown
	PointerUp
)

type EventHandler func(event Event)
