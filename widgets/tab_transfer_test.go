package widgets

import (
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
)

// 页面移交、原生拖放状态与占位预览。

func TestTransferRequestRequiresCommitAndExpires(t *testing.T) {
	for _, mode := range []string{"reject", "commit", "uncommitted", "target"} {
		t.Run(mode, func(t *testing.T) {
			source, target := NewTabView(), NewTabView()
			page := NewTabPage("page", nil)
			source.AppendPage(page)
			var saved *TabTransferRequest
			failure := errors.New("adapter rejected")
			calls := 0
			handler := func(r *TabTransferRequest) {
				calls++
				saved = r
				if r.Source != source || r.Target != target || r.Page != page || r.Index != 0 {
					t.Fatal("wrong request")
				}
				if source.TransferPage(page, target, 0) == nil {
					t.Fatal("recursive request accepted")
				}
				r.Handled = true
				switch mode {
				case "reject":
					r.Err = failure
				case "commit", "target":
					if err := r.Commit(); err != nil {
						t.Fatal(err)
					}
					if r.Commit() == nil {
						t.Fatal("repeated commit accepted")
					}
				}
			}
			if mode == "target" {
				target.ConnectTransferRequest(handler)
			} else {
				source.ConnectTransferRequest(handler)
			}
			err := source.TransferPage(page, target, 0)
			if calls != 1 || saved.Commit() == nil {
				t.Fatal("request delivery/lifetime")
			}
			if mode == "commit" || mode == "target" {
				if err != nil || page.Parent() != target {
					t.Fatalf("commit: %v", err)
				}
			} else {
				if err == nil || page.Parent() != source {
					t.Fatalf("rejection: %v", err)
				}
				if mode == "reject" && err != failure {
					t.Fatal("rejection error lost")
				}
			}
		})
	}
}

// Fake roots exercise ordinary WidgetBase mount/unmount without native windows.
type tabTransferRoot struct{ widget gui.Widget }

func (r *tabTransferRoot) Widget() gui.Widget                       { return r.widget }
func (*tabTransferRoot) RequestPaint() error                        { return nil }
func (*tabTransferRoot) RequestLayout()                             {}
func (*tabTransferRoot) ConnectFrame(func(time.Time)) signal.Handle { return signal.Handles(nil) }

type tabTransferHost struct {
	gui.WidgetBase
	root *tabTransferRoot
}

func (h *tabTransferHost) Root() gui.Root { return h.root }

func mountTabView(v *TabView) *tabTransferHost {
	h := &tabTransferHost{root: new(tabTransferRoot)}
	h.root.widget = h
	h.WidgetBase.AddChild(h, v)
	return h
}

