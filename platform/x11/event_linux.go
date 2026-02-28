package x11

import "github.com/jezek/xgb"

type EventQueue struct {
	emptyChan chan bool
	eventChan <-chan any
}

func newEventQueue() (q EventQueue, err error) {
	q.eventChan = platform.getEventChan()
	q.emptyChan = make(chan bool, 5000)
	return
}

func (q EventQueue) Destroy() {

}

func (q EventQueue) Post() {
	q.emptyChan <- true
}

func (q EventQueue) Poll() {
	select {
	case everr := <-q.eventChan:
		q.processEventOrError(everr)
	case <-q.emptyChan:
	default:
	}
}

func (q EventQueue) Wait() {
	select {
	case everr := <-q.eventChan:
		q.processEventOrError(everr)
	case <-q.emptyChan:
	}
}

func (q EventQueue) processEventOrError(everr any) {
	switch everr.(type) {
	case xgb.Event:

	case xgb.Error:

	case error:
		// c.conn read error
	case nil:
		// c.eventChan is closeds
	}
}
