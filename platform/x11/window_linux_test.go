package x11

import (
	"testing"

	"github.com/golang-gui/goui/platform/events"
)

func TestWindow(t *testing.T) {
	plat, err := NewPlatform()
	if err != nil {
		t.Fatal(err)
	}

	eventQueue, err := plat.NewEventQueue()
	if err != nil {
		t.Fatal(err)
	}

	quit := false
	win, err := plat.NewWindow(func(event events.Event) {
		if event.Type() == events.Close {
			quit = true
			eventQueue.Post()
		}
	})

	//win.SetTitle("TestWindow")
	win.Show()

	for !quit {
		eventQueue.Wait()
	}
}