func TestTransferPagePreservesObjectsAndPublishesCoherentSelection(t *testing.T) {
	for _, mounted := range []bool{false, true} {
		source, target := NewTabView(), NewTabView()
		a, b, c, d := NewTabPage("a", nil), NewTabPage("b", gui.NewTextInput()), NewTabPage("c", nil), NewTabPage("d", nil)
		source.AppendPage(a)
		source.AppendPage(b)
		source.AppendPage(c)
		target.AppendPage(d)
		source.SetCurrent(b)
		input := b.Child().(*gui.TextInput)
		input.SetText("retained text")
		if mounted {
			mountTabView(source)
			mountTabView(target)
		}
		mounts, unmounts, closes, selections := 0, 0, 0, 0
		input.ConnectMount(func() { mounts++ })
		input.ConnectUnmount(func() {
			unmounts++
			// Lifecycle callbacks cannot initiate a conflicting transfer or
			// remove/insert another page midway through the protected commit.
			if source.TransferPage(b, target, 0) == nil {
				t.Fatal("recursive transfer accepted")
			}
			source.RemovePage(c)
			target.AppendPage(NewTabPage("unexpected", nil))
		})
		source.ConnectCloseRequest(func(*TabPage) { closes++ })
		check := func() {
			t.Helper()
			if !slices.Equal(source.Pages(), []*TabPage{a, c}) || !slices.Equal(target.Pages(), []*TabPage{b, d}) ||
				source.Current() != c || target.Current() != b || b.view != target || b.Parent() != target ||
				a.Visible() || !b.Visible() || !c.Visible() || d.Visible() {
				t.Fatal("notification observed incomplete page/selection transfer")
			}
		}
		source.changed.Connect(check)
		target.changed.Connect(check)
		source.ConnectCurrent(func(p *TabPage) {
			check()
			if p != c {
				t.Fatal("source fallback")
			}
			selections++
		})
		target.ConnectCurrent(func(p *TabPage) {
			check()
			if p != b {
				t.Fatal("target selection")
			}
			selections++
		})
		if err := source.TransferPage(b, target, 0); err != nil {
			t.Fatal(err)
		}
		check()
		want := 0
		if mounted {
			want = 1
		}
		if mounts != want || unmounts != want || closes != 0 || selections != 2 || b.Child() != input || input.Text() != "retained text" {
			t.Fatalf("lifecycle=%d/%d closes=%d selections=%d retained=%q", mounts, unmounts, closes, selections, input.Text())
		}
		info := target.Snapshot()
		if len(info.Children) != 2 || info.Children[0].Role != RoleTabPanel || !info.Children[0].Selected || info.Children[0].Text != "b" {
			t.Fatalf("transfer snapshot=%+v", info)
		}
	}
}

func TestTransferPageValidationAndSameView(t *testing.T) {
	source, target := NewTabView(), NewTabView()
	a, b := NewTabPage("a", nil), NewTabPage("b", nil)
	source.AppendPage(a)
	source.AppendPage(b)
	for _, index := range []int{-1, 1} {
		if source.TransferPage(a, target, index) == nil {
			t.Fatal("invalid index accepted")
		}
	}
	if target.TransferPage(a, source, 0) == nil || source.TransferPage(nil, target, 0) == nil || source.TransferPage(a, nil, 0) == nil {
		t.Fatal("invalid ownership accepted")
	}
	nested := NewTabView()
	a.SetChild(nested)
	if source.TransferPage(a, nested, 0) == nil {
		t.Fatal("cycle accepted")
	}
	if !slices.Equal(source.Pages(), []*TabPage{a, b}) || source.Current() != a || len(target.Pages()) != 0 {
		t.Fatal("failure mutated source")
	}
	moved := 0
	source.ConnectMoved(func(p *TabPage, from, to int) {
		if p != a || from != 0 || to != 1 {
			t.Fatal("wrong reorder")
		}
		moved++
	})
	if err := source.TransferPage(a, source, 1); err != nil {
		t.Fatal(err)
	}
	if moved != 1 || source.Current() != a || !slices.Equal(source.Pages(), []*TabPage{b, a}) {
		t.Fatal("same-view transfer changed semantics")
	}
}

