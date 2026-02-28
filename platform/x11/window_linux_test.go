package x11

import (
	"testing"

	"github.com/golang-gui/goui/platform/common"
	"github.com/golang-gui/goui/platform/events"
)

func TestWindow(t *testing.T) {
	_, err := NewPlatform()
	if err != nil {
		t.Fatal(err)
	}

	quit := false

	q, err := newEventQueue()
	if err != nil {
		t.Fatal(err)
	}

	var win common.Window
	onEvent := func(event events.Event) {
		if event.Type() == events.Close {
			t.Log("window close")
			win.Destroy()
			quit = true
			q.Post()
		}
	}

	win, err = newWindow(onEvent)
	if err != nil {
		t.Fatal(err)
	}

	//win.SetTitle("TestWindow")
	win.Show()

	for !quit {
		q.Wait()
	}
}
