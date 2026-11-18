package main

import (
	"fmt"
	"image"
	"image/color"
	"os"
	"runtime"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform"
	"github.com/golang-gui/goui/platform/dragdrop"
	"github.com/golang-gui/goui/platform/events"
)

func runSource(path, want string, withPreview bool) error {
	if want != "copy" && want != "cancel" && want != "reject" {
		return fmt.Errorf("unknown -result %q", want)
	}
	if _, err := os.Stat(path); err != nil {
		return err
	}
	p, err := platform.NewPlatform(platform.DefaultName(), "org.golang-gui.DragDropProbe")
	if err != nil {
		return err
	}
	defer p.Destroy()
	loop, err := p.NewEventLoop()
	if err != nil {
		return err
	}
	defer loop.Destroy()
	var capability platform.DragDrop
	var down geometry.Point
	var pressed, started bool
	var result dragdrop.Result
	var gotEnd bool
	var begins, ends int
	var canvas nativeCanvas
	w, err := p.NewWindow(geometry.Size{Width: 640, Height: 360}, func(event events.Event) {
		canvas.event(event)
		switch e := event.(type) {
		case events.CloseEvent:
			loop.Quit()
		case events.PointerEvent:
			if e.EventType == events.PointerDown && e.Button == events.PointerButtonLeft {
				pressed, down = true, e.Position
			} else if e.EventType == events.PointerUp && e.Button == events.PointerButtonLeft {
				pressed = false
			} else if e.EventType == events.PointerMove && pressed && !started && e.Buttons&events.PointerButtonLeftDown != 0 {
				dx, dy := e.Position.X-down.X, e.Position.Y-down.Y
				if dx*dx+dy*dy <= 16 {
					return
				}
				data := new(dragdrop.Data)
				data.SetFiles([]string{path})
				started, pressed = true, false // Begin 可能同步进入原生事件循环。
				var preview dragdrop.Preview
				if withPreview {
					preview = dragdrop.Preview{Image: previewImage(), Scale: 1,
						Hotspot: geometry.Point{X: 12, Y: 12}}
				}
				if err := capability.Begin(1, data, dragdrop.Copy, preview, dragdrop.Feedback{}); err != nil {
					result.Err = err
					loop.Quit()
				}
			}
		case events.DragSourceEvent:
			if e.ID != 1 {
				result.Err = fmt.Errorf("unexpected source ID %d", e.ID)
				loop.Quit()
				return
			}
			if e.EventType == events.DragSourceBegin {
				begins++
				fmt.Println("native source Begin")
			} else if e.EventType == events.DragSourceEnd {
				ends++
				gotEnd, result = true, e.Result
				fmt.Printf("native source End: action=%d canceled=%t err=%v\n", result.Action, result.Canceled, result.Err)
				loop.Post(loop.Quit)
			}
		}
	}, platform.WindowOptions{})
	if err != nil {
		return err
	}
	defer w.Destroy()
	if err := canvas.init(p, w, true); err != nil {
		return err
	}
	defer canvas.painter.Destroy()
	if err := w.(platform.WindowManagement).SetTitle(fmt.Sprintf("GOUI DnD source | expect %s | preview %t", want, withPreview)); err != nil {
		return err
	}
	capability, err = p.NewDragDrop(w)
	if err != nil {
		return err
	}
	defer capability.Destroy()
	if err := w.(platform.WindowManagement).Show(); err != nil {
		return err
	}
	loop.Run()
	if result.Err != nil {
		return result.Err
	}
	if !gotEnd || begins != 1 || ends != 1 {
		return fmt.Errorf("expected one Begin/End: began=%d ended=%d result=%+v", begins, ends, result)
	}
	valid := result.Action == 0
	switch want {
	case "copy":
		valid = result.Action == dragdrop.Copy && !result.Canceled
	case "reject":
		valid = result.Action == 0 && !result.Canceled
	case "cancel":
		valid = result.Action == 0 && (result.Canceled || runtime.GOOS == "darwin")
		if !result.Canceled && runtime.GOOS == "darwin" {
			fmt.Println("NOTE: native backend did not distinguish cancellation from rejection; check the actual Escape operation")
		}
	}
	if !valid {
		return fmt.Errorf("expected %s, got %+v", want, result)
	}
	fmt.Printf("PASS: native source %s; one Begin/End\n", want)
	return nil
}

func previewImage() image.Image {
	img := image.NewNRGBA(image.Rect(0, 0, 48, 48))
	for y := 4; y < 44; y++ {
		for x := 4; x < 44; x++ {
			pixel := color.NRGBA{R: 20, G: 100, B: 230, A: 230}
			if x >= 24 {
				pixel = color.NRGBA{R: 255, G: 205, B: 20, A: 230}
			}
			if (x == 12 && y >= 8 && y <= 16) || (y == 12 && x >= 8 && x <= 16) {
				pixel = color.NRGBA{A: 255}
			}
			img.SetNRGBA(x, y, pixel)
		}
	}
	return img
}
