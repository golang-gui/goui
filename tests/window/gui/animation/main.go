// Frame signals and numeric animation window checks.
//
// Environment: Linux, Windows or macOS desktop. Record backend, scale and Linux WM.
// Run: go run ./tests/window/gui/animation. -auto checks public API sequencing
// and exits. Use GOUI_PLAT_PAINTER=software/opengl (also direct2d on Windows)
// and GOUI_PLAT_SCALE=2 to repeat at 2x.
// Initially the circle is stationary, the switch is off and the Popover is
// closed. Observing frames alone does not request painting.
// Steps and expected results:
//  1. Start animates the circle's size and position smoothly. After completion,
//     Check prints IDLE PASS. Start during animation redirects from the current
//     value; Stop retains it, while Finish reaches the final value.
//  2. Click the switch: the thumb transitions over 120 ms. Check shows no ongoing
//     frames after completion.
//  3. Popup starts a continuously clockwise busy indicator. Closing it stops
//     frame counts; reopening resumes. Hide/show and resize without reentry or hangs.
//  4. Snapshot renders the same instant twice offscreen with equal sizes/pixels
//     and no Frame signal. Repeat at 1x/2x on supported backends.
//
// -auto checks final values, idle frames, consistent offscreen output and Popover
// update/hide behavior. It prints ANIMATION PASS and exits with code 0; failed
// assertions panic. Successful startup alone does not verify manual interaction.
// Side effects: windows and terminal logs; no system settings are changed.
// Reset: restart. Exit: close the window or let -auto finish.
package main

import (
	"crypto/sha256"
	"flag"
	"fmt"
	"image"
	"log"
	"runtime"
	"time"

	"github.com/golang-gui/goui/animation"
	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/theme/modern"
	"github.com/golang-gui/goui/widgets"
)

type sample struct {
	gui.WidgetBase
	size, position float32
}