func TestTransferLastEditorPageRetainsState(t *testing.T) {
	for _, rejected := range []bool{false, true} {
		t.Run(fmt.Sprint("rejected=", rejected), func(t *testing.T) {
			source, target := NewTabView(), NewTabView()
			editor := gui.NewTextView()
			original := strings.Repeat("line content\n", 100)
			model := gui.NewTextModel(original)
			editor.SetModel(model)
			scroll := gui.NewScrollView()
			scroll.SetChild(editor)
			page := NewTabPage("only page", scroll)
			source.AppendPage(page)
			sourceHost := mountTabView(source)
			targetHost := mountTabView(target)
			defer sourceHost.RemoveChild(source)
			defer targetHost.RemoveChild(target)
			source.Measure(layout.Loose(geometry.Size{Width: 320, Height: 180}))
			source.Arrange(geometry.Rect(0, 0, 320, 180))
			if err := model.ReplaceAtomic(gui.TextRange{Start: 0, End: 4}, "EDIT"); err != nil {
				t.Fatal(err)
			}
			selection := gui.TextSelection{Anchor: 8, Caret: 3}
			editor.SetSelection(selection)
			scroll.SetScrollY(300)
			if scroll.ScrollY() != 300 {
				t.Fatalf("fixture did not scroll: %g", scroll.ScrollY())
			}
			if rejected {
				target.ConnectTransferRequest(func(r *TabTransferRequest) {
					r.Handled, r.Err = true, errors.New("test rejection")
				})
			}
			err := source.TransferPage(page, target, 0)
			owner := target
			if rejected {
				owner = source
			}
			if (err != nil) != rejected || page.Parent() != owner || owner.Current() != page {
				t.Fatalf("owner/selection after transfer: err=%v", err)
			}
			owner.Measure(layout.Loose(geometry.Size{Width: 320, Height: 180}))
			owner.Arrange(geometry.Rect(0, 0, 320, 180))
			if page.Child() != scroll || scroll.Child() != editor || editor.Model() != model ||
				editor.Selection() != selection || scroll.ScrollY() != 300 ||
				model.Text() != "EDIT"+original[4:] || !model.CanUndo() {
				t.Fatalf("editor state lost: selection=%+v scroll=%g history=%v", editor.Selection(), scroll.ScrollY(), model.CanUndo())
			}
			if !rejected {
				if len(source.Pages()) != 0 || source.Current() != nil || len(source.Snapshot().Children) != 0 {
					t.Fatal("last page left stale source state")
				}
				sourceHost.RemoveChild(source)
			}
			if !model.Undo() || model.Text() != original || !model.Redo() || model.Text() != "EDIT"+original[4:] {
				t.Fatal("retained editor history is not usable")
			}
		})
	}
}

func TestNativeTabVisibilityRetainsSlotAndPage(t *testing.T) {
	bar, view, pages := dragTestBar()
	bar.SetTransferable(true)
	before := bar.Measure(layout.Unbounded())
	run := &tabTransferDrag{view: view, page: pages[0]}
	bar.nativeDrag = run
	bar.syncDragVisibility()
	if !bar.items[pages[0]].Visible() {
		t.Fatal("hidden before native handoff")
	}
	run.started = true
	bar.syncDragVisibility()
	bar.Arrange(bar.Rect())
	item := bar.items[pages[0]]
	if item.Visible() || item.Rect().Width != 100 || bar.items[pages[1]].Rect().X != 104 || bar.Measure(layout.Unbounded()).Size != before.Size {
		t.Fatal("hidden tab lost its reserved geometry")
	}
	if !pages[0].Visible() || pages[0].Parent() != view || view.Current() != pages[0] {
		t.Fatal("hiding the tab changed page ownership/selection/visibility")
	}
	if got := item.Snapshot(); got.Visible || len(got.Children) != 0 {
		t.Fatal("hidden source exposed clickable content")
	}
	if gui.Pick(bar, geometry.Point{X: 40, Y: 20}) != bar.viewport {
		t.Fatal("hidden tab intercepted input")
	}
	run.ended = true
	bar.syncDragVisibility()
	if !item.Visible() || len(item.Snapshot().Children) == 0 {
		t.Fatal("cancel did not restore source")
	}
}

func TestSingleTabHandoffAttemptsPreviewAndRetainsPageOnFailure(t *testing.T) {
	// No window exists in this logical test: RenderWidget must report that
	// failure. A sole-page guard would incorrectly skip the attempt entirely.
	bar, view, pages := dragTestBar()
	view.RemovePage(pages[2])
	view.RemovePage(pages[1])
	bar.SetTransferable(true)
	bar.Arrange(bar.Rect())
	bar.beginDrag(pages[0], geometry.Point{X: 20, Y: 20})
	errors := 0
	bar.ConnectTransferError(func(error) { errors++ })
	if !bar.handoff(pages[0], nil, geometry.Point{X: 140, Y: 70}) || errors != 1 {
		t.Fatal("sole page was prevented from entering the handoff path")
	}
	if bar.nativeDrag != nil || !bar.items[pages[0]].Visible() || pages[0].Parent() != view || view.Current() != pages[0] {
		t.Fatal("failed preparation hid or removed the sole page")
	}
}

