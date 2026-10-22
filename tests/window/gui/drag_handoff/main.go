// Handoff from in-bar dragging to native DnD, without DevServer.
//
// Environment: interactive Linux/X11, Windows or macOS desktop. Record backend,
// WM and scale. Run: go run ./tests/window/gui/drag_handoff -expect accepted
// Native chrome is the default; optionally use -chrome integrated and repeat
// with GOUI_PLAT_SCALE=2. Initially SOURCE on the left contains a draggable bar;
// TARGET on the right accepts a drop. Each run verifies one operation.
// Steps and expected results:
//  1. -expect local: move horizontally over 4 DIP and release inside the bar.
//     Exactly one local Begin/End, no native Begin/End and no Click.
//  2. -expect accepted: move inside the bar, then down out of it, keep the left
//     button held and drop in the right window. One local Begin, no local
//     End/Cancel, one native Begin/End, identical Go object at the target,
//     successful Move, no Click, and a RenderWidget preview.
//  3. -expect canceled: enter the right window, press Esc before releasing.
//     Canceled=true, no Drop, no valid position and no Click.
//  4. -expect unaccepted: release in blank client space below the source button.
//     Action=0, not canceled, valid position, no Drop and no Click.
//  5. -expect click: a short click produces one Click and no local/native drag events.
//
// Platform differences: activate the AppKit window before dragging; preview and
// cancellation animation are native. With Integrated chrome the bar must not move
// the window; empty HeaderBar space still does.
// Side effects: this program's windows only; no files, clipboard or system changes.
// Reset: restart. Exit: automatically after one operation; closing before an
// operation is incomplete. Counts and object identity are asserted; independently
// inspect screenshots for position accuracy and preview appearance.
package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"
	"slices"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/theme/modern"
)

func main() {
	runtime.LockOSThread()
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL:", err)
		os.Exit(1)
	}
}

