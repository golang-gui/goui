// Detach a TabView page into a new window, without DevServer.
//
// Environment: interactive Linux/X11, Windows or macOS. Record OS, WM, actual
// backend and scale. Run from the repository root:
//
//	go run ./tests/window/widgets/tab_detach
//
// Initially one source window has A/B; A's input contains retained text.
// Default Native chrome and continuous mode; -chrome integrated checks HeaderBar.
// In continuous mode detach the first tab, then drop the source's last tab on the
// desktop: it returns without creating a window or exiting. That last tab can
// still transfer into an existing window; an empty source window stays open.
// The dragging source tab hides but retains its slot and visible page content;
// target tabs animate around a slot, without a vertical insertion line. Add tab
// allows further detachments, including from new windows. Native preview must
// not replace the pointer; Esc retains the original page. Close all windows to exit.
// For strict checks below use -expect detached and verify after one transfer.
// Steps and expected results:
//  1. Activate the source and drag Document A's middle to blank desktop outside
//     every test window. Release: A enters a new window, source retains B and
//     original text remains. Target size is source content plus this example's
//     layout overhead; position/transfer precede show. No visible default-position
//     move or transient source-tab restoration is allowed. Initial positioning
//     subtracts known bar origin/hotspot; arbitrary layouts and cross-screen DPI
//     pixel alignment are outside this check.
//  2. Verify and exit checks original page/input identity, mount counts, text,
//     selection and position, then destroys the source and checks target survival.
//     Success prints PASS and exits.
//  3. Restart -expect canceled. Leave the bar until native preview appears,
//     keep the left button held, Esc, release, then Verify. No new-window request;
//     A/B remain, Mount=1/Unmount=0. Verify without a real canceled gesture does
//     not exercise cancellation.
//  4. Restart -expect prepare-failure and repeat step 1. Simulated preparation
//     failure closes the still-hidden empty target before it receives a page;
//     source/mount state remains unchanged. This does not inject native creation failure.
//
// Optional rejection cases, each in a fresh run:
//  1. -expect rejected adds REJECT TARGET. Drop A there, then Verify: negotiation
//     refuses Drop, no detach request, unchanged source identity/mount counts.
//  2. -reject-target implies strict detached mode. Enter REJECT TARGET, keep
//     holding, leave all test windows and release on desktop, then Verify:
//     detachment succeeds. Both cases assert an actual rejected-target visit.
//
// Logical 2x: set GOUI_PLAT_SCALE=2 (PowerShell: $env:GOUI_PLAT_SCALE='2') before
// running and restore afterward; this does not verify mixed DPI. Activate AppKit
// windows first. Detachable sessions suppress failed-drop snapback; Esc still
// restores source. Windows uses an ordinary pointer outside windows but native
// accepted/rejected feedback over registered targets. Linux Integrated needs a
// compositor; record fallback. Do not move source during preparation: this makes
// the request stale. Native positioning/content-size tolerance is 1 DIP for
// integral rounding; the system may constrain requests.
// Strict Verify checks objects, ownership, text, lifecycle and position: PASS on
// success, FAIL/nonzero on failure or incomplete action. Inspect preview/release
// position independently. Side effects: only this program's windows, no settings
// or clipboard. Reset: restart. Exit: all windows in continuous mode, Verify in
// strict mode; direct close there is not verification. This covers imperative
// preparation/transfer; see ../ui_tab_transfer for declarative subtrees.
package main

import (
	"flag"
	"fmt"
	"math"
	"os"
	"runtime"
	"slices"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/theme/modern"
	"github.com/golang-gui/goui/widgets"
)

// Record fixed document-window overhead during its first normal layout.
// Detach reuses content size plus this example's caption/buttons/margins,
// without measuring a hidden target.
type frameProbe struct {
	gui.WidgetBase
	view                  *widgets.TabView
	initialSize, overhead geometry.Size
	sized                 bool
}

func (p *frameProbe) Arrange(rect geometry.Rectangle) {
	p.WidgetBase.Arrange(rect)
	if !p.sized && p.view.Rect().Width > 0 && p.view.Rect().Height > 0 {
		viewport := p.view.Rect().Size
		// Content minimums may enlarge the first allocation, so read existing DIP
		// bounds rather than treating requested size as actual. max retains native
		// border overhead in custom chrome without counting enlargement as negative
		// overhead or observing/laying out a hidden target.
		bounds := p.Window().Snapshot().Bounds.Size
		p.overhead = geometry.Size{Width: max(bounds.Width, p.initialSize.Width) - viewport.Width, Height: max(bounds.Height, p.initialSize.Height) - viewport.Height}
		p.sized = true
	}
}

func main() {
	runtime.LockOSThread()
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL:", err)
		os.Exit(1)
	}
}