func TestSoleNativeSourceKeepsStripHeight(t *testing.T) {
	bar, view, pages := dragTestBar()
	view.RemovePage(pages[2])
	view.RemovePage(pages[1])
	item := bar.items[pages[0]]
	item.SetMinSize(geometry.Size{Height: 48})
	before := bar.Measure(layout.Unbounded())
	bar.nativeDrag = &tabTransferDrag{page: pages[0], view: view, started: true}
	bar.syncDragVisibility()
	after := bar.Measure(layout.Unbounded())
	if before.Size != after.Size || after.Height < 48 || bar.slotCount() != 1 {
		t.Fatalf("hidden sole page collapsed measurement: before=%v after=%v", before, after)
	}
}

func TestIncomingGapAnimatesAndCloses(t *testing.T) {
	bar, target, pages := dragTestBar()
	bar.SetTransferable(true)
	bar.Arrange(geometry.Rect(0, 0, 500, 40))
	natural := bar.Measure(layout.Unbounded()).Size
	source := NewTabView()
	page := NewTabPage("incoming", nil)
	source.AppendPage(page)
	bar.incoming = &tabTransferDrag{view: source, page: page}
	bar.pointer = geometry.Point{X: 100, Y: 20}
	bar.previewMotion = true
	bar.Arrange(bar.Rect())
	neighbor := bar.items[pages[1]]
	if got := bar.Measure(layout.Unbounded()).Size; got != natural {
		t.Fatal("hovering a foreign tab changed the host's natural size")
	}
	if bar.incomingAt != 1 || neighbor.Rect().X != 104 || neighbor.motion.target != 208 || bar.contentWidth != 412 {
		t.Fatalf("gap not reserved: index=%d neighbor=%v target=%g content=%g", bar.incomingAt, neighbor.Rect(), neighbor.motion.target, bar.contentWidth)
	}
	bar.advanceMotion(70 * time.Millisecond)
	if neighbor.Rect().X <= 104 || neighbor.Rect().X >= 208 {
		t.Fatal("neighbor did not slide")
	}
	bar.advanceMotion(70 * time.Millisecond)
	if neighbor.Rect().X != 208 || len(target.Pages()) != 3 || len(bar.Snapshot().Children) != 3 || page.Parent() != source {
		t.Fatal("gap modified real pages or became a semantic tab")
	}
	bar.clearIncoming()
	if neighbor.Rect().X != 208 || neighbor.motion.target != 104 {
		t.Fatal("leaving target snapped instead of closing gap")
	}
	bar.advanceMotion(100 * time.Millisecond)
	bar.advanceMotion(100 * time.Millisecond)
	if neighbor.Rect().X != 104 || bar.contentWidth != 308 {
		t.Fatal("gap did not close")
	}
}

func TestIncomingWidthIncludesPlaceholder(t *testing.T) {
	bar, _, pages := dragTestBar()
	bar.SetTransferable(true)
	bar.SetTabWidthRange(60, 140)
	bar.Arrange(geometry.Rect(0, 0, 400, 40))
	source := NewTabView()
	page := NewTabPage("incoming", nil)
	source.AppendPage(page)
	bar.incoming = &tabTransferDrag{view: source, page: page}
	bar.previewMotion = true
	bar.Arrange(bar.Rect())
	// Four equally sized slots and three real 4-DIP gaps fit exactly.
	for _, page := range pages {
		if bar.items[page].width != 97 {
			t.Fatal("placeholder not included in equal widths")
		}
	}
	bar.advanceMotion(100 * time.Millisecond)
	bar.advanceMotion(100 * time.Millisecond)
	for _, page := range pages {
		if bar.items[page].Rect().Width != 97 {
			t.Fatal("animated width did not settle")
		}
	}
	bar.Arrange(geometry.Rect(0, 0, 240, 40))
	if !bar.overflow || bar.contentWidth != 252 || bar.viewportWidth != 192 {
		t.Fatal("placeholder not included in overflow")
	}
}

