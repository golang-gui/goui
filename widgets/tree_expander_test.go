package widgets

import (
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/events"
)

func TestTreeExpanderCompositionToggleAndSemantics(t *testing.T) {
	e := NewTreeExpander()
	child := splitTestChild(80, 20)
	e.SetChild(child)
	e.SetDepth(2)
	e.SetIndentation(10)
	e.SetExpandable(true)
	m := e.Measure(layout.Unbounded()).Size
	if m.Width != 128 || m.Height != 28 || e.Child() != child || len(e.Children()) != 2 {
		t.Fatal(m, e.Child(), e.Children())
	}
	e.Arrange(geometry.Rect(0, 0, 200, 28))
	if child.Rect().X != 44 || e.button.Rect().X != 24 {
		t.Fatal(child.Rect(), e.button.Rect())
	}
	calls := 0
	e.ConnectToggle(func() { calls++ })
	e.SetExpanded(true)
	if calls != 0 || e.Snapshot().Children[0].Name != "折叠" {
		t.Fatal("state synchronization emitted Toggle or wrong snapshot")
	}
	d, host := new(gui.EventDispatcher), &tabInputHost{root: e}
	dispatchTabPointer(t, d, host, events.PointerDown, 34, 14)
	dispatchTabPointer(t, d, host, events.PointerUp, 34, 14)
	if calls != 1 || !e.Expanded() {
		t.Fatal("toggle must request state without mutating it")
	}
	e.SetExpandable(false)
	dispatchTabPointer(t, d, host, events.PointerDown, 34, 14)
	dispatchTabPointer(t, d, host, events.PointerUp, 34, 14)
	if calls != 1 || len(e.Snapshot().Children) != 1 {
		t.Fatal("leaf disclosure remained interactive")
	}
}

func TestTreeViewRowAtIncludesIndentationClipsAndRefreshesState(t *testing.T) {
	v, _ := treeFixture()
	defer v.SetModel(nil)
	if _, _, ok := v.RowAt(geometry.Point{}); ok {
		t.Fatal("unlaid tree has rows")
	}
	v.SetExpandedIDs([]string{"root", "a"})
	treeLayout(v, 60, 14)
	row, bounds, ok := v.RowAt(geometry.Point{X: 1, Y: 1})
	if !ok || row.ID != "root" || bounds.Y != -14 || bounds.Height != 28 {
		t.Fatal(row, bounds, ok)
	}
	v.SetSelection([]string{"a"})
	row, _, ok = v.RowAt(geometry.Point{X: 299, Y: 20})
	if !ok || row.ID != "a" || !row.Selected {
		t.Fatal("row whitespace/state", row, ok)
	}
	if _, _, ok = v.RowAt(geometry.Point{X: 1, Y: 60}); ok {
		t.Fatal("outside viewport hit")
	}
	treeLayout(v, 60, 56)
	row, _, ok = v.RowAt(geometry.Point{X: 1, Y: 1})
	if !ok || row.ID != "nested" {
		t.Fatal("scroll retained old hit", row, ok)
	}
}
