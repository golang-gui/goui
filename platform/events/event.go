package events

// Event carries read-only platform facts. KeyEvent permits a synchronous
// default-action response, and native drag offer events permit a synchronous
// action response; their fact fields remain unchanged.
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
	DragSourceBegin
	DragSourceEnd
	DragEnter
	DragMotion
	DragLeave
	DragDrop
	DragData
)

type EventHandler func(event Event)
