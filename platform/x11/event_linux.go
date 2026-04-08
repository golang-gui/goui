package x11

import (
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/x11/libs/xlib"
)

type Event struct {
	events.EventBase
	Event xlib.Event
}

func (e *Event) Type() events.EventType {
	return events.Native
}

type EventQueue struct {
	syncChan  chan bool
	emptyChan chan bool
	eventChan chan xlib.Event
	wake      xlib.Event
}

func newEventQueue() (q *EventQueue, err error) {
	q = new(EventQueue)
	q.syncChan = make(chan bool, 1)
	q.emptyChan = make(chan bool, 1024)
	q.eventChan = make(chan xlib.Event, 1024)

	event := q.wake.ClientMessageEvent()
	event.Type = xlib.ClientMessage
	event.Window = platform.helper
	event.MessageType = platform.display.InternAtom("GOUI_WAKEUP", false)
	event.Format = 32
	event.L[0] = 0

	q.syncChan <- true
	go q.doGetEvent()
	return
}

func (q *EventQueue) Destroy() {
	close(q.syncChan)
	platform.display.SendEvent(platform.helper, false, 0, &q.wake)
}

func (q *EventQueue) Post() {
	select {
	case q.emptyChan <- true:
	default:
	}
}

func (q *EventQueue) Poll() {
	select {
	case event, ok := <-q.eventChan:
		if ok {
			if event.AnyEvent().Window != platform.helper {
				handleEvent(event)
			}
			q.syncChan <- true
		}
	case <-q.emptyChan:
	default:
	}
}

func (q *EventQueue) Wait() {
	select {
	case event, ok := <-q.eventChan:
		if ok {
			if event.AnyEvent().Window != platform.helper {
				handleEvent(event)
			}
			q.syncChan <- true
		}
	case <-q.emptyChan:
	}
}

func (q *EventQueue) getEvent() {
	event := platform.display.NextEvent()
	if event.AnyEvent().Window != platform.helper {
		handleEvent(event)
	}
}

func (q *EventQueue) doGetEvent() {
	defer close(q.eventChan)
	for range q.syncChan {
		event := platform.display.NextEvent()
		q.eventChan <- event
	}
}