func TestIncomingTabInsertionUsesSlotsNotAnimation(t *testing.T) {
	v, source := NewTabView(), NewTabView()
	page := NewTabPage("moving", nil)
	source.AppendPage(page)
	bar := NewTabBar()
	bar.SetView(v)
	bar.SetTransferable(true)
	bar.incoming = &tabTransferDrag{page: page, view: source}
	if !bar.validIncoming(bar.incoming) || bar.incomingIndex(geometry.Point{X: 90}) != 0 {
		t.Fatal("empty target not accepted")
	}
	for _, title := range []string{"a", "b", "c"} {
		v.AppendPage(NewTabPage(title, nil))
	}
	bar.Measure(layout.Loose(geometry.Size{Width: 350, Height: 40}))
	bar.Arrange(geometry.Rect(0, 0, 350, 40))
	bar.scroll = 30
	bar.incoming = &tabTransferDrag{page: page, view: source}
	for _, item := range bar.items {
		item.motion.x = -900
		item.Arrange(geometry.Rect(-900, 0, item.width, 40))
	}
	first := bar.items[v.Pages()[0]].width
	for _, tc := range []struct {
		x    float32
		want int
	}{{-50, 0}, {first/2 - 31, 0}, {first/2 - 29, 1}, {2000, 3}} {
		if got := bar.incomingIndex(geometry.Point{X: tc.x + bar.viewport.Rect().X}); got != tc.want {
			t.Fatalf("x=%g index=%d want=%d", tc.x, got, tc.want)
		}
	}
	bar.SetTransferable(false)
	if bar.incoming != nil {
		t.Fatal("disabled target retained preview")
	}
}

func TestIncomingSameViewReusesSourceSlot(t *testing.T) {
	// A=[0,100], B=[104,204], C=[208,308]. Native handoff leaves the
	// source page owned by the source; its one reserved slot moves with the
	// incoming preview, including in a second bar showing the same view.
	for _, tc := range []struct {
		name     string
		page     int
		x        float32
		index    int
		boundary float32
	}{
		{"A before B", 0, 140, 0, 0},
		{"A after B", 0, 180, 1, 104},
		{"A after C", 0, 300, 2, 208},
		{"B before A", 1, 20, 0, 0},
		{"B before C", 1, 230, 1, 104},
		{"B after C", 1, 300, 2, 208},
		{"C before B", 2, 140, 1, 104},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bar, view, pages := dragTestBar()
			bar.SetTransferable(true)
			bar.incoming = &tabTransferDrag{page: pages[tc.page], view: view}
			if got := bar.incomingIndex(geometry.Point{X: tc.x}); got != tc.index {
				t.Fatalf("visible x=%g: insertion=%d want=%d", tc.x, got, tc.index)
			}
			// Repeat in an overflowing viewport with arrows and a legal scroll
			// offset. The gap and hit testing must use the same canonical slot.
			bar.Measure(layout.Loose(geometry.Size{Width: 250, Height: 40}))
			bar.Arrange(geometry.Rect(0, 0, 250, 40))
			bar.scroll = 30
			for _, item := range bar.items {
				item.motion.x = -900
				item.Arrange(geometry.Rect(-900, 0, item.width, 40))
			}
			bar.pointer = geometry.Point{X: bar.viewport.Rect().X + tc.x - 30}
			index, boundary := bar.incomingSlot(bar.pointer)
			if index != tc.index || boundary != tc.boundary {
				t.Fatalf("scrolled slot=(%d,%g), want=(%d,%g)", index, boundary, tc.index, tc.boundary)
			}
			bar.previewMotion = true
			bar.syncDragVisibility()
			bar.updateIncomingSlot()
			bar.positionTabs(40)
			if bar.slotCount() != 3 || bar.items[pages[tc.page]].Visible() || bar.items[pages[tc.page]].motion.target != tc.boundary {
				t.Fatal("same-view drag created a duplicate slot or left the tab visible")
			}
			at := 0
			for _, page := range pages {
				if page == pages[tc.page] {
					continue
				}
				if at == tc.index {
					at++
				}
				if got := bar.items[page].motion.target; got != float32(at)*104 {
					t.Fatalf("neighbor slot=%g want=%g", got, float32(at)*104)
				}
				at++
			}
		})
	}
}

