// GUI drag-and-drop window checks, without DevServer.
//
// Environment: Linux/X11, Windows or macOS desktop. Record OS, Linux WM, actual
// backend and scale. Use GOUI_PLAT_PAINTER=software/opengl (also direct2d on Windows)
// and GOUI_PLAT_SCALE=1 or 2; requested values do not prove the actual backend.
// Run: go run ./tests/window/gui/dragdrop -second-window -popover
// PowerShell: $env:GOUI_PLAT_SCALE='2'; go run ./tests/window/gui/dragdrop
// Custom previews are enabled on all platforms; -preview=false checks native fallback.
// Repeat -file /absolute/path for files/directories; otherwise the program creates
// a temporary sample with Chinese characters and spaces in its name. -large-text
// offers 256 KiB text. -actions enables Copy/Move/Link for non-file sources;
// file sources always offer Copy only.
// Initially Local, Text, Files, URLs and MIME sources sit above their matching
// targets, with Reject, Prepare/Begin/Drop/End/Click/FAIL counts and the last result.
// -second-window adds matching targets; -popover adds a floating-panel toggle.
// Each source offers one format; verify Files and URLs in separate target areas.
// Steps and expected results:
//  1. Click a source: only Click increments. Drop on its matching target:
//     Prepare/Begin/Drop/End each increment once, Click stays unchanged and FAIL=0.
//     Local checks object identity; Text/URLs/MIME check fixed payloads; Files
//     checks the paths provided by the source.
//  2. Enable Reject and drop: no Drop and End action=0. Disable to accept again.
//     Esc or release outside a target adds no Drop/Click and reports action=0;
//     canceled=true only where the platform distinguishes cancellation. Retry afterward.
//  3. Drop in matching targets in the second window or visible Popover: results
//     match step 1. Hide the Popover to inspect status. Repeat without duplicate
//     Drop, unmatched Leave or FAIL.
//  4. Run two processes and transfer Text/URLs/MIME between matching targets:
//     fixed payloads show VERIFIED; Local must be rejected across processes.
//     With -large-text in both, transfer complete 256 KiB text in each direction
//     and verify it, including X11 INCR.
//  5. Drop Files into an empty file-manager directory; independently check copies
//     and that sources remain. Drop multiple files/directories back: the window
//     shows a count and the terminal lists absolute paths. Unmatched external
//     payloads show RECEIVED, not VERIFIED. Files/URLs need separate targets
//     because both can use text/uri-list on X11.
//  6. The default preview is blue/yellow with transparent edges and a black cross
//     hotspot: 48 DIP image, cross at (12,12) DIP under the pointer. Check logical
//     size, hotspot and transparency at 1x/2x and record backend sampling differences.
//     Restart with -preview=false: native fallback preserves drop results. With
//     -actions, use system modifiers and verify matching Drop/End actions;
//     an unreported action has not been verified.
//
// Platform differences: record native previews, modifier actions and file-manager
// behavior separately. Side effects: creates temporary sources and removes them
// on normal exit; supplied -file paths are unmodified, and no files/URLs open
// automatically. Finish file-manager copying before exiting and remove its copies.
// Reset: restart. Exit: close either main window; nonzero FAIL gives a nonzero exit.
package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/theme/modern"
)

type fileFlags []string

func (f *fileFlags) String() string { return strings.Join(*f, ", ") }
func (f *fileFlags) Set(value string) error {
	path, err := filepath.Abs(value)
	if err != nil {
		return err
	}
	if _, err := os.Stat(path); err != nil {
		return err
	}
	*f = append(*f, path)
	return nil
}

