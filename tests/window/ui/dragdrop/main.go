// Declarative drag-and-drop window checks, without DevServer.
//
// Environment: Linux/X11, Windows or macOS desktop. Record OS, Linux WM, actual
// backend and scale. Use GOUI_PLAT_PAINTER=software/opengl (also direct2d on Windows)
// and GOUI_PLAT_SCALE=1 or 2. Call ui.Run from main on the platform startup thread.
// Run: go run ./tests/window/ui/dragdrop
// PowerShell: $env:GOUI_PLAT_SCALE='2'; go run ./tests/window/ui/dragdrop
// By default Data+OnPrepare supplies a custom preview. -preview=false uses Data
// alone without OnPrepare, checking native fallback. With previews enabled,
// -widget-preview finds the source by ID through ui.App.FindWidget and renders
// it using gui.RenderWidget.
// Initially a 760x480 window has Drag source above Drop target. Rebuild, Source
// Enabled and Bindings are usable; Revision=0, counts/FAIL=0, Last drop=none,
// Source Enabled=true and Bindings=true.
// Steps and expected results:
//  1. Click source: only Click+1. Drag to target in default mode: Prepare sees
//     Data's preset text; Prepare/Begin/Drop/End each increase once, Click stays
//     unchanged and FAIL=0. Last drop retains the current revision's text after
//     End. Repeat with exactly one delivery per signal.
//  2. Rebuild increments Revision. Drag twice: each drop has the new text, old
//     source/target callbacks never run, and counts are not duplicated.
//  3. Set Source Enabled=false: clicking still works, but dragging produces no
//     Prepare/Begin/Drop/End. Re-enable to resume dropping.
//  4. Set Bindings=false: source and target drag controllers are removed, with
//     no new drag counts. Restore to resume exactly-once delivery.
//  5. Release outside any target or press Esc: no Drop or accidental Click,
//     End action=0, and canceled=true only where observable. Retry successfully.
//  6. Default preview: 48 DIP blue/yellow image, transparent edge, black cross
//     hotspot at pointer-relative (12,12) DIP, before/after Rebuild. Resize and
//     repeat steps 1-5 at 1x/2x; size, hotspot, transparency, hits and counts stay
//     correct. Restart -preview=false: native fallback, Prepare=0 and normal
//     Begin/Drop/End counts with equal data.
//  7. Restart -widget-preview: show the current button's rendered image, correct
//     1x/2x size and 12 DIP hotspot. Repeat after Rebuild. Missing ID, render
//     failure or invalid scale increments FAIL.
//
// Platform differences: native cancellation observability, layout and backend
// are recorded separately. This checks UI reconciliation; tests/window/gui/dragdrop
// covers native files and cross-process formats.
// Side effects: no files, clipboard or settings changed. Reset: restart.
// Exit: close the window; nonzero FAIL gives a nonzero exit.
package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"log"
	"os"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/theme/modern"
	"github.com/golang-gui/goui/ui"
)

