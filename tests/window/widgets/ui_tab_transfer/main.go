// Declarative TabView cross-window subtree transfer checks, without DevServer.
//
// Environment: interactive Linux/X11, Windows or macOS. Record OS, WM, actual
// backend and scale. Run from the repository root:
//
//	go run ./tests/window/widgets/ui_tab_transfer
//
// Initially source has A/B and target C; inputs contain retained A/B/C. Default
// Native chrome, -expect transferred. Run each mode independently from initial state.
// Steps and expected results:
//  1. Activate source and replace A's selected input text with edited A.
//  2. Drag Document A down out of its bar, then into target's bar and release.
//     Source keeps B; target displays original A with edited A, and TRANSFER
//     logs once. No replacement input is constructed to copy its text.
//  3. Rebuild on target retains A text/editability; edit to after move.
//  4. Verify and close source checks page/input/State identity, View Mount=1 and
//     Unmount=0, and text signal connections. Target survives source close; after
//     that UI rebuild completes, PASS prints and the program exits.
//  5. Restart -expect canceled without step 1: retain initial retained A. Drag
//     outside the source until preview appears, keep holding, Esc, release and
//     Verify either window. Zero transfers or extra GUI/View unmounts; source
//     retains A. Do not perform steps 3/4. Unchanged state without the actual
//     canceled gesture does not verify cancellation.
//  6. Restart -expect detached with only source. Edit as step 1 and drop A on
//     blank desktop outside every test window; do not move source or change scale
//     afterward. A genuinely empty target uses source content size plus this
//     example's overhead; position/transfer precede show. Follow steps 3/4.
//     No visible repositioning from native defaults or transient source-tab
//     restoration after release. Verify allows 1 DIP integral rounding for
//     position/content sizes, not cross-screen pixel alignment.
//
// Optional cases, one independent run per mode:
//  1. -expect prepare-failure or prepare-cancel: leave retained A unchanged,
//     drag it to desktop. Target stays hidden. failure uses an empty page with
//     the same ID to reject transfer; cancel cancels explicitly. Requests are
//     not reusable and source retains A/B. Source Verify checks no GUI/View
//     unmount. Do not move source during preparation.
//  2. -expect target-destroy: after steps 1/2, A's GUI Unmount destroys target.
//     Source restores A/B with A selected. Rebuild there, edit the original input
//     to after move, then Verify. Zero OnTransfer/DetachRequest, original State
//     and connections retained, GUI Mount/Unmount=2/1 and View Unmount=0. Removed
//     target declarations do not resurrect it on rebuild.
//  3. -expect source-destroy: A's GUI Mount in target destroys source. Follow
//     steps 3/4 on target: one OnTransfer, original State/input remain editable
//     through rebuilds, and source declarations disappear. Source View destruction
//     must not clear A's reconciliation records during handoff.
//  4. -same-window: source/target share one Root. Follow steps 1-4, dropping A on
//     C's right half for C/A order. Verify and remove source removes its container,
//     not the window, and target retains original page/State. -target-first puts
//     target on the left to check target-first updates. Only transferred/canceled
//     apply; canceled follows step 5 without editing retained A. With Integrated,
//     both bars are HeaderBar children; tab dragging must not move the window.
//     At 2x, at least 2400x900 physical pixels is recommended; this is not mixed DPI.
//
// -chrome integrated also checks caption/tab drag separation. Logical 2x:
// set GOUI_PLAT_SCALE=2 (PowerShell: $env:GOUI_PLAT_SCALE='2') and restore afterward.
// Platform differences: activate macOS first; detachable sessions suppress failed
// snapback while Esc restores source. Windows uses an ordinary pointer in valid
// desktop detach space and native rejection feedback over rejecting app targets.
// Linux Integrated needs composition; record fallback. Select all uses Control+A,
// Command+A on macOS. Verify asserts identities, connections, lifecycle and rebuild
// results: PASS on success, nonzero for errors/incomplete actions. Independently
// inspect previews, real typing and release position.
// Side effects: only test windows; no settings/clipboard/files. Reset: restart.
// Exit: Verify exits automatically; closing without verification returns failure.
package main

import (
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"slices"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/theme/modern"
	"github.com/golang-gui/goui/ui"
	"github.com/golang-gui/goui/widgets"
	wui "github.com/golang-gui/goui/widgets/ui"
)

type pageState struct {
	input                            *gui.TextInput
	connection                       signal.Handle
	updates, notifications, unmounts int
	last                             string
	guiMounts, guiUnmounts           int
	lifecycle                        signal.Handles
}

type pageView struct {
	ui.ViewBase[pageView]
	key       string
	states    map[string]*pageState
	lifecycle func(*gui.TextInput, bool)
}