func TestIncomingDropRevalidatesAfterLayoutOrSourceChange(t *testing.T) {
	for _, removeSource := range []bool{false, true} {
		t.Run(fmt.Sprint("remove-source=", removeSource), func(t *testing.T) {
			bar, target, pages := dragTestBar()
			bar.SetTransferable(true)
			source := NewTabView()
			page := NewTabPage("incoming", nil)
			source.AppendPage(page)
			run := &tabTransferDrag{page: page, view: source}
			bar.incoming = run
			bar.pointer = geometry.Point{X: 175, Y: 20}
			bar.previewMotion = true
			bar.Arrange(geometry.Rect(0, 0, 460, 40))
			// Initial 100-DIP slots: x=175 is after the second midpoint (154).
			if bar.incomingAt != 2 || bar.items[pages[2]].motion.target != 312 {
				t.Fatalf("initial slot=%d next=%g", bar.incomingAt, bar.items[pages[2]].motion.target)
			}
			// No new Motion is sent: normal layout must refresh the marker and
			// Drop must use 140-DIP slots (second midpoint=214), not the old index.
			bar.SetTabWidthRange(140, 140)
			bar.Measure(layout.Loose(geometry.Size{Width: 600, Height: 40}))
			bar.Arrange(geometry.Rect(0, 0, 600, 40))
			if bar.incomingAt != 1 || bar.items[pages[1]].motion.target != 288 {
				t.Fatalf("relayout retained stale gap: slot=%d next=%g", bar.incomingAt, bar.items[pages[1]].motion.target)
			}
			if removeSource {
				source.RemovePage(page)
			}
			data := new(gui.DragData)
			data.SetLocal(tabTransferFormat, run)
			drop := &gui.DropRequest{Data: data, Action: gui.DragMove, Position: bar.pointer}
			bar.dropIncoming(drop)
			if removeSource {
				if drop.Accepted || run.committed || page.Parent() != nil || !slices.Equal(target.Pages(), pages) {
					t.Fatal("removed source was accepted or changed target")
				}
				return
			}
			if !drop.Accepted || !run.committed || target.Current() != page || len(source.Pages()) != 0 ||
				!slices.Equal(target.Pages(), []*TabPage{pages[0], page, pages[1], pages[2]}) {
				t.Fatal("drop did not use current geometry and original page")
			}
		})
	}
}

// 拖出窗口的请求与生命周期。

// Only Root operations are used; no native window or display is created.
type detachTestWindow struct {
	gui.Window
	widget gui.Widget
}

func (w *detachTestWindow) Widget() gui.Widget { return w.widget }

func (*detachTestWindow) RequestLayout() {}

