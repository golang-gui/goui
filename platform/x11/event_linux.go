package x11

import (
	"errors"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/x11/libs/libc"
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
	pipe  [2]int32
	event int32
	empty [4]byte
	dummy [32]byte
}

func newEventQueue() (q *EventQueue, err error) {
	q = new(EventQueue)
	err = libc.Pipe(&q.pipe)
	if err != nil {
		return nil, err
	}

	for _, fd := range q.pipe {
		sf := libc.Fcntl(fd, libc.F_GETFL, 0)
		df := libc.Fcntl(fd, libc.F_GETFD, 0)

		if sf != -1 && df != -1 {
			if libc.Fcntl(fd, libc.F_SETFL, uintptr(sf)|libc.O_NONBLOCK) == -1 ||
				libc.Fcntl(fd, libc.F_SETFD, uintptr(df)|libc.FD_CLOEXEC) == -1 {
				return nil, errors.New("set flags for empty event pipe failed")
			}
		}
	}
	q.event = platform.display.ConnectionNumber()
	return
}

func (q *EventQueue) Destroy() {
	libc.Close(q.pipe[0])
	libc.Close(q.pipe[1])
}

func (q *EventQueue) Post() {
	for {
		result, errno := libc.Write(q.pipe[1], q.empty[:1])
		if result == 1 || (result == -1 && errno != libc.EINTR) {
			break
		}
	}
}

func (q *EventQueue) Poll() {
	q.readEmpty()

	platform.display.Pending()
	if platform.display.QLength() != 0 {
		q.getEvent()
	}
	platform.display.Flush()
}

func (q *EventQueue) Wait() {
	q.pollRead(-1)
	q.Poll()
}

func (q *EventQueue) readEmpty() {
	var dummy [64]byte
	for {
		result, errno := libc.Read(q.pipe[0], dummy[:])
		if result == -1 && errno != libc.EINTR {
			break
		}
	}
}

func (q *EventQueue) getEvent() {
	event := platform.display.NextEvent()
	if event.AnyEvent().Window != platform.helper {
		handleEvent(event)
	}
}

func (q *EventQueue) pollRead(timeout int) {
	fds := []libc.PollFd{
		{
			Fd:     q.event,
			Events: libc.POLLIN,
		},
		{
			Fd:     q.pipe[0],
			Events: libc.POLLIN,
		},
	}

	for platform.display.Pending() == 0 {
		ret, errno := libc.Poll(fds, timeout)
		if ret == -1 && errno != libc.EINTR && errno != libc.EAGAIN {
			return
		}
		for _, pfd := range fds {
			if pfd.REvents&libc.POLLIN != 0 {
				return
			}
		}
	}
}
