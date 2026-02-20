package x11

import (
	"fmt"
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
		switch event.Type() {
		case events.Close:
			quit = true
			eventQueue.Post()
			println("window close")
		case events.Size:
			se := event.(*events.SizeEvent)
			fmt.Printf("window size %dx%d\n", se.Width, se.Height)
		}
	})
	if err != nil {
		t.Fatal(err)
	}

	//win.SetTitle("TestWindow")
	win.Show()

	for !quit {
		eventQueue.Wait()
	}
}