func (*detachTestWindow) RequestPaint() error                        { return nil }
func (*detachTestWindow) ConnectFrame(func(time.Time)) signal.Handle { return signal.Handles(nil) }

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
			bar.ConnectDetachRequest(func(r *TabDetachRequest, handled *bool) {
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
	bar.ConnectDetachRequest(func(*TabDetachRequest, *bool) { requests++ })
	bar.requestDetach(run, gui.DragResult{PositionValid: true})
	if requests != 0 || bar.pendingDetach != nil || run.page.Parent() != run.view {
		t.Fatal("unaccepted sole page requested a new window")
	}
}

func TestDetachContentSizeUsesCurrentViewportNotInactivePage(t *testing.T) {
	bar, run := detachFixture()
	run.view.Measure(layout.Loose(geometry.Size{Width: 640, Height: 360}))
	run.view.Arrange(geometry.Rect(20, 30, 640, 360))
	// The inactive page deliberately has stale geometry from another layout.
	run.view.SetCurrent(run.view.Pages()[1])
	run.page.Arrange(geometry.Rect(0, 0, 110, 70))
	var request *TabDetachRequest
	bar.ConnectDetachRequest(func(r *TabDetachRequest, handled *bool) { request = r; *handled = true })
	run.hotspot = geometry.Point{X: 30, Y: 12}
	bar.requestDetach(run, gui.DragResult{PositionValid: true, Position: geometry.Point{X: 800, Y: 200}})
	if request == nil || request.ContentSize != (geometry.Size{Width: 640, Height: 360}) ||
		request.Position != (geometry.Point{X: 800, Y: 200}) || request.Hotspot != run.hotspot {
		t.Fatalf("request did not snapshot viewport and input: %+v", request)
	}
	run.view.Arrange(geometry.Rect(20, 30, 400, 240))
	if request.ContentSize != (geometry.Size{Width: 640, Height: 360}) {
		t.Fatal("pending request changed with source layout")
	}
	target := NewTabView()
	if err := request.TransferTo(target, 0); err != nil {
		t.Fatal(err)
	}
	// Initial content size must not become a permanent page constraint.
	target.Measure(layout.Loose(geometry.Size{Width: 300, Height: 180}))
	target.Arrange(geometry.Rect(0, 0, 300, 180))
	if run.page.Rect().Size != (geometry.Size{Width: 300, Height: 180}) {
		t.Fatalf("transfer imposed a lasting content constraint: %v", run.page.Rect())
	}
}

func TestDetachRequestRetainedUntilSingleCommit(t *testing.T) {
	bar, run := detachFixture()
	var request *TabDetachRequest
	bar.ConnectDetachRequest(func(r *TabDetachRequest, handled *bool) { request = r; *handled = true })
	bar.requestDetach(run, gui.DragResult{PositionValid: true})
	if request == nil || run.page.Parent() != run.view {
		t.Fatal("preparation lost page")
	}
	if bar.items[run.page].Visible() {
		t.Fatal("source tab reappeared while preparing the detached window")
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
			bar.ConnectDetachRequest(func(r *TabDetachRequest, handled *bool) { request = r; *handled = true })
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
			if item := bar.items[run.page]; item != nil && !item.Visible() {
				t.Fatal("source tab stayed hidden after preparation was cancelled")
			}
		})
	}
}

func TestPendingDetachPreservesSlotAndCancellationRestoresTab(t *testing.T) {
	bar, run := detachFixture()
	bar.SetTabWidthRange(100, 100)
	bar.Measure(layout.Loose(geometry.Size{Width: 300, Height: 40}))
	bar.Arrange(geometry.Rect(0, 0, 300, 40))
	item, next := bar.items[run.page], bar.items[run.view.Pages()[1]]
	before, height := next.Rect(), bar.Measure(layout.Unbounded()).Height
	var request *TabDetachRequest
	bar.ConnectDetachRequest(func(r *TabDetachRequest, handled *bool) { request = r; *handled = true })
	bar.requestDetach(run, gui.DragResult{PositionValid: true})
	bar.Measure(layout.Loose(geometry.Size{Width: 300, Height: 40}))
	bar.Arrange(geometry.Rect(0, 0, 300, 40))
	if item.Visible() || next.Rect() != before || bar.Measure(layout.Unbounded()).Height != height {
		t.Fatal("pending detach collapsed the reserved slot or row height")
	}
	request.Cancel()
	if !item.Visible() || bar.pendingDetach != nil || run.page.Parent() != run.view {
		t.Fatal("cancel did not restore the original source tab")
	}
}

