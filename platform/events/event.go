package events

// Event carries read-only platform facts. KeyEvent additionally permits a
// synchronous default-action response; its key data remains unchanged.
// Only event types declared by this package can implement it.
type Event interface {
	Type() EventType
	isEvent()
}

type EventType int

const (
	CloseRequest EventType = iota
	Size
	Paint
	PointerEnter
	PointerLeave
	PointerMove
	PointerDown
	PointerUp
	Wheel
	KeyDown
	KeyUp
	Focus
	State
)

type EventHandler func(event Event)
