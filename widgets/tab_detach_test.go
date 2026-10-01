package widgets

import (
	"errors"
	"math"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
)

// Only Root operations are used; no native window or display is created.
type detachTestWindow struct {
	gui.Window
	widget gui.Widget
}

func (w *detachTestWindow) Widget() gui.Widget      { return w.widget }
func (*detachTestWindow) RequestLayout()            {}
func (*detachTestWindow) RequestPaint() error       { return nil }
func (*detachTestWindow) FocusedWidget() gui.Widget { return nil }

type detachTestHost struct {
	gui.WidgetBase
	window *detachTestWindow
}

func (h *detachTestHost) Root() gui.Root { return h.window }

func detachFixture() (*TabBar, *tabTransferDrag) {
	view := NewTabView()
	page := NewTabPage("document", gui.NewLabel("retained"))
	view.AppendPage(page)
	view.AppendPage(NewTabPage("remaining", gui.NewLabel("remaining")))
	bar := NewTabBar()
	bar.SetView(view)
	bar.SetTransferable(true)
	host := &detachTestHost{window: new(detachTestWindow)}
	host.window.widget = host
	host.WidgetBase.AddChild(host, view)
	host.WidgetBase.AddChild(host, bar)
	return bar, &tabTransferDrag{view: view, page: page, window: host.window, ended: true, hotspot: geometry.Point{X: 30, Y: 12}}
}

func TestDetachRequestEligibility(t *testing.T) {
	for _, tc := range []struct {
		name   string
		result gui.DragResult
		want   bool
	}{
		{"unaccepted", gui.DragResult{PositionValid: true}, true},
		{"accepted", gui.DragResult{Action: gui.DragMove, PositionValid: true}, false},
		{"cancel", gui.DragResult{Canceled: true}, false},
		{"error", gui.DragResult{Err: errors.New("native")}, false},
		{"unknown position", gui.DragResult{}, false},
		{"local rejection", gui.DragResult{LocalDrop: true, PositionValid: true}, false},
		{"local negotiation rejection without Drop", gui.DragResult{LocalTarget: true, PositionValid: true}, false},
		{"nonfinite", gui.DragResult{PositionValid: true, Position: geometry.Point{X: float32(math.NaN())}}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bar, run := detachFixture()
			calls := 0
			bar.ConnectDetachRequest(func(r *TabDetachRequest) {
				calls++
				if r.Page != run.page || r.Window != run.window || r.Hotspot != run.hotspot || run.page.Parent() != run.view {
					t.Fatal("request detached or changed original page")
				}
				r.Cancel()
			})
			bar.requestDetach(run, tc.result)
			if (calls == 1) != tc.want || bar.pendingDetach != nil {
				t.Fatalf("calls=%d pending=%v", calls, bar.pendingDetach != nil)
			}
		})
	}
}

func TestSingleTabDoesNotRequestNewWindow(t *testing.T) {
	bar, run := detachFixture()
	run.view.RemovePage(run.view.Pages()[1])
	requests := 0
	bar.ConnectDetachRequest(func(*TabDetachRequest) { requests++ })
	bar.requestDetach(run, gui.DragResult{PositionValid: true})
	if requests != 0 || bar.pendingDetach != nil || run.page.Parent() != run.view {
		t.Fatal("unaccepted sole page requested a new window")
	}
}

func TestDetachRequestRetainedUntilSingleCommit(t *testing.T) {
	bar, run := detachFixture()
	var request *TabDetachRequest
	bar.ConnectDetachRequest(func(r *TabDetachRequest) { request = r })
	bar.requestDetach(run, gui.DragResult{PositionValid: true})
	if request == nil || run.page.Parent() != run.view {
		t.Fatal("preparation lost page")
	}
	target := NewTabView()
	host := &detachTestHost{window: new(detachTestWindow)}
	host.window.widget = host
	host.WidgetBase.AddChild(host, target)
	input := run.page.Child()
	if err := request.TransferTo(target, 0); err != nil {
		t.Fatal(err)
	}
	if request.TransferTo(run.view, 0) == nil || bar.pendingDetach != nil || run.page.Parent() != target || run.page.Child() != input {
		t.Fatal("commit not single-use or identity lost")
	}
}

func TestDetachRequestInvalidation(t *testing.T) {
	for _, action := range []string{"cancel", "disable", "set view", "new drag", "remove page", "last page", "unmount", "failed commit"} {
		t.Run(action, func(t *testing.T) {
			bar, run := detachFixture()
			bar.SetReorderable(true)
			var request *TabDetachRequest
			bar.ConnectDetachRequest(func(r *TabDetachRequest) { request = r })
			bar.requestDetach(run, gui.DragResult{PositionValid: true})
			switch action {
			case "last page":
				run.view.RemovePage(run.view.Pages()[1])
			case "cancel":
				request.Cancel()
				request.Cancel()
			case "disable":
				bar.SetTransferable(false)
			case "set view":
				bar.SetView(nil)
			case "new drag":
				bar.beginDrag(run.page, geometry.Point{})
			case "remove page":
				run.view.RemovePage(run.page)
			case "unmount":
				bar.Parent().(*detachTestHost).WidgetBase.RemoveChild(bar)
			case "failed commit":
				if request.TransferTo(nil, 0) == nil {
					t.Fatal("nil target accepted")
				}
			}
			if request.TransferTo(NewTabView(), 0) == nil || bar.pendingDetach != nil {
				t.Fatal("stale request remained valid")
			}
			if action != "remove page" && run.page.Parent() != run.view {
				t.Fatal("failed preparation lost source page")
			}
		})
	}
}