func TestDetachWithoutListenerDoesNotRetainHiddenTab(t *testing.T) {
	bar, run := detachFixture()
	bar.requestDetach(run, gui.DragResult{PositionValid: true})
	if bar.pendingDetach != nil || !bar.items[run.page].Visible() {
		t.Fatal("unhandled request retained a hidden tab")
	}
}

func TestDetachRequestUsesFinalHandledResult(t *testing.T) {
	for _, tc := range []struct {
		name    string
		results []bool
		retain  bool
	}{
		{"declined", []bool{false}, false},
		{"later decline", []bool{true, false}, false},
		{"later accept", []bool{false, true}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bar, run := detachFixture()
			var request *TabDetachRequest
			calls := 0
			for _, result := range tc.results {
				bar.ConnectDetachRequest(func(r *TabDetachRequest, handled *bool) {
					if request == nil && *handled {
						t.Fatal("handled must initially be false")
					}
					request = r
					calls++
					*handled = result
				})
			}
			bar.requestDetach(run, gui.DragResult{PositionValid: true})
			if calls != len(tc.results) || request == nil {
				t.Fatal("request was not queried in connection order")
			}
			if request.valid() != tc.retain || (bar.pendingDetach != nil) != tc.retain ||
				bar.items[run.page].Visible() == tc.retain || run.page.Parent() != run.view {
				t.Fatal("final handled result did not determine source reservation")
			}
			request.Cancel()
			if bar.pendingDetach != nil || !bar.items[run.page].Visible() {
				t.Fatal("cancel did not restore the original tab")
			}
		})
	}
}

func TestInactiveDetachConnectionsLeaveDefaultUnhandled(t *testing.T) {
	for _, action := range []string{"block", "disconnect"} {
		t.Run(action, func(t *testing.T) {
			bar, run := detachFixture()
			handle := bar.ConnectDetachRequest(func(*TabDetachRequest, *bool) {
				t.Fatal("inactive connection invoked")
			})
			if action == "block" {
				handle.Block()
			} else {
				handle.Disconnect()
			}
			bar.requestDetach(run, gui.DragResult{PositionValid: true})
			if bar.pendingDetach != nil || !bar.items[run.page].Visible() || run.page.Parent() != run.view {
				t.Fatal("inactive connection retained a request or lost its source page")
			}
		})
	}
}

func TestSynchronousDetachCommitIsNotUndoneByUnhandledResult(t *testing.T) {
	bar, run := detachFixture()
	target := NewTabView()
	calls := 0
	bar.ConnectDetachRequest(func(r *TabDetachRequest, handled *bool) {
		calls++
		if err := r.TransferTo(target, 0); err != nil {
			t.Fatal(err)
		}
		// The request was consumed synchronously, not retained for later work.
	})
	bar.ConnectDetachRequest(func(*TabDetachRequest, *bool) {
		t.Fatal("later handler received an already consumed request")
	})
	bar.requestDetach(run, gui.DragResult{PositionValid: true})
	if calls != 1 || bar.pendingDetach != nil || run.page.Parent() != target {
		t.Fatal("default cancellation undid an already committed transfer")
	}
}

func TestOneShotDetachListenerCanRetainRequestAfterDisconnecting(t *testing.T) {
	bar, run := detachFixture()
	var request *TabDetachRequest
	var handle signal.Handle
	handle = bar.ConnectDetachRequest(func(r *TabDetachRequest, handled *bool) {
		request = r
		*handled = true
		handle.Disconnect()
	})
	bar.requestDetach(run, gui.DragResult{PositionValid: true})
	if request == nil || !request.valid() || bar.pendingDetach != request || bar.items[run.page].Visible() {
		t.Fatal("disconnecting an already invoked listener cancelled its retained request")
	}
	request.Cancel()
	if bar.pendingDetach != nil || !bar.items[run.page].Visible() {
		t.Fatal("retained one-shot request did not restore its source on cancel")
	}
}
