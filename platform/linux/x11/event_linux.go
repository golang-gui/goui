package x11

import (
	"errors"

	"github.com/golang-gui/goui/platform/internal/eventloop"
	"github.com/golang-gui/goui/platform/linux/libs/libc"
)

type EventLoop struct {
	state eventloop.State
	pipe  [2]int32
	event int32
	empty [4]byte
}

func newEventLoop() (l *EventLoop, err error) {
	l = new(EventLoop)
	err = libc.Pipe(&l.pipe)
	if err != nil {
		return nil, err
	}

	for _, fd := range l.pipe {
		sf := libc.Fcntl(fd, libc.F_GETFL, 0)
		df := libc.Fcntl(fd, libc.F_GETFD, 0)

		if sf != -1 && df != -1 {
			if libc.Fcntl(fd, libc.F_SETFL, uintptr(sf)|libc.O_NONBLOCK) == -1 ||
				libc.Fcntl(fd, libc.F_SETFD, uintptr(df)|libc.FD_CLOEXEC) == -1 {
				l.closePipe()
				return nil, errors.New("set flags for empty event pipe failed")
			}
		}
	}
	l.event = platform.display.ConnectionNumber()
	return
}

func (l *EventLoop) Post(task func()) {
	if l.state.Post(task) {
		l.wake()
	}
}

func (l *EventLoop) Run() {
	defer l.state.Quit()
	if l.state.Destroyed() {
		return
	}

	for !l.state.Quitting() {
		if !l.wait() {
			return
		}
		l.readWake()
		eventloop.RunTasks(&l.state)
		if l.state.Quitting() {
			return
		}
		l.processEvents()
	}
}

func (l *EventLoop) Quit() {
	if l.state.Quit() {
		l.wake()
	}
}

func (l *EventLoop) Destroy() {
	l.state.Destroy()
	l.closePipe()
}

func (l *EventLoop) wake() {
	if l.pipe[1] < 0 {
		return
	}
	for {
		result, errno := libc.Write(l.pipe[1], l.empty[:1])
		if result == 1 || (result == -1 && errno != libc.EINTR) {
			break
		}
	}
}

func (l *EventLoop) processEvents() {
	platform.display.Pending()
	for platform.display.QLength() != 0 {
		l.getEvent()
		if l.state.Quitting() {
			return
		}
	}
	platform.display.Flush()
}

func (l *EventLoop) readWake() {
	var dummy [64]byte
	for {
		result, errno := libc.Read(l.pipe[0], dummy[:])
		if result == 0 {
			return
		}
		if result == -1 && errno != libc.EINTR {
			return
		}
	}
}

func (l *EventLoop) getEvent() {
	event := platform.display.NextEvent()
	if event.AnyEvent().Window != platform.helper {
		handleEvent(event)
	}
}

func (l *EventLoop) wait() bool {
	fds := []libc.PollFd{
		{
			Fd:     l.event,
			Events: libc.POLLIN,
		},
		{
			Fd:     l.pipe[0],
			Events: libc.POLLIN,
		},
	}

	for platform.display.Pending() == 0 {
		ret, errno := libc.Poll(fds, -1)
		if ret == -1 {
			if errno == libc.EINTR || errno == libc.EAGAIN {
				continue
			}
			return false
		}
		for _, pfd := range fds {
			if pfd.REvents&libc.POLLIN != 0 {
				return true
			}
		}
	}
	return true
}

func (l *EventLoop) closePipe() {
	if l.pipe[0] >= 0 {
		libc.Close(l.pipe[0])
		l.pipe[0] = -1
	}
	if l.pipe[1] >= 0 {
		libc.Close(l.pipe[1])
		l.pipe[1] = -1
	}
}