func run() error {
	expect := flag.String("expect", "accepted", "accepted, canceled, unaccepted, local, click")
	chrome := flag.String("chrome", "native", "native or integrated")
	flag.Parse()
	if !slices.Contains([]string{"accepted", "canceled", "unaccepted", "local", "click"}, *expect) ||
		!slices.Contains([]string{"native", "integrated"}, *chrome) {
		return fmt.Errorf("invalid expectation or chrome")
	}
	app, err := gui.NewApplication("org.golang-gui.DragHandoffProbe")
	if err != nil {
		return err
	}
	app.SetStyleSheet(modern.Sheet(modern.Options{}))
	mode := gui.WindowChromeNative
	if *chrome == "integrated" {
		mode = gui.WindowChromeIntegrated
	}
	options := &gui.WindowOptions{Size: geometry.Size{Width: 400, Height: 260}, Chrome: mode}
	src, err := app.NewWindow(options)
	if err != nil {
		return err
	}
	defer src.Destroy()
	dst, err := app.NewWindow(options)
	if err != nil {
		return err
	}
	defer dst.Destroy()
	_ = src.SetTitle("GOUI handoff SOURCE")
	_ = dst.SetTitle("GOUI handoff TARGET")
	src.ConnectCloseRequest(func(*bool) { app.Quit() })
	dst.ConnectCloseRequest(func(*bool) { app.Quit() })
	localBegins, localEnds, localCancels, begins, ends, drops, clicks := 0, 0, 0, 0, 0, 0, 0
	var result gui.DragResult
	var failure error
	finish := func() { app.Post(app.Quit) }
	fail := func(err error) { failure = err; finish() }
	button := gui.NewButton()
	button.SetChild(gui.NewLabel("Drag here, then leave this strip"))
	button.SetMinSize(geometry.Size{Height: 70})
	button.ConnectClicked(func() { clicks++; finish() })
	column := gui.NewLinearBox(layout.DirectionVertical)
	column.SetCrossAlign(layout.CrossStretch)
	column.SetPadding(12)
	column.SetSpacing(12)
	if mode == gui.WindowChromeIntegrated {
		header := gui.NewHeaderBar()
		header.SetChild(gui.NewLabel("Handoff source"))
		column.AddChild(header)
	}
	column.AddChild(button)
	column.AddChild(gui.NewLabel("Release here for unaccepted"))
	src.SetWidget(column)
	receiver := gui.NewButton()
	receiver.SetChild(gui.NewLabel("Drop local object here"))
	dst.SetWidget(receiver)
	value := new(int)
	*value = 42
	source := gui.NewDragSource()
	source.SetActions(gui.DragMove)
	source.ConnectBegin(func() { begins++; fmt.Println("native Begin") })
	source.ConnectEnd(func(r gui.DragResult) {
		ends++
		result = r
		fmt.Printf("native End %+v\n", r)
		finish()
	})
	drag := gui.NewDragEventController()
	drag.SetThreshold(4)
	var hotspot geometry.Point
	drag.ConnectBegin(func(p geometry.Point, _ events.Modifiers) {
		localBegins++
		hotspot = p
		fmt.Println("local Begin")
	})
	drag.ConnectUpdate(func(p geometry.Point, _ events.Modifiers) {
		if p.X >= 0 && p.Y >= 0 && p.X < button.Rect().Width && p.Y < button.Rect().Height {
			return
		}
		bitmap, err := gui.RenderWidget(button, 1)
		if err != nil {
			fail(err)
			return
		}
		data := new(gui.DragData)
		data.SetLocal("handoff", value)
		if err := source.Begin(drag, data, gui.DragPreview{Image: bitmap, Scale: 1, Hotspot: hotspot}); err != nil {
			fail(err)
		}
	})
	drag.ConnectEnd(func(geometry.Point, events.Modifiers) { localEnds++; finish() })
	drag.ConnectCancel(func() { localCancels++ })
	button.AddEventController(drag)
	target := gui.NewDropTarget(gui.LocalFormat("handoff"))
	target.SetActions(gui.DragMove)
	target.ConnectDrop(func(e *gui.DropRequest) {
		drops++
		got, ok := e.Data.Local("handoff")
		if !ok || got != value || e.Action != gui.DragMove {
			fail(fmt.Errorf("incorrect local payload/action"))
			return
		}
		e.Accepted = true
		fmt.Println("Drop: same Go object")
	})
	target.ConnectError(fail)
	receiver.AddEventController(target)
	if err := src.Show(); err != nil {
		return err
	}
	if err := dst.Show(); err != nil {
		return err
	}
	if err := src.SetPosition(nil, geometry.Point{X: 60, Y: 100}); err != nil {
		return err
	}
	if err := dst.SetPosition(nil, geometry.Point{X: 540, Y: 100}); err != nil {
		return err
	}
	fmt.Printf("OS=%s chrome=%s requested painter=%s scale=%s expect=%s\n", runtime.GOOS, *chrome,
		os.Getenv("GOUI_PLAT_PAINTER"), os.Getenv("GOUI_PLAT_SCALE"), *expect)
	app.Run()
	fmt.Printf("counts local=%d/%d cancel=%d native=%d/%d drop=%d click=%d\n", localBegins, localEnds, localCancels, begins, ends, drops, clicks)
	if failure != nil {
		return failure
	}
	if *expect == "click" {
		if clicks != 1 || localBegins+localEnds+localCancels+begins+ends+drops != 0 {
			return fmt.Errorf("incorrect click counters")
		}
	} else if *expect == "local" {
		if localBegins != 1 || localEnds != 1 || localCancels+begins+ends+drops+clicks != 0 {
			return fmt.Errorf("incorrect local drag counters")
		}
	} else {
		if localBegins != 1 || begins != 1 || ends != 1 || localEnds+localCancels+clicks != 0 {
			return fmt.Errorf("incorrect handoff counters")
		}
		if err := result.Validate(); err != nil {
			return err
		}
		switch *expect {
		case "accepted":
			if drops != 1 || result.Action != gui.DragMove || !result.PositionValid {
				return fmt.Errorf("expected successful Move: %+v", result)
			}
		case "canceled":
			if drops != 0 || !result.Canceled {
				return fmt.Errorf("expected cancellation: %+v", result)
			}
		case "unaccepted":
			if drops != 0 || result.Action != 0 || result.Canceled || result.Err != nil || !result.PositionValid {
				return fmt.Errorf("expected unaccepted release: %+v", result)
			}
		}
	}
	fmt.Println("PASS", *expect)
	return nil
}
