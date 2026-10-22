// Cross-window TabView page transfer checks, without DevServer.
//
// Environment: interactive Linux/X11, Windows or macOS. Record OS, actual backend,
// WM and scale. Run from the repository root:
//
//	go run ./tests/window/widgets/tab_transfer
//
// Default continuous mode starts with two pages on each side and permits repeated
// transfers both ways, including a source's last page. The source tab hides while
// its slot/content remain; target tabs animate aside to form a stable slot rather
// than a vertical line. Leaving/Esc retracts the slot and restores source. Dropping
// a last tab on the desktop creates no window; successful transfer leaves the
// empty source window open. Add tab remains available. Linux must retain both
// system pointer and independent preview outside the bar; Esc retains ownership.
// Close all windows to exit; continuous mode does not report automatic PASS.
// Use -expect accepted for strict checks. -editor, -same-window or nondefault
// -target also select strict accepted mode implicitly.
// Strict initial state: two Native windows, source A/B and target T, with retained
// text in A's input. Both bars explicitly enable Reorderable and Transferable.
// Strict steps and expected results (-expect accepted):
//  1. Activate source, optionally edit A, then drag A outside its bar and onto
//     T's left half. Observe the insertion slot, then release. Source falls back
//     to B; target selects A. Preview retains hotspot and original text remains;
//     dragging neither moves the window nor causes an extra click.
//  2. Verify and exit on the target asserts original page/input identity, text,
//     ownership, selection and one Unmount/Mount, then destroys source and checks
//     target survival before exiting.
//  3. Restart -expect canceled. Leave source until preview appears, keep holding
//     into target, Esc, release, then Verify on the right. A/B and T stay unchanged
//     with no extra lifecycle calls. Unchanged state without an actual canceled
//     gesture does not verify this path.
//
// Optional cases, one fresh run per mode; exhaustive combinations are unnecessary:
//  1. -expect target-destroy: step 1 destroys target during A's Unmount. A returns
//     to its original selected source slot/text. Verify on the left: one unmount
//     and remount, one transfer error, no new-window detach request.
//  2. -expect source-destroy: selection notification destroys source after A
//     belongs to target. It stays editable; Verify on the right confirms success
//     was not rolled back and no transfer error occurred.
//  3. -same-window: transfer between two TabViews in one window; Verify does not
//     destroy their shared owner.
//  4. -target empty: an empty target retains a receiving bar row. Drop there:
//     it contains only original A and selects it. May combine with -same-window.
//  5. -target overflow: T1..T8. Hold A just left of the right scroll arrow without
//     moving, wait for autoscroll to T8, and release with the slot after T8.
//     Verify appends/selects A and retains T1..T8 order. Do not release midway;
//     hovering must reveal hidden tabs. May combine with -same-window or canceled.
//     An explicit minimum prevents full-row desired width from enlarging the window.
//  6. -same-window -target mirror: both bars show one A/B/C TabView, content on
//     the left only. Drag A down out of the left and onto B's right half in the
//     right bar, before C (not after the last tab). Both become B/A/C, A selected,
//     with no input unmount/remount. Verify expects one Moved(0,1), no CloseRequest.
//     With -expect canceled use Esc: A/B/C unchanged, Moved=0.
//  7. -chrome integrated: bars inside HeaderBars transfer tabs rather than moving
//     the window; empty caption space still moves it. Linux needs composition;
//     at 2x a same-window desktop of at least 2200x900 physical pixels is recommended.
//  8. -editor uses editor.go's selection/scroll/undo/redo/input-after-source-close
//     steps. Only accepted/canceled; do not combine with same-window/target options.
//  9. -target relayout: T1/T2/T3 start at 100 DIP and widen to 140 on native drag
//     entry. Hold A about 175 DIP from the left edge (old T2 right half, new T2
//     left half), wait for width/slot updates and release without further motion.
//     Verify expects T1/A/T2/T3, not stale T1/T2/A/T3. Canceled retains original
//     order. RELAYOUT must appear before cancellation/drop to cover this case.
//  10. After entering as in case 9, hold over 35 seconds, then drop or Esc.
//     No page moves or session timeout while waiting; results match case 9.
//     Record the hold duration independently; this program does not assert it.
//
// Logical 2x: set GOUI_PLAT_SCALE=2 (PowerShell: $env:GOUI_PLAT_SCALE='2') and
// restore afterward; mixed DPI is not covered. On macOS activate source through
// noncontrol space, and target if needed before Verify. Native previews/snapback
// vary; record actual chrome/backend. Strict Verify prints PASS and exits for
// valid identity/order/selection/lifecycle, FAIL/nonzero otherwise. Independently
// inspect preview, slot animation and accidental window movement.
// Side effects: these windows and in-memory state only, no clipboard/files/settings.
// Reset: restart. Exit: all windows in continuous mode; strict Verify exits
// automatically, and direct close is incomplete. New-window detach and declarative
// subtree transfer are covered by ../tab_detach and ../ui_tab_transfer.
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
	"github.com/golang-gui/goui/theme/modern"
	"github.com/golang-gui/goui/widgets"
)