func main() {
	preview := flag.Bool("preview", true, "show a 48 DIP image with a marked 12 DIP hotspot")
	widgetPreview := flag.Bool("widget-preview", false, "render the current source widget by ID for its preview")
	flag.Parse()
	clicks, drops, builds, prepares, begins, ends, failures := 0, 0, 0, 0, 0, 0, 0
	enabled, bindings, active := true, true, false
	clickStart, dropStart := 0, 0
	status, lastDrop := "Drag the upper button to the lower one", "Last drop: none"
	check := func(ok bool, reason string) {
		if !ok {
			failures++
			status = "FAIL: " + reason
			fmt.Println(status)
		}
	}
	err := ui.Run("org.golang-gui.UIDragDropProbe", func(app ui.App) ui.RootView {
		generation := builds
		var source *ui.DragSourceView
		var target *ui.DropTargetView
		if bindings {
			data := new(ui.DragData)
			data.SetText(fmt.Sprintf("ui-drag-value/%d", generation))
			source = ui.DragSource().Data(data).Enabled(enabled)
			if *preview {
				source.OnPrepare(func(request *ui.DragPrepare) {
					check(generation == builds, "stale source Prepare callback")
					check(!active, "Prepare while another drag is active")
					prepares++
					value, ok := request.Data.Text()
					check(ok && value == fmt.Sprintf("ui-drag-value/%d", generation), "declared data missing during Prepare")
					if *widgetPreview {
						widget := app.FindWidget("dragdrop", "source")
						check(widget != nil, "source ID lookup: widget not found")
						if widget != nil {
							img, renderErr := gui.RenderWidget(widget, 0)
							check(renderErr == nil, fmt.Sprintf("source preview render: %v", renderErr))
							if renderErr == nil {
								request.Preview.Image = img
								if width := widget.Rect().Width; width > 0 {
									request.Preview.Scale = float32(img.Bounds().Dx()) / width
								}
								check(request.Preview.Scale > 0, "source preview scale")
							}
						}
					} else {
						request.Preview.Image = previewImage()
						request.Preview.Scale = 1
					}
					request.Preview.Hotspot = geometry.Point{X: 12, Y: 12}
					status = fmt.Sprintf("Prepare revision=%d", generation)
					fmt.Println(status)
					app.RequestUpdate()
				})
			}
			source.OnBegin(func() {
				check(!active, "overlapping source Begin")
				active = true
				clickStart, dropStart = clicks, drops
				begins++
				status = "Begin"
				fmt.Println(status)
				app.RequestUpdate()
			}).OnEnd(func(result ui.DragResult) {
				started := active
				check(started || result.Err != nil, "End without Begin or startup error")
				if started {
					check(clicks == clickStart, "drag also triggered Click")
					check(drops-dropStart <= 1, "multiple Drops from one source")
				}
				if started && drops > dropStart {
					check(result.Action == ui.DragCopy, "accepted Drop did not finish Copy")
				}
				check(result.Err == nil, fmt.Sprintf("source error: %v", result.Err))
				active = false
				ends++
				status = fmt.Sprintf("End action=%d canceled=%t error=%v", result.Action, result.Canceled, result.Err)
				fmt.Println(status)
				app.RequestUpdate()
			})
			target = ui.DropTarget(ui.DragFormatText).OnDrop(func(e *ui.DropRequest) {
				check(generation == builds, "stale target Drop callback")
				value, ok := e.Data.Text()
				expected := fmt.Sprintf("ui-drag-value/%d", builds)
				e.Accepted = ok && value == expected
				check(e.Accepted, fmt.Sprintf("text=%q expected=%q", value, expected))
				if e.Accepted {
					drops++
					lastDrop = fmt.Sprintf("Last drop: %q, total=%d", value, drops)
					status = "VERIFIED: current source and target callbacks"
					fmt.Println(lastDrop)
				}
				app.RequestUpdate()
			}).OnError(func(err error) { check(false, "target error: "+err.Error()); app.RequestUpdate() })
		}
		return ui.Root().StyleSheet(modern.Sheet(modern.Options{})).Windows(
			ui.Window("dragdrop").Title("GOUI UI DragDrop acceptance").Size(760, 480).Content(
				ui.VBox(
					ui.Label(fmt.Sprintf("Revision=%d | Prepare=%d Begin=%d Drop=%d End=%d Click=%d FAIL=%d",
						builds, prepares, begins, drops, ends, clicks, failures)),
					ui.Button("Drag source").ID("source").MinSize(400, 100).MainWeight(1).
						DragSource(source).OnClick(func() {
						clicks++
						fmt.Printf("Click count=%d\n", clicks)
						app.RequestUpdate()
					}),
					ui.Button("Drop target").ID("target").MinSize(400, 100).MainWeight(1).DropTarget(target),
					ui.HBox(
						ui.Button("Rebuild").OnClick(func() { builds++; fmt.Printf("Rebuild revision=%d\n", builds); app.RequestUpdate() }),
						ui.Button(fmt.Sprintf("Source Enabled: %t", enabled)).OnClick(func() { enabled = !enabled; app.RequestUpdate() }),
						ui.Button(fmt.Sprintf("Bindings: %t", bindings)).OnClick(func() { bindings = !bindings; app.RequestUpdate() }),
					).Spacing(8),
					ui.Label(status), ui.Label(lastDrop),
				).Spacing(12).Padding(16).CrossAlign(layout.CrossStretch),
			),
		)
	})
	if err != nil {
		log.Fatal(err)
	}
	if failures != 0 {
		fmt.Fprintf(os.Stderr, "%d UI dragdrop failures\n", failures)
		os.Exit(1)
	}
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
