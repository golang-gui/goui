package events

type Event interface {
	Type() EventType
	Accept()
	Ignore()
	Accepted() (b bool)
	SetAccepted(b bool)
}

type EventType int

const (
	Native EventType = iota
	Close
	Size
	Paint
	Scale
)

type EventHandler func(event Event)

type EventBase struct {
	accepted bool
}

func (e *EventBase) Accept() {
	e.accepted = true
}

func (e *EventBase) Ignore() {
	e.accepted = false
}

func (e *EventBase) Accepted() bool {
	return e.accepted
}

func (e *EventBase) SetAccepted(v bool) {
	e.accepted = v
}
