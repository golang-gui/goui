package ui

import (
	"math"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/gui"
)

func TestHeaderBarReconcilesChildAndDefaults(t *testing.T) {
	root := newRoot()
	t.Cleanup(root.unmountWindow)
	header := root.update(HeaderBar(Label("first"))).(*gui.HeaderBar)
	label := header.Child().(*gui.Label)
	defaults := gui.NewHeaderBar()
	if header.Padding() != defaults.Padding() || header.MinSize() != defaults.MinSize() {
		t.Fatal("GUI defaults were overwritten")
	}
	updated := root.update(HeaderBar(nil).Child(Label("second")).Padding(0).MinHeight(64))
	if updated != header || header.Child() != label || label.Text() != "second" {
		t.Fatal("header/child reuse or update failed")
	}
	if header.Padding() != 0 || header.MinSize().Height != 64 {
		t.Fatal("explicit modifiers were not applied")
	}
	for _, padding := range []float32{-1, float32(math.NaN()), float32(math.Inf(1))} {
		root.update(HeaderBar(nil).Padding(padding))
		if header.Padding() != 0 {
			t.Fatal("invalid padding was not normalized")
		}
	}
	root.update(HeaderBar(Button("replacement")))
	if _, ok := header.Child().(*gui.Button); !ok || label.Parent() != nil {
		t.Fatal("child type replacement failed")
	}
	root.update(HeaderBar(nil))
	if header.Child() != nil || len(header.Children()) != 0 {
		t.Fatal("empty slot did not remove content")
	}
	if header.Padding() != defaults.Padding() || header.MinSize() != defaults.MinSize() {
		t.Fatal("omitted modifiers did not restore GUI defaults")
	}
}

// The binding's signal callback delegates to extendDragRegion. GUI tests exercise
// actual Chrome queries/occlusion; here we isolate declarative state and matching.
func TestHeaderBarDragNamesReconcileAndMatchExactTarget(t *testing.T) {
	root := newRoot()
	t.Cleanup(root.unmountWindow)
	build := func(name string) *HeaderBarView {
		return HeaderBar(HBox(Label("Title").Name(name), Button("Save")).Name("row"))
	}
	names := []string{"row", "title", ""}
	view := build("title").DragNames(names...)
	names[1] = "mutated"
	header := root.update(view).(*gui.HeaderBar)
	state := root.root.state.(*headerBarState)
	handle := state.drag
	arrange := func() {
		header.Arrange(geometry.Rect(30, 40, 300, 48))
		row := header.Child()
		row.Arrange(geometry.Rect(8, 8, 284, 32))
		row.Children()[0].Arrange(geometry.Rect(0, 0, 80, 32))
		row.Children()[1].Arrange(geometry.Rect(100, 0, 80, 32))
	}
	query := func(point geometry.Point, initial bool) bool {
		state.extendDragRegion(header, point, &initial)
		return initial
	}
	arrange()
	if !query(geometry.Point{X: 20, Y: 20}, false) {
		t.Fatal("exact name not matched or caller slice retained")
	}
	if query(geometry.Point{X: 120, Y: 20}, false) {
		t.Fatal("row name matched a button descendant")
	}
	if !query(geometry.Point{X: 250, Y: 20}, false) {
		t.Fatal("row's own empty area did not match")
	}
	if query(geometry.Point{X: 1, Y: 1}, false) {
		t.Fatal("empty ID matched")
	}
	if !query(geometry.Point{X: 1, Y: 1}, true) {
		t.Fatal("automatic result was suppressed")
	}
	if query(geometry.Point{X: 350, Y: 20}, false) {
		t.Fatal("out-of-bounds point matched")
	}

	root.update(build("renamed").DragNames("title"))
	arrange()
	if query(geometry.Point{X: 20, Y: 20}, false) {
		t.Fatal("stale widget name used")
	}
	root.update(build("renamed").DragNames("title").DragNames("renamed"))
	arrange()
	if !query(geometry.Point{X: 20, Y: 20}, false) {
		t.Fatal("new name list not applied")
	}
	if state != root.root.state || handle != state.drag {
		t.Fatal("rebuild replaced the state or signal connection")
	}
	oldChild := header.Child()
	root.update(HeaderBar(Image(nil).Name("renamed")).DragNames("renamed"))
	header.Arrange(geometry.Rect(30, 40, 300, 48))
	header.Child().Arrange(geometry.Rect(8, 8, 100, 32))
	if header.Child() == oldChild || !query(geometry.Point{X: 20, Y: 20}, false) {
		t.Fatal("name matching retained a replaced child")
	}
	root.update(build("renamed"))
	arrange()
	if query(geometry.Point{X: 20, Y: 20}, false) {
		t.Fatal("omitted names did not clear override")
	}
}

type headerDragDisconnectProbe struct {
	signal.Handle
	calls int
}

func (h *headerDragDisconnectProbe) Disconnect() {
	h.calls++
	h.Handle.Disconnect()
}

func TestHeaderBarUnmountDisconnectsDragNames(t *testing.T) {
	root := newRoot()
	root.update(HeaderBar(Label("Title").Name("title")).DragNames("title"))
	state := root.root.state.(*headerBarState)
	probe := &headerDragDisconnectProbe{Handle: state.drag}
	state.drag = probe
	root.update(Label("replacement"))
	if probe.calls != 1 || state.drag != nil || state.dragNames != nil {
		t.Fatal("unmount retained names or signal connection")
	}
	root.unmountWindow()
}
