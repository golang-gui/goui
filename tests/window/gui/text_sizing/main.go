// Text sizing and single-line appearance regression. Requires a native desktop,
// fonts and typography libraries. Run: go run ./tests/window/gui/text_sizing
// [-capture-dir DIR] [-inspect]. DIR must exist outside the repository.
// Linux: optionally set GOUI_PLAT_PAINTER=software/opengl, GOUI_PLAT_SCALE=1/2,
// XMODIFIERS=@im=none. Windows additionally verifies direct2d; macOS uses Cmd.
// Initially Basic/Modern Light/Modern Dark show a TextInput, a bare TextView and
// a capped ScrollView/TextView. Automatically compare same-backend RGBA pixels
// of equal, unfocused single-line fields (including placeholders), then enter
// and delete LF/long wrapped text through Window.DispatchEvent/CommitText.
// Assert content growth, the 96-DIP cap, scrollbar ranges, shrink/offset reset,
// baseline stability, and shared-model geometry isolation after a resize.
// Captures are optional. No clipboard or system settings are changed. Automatic
// success closes the window; early close/failure is nonzero. -inspect keeps the
// completed window open. Restart resets everything. Physical IME/candidate
// positioning remains manual; simulated commits do not verify physical IME.
package main

import (
	"flag"
	"fmt"
	"image"
	"image/png"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/theme/basic"
	"github.com/golang-gui/goui/theme/modern"
)

func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
func check(ok bool, message string) {
	if !ok {
		log.Fatal(message)
	}
}

