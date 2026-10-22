// TextInput/TextView placeholder window checks.
// Environment: native desktop, fonts and typography libraries. Record OS/WM,
// actual Painter and 1x/2x. Run:
//
//	go run ./tests/window/gui/textplaceholder [-inspect] [-capture-dir DIR]
//
// DIR must be an existing temporary directory outside the repository. Use
// GOUI_PLAT_PAINTER=software/opengl and GOUI_PLAT_SCALE=1 or 2; also test direct2d
// on Windows. macOS editing shortcuts use Command.
// Initially Basic, Modern Light and Modern Dark each show an empty input and
// a multiline view; the multiline views share an empty model.
// Automatic steps and expected results:
//  1. All placeholders produce pixels while values/model length remain zero,
//     with or without focus. Toggling visibility preserves measurement, baseline
//     and extent, and all changed pixels stay inside the content clip.
//  2. Commit a space through Window.CommitText: the input placeholder disappears;
//     normal keyboard select-all/delete restores it. Changing the placeholder
//     retains selection. Multiline input synchronizes all views, and one undo
//     restores the shared empty document and all its placeholders.
//  3. Wrapping toggles, explicit newlines, larger fonts and a single-column theme
//     change still paint correctly. Other themes stay independent. Readonly and
//     disabled fields retain hints; re-enabled inputs accept text. Hints add no bars.
//  4. Unmount/remount restores placeholders. Success exits automatically; failures
//     or closing early return nonzero.
//
// -inspect keeps the successful window open. Type/delete/undo in both editor
// types and narrow the window to inspect wrapping and clipping. Real Chinese IME
// preedit hides the hint, cancel restores it, and commit leaves only real text;
// inspect candidate placement near the caret. Automated checks do not verify IME
// or desktop composition. Pixel checks compare visibility changes within the same
// field in sRGB RGBA at integer physical-pixel centers inside the content area;
// other text/borders must be unchanged. There is no cross-font golden image.
// Side effects: creates windows; -capture-dir writes PNGs. No system changes or
// automatic clipboard access. Reset: restart. Exit: completion or close the window;
// no DevServer is used.
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

type editor interface {
	gui.Widget
	Placeholder() string
	SetPlaceholder(string)
}

