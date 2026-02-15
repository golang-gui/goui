package win32

import (
	"runtime"
	"testing"

	"github.com/golang-gui/goui/platform/events"
)

func TestWindow(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	quit := false

	q, err := newEventQueue()
	if err != nil {
		t.Fatal(err)
	}

	onEvent := func(event events.Event) {
		switch event.Type() {
		case events.Close:
			ce := event.(*events.CloseEvent)
			t.Log("window close")
			ce.Window.Destroy()
			quit = true
			q.Post()
		case events.Size:
			se := event.(*events.SizeEvent)
			t.Logf("window size %dx%d", se.Width, se.Height)
		}
	}

	win, err := newWindow(onEvent)
	if err != nil {
		t.Fatal(err)
	}

	win.SetTitle("TestWindow")
	win.Show()

	for !quit {
		q.Wait()
	}
}