func run() error {
	expect := flag.String("expect", "interactive", "interactive, detached, canceled, rejected or prepare-failure")
	rejectTarget := flag.Bool("reject-target", false, "also show a target rejecting native negotiation")
	chrome := flag.String("chrome", "native", "native or integrated")
	flag.Parse()
	if *expect == "interactive" && *rejectTarget {
		*expect = "detached"
	}
	if !slices.Contains([]string{"interactive", "detached", "canceled", "rejected", "prepare-failure"}, *expect) {
		return fmt.Errorf("invalid expectation")
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
	app, err := gui.NewApplication("org.golang-gui.TabDetachProbe")
	if err != nil {
		return err
	}
	app.SetStyleSheet(modern.Sheet(modern.Options{}))
	options := &gui.WindowOptions{Size: geometry.Size{Width: 480, Height: 320}, Chrome: decoration}
	source, err := app.NewWindow(options)
	if err != nil {
		return err
	}
	defer source.Destroy()
	var target gui.Window
	defer func() {
		if target != nil {
			target.Destroy()
		}
	}()
	_ = source.SetTitle("GOUI DETACH SOURCE")
	source.ConnectCloseRequest(func(*bool) { app.Quit() })
	input := gui.NewTextInput()
	input.SetText("retained text")
	text := input.Text()
	input.ConnectText(func(value string) { text = value })
	a, b := widgets.NewTabPage("Document A", input), widgets.NewTabPage("Document B", gui.NewLabel("B retained"))
	sv := widgets.NewTabView()
	sv.AppendPage(a)
	sv.AppendPage(b)
	mounts, unmounts, requests := 0, 0, 0
	rejectVisits, rejectDrops := 0, 0
	input.ConnectMount(func() { mounts++ })
	input.ConnectUnmount(func() { unmounts++ })
	var failure error
	verified := false
	fail := func(err error) {
		if failure == nil {
			failure = err
		}
		app.Post(app.Quit)
	}
	var tv *widgets.TabView
	var requestedPosition geometry.Point
	var contentSize geometry.Size
	verify := func() {
		if (*rejectTarget || *expect == "rejected") && (rejectVisits == 0 || rejectDrops != 0) {
			fail(fmt.Errorf("expected negotiation refusal: visits=%d drops=%d", rejectVisits, rejectDrops))
			return
		}
		if *expect == "detached" {
			if requests != 1 || target == nil || tv == nil || a.Parent() != tv || a.Child() != input || input.Text() != text ||
				!slices.Equal(sv.Pages(), []*widgets.TabPage{b}) || !slices.Equal(tv.Pages(), []*widgets.TabPage{a}) || tv.Current() != a || mounts != 2 || unmounts != 1 {
				fail(fmt.Errorf("identity/order/lifecycle: requests=%d mounts=%d unmounts=%d", requests, mounts, unmounts))
				return
			}
			origin, err := target.Position(source)
			if err != nil {
				fail(err)
				return
			}
			self, err := target.Position(target)
			if err != nil {
				fail(err)
				return
			}
			if self != (geometry.Point{}) {
				fail(fmt.Errorf("client self-position=%v, want zero", self))
				return
			}
			if math.Abs(float64(origin.X-requestedPosition.X)) > 1 || math.Abs(float64(origin.Y-requestedPosition.Y)) > 1 {
				fail(fmt.Errorf("position=%v requested=%v", origin, requestedPosition))
				return
			}
			actualSize := tv.Rect().Size
			if math.Abs(float64(actualSize.Width-contentSize.Width)) > 1 || math.Abs(float64(actualSize.Height-contentSize.Height)) > 1 {
				fail(fmt.Errorf("content size=%v requested=%v", actualSize, contentSize))
				return
			}
			fmt.Printf("position=%v content=%v\n", origin, actualSize)
			source.Destroy()
			if a.Destroyed() || a.Root() != target || input.Text() != text {
				fail(fmt.Errorf("source destruction affected target"))
				return
			}
		} else {
			wantRequests := 0
			if *expect == "prepare-failure" {
				wantRequests = 1
			}
			if requests != wantRequests || target != nil || !slices.Equal(sv.Pages(), []*widgets.TabPage{a, b}) || a.Child() != input || input.Text() != text || mounts != 1 || unmounts != 0 {
				fail(fmt.Errorf("canceled/failed preparation changed source: requests=%d mounts=%d unmounts=%d", requests, mounts, unmounts))
				return
			}
		}
		verified = true
		app.Post(app.Quit)
	}
	build := func(view *widgets.TabView, label string, size geometry.Size) (*frameProbe, *widgets.TabBar) {
		bar := widgets.NewTabBar()
		bar.SetView(view)
		bar.SetReorderable(true)
		bar.SetTransferable(true)
		bar.ConnectTransferError(fail)
		button := gui.NewButton()
		button.SetChild(gui.NewLabel("Verify and exit"))
		button.ConnectClicked(verify)
		column := &frameProbe{view: view, initialSize: size}
		column.SetLayoutManager(&layout.LinearLayout{Direction: layout.DirectionVertical, CrossAlign: layout.CrossStretch, Padding: 12, Spacing: 8})
		column.WidgetBase.AddChild(column, bar)
		column.WidgetBase.AddChild(column, gui.NewLabel(label))
		column.WidgetBase.AddChild(column, button)
		column.WidgetBase.AddChild(column, view)
		return column, bar
	}
	content, sb := build(sv, "Drag A onto empty desktop, then Verify", options.Size)
	sb.ConnectDetachRequest(func(request *widgets.TabDetachRequest, handled *bool) {
		requests++
		if *expect == "rejected" {
			request.Cancel()
			fail(fmt.Errorf("local negotiation rejection triggered detach"))
			return
		}
		if requests != 1 {
			request.Cancel()
			fail(fmt.Errorf("duplicate detach request"))
			return
		}
		contentSize = request.ContentSize
		targetOptions := *options
		targetOptions.Size = contentSize.Add(content.overhead)
		// Source and target use equal bar margins; use the known bar origin for
		// initial target positioning.
		requestedPosition = request.Position.Add(sb.Snapshot().Bounds.Pos.Add(request.Hotspot).Scale(-1))
		sourceOrigin, err := source.Position(nil)
		if err != nil {
			request.Cancel()
			fail(err)
			return
		}
		target, err = app.NewWindow(&targetOptions)
		if err != nil {
			request.Cancel()
			fail(err)
			return
		}
		_ = target.SetTitle("GOUI DETACHED TARGET")
		*handled = true
		target.ConnectCloseRequest(func(*bool) { request.Cancel(); app.Quit() })
		tv = widgets.NewTabView()
		targetContent, _ := build(tv, "Retained page in new window", targetOptions.Size)
		target.SetWidget(targetContent)
		abort := func(err error) { request.Cancel(); target.Destroy(); target = nil; fail(err) }
		prepare := func() {
			observed, err := source.Position(nil)
			if err != nil {
				abort(err)
				return
			}
			if observed != sourceOrigin {
				abort(fmt.Errorf("source moved during preparation"))
				return
			}
			if err := target.SetPosition(source, requestedPosition); err != nil {
				abort(err)
				return
			}
			if err := request.TransferTo(tv, 0); err != nil {
				abort(err)
				return
			}
			fmt.Printf("prepared target: position=%v content=%v\n", requestedPosition, contentSize)
		}
		if *expect == "prepare-failure" {
			request.Cancel()
			target.Destroy()
			target = nil
			fmt.Println("injected preparation failure; source retained")
			return
		}
		prepare()
		if target != nil {
			if err := target.Show(); err != nil {
				// Preserve a target that has received the original page even if Show fails.
				fail(err)
			}
		}
	})
	source.SetWidget(content)
	if err := source.Show(); err != nil {
		return err
	}
	if err := source.SetPosition(nil, geometry.Point{X: 50, Y: 100}); err != nil {
		return err
	}
	if *rejectTarget || *expect == "rejected" {
		reject, err := app.NewWindow(&gui.WindowOptions{Size: geometry.Size{Width: 340, Height: 200}, Chrome: decoration})
		if err != nil {
			return err
		}
		defer reject.Destroy()
		_ = reject.SetTitle("GOUI REJECT TARGET")
		reject.ConnectCloseRequest(func(*bool) { app.Quit() })
		label := gui.NewLabel("Native negotiation rejects here")
		drop := gui.NewDropTarget(gui.LocalFormat("goui.widgets.tab-page"))
		drop.SetActions(gui.DragMove)
		refuse := func(e *gui.DragMotion) { rejectVisits++; e.Action = 0 }
		drop.ConnectEnter(refuse)
		drop.ConnectMotion(refuse)
		drop.ConnectDrop(func(*gui.DropRequest) { rejectDrops++ })
		label.AddEventController(drop)
		reject.SetWidget(label)
		if err := reject.Show(); err != nil {
			return err
		}
		if err := reject.SetPosition(source, geometry.Point{X: 540, Y: 0}); err != nil {
			return err
		}
	}
	fmt.Printf("OS=%s painter=%s scale=%s chrome=%s expect=%s\n", runtime.GOOS, os.Getenv("GOUI_PLAT_PAINTER"), os.Getenv("GOUI_PLAT_SCALE"), *chrome, *expect)
	app.Run()
	if failure != nil {
		return failure
	}
	if !verified {
		return fmt.Errorf("closed without verification")
	}
	fmt.Printf("PASS %s requests=%d mounts=%d unmounts=%d text=%q rejection-visits=%d drops=%d\n", *expect, requests, mounts, unmounts, text, rejectVisits, rejectDrops)
	return nil
}
