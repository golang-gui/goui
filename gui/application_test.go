package gui

import (
	"testing"

	"github.com/golang-gui/goui/platform"
)

// quitCountLoop is a minimal EventLoop that only records Quit calls; removeWindow
// touches nothing else on the loop.
type quitCountLoop struct {
	platform.EventLoop
	quits int
}

func (l *quitCountLoop) Quit() { l.quits++ }

func TestApplicationQuitsOnLastWindowClosed(t *testing.T) {
	loop := &quitCountLoop{}
	w1, w2 := &window{}, &window{}
	a := &application{loop: loop, windows: []*window{w1, w2}, quitOnLastWindowClosed: true}

	a.removeWindow(w1)
	if loop.quits != 0 {
		t.Fatalf("must not quit while a window remains, quits=%d", loop.quits)
	}
	a.removeWindow(w2)
	if loop.quits != 1 {
		t.Fatalf("must quit when the last window closes, quits=%d", loop.quits)
	}
}

func TestApplicationDoesNotQuitWhenDisabled(t *testing.T) {
	loop := &quitCountLoop{}
	w := &window{}
	a := &application{loop: loop, windows: []*window{w}, quitOnLastWindowClosed: false}

	a.removeWindow(w)
	if loop.quits != 0 {
		t.Fatalf("must not quit when QuitOnLastWindowClosed is off, quits=%d", loop.quits)
	}
}