func (s *sample) Paint(p gui.Painter) {
	p.FillEllipse(geometry.Point{X: 100 + s.position, Y: 100}, s.size, s.size, graphics.Color{R: .15, G: .45, B: .8, A: 1})
}
func digest(img image.Image) [32]byte {
	h := sha256.New()
	for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y; y++ {
		for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x++ {
			r, g, b, a := img.At(x, y).RGBA()
			h.Write([]byte{byte(r >> 8), byte(g >> 8), byte(b >> 8), byte(a >> 8)})
		}
	}
	var sum [32]byte
	copy(sum[:], h.Sum(nil))
	return sum
}
func main() {
	auto := flag.Bool("auto", false, "自动验证帧调度和终值后退出")
	flag.Parse()
	runtime.LockOSThread()
	app, err := gui.NewApplication("org.golang-gui.Animation")
	if err != nil {
		log.Fatal(err)
	}
	app.SetStyleSheet(modern.Sheet(modern.Options{}))
	w, err := app.NewWindow(&gui.WindowOptions{Size: geometry.Size{Width: 700, Height: 430}, Chrome: gui.WindowChromeIntegrated})
	if err != nil {
		log.Fatal(err)
	}
	if err = w.SetTitle("GOUI animation GUI"); err != nil {
		log.Fatal(err)
	}
	content := &sample{size: 20}
	content.SetMinSize(geometry.Size{Width: 500, Height: 210})
	size := animation.New(float32(20), animation.Float32)
	position := animation.New(float32(0), animation.Float32)
	size.ConnectUpdate(func(v float32) { content.size = v; content.RequestPaint() })
	position.ConnectUpdate(func(v float32) { content.position = v; content.RequestPaint() })
	frames, popupFrames := 0, 0
	w.ConnectFrame(func(time.Time) { frames++ })
	toolbar := gui.NewLinearBox(layout.DirectionHorizontal)
	toolbar.SetSpacing(8)
	button := func(text string, fn func()) *gui.Button {
		b := gui.NewButton()
		b.SetChild(gui.NewLabel(text))
		b.ConnectClicked(fn)
		toolbar.AddChild(b)
		return b
	}
	start := func() {
		target := float32(80)
		offset := float32(200)
		if size.CurrentValue() > 50 {
			target = 20
			offset = 0
		}
		if err := size.AnimateTo(w, target, 300*time.Millisecond, animation.EaseOutCubic); err != nil {
			panic(err)
		}
		if err := position.AnimateTo(w, offset, 600*time.Millisecond, animation.Linear); err != nil {
			panic(err)
		}
	}
	button("Start", start)
	button("Stop", func() { size.Stop(); position.Stop() })
	button("Finish", func() { size.Finish(); position.Finish() })
	button("Check", func() {
		fmt.Printf("CHECK frames=%d size=%.2f position=%.2f running=%v/%v\n", frames, size.CurrentValue(), position.CurrentValue(), size.Running(), position.Running())
	})
	snapshot := func() {
		before := frames
		first, err := gui.RenderWidget(content, 1)
		if err != nil {
			panic(err)
		}
		second, err := gui.RenderWidget(content, 1)
		if err != nil {
			panic(err)
		}
		if frames != before || first.Bounds() != second.Bounds() || digest(first) != digest(second) {
			panic("offscreen draw advanced frame/value")
		}
		fmt.Println("SNAPSHOT PASS")
	}
	button("Snapshot", snapshot)
	anchor := button("Popup", func() {})
	popup := gui.NewPopover(anchor, &gui.PopoverOptions{Transparent: true})
	spinner := widgets.NewProgressBar()
	spinner.SetShape(widgets.ProgressCircular)
	spinner.SetMinSize(geometry.Size{Width: 60, Height: 60})
	spinner.SetIndeterminate(true)
	popup.SetWidget(spinner)
	popup.SetPlacement(gui.PopoverPlacementBottom)
	popup.ConnectFrame(func(time.Time) { popupFrames++ })
	anchor.ConnectClicked(func() {
		if popup.Visible() {
			popup.Hide()
		} else if err := popup.Show(); err != nil {
			panic(err)
		}
	})
	switcher := widgets.NewSwitch()
	body := gui.NewLinearBox(layout.DirectionVertical)
	body.SetSpacing(16)
	body.SetPadding(20)
	body.AddChild(toolbar)
	body.AddChild(switcher)
	body.AddChild(content)
	w.SetWidget(body)
	w.ConnectDestroy(func() { size.Stop(); position.Stop(); popup.Destroy() })
	if err := w.Show(); err != nil {
		panic(err)
	}
	after := func(delay time.Duration, fn func()) {
		timer := app.NewTimer()
		timer.ConnectTimeout(fn)
		if err := timer.StartOnce(delay); err != nil {
			panic(err)
		}
	}
	if *auto {
		idle := 0
		after(200*time.Millisecond, start)
		after(time.Second, func() {
			if size.Running() || position.Running() || size.CurrentValue() != 80 || position.CurrentValue() != 200 {
				panic("animation endpoint")
			}
			snapshot()
			idle = frames
		})
		after(1200*time.Millisecond, func() {
			if frames-idle > 2 {
				panic("idle frame loop")
			}
			fmt.Printf("IDLE PASS frames=%d\n", frames)
			if err := popup.Show(); err != nil {
				panic(err)
			}
		})
		after(1600*time.Millisecond, func() {
			if popupFrames < 3 {
				panic("popover frame source not running")
			}
			popup.Hide()
			idle = popupFrames
		})
		after(1850*time.Millisecond, func() {
			if popupFrames != idle {
				panic("hidden popup still updating")
			}
			if err := popup.Show(); err != nil {
				panic(err)
			}
		})
		after(2150*time.Millisecond, func() {
			if popupFrames <= idle {
				panic("popup did not resume")
			}
			popup.Hide()
			fmt.Printf("ANIMATION PASS frames=%d popupFrames=%d\n", frames, popupFrames)
			w.Destroy()
		})
	}
	app.Run()
}