func (v *pageView) Build() ui.View { return v }
func (v *pageView) Mount(ctx ui.BuildContext) gui.Widget {
	if v.states[v.key] != nil {
		panic("page was recreated: " + v.key)
	}
	input := gui.NewTextInput()
	input.SetText("retained " + v.key)
	s := &pageState{input: input, last: input.Text()}
	s.connection = input.ConnectText(func(value string) { s.notifications++; s.last = value })
	s.lifecycle = signal.Handles{input.ConnectMount(func() {
		s.guiMounts++
		if v.lifecycle != nil {
			v.lifecycle(input, true)
		}
	}), input.ConnectUnmount(func() {
		s.guiUnmounts++
		if v.lifecycle != nil {
			v.lifecycle(input, false)
		}
	})}
	v.states[v.key] = s
	ctx.SetState(s)
	return input
}
func (v *pageView) Update(ctx ui.BuildContext, _ gui.Widget) {
	ctx.State().(*pageState).updates++
}
func (*pageView) Unmount(ctx ui.BuildContext, _ gui.Widget) {
	s := ctx.State().(*pageState)
	s.unmounts++
	s.connection.Disconnect()
	s.lifecycle.Disconnect()
}

func main() {
	expect := flag.String("expect", "transferred", "transferred, canceled, detached, prepare-failure, prepare-cancel, target-destroy or source-destroy")
	chrome := flag.String("chrome", "native", "native or integrated")
	sameWindow := flag.Bool("same-window", false, "two tab views in one UI Root")
	targetFirst := flag.Bool("target-first", false, "build target before source; requires same-window")
	flag.Parse()
	if !slices.Contains([]string{"transferred", "canceled", "detached", "prepare-failure", "prepare-cancel", "target-destroy", "source-destroy"}, *expect) {
		log.Fatal("invalid expectation")
	}
	decoration := ui.WindowChromeNative
	if *chrome == "integrated" {
		decoration = ui.WindowChromeIntegrated
	} else if *chrome != "native" {
		log.Fatal("invalid chrome")
	}
	if *sameWindow {
		if *expect != "transferred" && *expect != "canceled" {
			log.Fatal("same-window supports transferred or canceled")
		}
		if err := runSameWindow(*expect, decoration, *targetFirst); err != nil {
			log.Fatal(err)
		}
		return
	}
	if *targetFirst {
		log.Fatal("target-first requires same-window")
	}
	models := map[string][]string{"source": {"A", "B"}, "target": {"C"}}
	detaching := *expect == "detached" || *expect == "prepare-failure" || *expect == "prepare-cancel"
	if detaching {
		delete(models, "target")
	}
	frames := make(map[string]*frameWidget)
	states := make(map[string]*pageState)
	var original *widgets.TabPage
	transfers, builds := 0, 0
	requests := 0
	destructions := 0
	var pending *wui.TabDetachRequest
	var requestedPosition, sourceOrigin geometry.Point
	var contentSize, targetSize geometry.Size
	verified, positioned := false, false
	var failure error
	err := ui.Run("org.golang-gui.UITabTransfer", func(app ui.App) ui.RootView {
		builds++
		fail := func(err error) {
			if failure == nil {
				failure = err
			}
			log.Print("FAIL ", err)
			app.Quit()
		}
		transfer := func(change wui.TabTransfer) {
			transfers++
			from, to := models[change.SourceWindow], models[change.TargetWindow]
			index := slices.Index(from, change.PageID)
			if index < 0 || change.Index < 0 || change.Index > len(to) {
				fail(fmt.Errorf("invalid model transfer: %+v", change))
				return
			}
			models[change.SourceWindow] = slices.Delete(from, index, index+1)
			models[change.TargetWindow] = slices.Insert(to, change.Index, change.PageID)
			if *expect == "source-destroy" {
				delete(models, "source")
			}
			log.Printf("TRANSFER %+v", change)
			app.RequestUpdate()
		}
		lifecycle := func(input *gui.TextInput, mounted bool) {
			if destructions != 0 || original == nil {
				return
			}
			id := ""
			if *expect == "target-destroy" && !mounted {
				id = "target"
				delete(models, id)
			} else if *expect == "source-destroy" && mounted && input.Window() == app.FindWindow("target") {
				id = "source"
			}
			if id == "" {
				return
			}
			window := app.FindWindow(id)
			if window == nil {
				fail(fmt.Errorf("missing %s at lifecycle boundary", id))
				return
			}
			destructions++
			window.Destroy()
			app.RequestUpdate()
			log.Printf("DESTROY %s during GUI mounted=%v", id, mounted)
		}
		detach := func(request *wui.TabDetachRequest, handled *bool) {
			requests++
			if !detaching || requests != 1 || request.PageID != "A" {
				request.Cancel()
				fail(fmt.Errorf("unexpected detach request"))
				return
			}
			var err error
			sourceOrigin, err = request.Window.Position(nil)
			if err != nil {
				request.Cancel()
				fail(err)
				return
			}
			pending, contentSize = request, request.ContentSize
			sourceFrame := frames["source"]
			if !sourceFrame.sized {
				request.Cancel()
				pending = nil
				fail(fmt.Errorf("source content not arranged"))
				return
			}
			targetSize = contentSize.Add(sourceFrame.overhead)
			bar := app.FindWidget("source", "tab-bar")
			requestedPosition = request.Position.Add(bar.Snapshot().Bounds.Pos.Add(request.Hotspot).Scale(-1))
			*handled = true
			models["target"] = []string{} // The target is genuinely empty; no placeholder page is created before transfer.
			if *expect == "prepare-failure" {
				models["target"] = []string{"A"} // conflicting page ID, no second editor
			}
			app.RequestUpdate()
		}
		prepare := func(_ *frameWidget) {
			request := pending
			if request == nil {
				return
			}
			target := app.FindWindow("target")
			view, _ := app.FindWidget("target", "documents").(*widgets.TabView)
			abort := func(err error) {
				pending = nil
				request.Cancel()
				if target != nil {
					target.Destroy()
				}
				delete(models, "target")
				app.RequestUpdate()
				if err != nil {
					fail(err)
				}
			}
			if target == nil || view == nil {
				abort(fmt.Errorf("target preparation missing UI ownership"))
				return
			}
			if err := target.SetPosition(request.Window, requestedPosition); err != nil {
				abort(err)
				return
			}
			if *expect == "prepare-failure" || *expect == "prepare-cancel" {
				if *expect == "prepare-failure" {
					if len(view.Pages()) != 1 || view.Pages()[0] == original || view.Pages()[0].Child() != nil {
						abort(fmt.Errorf("missing independent same-ID target page"))
						return
					}
					if err := request.TransferTo(view, 0); err == nil {
						abort(fmt.Errorf("duplicate page ID accepted"))
						return
					} else {
						log.Printf("EXPECTED transfer rejection: %v", err)
					}
				} else {
					request.Cancel()
				}
				if request.TransferTo(view, 0) == nil {
					abort(fmt.Errorf("consumed request committed"))
					return
				}
				abort(nil)
				log.Printf("EXPECTED %s; prepared target discarded, source retained", *expect)
				return
			}
			origin, err := request.Window.Position(nil)
			if err != nil {
				abort(err)
				return
			}
			if origin != sourceOrigin {
				abort(fmt.Errorf("source moved during preparation"))
				return
			}
			if err := request.TransferTo(view, 0); err != nil {
				abort(err)
				return
			}
			pending = nil
			log.Printf("PREPARED UI target position=%v content=%v", requestedPosition, contentSize)
		}
		verify := func() {
			state := states["A"]
			windowID := "target"
			if *expect == "canceled" || *expect == "prepare-failure" || *expect == "prepare-cancel" || *expect == "target-destroy" {
				windowID = "source"
			}
			view, _ := app.FindWidget(windowID, "documents").(*widgets.TabView)
			if view == nil || original == nil || !slices.Contains(view.Pages(), original) || original.Child() != state.input || state.unmounts != 0 || app.FindWidget(windowID, "A") != original {
				fail(fmt.Errorf("original page/input/State not retained"))
				return
			}
			if *expect == "target-destroy" {
				if destructions != 1 || transfers != 0 || requests != 0 || app.FindWindow("target") != nil ||
					len(view.Pages()) != 2 || view.Pages()[0] != original || view.Current() != original ||
					state.guiMounts != 2 || state.guiUnmounts != 1 || state.updates < 3 || state.notifications < 2 ||
					state.last != "after move" || state.input.Text() != "after move" {
					fail(fmt.Errorf("UI rollback identity/rebuild/lifecycle failed: destructions=%d transfers=%d requests=%d lifecycle=%d/%d updates=%d text=%q", destructions, transfers, requests, state.guiMounts, state.guiUnmounts, state.updates, state.input.Text()))
					return
				}
				verified = true
				log.Printf("PASS UI target-destroy + rebuild + editing; original page/State retained, GUI lifecycle=2/1, View Unmount=0")
				app.Quit()
				return
			}
			if windowID == "source" {
				wantRequests := 0
				if detaching {
					wantRequests = 1
				}
				if requests != wantRequests || transfers != 0 || state.input.Text() != "retained A" || state.guiMounts != 1 || state.guiUnmounts != 0 || (detaching && app.FindWindow("target") != nil) {
					fail(fmt.Errorf("cancel changed source"))
					return
				}
				verified = true
				log.Printf("PASS UI %s; original page/State retained without unmount", *expect)
				app.Quit()
				return
			}
			if transfers != 1 || state.updates < 3 || state.notifications < 2 || state.last != "after move" || state.input.Text() != "after move" {
				fail(fmt.Errorf("transfer/rebuild/edit checks: transfers=%d updates=%d notifications=%d text=%q", transfers, state.updates, state.notifications, state.input.Text()))
				return
			}
			if state.guiMounts != 2 || state.guiUnmounts != 1 {
				fail(fmt.Errorf("GUI lifecycle=%d/%d", state.guiMounts, state.guiUnmounts))
				return
			}
			if *expect == "source-destroy" && (destructions != 1 || app.FindWindow("source") != nil || requests != 0) {
				fail(fmt.Errorf("source lifecycle destruction not observed"))
				return
			}
			if *expect == "detached" {
				if requests != 1 || pending != nil {
					fail(fmt.Errorf("detach request not committed once"))
					return
				}
				source, target := app.FindWindow("source"), app.FindWindow("target")
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
				actualSize := view.Rect().Size
				if math.Abs(float64(actualSize.Width-contentSize.Width)) > 1 || math.Abs(float64(actualSize.Height-contentSize.Height)) > 1 {
					fail(fmt.Errorf("content size=%v requested=%v", actualSize, contentSize))
					return
				}
				log.Printf("POSITION actual=%v content=%v", origin, actualSize)
			}
			delete(models, "source")
			app.RequestUpdate()
			app.Post(func() {
				if app.FindWindow("source") != nil || original.Child() != state.input || state.input.Window() != app.FindWindow("target") || state.unmounts != 0 || state.input.Text() != "after move" {
					fail(fmt.Errorf("source destruction damaged target"))
					return
				}
				verified = true
				log.Printf("PASS UI transfer + rebuild + source close; same page/input/State, updates=%d notifications=%d", state.updates, state.notifications)
				app.Quit()
			})
		}
		var windows []ui.WindowView
		for _, id := range []string{"source", "target"} {
			keys, exists := models[id]
			if !exists {
				continue
			}
			var pages []*wui.TabPageView
			for _, key := range keys {
				if id == "target" && *expect == "prepare-failure" {
					pages = append(pages, wui.TabPage(key, nil).Title("Document A"))
					continue
				}
				p := &pageView{key: key, states: states}
				if key == "A" {
					p.lifecycle = lifecycle
				}
				p.Self = p
				p.ID("input-" + key)
				pages = append(pages, wui.TabPage(key, p).Title("Document "+key))
			}
			bar := wui.TabBar("documents").ID("tab-bar").Reorderable(true).Transferable(true).OnTransferError(fail).OnDetachRequest(detach)
			var heading ui.View = bar
			if *chrome == "integrated" {
				heading = ui.HeaderBar(bar).Padding(6)
			}
			content := ui.VBox(
				heading,
				ui.Label(fmt.Sprintf("%s / builds %d", id, builds)),
				ui.HBox(ui.Button("Rebuild").OnClick(app.RequestUpdate), ui.Button("Verify and close source").OnClick(verify)).Spacing(8),
				wui.TabView(pages...).ID("documents").OnTransfer(transfer),
			).CrossAlign(layout.CrossStretch).Spacing(12).Padding(12)
			size := geometry.Size{Width: 520, Height: 340}
			if id == "target" && detaching {
				size = targetSize
			}
			frame := &frameView{key: id, frames: frames, child: content, size: size}
			frame.Self = frame
			if id == "target" && pending != nil {
				frame.after = prepare
			}
			windows = append(windows, ui.Window(id).Title("GOUI UI TRANSFER "+id).Chrome(decoration).Size(size.Width, size.Height).Content(frame))
		}
		if !positioned {
			positioned = true
			app.Post(func() {
				source, target := app.FindWindow("source"), app.FindWindow("target")
				if source == nil || (!detaching && target == nil) {
					fail(fmt.Errorf("initial windows missing"))
					return
				}
				if err := source.SetPosition(nil, geometry.Point{X: 50, Y: 80}); err != nil {
					fail(err)
					return
				}
				// Independent initial requests: the source's asynchronous native
				// move need not have completed when the second request is sent.
				if target != nil {
					if err := target.SetPosition(nil, geometry.Point{X: 650, Y: 80}); err != nil {
						fail(err)
						return
					}
				}
				original = app.FindWidget("source", "documents").(*widgets.TabView).Pages()[0]
				log.Print("READY UI transfer")
			})
		}
		return ui.Root().StyleSheet(modern.Sheet(modern.Options{})).Windows(windows...)
	})
	if err != nil {
		failure = err
	}
	if failure != nil || !verified {
		fmt.Fprintln(os.Stderr, "FAIL: verification incomplete:", failure)
		os.Exit(1)
	}
}
