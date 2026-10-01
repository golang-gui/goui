package main

import (
	"fmt"
	"log"
	"slices"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/theme/modern"
	"github.com/golang-gui/goui/ui"
	"github.com/golang-gui/goui/widgets"
	wui "github.com/golang-gui/goui/widgets/ui"
)

// 同 Root 仍然需要转移协调记录，而不是只修改 GUI 父节点。
// 两种初始左右顺序覆盖源先更新、目标先更新；移除源时保留外层布局槽位，
// 避免将普通位置协调的子节点移位与页面转移混在一起。
func runSameWindow(expect string, chrome ui.WindowChromeMode, targetFirst bool) error {
	models := map[string][]string{"source": {"A", "B"}, "target": {"C"}}
	states := make(map[string]*pageState)
	frames := make(map[string]*frameWidget)
	order := []string{"source", "target"}
	if targetFirst {
		slices.Reverse(order)
	}
	var original *widgets.TabPage
	var sourceView, targetView *widgets.TabView
	transfers, builds := 0, 0
	positioned, removed, verified := false, false, false
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
			if change.SourceWindow != "main" || change.TargetWindow != "main" ||
				change.SourceView != "source" || change.TargetView != "target" ||
				change.PageID != "A" || change.Index != 1 || transfers != 0 {
				fail(fmt.Errorf("unexpected same Root transfer: %+v", change))
				return
			}
			transfers++
			models["source"], models["target"] = []string{"B"}, []string{"C", "A"}
			log.Printf("TRANSFER %+v", change)
			app.RequestUpdate()
		}
		verify := func() {
			s := states["A"]
			if original == nil || original.Child() != s.input || s.unmounts != 0 ||
				app.FindWidget("main", "A") != original ||
				app.FindWidget("main", "input-A") != s.input || s.input.Window() != app.FindWindow("main") ||
				app.FindWidget("main", "source") != sourceView || app.FindWidget("main", "target") != targetView {
				fail(fmt.Errorf("same Root lost page, State, widget identity or ID lookup"))
				return
			}
			if expect == "canceled" {
				if transfers != 0 || !slices.EqualFunc(sourceView.Pages(), []string{"Document A", "Document B"}, func(p *widgets.TabPage, title string) bool { return p.Title() == title }) ||
					len(targetView.Pages()) != 1 || sourceView.Current() != original ||
					s.guiMounts != 1 || s.guiUnmounts != 0 || s.input.Text() != "retained A" {
					fail(fmt.Errorf("same Root cancellation changed source"))
					return
				}
				verified = true
				log.Printf("PASS UI same Root canceled target-first=%v, original page/State retained, GUI lifecycle=1/0", targetFirst)
				app.Quit()
				return
			}
			if transfers != 1 || len(sourceView.Pages()) != 1 || sourceView.Pages()[0].Title() != "Document B" ||
				!slices.EqualFunc(targetView.Pages(), []string{"Document C", "Document A"}, func(p *widgets.TabPage, title string) bool { return p.Title() == title }) ||
				targetView.Pages()[1] != original || targetView.Current() != original ||
				s.guiMounts != 2 || s.guiUnmounts != 1 || s.updates < 3 || s.notifications < 2 ||
				s.last != "after move" || s.input.Text() != "after move" {
				fail(fmt.Errorf("same Root transfer/rebuild/edit failed: transfers=%d lifecycle=%d/%d updates=%d notifications=%d text=%q", transfers, s.guiMounts, s.guiUnmounts, s.updates, s.notifications, s.input.Text()))
				return
			}
			removed = true
			app.RequestUpdate()
			app.Post(func() {
				// View.Unmount disconnects B's lifecycle observers before GUI
				// detachment. Check the detached tree, not a disconnected counter
				// or final window-owned Destroyed state.
				if app.FindWindow("main") == nil || app.FindWidget("main", "source") != nil || sourceView.Root() != nil || sourceView.Parent() != nil ||
					states["B"].unmounts != 1 || states["B"].input.Root() != nil ||
					app.FindWidget("main", "target") != targetView || targetView.Destroyed() || original.Child() != s.input ||
					s.input.Destroyed() || s.input.Text() != "after move" || s.unmounts != 0 || s.guiMounts != 2 || s.guiUnmounts != 1 {
					fail(fmt.Errorf("source removal: window=%v sourceLookup=%v sourceDetached=%v BUnmount=%d/%d targetSame=%v targetDestroyed=%v inputSame=%v inputDestroyed=%v text=%q ViewUnmount=%d GUI=%d/%d", app.FindWindow("main") != nil, app.FindWidget("main", "source") != nil, sourceView.Root() == nil && sourceView.Parent() == nil, states["B"].unmounts, states["B"].guiUnmounts, app.FindWidget("main", "target") == targetView, targetView.Destroyed(), original.Child() == s.input, s.input.Destroyed(), s.input.Text(), s.unmounts, s.guiMounts, s.guiUnmounts))
					return
				}
				verified = true
				log.Printf("PASS UI same Root transfer/rebuild/edit/source removal target-first=%v, GUI lifecycle=2/1, View Unmount=0", targetFirst)
				app.Quit()
			})
		}
		var panes []ui.View
		for _, id := range order {
			var content ui.View = ui.Label("Source removed")
			if id != "source" || !removed {
				var pages []*wui.TabPageView
				for _, key := range models[id] {
					p := &pageView{key: key, states: states}
					p.Self = p
					p.ID("input-" + key)
					pages = append(pages, wui.TabPage(key, p).Title("Document "+key))
				}
				bar := wui.TabBar(id).ID(id + "-bar").Reorderable(true).Transferable(true).
					OnTransferError(fail).OnDetachRequest(func(r *wui.TabDetachRequest) {
					r.Cancel()
					fail(fmt.Errorf("unexpected detach request"))
				})
				var heading ui.View = bar
				if chrome == ui.WindowChromeIntegrated {
					heading = ui.HeaderBar(bar).Padding(6)
				}
				content = ui.VBox(heading,
					ui.Label(fmt.Sprintf("%s / builds %d: Drop A AFTER C", id, builds)),
					ui.HBox(ui.Button("Rebuild").OnClick(app.RequestUpdate), ui.Button("Verify and remove source").OnClick(verify)).Spacing(8),
					wui.TabView(pages...).ID(id).OnTransfer(transfer),
				).Spacing(12).Padding(12)
			}
			frame := &frameView{key: id, frames: frames, child: content}
			frame.Self = frame
			frame.MinWidth(520).MaxWidth(520)
			panes = append(panes, frame)
		}
		var content ui.View = ui.HBox(panes...).Spacing(16).CrossAlign(layout.CrossStretch)
		if !positioned {
			positioned = true
			app.Post(func() {
				w := app.FindWindow("main")
				w.SetMinSize(geometry.Size{Width: 800, Height: 280})
				if err := w.SetPosition(nil, geometry.Point{X: 50, Y: 80}); err != nil {
					fail(err)
					return
				}
				sourceView, _ = app.FindWidget("main", "source").(*widgets.TabView)
				targetView, _ = app.FindWidget("main", "target").(*widgets.TabView)
				original = sourceView.Pages()[0]
				log.Printf("READY UI same Root expect=%s target-first=%v", expect, targetFirst)
			})
		}
		return ui.Root().StyleSheet(modern.Sheet(modern.Options{})).Windows(
			ui.Window("main").Title("GOUI UI SAME ROOT").Chrome(chrome).Size(1100, 360).Content(content))
	})
	if err != nil {
		return err
	}
	if failure != nil {
		return failure
	}
	if !verified {
		return fmt.Errorf("verification incomplete")
	}
	return nil
}