func main() {
	runtime.LockOSThread()
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	var files fileFlags
	flag.Var(&files, "file", "file/directory to offer; repeatable; default creates a temporary sample")
	large := flag.Bool("large-text", false, "offer exactly 256 KiB UTF-8 text")
	preview := flag.Bool("preview", true, "show a 48 DIP image with a marked 12 DIP hotspot")
	second := flag.Bool("second-window", false, "open a second drop target window")
	popup := flag.Bool("popover", false, "enable the modeless Popover target toggle")
	actions := flag.Bool("actions", false, "allow Copy/Move/Link for non-file sources")
	flag.Parse()
	if len(files) == 0 {
		dir, err := os.MkdirTemp("", "goui-dnd-")
		if err != nil {
			return err
		}
		defer os.RemoveAll(dir)
		path := filepath.Join(dir, "GOUI 中文 sample.txt")
		if err := os.WriteFile(path, []byte("GOUI drag-and-drop sample\n中文 / spaces / 100%\n"), 0600); err != nil {
			return err
		}
		files = append(files, path)
	}
	fmt.Printf("source files=%q; OS=%s; requested painter=%q scale=%q; custom preview=%t\n", files, runtime.GOOS, os.Getenv("GOUI_PLAT_PAINTER"), os.Getenv("GOUI_PLAT_SCALE"), *preview)
	app, err := gui.NewApplication("org.golang-gui.DragDropGUIProbe")
	if err != nil {
		return err
	}
	app.SetStyleSheet(modern.Sheet(modern.Options{}))
	win, err := app.NewWindow(&gui.WindowOptions{Size: geometry.Size{Width: 780, Height: 540}})
	if err != nil {
		return err
	}
	defer win.Destroy()
	if err := win.SetTitle("GOUI GUI DragDrop acceptance"); err != nil {
		return err
	}
	p := newProbe(*large)
	p.files = append([]string(nil), files...)
	root := gui.NewLinearBox(layout.DirectionVertical)
	root.SetPadding(16)
	root.SetSpacing(10)
	root.SetCrossAlign(layout.CrossStretch)
	root.AddChild(gui.NewLabel("Five independent formats. Local stays in this application."))
	sources := gui.NewLinearBox(layout.DirectionHorizontal)
	sources.SetSpacing(8)
	for _, name := range []string{"Local", "Text", "Files", "URLs", "MIME"} {
		button := gui.NewButton()
		button.SetID("source-" + strings.ToLower(name))
		button.SetChild(gui.NewLabel(name))
		button.SetMainWeight(1)
		button.SetMinSize(geometry.Size{Width: 90, Height: 90})
		button.ConnectClicked(func() { p.clicks++; p.report("Click %s", name) })
		source := gui.NewDragSource()
		if *actions && name != "Files" {
			source.SetActions(gui.DragCopy | gui.DragMove | gui.DragLink)
		}
		source.ConnectPrepare(func(e *gui.DragPrepare) {
			p.check(!p.active, "overlapping source session")
			p.active, p.begun = true, false
			p.prepares++
			p.clickStart, p.dropStart = p.clicks, p.drops
			p.dropAction = 0
			data := new(gui.DragData)
			switch name {
			case "Local":
				data.SetLocal("probe", p.local)
			case "Text":
				data.SetText(p.text)
			case "Files":
				data.SetFiles(files)
			case "URLs":
				data.SetURLs(probeURLs)
			case "MIME":
				data.SetBytes(probeMIME, probeBytes)
			}
			e.Data = data
			if *preview {
				e.Preview = gui.DragPreview{Image: previewImage(), Scale: 1,
					Hotspot: geometry.Point{X: 12, Y: 12}}
			}
			p.report("Prepare %s", name)
		})
		source.ConnectBegin(func() {
			p.check(p.active && !p.begun, "unexpected Begin")
			p.begun = true
			p.begins++
			p.report("Begin %s", name)
		})
		source.ConnectEnd(func(result gui.DragResult) {
			p.check(p.active, "duplicate End")
			p.check(p.begun || result.Err != nil, "End without Begin or startup error")
			p.check(p.clicks == p.clickStart, "drag also triggered Click")
			p.check(p.drops-p.dropStart <= 1, "one source generated multiple Drops")
			if p.dropAction != 0 {
				p.check(result.Action == p.dropAction, "Drop and End actions differ")
			}
			if result.Err != nil {
				p.check(false, result.Err.Error())
			}
			p.active = false
			p.ends++
			p.report("End %s: action=%d canceled=%t error=%v", name, result.Action, result.Canceled, result.Err)
		})
		button.AddEventController(source)
		sources.AddChild(button)
	}
	root.AddChild(sources)
	root.AddChild(p.targets("main"))
	reject := gui.NewButton()
	rejectLabel := gui.NewLabel("Reject: OFF (click to toggle)")
	reject.SetChild(rejectLabel)
	reject.ConnectClicked(func() {
		p.reject = !p.reject
		rejectLabel.SetText(fmt.Sprintf("Reject: %t (click to toggle)", p.reject))
		p.report("Reject=%t", p.reject)
	})
	toolbar := gui.NewLinearBox(layout.DirectionHorizontal)
	toolbar.SetSpacing(8)
	toolbar.AddChild(reject)
	if *popup {
		floating := gui.NewPopover(sources, nil)
		defer floating.Destroy()
		floating.SetPosition(geometry.Point{Y: 240})
		floating.SetWidget(p.targets("popover"))
		toggle := gui.NewButton()
		toggle.SetChild(gui.NewLabel("Show/Hide Popover"))
		toggle.ConnectClicked(func() {
			if floating.Visible() {
				floating.Hide()
				return
			}
			if err := floating.Show(); err != nil {
				p.check(false, err.Error())
			}
		})
		toolbar.AddChild(toggle)
	}
	root.AddChild(toolbar)
	root.AddChild(p.counts)
	root.AddChild(p.status)
	win.SetWidget(root)
	win.ConnectCloseRequest(func(*bool) { app.Quit() })
	if err := win.Show(); err != nil {
		return err
	}
	if *second {
		other, err := app.NewWindow(&gui.WindowOptions{Size: geometry.Size{Width: 780, Height: 220}})
		if err != nil {
			return err
		}
		defer other.Destroy()
		if err := other.SetTitle("GOUI second DropTarget"); err != nil {
			return err
		}
		other.SetWidget(p.targets("second"))
		other.ConnectCloseRequest(func(*bool) { app.Quit() })
		if err := other.Show(); err != nil {
			return err
		}
	}
	app.Run()
	if p.failures != 0 {
		return fmt.Errorf("DnD probe recorded %d failures", p.failures)
	}
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
