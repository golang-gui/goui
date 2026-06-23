package cocoa

import (
	"errors"

	. "github.com/golang-gui/goui/platform/darwin/frameworks/appkit"
	. "github.com/golang-gui/goui/platform/darwin/frameworks/core_foundation"
	"github.com/golang-gui/goui/platform/internal/eventloop"

	"github.com/ebitengine/purego/objc"
)

const eventLoopTimerInterval = CFTimeInterval(24 * 60 * 60)

type EventLoop struct {
	state   eventloop.State
	runLoop CFRunLoopRef
	timer   CFRunLoopTimerRef
	handler objc.Block
}

func newEventLoop() (l *EventLoop, err error) {
	l = new(EventLoop)
	l.runLoop = CFRunLoopGetMain()
	if l.runLoop == 0 {
		return nil, errors.New("get main run loop failed")
	}

	l.handler = objc.NewBlock(func(_ objc.Block, _ CFRunLoopTimerRef) {
		l.runTasks()
	})

	fireDate := CFAbsoluteTimeGetCurrent() + eventLoopTimerInterval
	l.timer = CFRunLoopTimerCreateWithHandler(fireDate, eventLoopTimerInterval, l.handler)
	if l.timer == 0 {
		l.handler.Release()
		return nil, errors.New("create event loop timer failed")
	}

	CFRunLoopAddTimer(l.runLoop, l.timer, KCFRunLoopCommonModes)
	return l, nil
}

func (l *EventLoop) Post(task func()) {
	if l.state.Post(task) {
		l.wake()
	}
}

func (l *EventLoop) Run() {
	defer l.state.Quit()
	if l.state.Destroyed() || l.state.Quitting() {
		return
	}
	NSApp.Run()
}

func (l *EventLoop) Quit() {
	if l.state.Quit() {
		l.wake()
	}
}

func (l *EventLoop) Destroy() {
	if l.timer == 0 {
		return
	}

	l.state.Destroy()
	CFRunLoopTimerInvalidate(l.timer)
	CFRelease(l.timer)
	l.timer = 0
	if l.handler != 0 {
		l.handler.Release()
		l.handler = 0
	}
}

func (l *EventLoop) wake() {
	if l.timer == 0 {
		return
	}
	CFRunLoopTimerSetNextFireDate(l.timer, CFAbsoluteTimeGetCurrent())
	CFRunLoopWakeUp(l.runLoop)
}

func (l *EventLoop) runTasks() {
	eventloop.RunTasks(&l.state)
	if l.state.Quitting() {
		NSApp.Stop()
	}
}
