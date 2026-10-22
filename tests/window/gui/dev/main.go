// DevServer reference, text and Popover window checks.
//
// Environment: Linux/X11, Windows or macOS desktop with native font libraries.
// Record backend and 1x/2x scale. Run from the repository root:
//
//	GOUI_DEV_ADDR=:8888 go run ./tests/window/gui/dev
//
// PowerShell: $env:GOUI_DEV_ADDR=':8888'; go run ./tests/window/gui/dev
// -verify checks this protocol and closes the window; -addr selects the server URL.
// Use GOUI_PLAT_PAINTER=software/opengl (also direct2d on Windows) and
// GOUI_PLAT_SCALE=2 for 2x. Initially the Native window has no explicit Window
// or Widget IDs; fields contain single-line abc, multiline line1/line2 and readonly text.
// Steps and expected results:
//  1. GET http://127.0.0.1:8888/goui/dev/snapshot: Window and semantic nodes have
//     distinct nonempty refs and empty IDs. The Window ref survives later snapshots.
//  2. POST pointer_down/up events to the Window ref at main-editor's Bounds.
//     POST /text with {"target":"window-ref","text":"\u4f60👋é"} inserts at the
//     caret or replaces selection. Send Ctrl+A and Ctrl+Z through /event (Command
//     on macOS) to check replacement and undo. Empty /text does not delete selection.
//     Event completion is not paint completion; reread the snapshot to confirm.
//  3. Commit to the readonly field: ok=false and neither it nor main-editor changes.
//     A real nonempty IME composition also rejects submission without discarding
//     preedit; submission resumes after commit/cancel. -verify does not test real IME.
//  4. Show the modal input panel: Window.popovers includes a separate ref and
//     subtree in local DIP coordinates. Click popup-editor through that Popover's
//     Bounds/ref; text changes only there, and submission through the Window ref
//     fails. Ctrl/Command+A and Esc use its modal dispatch. Closing invalidates
//     the ref; reopening produces a new one.
//  5. Open the built-in menu: it appears in the snapshot even though the demo
//     does not own its Popover. Clicking the action's Bounds increments menu
//     once and closes it. This step verifies pointer activation only.
//  6. Submit multiline text to the multiline editor: snapshot and display retain
//     newlines. Single-line input normalizes them to spaces. Unknown refs, Widget
//     refs and a missing text property all fail explicitly.
//
// Platform differences: Popovers have no native keyboard focus; programmatic
// submission here does not verify native IME delegation to Popovers.
// Side effects: creates native windows and opens the GOUI_DEV_ADDR HTTP service;
// no clipboard or file access. This verifies DevServer itself, not other widgets.
// Reset: restart. Exit: close the main window. Automatic failure prints FAIL and
// exits nonzero. Restore the previous environment values afterward.
package main

import (
	"flag"
	"fmt"
	"image"
	"log"
	"os"
	"runtime"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	_ "github.com/golang-gui/goui/gui/dev"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/theme/modern"
)

type backendProbe struct {
	gui.WidgetBase
	reported bool
}

func (probe *backendProbe) Paint(p gui.Painter) {
	if probe.reported {
		return
	}
	probe.reported = true
	resource, err := p.NewImage(image.NewRGBA(image.Rect(0, 0, 1, 1)))
	if err != nil {
		panic(err)
	}
	fmt.Printf("backend=%T pixel-scale=%g\n", resource, p.PixelScale())
	resource.Destroy()
}

func main() {
	runtime.LockOSThread()
	verify := flag.Bool("verify", false, "verify the live dev protocol and exit")
	addr := flag.String("addr", "http://127.0.0.1:8888", "dev server URL")
	flag.Parse()
	if *verify && os.Getenv("GOUI_DEV_ADDR") == "" {
		log.Fatal("-verify requires GOUI_DEV_ADDR before starting the process")
	}
	app, err := gui.NewApplication("org.golang-gui.DevProtocolTest")
	if err != nil {
		log.Fatal(err)
	}
	app.SetStyleSheet(modern.Sheet(modern.Options{}))
	win, err := app.NewWindow(&gui.WindowOptions{Size: geometry.Size{Width: 640, Height: 480}})
	if err != nil {
		log.Fatal(err)
	}
	defer win.Destroy()
	if err := win.SetTitle("GOUI Dev protocol — no IDs"); err != nil {
		log.Fatal(err)
	}
	root := gui.NewLinearBox(layout.DirectionVertical)
	root.SetPadding(16)
	root.SetSpacing(10)
	root.SetCrossAlign(layout.CrossStretch)
	root.AddChild(&backendProbe{})
	root.AddChild(gui.NewLabel("dev ref：窗口、模态面板、内置菜单和 Unicode 文本"))
	input := gui.NewTextInput()
	input.SetName("main-editor")
	input.SetText("abc")
	root.AddChild(input)
	readOnly := gui.NewTextInput()
	readOnly.SetName("readonly-editor")
	readOnly.SetText("read-only")
	readOnly.SetReadOnly(true)
	root.AddChild(readOnly)
	editor := gui.NewTextView()
	editor.SetName("multiline-editor")
	editor.SetModel(gui.NewTextModel("line1\nline2"))
	editor.SetMainWeight(1)
	root.AddChild(editor)
	open := gui.NewButton()
	open.SetName("show-popup")
	open.SetChild(gui.NewLabel("显示模态输入面板"))
	p := gui.NewPopover(open, nil)
	defer p.Destroy()
	p.SetModal(true)
	p.SetPlacement(gui.PopoverPlacementBottom)
	panel := gui.NewLinearBox(layout.DirectionVertical)
	panel.SetPadding(12)
	panel.SetSpacing(8)
	panel.SetCrossAlign(layout.CrossStretch)
	panel.SetMinSize(geometry.Size{Width: 320})
	panel.AddChild(gui.NewLabel("已确认文本，不模拟原生 IME"))
	popupInput := gui.NewTextInput()
	popupInput.SetName("popup-editor")
	popupInput.SetText("popup")
	panel.AddChild(popupInput)
	close := gui.NewButton()
	close.SetName("close-popup")
	close.SetChild(gui.NewLabel("关闭面板"))
	close.ConnectClicked(p.Hide)
	panel.AddChild(close)
	p.SetWidget(panel)
	p.ConnectDismissRequest(p.Hide)
	open.ConnectClicked(func() {
		if err := p.Show(); err != nil {
			panic(err)
		}
	})
	root.AddChild(open)
	menuButton := gui.NewMenuButton()
	menuButton.SetName("show-menu")
	menuButton.SetChild(gui.NewLabel("内置菜单"))
	menu := gui.NewMenu()
	count := 0
	status := gui.NewLabel("menu=0")
	status.SetName("menu-count")
	menu.Append("菜单操作", func() { count++; status.SetText(fmt.Sprintf("menu=%d", count)) })
	menu.Append("另一操作", func() {})
	menuButton.SetMenu(menu)
	root.AddChild(menuButton)
	root.AddChild(status)
	win.SetWidget(root)
	if err := win.Show(); err != nil {
		log.Fatal(err)
	}
	var result chan error
	if *verify {
		result = make(chan error, 1)
		go func() {
			result <- verifyProtocol(*addr)
			app.Post(win.Destroy)
		}()
	}
	app.Run()
	if *verify {
		if err := <-result; err != nil {
			log.Fatalf("FAIL: %v", err)
		}
		fmt.Println("PASS: native Window/Popover dev input, Unicode, undo and refs")
	}
}