func main() {
	runtime.LockOSThread()
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL:", err)
		os.Exit(1)
	}
}

func run() error {
	expect := flag.String("expect", "interactive", "interactive, accepted, canceled, target-destroy or source-destroy")
	sameWindow := flag.Bool("same-window", false, "two tab views in one native window")
	targetMode := flag.String("target", "single", "single, empty, overflow, relayout or mirror (requires -same-window)")
	chrome := flag.String("chrome", "native", "native or integrated")
	editor := flag.Bool("editor", false, "editor page, history and viewport retention")
	flag.Parse()
	if *expect == "interactive" && (*sameWindow || *targetMode != "single" || *editor) {
		*expect = "accepted"
	}
	if !slices.Contains([]string{"interactive", "accepted", "canceled", "target-destroy", "source-destroy"}, *expect) {
		return fmt.Errorf("invalid expectation")
	}
	if !slices.Contains([]string{"single", "empty", "overflow", "relayout", "mirror"}, *targetMode) ||
		(*targetMode == "mirror" && !*sameWindow) ||
		((*sameWindow || *targetMode != "single") && *expect != "accepted" && *expect != "canceled") {
		return fmt.Errorf("invalid target or combination with destruction mode")
	}
	decoration := gui.WindowChromeNative
	if *chrome == "integrated" {
		decoration = gui.WindowChromeIntegrated
	} else if *chrome != "native" {
		return fmt.Errorf("invalid chrome")
	}
	if *expect == "interactive" {
		return runInteractive(decoration)
	}
	if *editor {
		if *sameWindow || *targetMode != "single" || (*expect != "accepted" && *expect != "canceled") {
			return fmt.Errorf("editor supports only accepted/canceled in separate windows")
		}
		return runEditorTransfer(decoration, *expect == "canceled")
	}
	app, err := gui.NewApplication("org.golang-gui.TabTransferProbe")
	if err != nil {
		return err
	}
	app.SetStyleSheet(modern.Sheet(modern.Options{}))
	options := &gui.WindowOptions{Size: geometry.Size{Width: 480, Height: 320}, Chrome: decoration}
	if *sameWindow {
		options.Size.Width = 1000
	}
	source, err := app.NewWindow(options)
	if err != nil {
		return err
	}
	defer source.Destroy()
	// Permit actual overflow instead of deriving a WM minimum from all tabs.
	source.SetMinSize(geometry.Size{Width: 400, Height: 280})
	target := source
	if !*sameWindow {
		target, err = app.NewWindow(options)
		if err != nil {
			return err
		}
		defer target.Destroy()
		target.SetMinSize(geometry.Size{Width: 400, Height: 280})
		_ = target.SetTitle("GOUI tabs TARGET")
		target.ConnectCloseRequest(func(*bool) { app.Quit() })
	}
	_ = source.SetTitle("GOUI tabs SOURCE")
	source.ConnectCloseRequest(func(*bool) { app.Quit() })
	sv, tv := widgets.NewTabView(), widgets.NewTabView()
	input := gui.NewTextInput()
	input.SetText("retained text")
	a, b, c := widgets.NewTabPage("Document A", input), widgets.NewTabPage("Document B", gui.NewLabel("B retained")), widgets.NewTabPage("Target T", gui.NewLabel("T retained"))
	mounts, unmounts := 0, 0
	destructions, transferErrors, detachRequests := 0, 0, 0
	input.ConnectMount(func() { mounts++ })
	input.ConnectUnmount(func() {
		unmounts++
		if *expect == "target-destroy" && destructions == 0 {
			destructions++
			target.Destroy()
		}
	})
	sv.AppendPage(a)
	sv.AppendPage(b)
	switch *targetMode {
	case "single":
		tv.AppendPage(c)
	case "overflow":
		for i := 1; i <= 8; i++ {
			title := fmt.Sprintf("Target T%d", i)
			tv.AppendPage(widgets.NewTabPage(title, gui.NewLabel(title)))
		}
	case "relayout":
		for i := 1; i <= 3; i++ {
			tv.AppendPage(widgets.NewTabPage(fmt.Sprintf("T%d", i), nil))
		}
	case "mirror":
		c.SetTitle("Document C")
		sv.AppendPage(c)
		tv = sv
	}
	sourcePages := sv.Pages()
	targetPages, targetCurrent := tv.Pages(), tv.Current()
	moves, closes := 0, 0
	var moveError error
	sv.ConnectMoved(func(page *widgets.TabPage, from, to int) {
		if *targetMode == "mirror" && (page != a || from != 0 || to != 1) {
			moveError = fmt.Errorf("unexpected move: page=%q %d -> %d", page.Title(), from, to)
		}
		moves++
	})
	sv.ConnectCloseRequest(func(*widgets.TabPage) { closes++ })
	sv.ConnectCurrent(func(page *widgets.TabPage) {
		if *expect == "source-destroy" && page == b && destructions == 0 {
			destructions++
			source.Destroy()
		}
	})
	text := "retained text"
	input.ConnectText(func(value string) { text = value })
	var failure error
	verified := false
	relayout := false
	check := gui.NewButton()
	check.SetChild(gui.NewLabel("Verify and exit"))
	check.ConnectClicked(func() {
		wantOwner, wantMounts, wantUnmounts := sv, 1, 0
		wantSource, wantTarget := slices.Clone(sourcePages), slices.Clone(targetPages)
		wantCurrent := targetCurrent
		wantSourceCurrent := a
		accepted := *expect == "accepted" || *expect == "source-destroy"
		if accepted {
			wantOwner, wantMounts, wantUnmounts = tv, 2, 1
			wantSource = []*widgets.TabPage{b}
			wantSourceCurrent = b
			index := 0
			if *targetMode == "overflow" {
				index = len(wantTarget)
			}
			if *targetMode == "relayout" {
				index = 1
			}
			wantTarget = slices.Insert(wantTarget, index, a)
			wantCurrent = a
			if *targetMode == "mirror" {
				wantOwner, wantMounts, wantUnmounts = sv, 1, 0
				wantSource, wantTarget = []*widgets.TabPage{b, a, c}, []*widgets.TabPage{b, a, c}
				wantSourceCurrent = a
			}
		}
		if *expect == "target-destroy" {
			wantMounts, wantUnmounts = 2, 1
			wantTarget = nil
		}
		if *expect == "source-destroy" {
			wantSource = nil
		}
		if a.Parent() != wantOwner || a.Child() != input || input.Text() != text || mounts != wantMounts || unmounts != wantUnmounts ||
			!slices.Equal(sv.Pages(), wantSource) || !slices.Equal(tv.Pages(), wantTarget) ||
			(!sv.Destroyed() && sv.Current() != wantSourceCurrent) ||
			(!tv.Destroyed() && tv.Current() != wantCurrent) || a.Destroyed() {
			failure = fmt.Errorf("identity/order/lifecycle mismatch: parent=%T mounts=%d unmounts=%d text=%q", a.Parent(), mounts, unmounts, input.Text())
		} else if accepted && !*sameWindow {
			source.Destroy()
			if a.Root() != target || a.Destroyed() || input.Text() != text {
				failure = fmt.Errorf("source destruction affected moved page")
			}
		}
		wantDestructions, wantErrors := 0, 0
		if *expect == "target-destroy" || *expect == "source-destroy" {
			wantDestructions = 1
		}
		if *expect == "target-destroy" {
			wantErrors = 1
			if !tv.Destroyed() || sv.Current() != a || !a.Visible() || b.Visible() || a.Root() != source {
				failure = fmt.Errorf("rollback lost source selection or root")
			}
		}
		if *expect == "source-destroy" && !sv.Destroyed() {
			failure = fmt.Errorf("source survived its destruction callback")
		}
		if destructions != wantDestructions || transferErrors != wantErrors || detachRequests != 0 {
			failure = fmt.Errorf("destructions=%d errors=%d detach=%d", destructions, transferErrors, detachRequests)
		}
		wantMoves := 0
		if *targetMode == "mirror" && accepted {
			wantMoves = 1
		}
		if moves != wantMoves || closes != 0 {
			failure = fmt.Errorf("moves=%d want=%d closes=%d", moves, wantMoves, closes)
		}
		if moveError != nil {
			failure = moveError
		}
		if *targetMode == "relayout" && !relayout {
			failure = fmt.Errorf("target layout was not changed during native drag")
		}
		verified = true
		app.Post(app.Quit)
	})
	build := func(view *widgets.TabView, extra gui.Widget, includePage bool) gui.Widget {
		bar := widgets.NewTabBar()
		bar.SetView(view)
		bar.SetReorderable(true)
		bar.SetTransferable(true)
		if *targetMode == "relayout" && view == tv {
			bar.SetTabWidthRange(100, 100)
			// Subscribe after the bar's normal Enter handler, so that the
			// original insertion marker exists before requesting new layout.
			for _, controller := range bar.EventControllers() {
				if drop, ok := controller.(*gui.DropTarget); ok {
					drop.ConnectEnter(func(*gui.DragMotion) {
						if !relayout {
							relayout = true
							bar.SetTabWidthRange(140, 140)
							fmt.Println("RELAYOUT target tabs 100 -> 140 DIP; drop BEFORE T2")
						}
					})
				}
			}
		}
		bar.ConnectTransferError(func(err error) {
			transferErrors++
			if *expect == "target-destroy" && destructions == 1 {
				fmt.Printf("EXPECTED transfer error: %v\n", err)
				return
			}
			failure = err
			app.Post(app.Quit)
		})
		bar.ConnectDetachRequest(func(request *widgets.TabDetachRequest, handled *bool) {
			detachRequests++
			request.Cancel()
		})
		column := gui.NewLinearBox(layout.DirectionVertical)
		column.SetMainWeight(1)
		column.SetCrossAlign(layout.CrossStretch)
		column.SetSpacing(8)
		column.SetPadding(12)
		if decoration == gui.WindowChromeIntegrated {
			header := gui.NewHeaderBar()
			header.SetChild(bar)
			column.AddChild(header)
		} else {
			column.AddChild(bar)
		}
		column.AddChild(extra)
		if includePage {
			column.AddChild(view)
		}
		return column
	}
	instruction := "Drag A to the left half of T"
	if *targetMode == "empty" {
		instruction = "Drop A into the empty top bar"
	} else if *targetMode == "overflow" {
		instruction = "Hover at right edge; drop AFTER T8"
	} else if *targetMode == "mirror" {
		instruction = "Drop A AFTER B in the right bar"
	} else if *targetMode == "relayout" {
		instruction = "Hold over T2; wait for resize; drop BEFORE T2"
	}
	if *sameWindow {
		row := gui.NewLinearBox(layout.DirectionHorizontal)
		row.SetCrossAlign(layout.CrossStretch)
		row.AddChild(build(sv, gui.NewLabel(instruction), true))
		row.AddChild(build(tv, check, *targetMode != "mirror"))
		source.SetWidget(row)
	} else if *expect == "target-destroy" {
		source.SetWidget(build(sv, check, true))
		target.SetWidget(build(tv, gui.NewLabel("Target closes during A Unmount"), true))
	} else {
		source.SetWidget(build(sv, gui.NewLabel(instruction), true))
		target.SetWidget(build(tv, check, true))
	}
	if err = source.Show(); err != nil {
		return err
	}
	if err = source.SetPosition(nil, geometry.Point{X: 50, Y: 100}); err != nil {
		return err
	}
	if !*sameWindow {
		if err = target.Show(); err != nil {
			return err
		}
		if err = target.SetPosition(nil, geometry.Point{X: 600, Y: 100}); err != nil {
			return err
		}
	}
	fmt.Printf("OS=%s painter=%s scale=%s chrome=%s expect=%s target=%s same-window=%v\n", runtime.GOOS, os.Getenv("GOUI_PLAT_PAINTER"), os.Getenv("GOUI_PLAT_SCALE"), *chrome, *expect, *targetMode, *sameWindow)
	app.Run()
	if failure != nil {
		return failure
	}
	if !verified {
		return fmt.Errorf("closed without verification")
	}
	fmt.Printf("PASS %s mounts=%d unmounts=%d destructions=%d errors=%d detach=%d moves=%d closes=%d text=%q\n", *expect, mounts, unmounts, destructions, transferErrors, detachRequests, moves, closes, text)
	return nil
}