func main() {
	runtime.LockOSThread()
	inspect := flag.Bool("inspect", false, "keep window open after checks")
	out := flag.String("capture-dir", "", "existing temporary PNG directory")
	flag.Parse()
	app, err := gui.NewApplication("org.golang-gui.TextPlaceholder")
	must(err)
	win, err := app.NewWindow(&gui.WindowOptions{Size: geometry.Size{Width: 1050, Height: 430}, Chrome: gui.WindowChromeIntegrated})
	must(err)
	defer win.Destroy()
	must(win.SetTitle("GOUI Placeholder — Basic / Modern Light / Dark"))
	model := gui.NewTextModel("")
	row := gui.NewLinearBox(layout.DirectionHorizontal)
	row.SetSpacing(12)
	row.SetPadding(12)
	row.SetCrossAlign(layout.CrossStretch)
	var inputs [3]*gui.TextInput
	var views [3]*gui.TextView
	var scopes [3]*gui.StyleScope
	for i, title := range []string{"Basic", "Modern Light", "Modern Dark"} {
		column := gui.NewLinearBox(layout.DirectionVertical)
		column.SetSpacing(10)
		column.SetPadding(8)
		column.SetCrossAlign(layout.CrossStretch)
		column.AddChild(&painterProbe{Label: gui.NewLabel(title)})
		input := gui.NewTextInput()
		input.SetPlaceholder("Name 中文 👩‍💻 " + strings.Repeat("long hint ", 8))
		input.SetName(title + " name")
		column.AddChild(input)
		view := gui.NewTextView()
		view.SetModel(model)
		view.SetPlaceholder(title + " notes 中文\n" + strings.Repeat("Describe your issue here. ", 12))
		view.SetName(title + " notes")
		scroll := gui.NewScrollView()
		scroll.SetMainWeight(1)
		scroll.SetChild(view)
		column.AddChild(scroll)
		column.AddChild(gui.NewLabel("输入后隐藏，清空后恢复"))
		scope := gui.NewStyleScope()
		scope.SetMainWeight(1)
		if i == 0 {
			scope.SetStyleSheet(basic.Sheet())
		} else {
			scope.SetStyleSheet(modern.Sheet(modern.Options{Dark: i == 2}))
		}
		surface := gui.NewBorderBox()
		surface.SetResizeWidth(0)
		surface.SetStyleName("text-view")
		surface.SetChild(column)
		scope.SetChild(surface)
		row.AddChild(scope)
		inputs[i], views[i], scopes[i] = input, view, scope
	}
	win.SetWidget(row)
	must(win.Show())
	primary, _ := gui.ModPrimary.Resolve()
	key := func(k events.Key, mods events.Modifiers) {
		for _, kind := range []events.EventType{events.KeyDown, events.KeyUp} {
			must(win.DispatchEvent(events.KeyEvent{EventType: kind, Key: k, Modifiers: mods}))
		}
	}
	verifyHint := func(w editor) {
		check(w.Snapshot().TextEditing.Length == 0 && w.Snapshot().TextEditing.Placeholder == w.Placeholder(), "hint changed value/semantics")
		measurement := w.Measure(layout.Unbounded())
		var size geometry.Size
		if view, ok := w.(*gui.TextView); ok {
			size = view.ContentSize()
		}
		shown, err := gui.RenderWidget(w, 0)
		must(err)
		text := w.Placeholder()
		w.SetPlaceholder("")
		hidden, err := gui.RenderWidget(w, 0)
		must(err)
		w.SetPlaceholder(text)
		check(w.Measure(layout.Unbounded()) == measurement, "hint changed measurement/baseline")
		if view, ok := w.(*gui.TextView); ok {
			check(view.ContentSize() == size, "hint changed scrollable content size")
		}
		check(shown.Bounds() == hidden.Bounds(), "hint changed render bounds")
		sx := float32(shown.Bounds().Dx()) / w.Rect().Width
		sy := float32(shown.Bounds().Dy()) / w.Rect().Height
		changed := 0
		for y := 0; y < shown.Bounds().Dy(); y++ {
			for x := 0; x < shown.Bounds().Dx(); x++ {
				ar, ag, ab, aa := shown.At(x, y).RGBA()
				br, bg, bb, ba := hidden.At(x, y).RGBA()
				if ar != br || ag != bg || ab != bb || aa != ba {
					px, py := (float32(x)+.5)/sx, (float32(y)+.5)/sy
					check(px >= 4 && py >= 4 && px <= w.Rect().Width-4 && py <= w.Rect().Height-4, fmt.Sprintf("hint escaped content clip at %g,%g", px, py))
					changed++
				}
			}
		}
		check(changed > 0, "empty editor did not paint placeholder")
	}
	done, stage := false, 0
	deadline := time.Now().Add(15 * time.Second)
	timer := app.NewTimer()
	defer timer.Stop()
	timer.ConnectTimeout(func() {
		check(time.Now().Before(deadline), "window did not complete initial layout/checks within 15 seconds")
		if inputs[0].Rect().Width <= 0 || views[0].Rect().Height <= 0 {
			return // Native expose and the first layout can follow timer startup.
		}
		switch stage {
		case 0:
			img, err := gui.RenderWidget(inputs[0], 0)
			must(err)
			log.Printf("OS=%s typography=%s scale=%g", runtime.GOOS, app.Typography().Name(), float32(img.Bounds().Dx())/inputs[0].Rect().Width)
			for i, input := range inputs {
				win.SetFocusedWidget(nil)
				verifyHint(input)
				win.SetFocusedWidget(input)
				verifyHint(input)
				must(win.CommitText(" "))
				check(input.Text() == " ", "space input was confused with empty field")
				input.SetSelection(gui.TextSelection{Anchor: 0, Caret: 1})
				input.SetPlaceholder(input.Placeholder() + " updated")
				check(input.Selection() == (gui.TextSelection{Anchor: 0, Caret: 1}) && input.Text() == " ", "hint setter changed selection/value")
				key(events.KeyA, primary)
				key(events.KeyBackspace, 0)
				verifyHint(input)
				verifyHint(views[i])
			}
			win.SetFocusedWidget(views[0])
			must(win.CommitText(" "))
			for _, view := range views {
				check(view.Snapshot().TextEditing.Length == 1, "shared document commit failed")
			}
			key(events.KeyZ, primary)
			check(model.Len() == 0 && model.CanRedo(), "shared undo failed")
		case 1:
			for _, view := range views {
				verifyHint(view)
				view.SetWrapMode(gui.WrapNone)
			}
			scopes[1].SetStyleSheet(modern.Sheet(modern.Options{Dark: true, FontSize: 18}))
		case 2:
			for i, view := range views {
				verifyHint(view)
				view.SetWrapMode(gui.WrapWordChar)
				inputs[i].SetReadOnly(true)
				view.SetReadOnly(true)
			}
		case 3:
			for i, view := range views {
				verifyHint(inputs[i])
				verifyHint(view)
				inputs[i].SetEnabled(false)
				view.SetEnabled(false)
			}
		case 4:
			for i, view := range views {
				verifyHint(inputs[i])
				verifyHint(view)
				inputs[i].SetEnabled(true)
				view.SetEnabled(true)
				inputs[i].SetReadOnly(false)
				view.SetReadOnly(false)
			}
			win.SetWidget(nil)
			win.SetWidget(row)
		case 5:
			for i, view := range views {
				verifyHint(inputs[i])
				verifyHint(view)
			}
			scopes[1].SetStyleSheet(modern.Sheet(modern.Options{}))
		case 6:
			verifyHint(inputs[1])
			verifyHint(views[1])
			if *out != "" {
				img, err := gui.RenderWidget(row, 0)
				must(err)
				f, err := os.Create(filepath.Join(*out, "placeholder.png"))
				must(err)
				must(png.Encode(f, img))
				must(f.Close())
			}
			done = true
			log.Print("PASS placeholder pixels/clip, values/history, focus, shared views, wrapping, themes/font, readonly/disabled and remount")
			timer.Stop()
			if !*inspect {
				win.Destroy()
			}
		}
		stage++
	})
	must(timer.Start(150 * time.Millisecond))
	app.Run()
	check(done, "window closed before verification completed")
}

type painterProbe struct {
	*gui.Label
	reported bool
}

func (p *painterProbe) Paint(painter gui.Painter) {
	if !p.reported {
		p.reported = true
		img, err := painter.NewImage(image.NewRGBA(image.Rect(0, 0, 1, 1)))
		must(err)
		log.Printf("painter image=%T scale=%g", img, painter.PixelScale())
		img.Destroy()
	}
	p.Label.Paint(painter)
}
