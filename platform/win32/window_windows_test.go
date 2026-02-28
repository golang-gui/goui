package win32

import (
	"image/color"
	"image/draw"
	"math/rand"
	"runtime"
	"testing"

	"github.com/golang-gui/goui/platform/events"
)

func randColors(count int) []color.Color {
	colors := make([]color.Color, count)
	for i := range colors {
		colors[i] = color.RGBA{
			R: uint8(rand.Intn(255)),
			G: uint8(rand.Intn(255)),
			B: uint8(rand.Intn(255)),
			A: 255,
		}
	}
	return colors
}

func drawColors(img draw.Image, colors []color.Color) {
	bounds := img.Bounds()

	inter := bounds.Dx() / len(colors)
	selColor := func(x int) color.Color {
		for i := range colors {
			if i*inter <= x && x < (i+1)*inter {
				return colors[i]
			}
		}
		return colors[0]
	}

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			img.Set(x, y, selColor(x))
		}
	}
}

func TestWindow(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	plat, err := NewPlatform()
	if err != nil {
		t.Fatal(err)
	}

	quit := false

	eventQueue, err := plat.NewEventQueue()
	if err != nil {
		t.Fatal(err)
	}

	var width, height uint
	colors := randColors(7)

	onEvent := func(event events.Event) {
		switch ev := event.(type) {
		case *events.CloseEvent:
			t.Log("window close")
			ev.Window.Destroy()
			quit = true
			eventQueue.Post()
		case *events.SizeEvent:
			t.Logf("window size %dx%d", ev.Width, ev.Height)
			width, height = ev.Width, ev.Height
		case *events.PaintEvent:
			t.Logf("window paint %dx%d", width, height)
			img, _ := plat.NewImage(width, height)
			drawColors(img, colors)
			ev.Window.Draw(img)
			ev.Accept()
		case *events.ScaleEvent:
			t.Log("window scale", ev.ScaleFactor)
		}
	}

	win, err := newWindow(onEvent)
	if err != nil {
		t.Fatal(err)
	}

	scale, _ := win.ScaleFactor()
	t.Log("scale:", scale)

	win.SetTitle("TestWindow")
	win.Show()

	for !quit {
		eventQueue.Wait()
	}
}