func main() {
	runtime.LockOSThread()
	out := flag.String("capture-dir", "", "existing temporary PNG directory")
	inspect := flag.Bool("inspect", false, "keep completed window open")
	flag.Parse()
	app, err := gui.NewApplication("org.golang-gui.TextSizing")
	must(err)
	win, err := app.NewWindow(&gui.WindowOptions{Size: geometry.Size{Width: 1050, Height: 480}, Chrome: gui.WindowChromeIntegrated})
	must(err)
	defer win.Destroy()
	root := gui.NewLinearBox(layout.DirectionHorizontal)
	root.SetCrossAlign(layout.CrossStretch)
	var inputs [3]*gui.TextInput
	var views, growing [3]*gui.TextView
	var scrolls [3]*gui.ScrollView
	model := gui.NewTextModel("")
	for i, title := range []string{"Basic", "Modern Light", "Modern Dark"} {
		column := gui.NewLinearBox(layout.DirectionVertical)
		column.SetPadding(12)
		column.SetSpacing(8)
		column.SetCrossAlign(layout.CrossStretch)
		column.AddChild(&renderingProbe{Label: gui.NewLabel(title)})
		inputs[i] = gui.NewTextInput()
		views[i] = gui.NewTextView()
		views[i].SetWrapMode(gui.WrapNone)
		inputs[i].SetPlaceholder("Find 中文")
		views[i].SetPlaceholder("Find 中文")
		column.AddChild(inputs[i])
		column.AddChild(views[i])
		growing[i] = gui.NewTextView()
		growing[i].SetModel(model)
		growing[i].SetPlaceholder("Multiline")
		scrolls[i] = gui.NewScrollView()
		scrolls[i].SetDirection(gui.ScrollVertical)
		scrolls[i].SetMainWeight(0)
		scrolls[i].SetMaxSize(geometry.Size{Height: 96})
		scrolls[i].SetChild(growing[i])
		column.AddChild(scrolls[i])
		scope := gui.NewStyleScope()
		scope.SetMainWeight(1)
		if i == 0 {
			scope.SetStyleSheet(basic.Sheet())
		} else {
			scope.SetStyleSheet(modern.Sheet(modern.Options{Dark: i == 2}))
		}
		scope.SetChild(column)
		root.AddChild(scope)
	}
	win.SetWidget(root)
	must(win.Show())
	primary, _ := gui.ModPrimary.Resolve()
	key := func(k events.Key) {
		for _, kind := range []events.EventType{events.KeyDown, events.KeyUp} {
			mods := events.Modifiers(0)
			if k == events.KeyA {
				mods = primary
			}
			must(win.DispatchEvent(events.KeyEvent{EventType: kind, Key: k, Modifiers: mods}))
		}
	}
	compare := func() {
		win.SetFocusedWidget(nil)
		for i := range inputs {
			check(inputs[i].Rect().Size == views[i].Rect().Size, fmt.Sprintf("single-line bounds %d: input=%v view=%v", i, inputs[i].Rect(), views[i].Rect()))
			a, err := gui.RenderWidget(inputs[i], 0)
			must(err)
			b, err := gui.RenderWidget(views[i], 0)
			must(err)
			check(a.Bounds() == b.Bounds(), "single-line raster size")
			for y := a.Bounds().Min.Y; y < a.Bounds().Max.Y; y++ {
				for x := a.Bounds().Min.X; x < a.Bounds().Max.X; x++ {
					ar, ag, ab, aa := a.At(x, y).RGBA()
					br, bg, bb, ba := b.At(x, y).RGBA()
					check(ar == br && ag == bg && ab == bb && aa == ba, fmt.Sprintf("single-line pixels differ theme=%d at %d,%d", i, x, y))
				}
			}
		}
	}
	step, complete := 0, false
	var oneLine [3]float32
	timer := app.NewTimer()
	defer timer.Stop()
	timer.ConnectTimeout(func() {
		switch step {
		case 0:
			compare()
			for i := range inputs {
				oneLine[i] = scrolls[i].Rect().Height
			}
			win.SetFocusedWidget(growing[0])
			must(win.CommitText("first"))
		case 1:
			for i := range inputs {
				check(scrolls[i].Rect().Height == oneLine[i], "single line changed height")
			}
			must(win.CommitText("\nsecond"))
		case 2:
			for i := range inputs {
				expected := 2*oneLine[i] - 2*growing[i].Padding()
				check(scrolls[i].Rect().Height == expected, fmt.Sprintf("multiline height %d: got=%g want=%g", i, scrolls[i].Rect().Height, expected))
			}
			must(win.CommitText(strings.Repeat("\nrow", 80)))
		case 3:
			for i := range inputs {
				check(scrolls[i].Rect().Height == 96 && scrolls[i].Snapshot().MaxScrollY > 0, "cap/scroll range")
			}
			key(events.KeyA)
			key(events.KeyBackspace)
		case 4:
			for i := range inputs {
				check(scrolls[i].Rect().Height == oneLine[i] && scrolls[i].ScrollY() == 0 && scrolls[i].Snapshot().MaxScrollY == 0, "shrink left scroll range")
				inputs[i].SetText("Ag中文 é 👩‍💻")
				views[i].Model().SetText("Ag中文 é 👩‍💻")
			}
		case 5:
			compare()
			win.SetFocusedWidget(growing[0])
			must(win.CommitText(strings.Repeat("wrapped ", 100)))
		case 6:
			for i := range inputs {
				check(model.LineCount() == 1 && scrolls[i].Rect().Height == 96 && scrolls[i].Snapshot().MaxScrollY > 0, "soft wrapping failed to grow")
			}
			img, e := gui.RenderWidget(root, 0)
			must(e)
			scale := float32(img.Bounds().Dx()) / root.Rect().Width
			must(win.DispatchEvent(events.SizeEvent{Width: 900, Height: 480, PixelWidth: 900 * scale, PixelHeight: 480 * scale}))
		case 7:
			compare()
			// Equal model, different view width: reflow one view through Measure
			// and verify its peer's native extent is unchanged.
			before := growing[1].ContentSize()
			growing[0].Measure(layout.Loose(geometry.Size{Width: 120, Height: 96}))
			check(growing[1].ContentSize() == before, "shared model leaked reflow")
			// Return to normal host layout before capturing: the measurement
			// above intentionally used a width different from the allocation.
			growing[0].RequestLayout()
		case 8:
			if *out != "" {
				img, e := gui.RenderWidget(root, 0)
				must(e)
				f, e := os.Create(filepath.Join(*out, "text-sizing.png"))
				must(e)
				must(png.Encode(f, img))
				must(f.Close())
			}
			complete = true
			timer.Stop()
			log.Printf("PASS: 9 text sizing stages, Basic/Modern Light/Modern Dark")
			if !*inspect {
				win.Destroy()
			}
		}
		step++
	})
	must(timer.Start(120 * time.Millisecond))
	app.Run()
	check(complete, "window closed before verification")
}

type renderingProbe struct {
	*gui.Label
	reported bool
}

func (p *renderingProbe) Paint(painter gui.Painter) {
	if !p.reported {
		p.reported = true
		img, err := painter.NewImage(image.NewRGBA(image.Rect(0, 0, 1, 1)))
		must(err)
		log.Printf("OS=%s typography=%s painter image=%T scale=%g", runtime.GOOS, gui.App.Typography().Name(), img, painter.PixelScale())
		img.Destroy()
	}
	p.Label.Paint(painter)
}
