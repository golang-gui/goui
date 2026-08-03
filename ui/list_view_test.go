package ui

import (
	"fmt"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
)

func itemText(index int, data any) string {
	return fmt.Sprintf("item %d: %v", index, data)
}

func TestListViewMountsModelAndDelegate(t *testing.T) {
	root := newRoot()
	model := gui.NewSliceListModel([]int{10, 20, 30})

	widget := root.update(ListView(model, func(i int, data any) View {
		return Label(itemText(i, data))
	}))
	lv, ok := widget.(*gui.ListView)
	if !ok {
		t.Fatalf("updated %T, want *gui.ListView", widget)
	}
	if lv.Model() != model {
		t.Fatal("model should be mounted")
	}
	if _, ok := lv.Delegate().(*uiItemDelegate); !ok {
		t.Fatalf("delegate should be uiItemDelegate, got %T", lv.Delegate())
	}
}

func TestListViewCoordinatesItems(t *testing.T) {
	root := newRoot()
	model := gui.NewSliceListModel([]string{"a", "b", "c"})

	lv := root.update(ListView(model, func(i int, data any) View {
		return Label(itemText(i, data))
	})).(*gui.ListView)

	// Drive the virtualization directly: Bind coordinates the declarative
	// row into the shell (LinearBox) mounted under the list.
	lv.LayoutVisible(geometry.Size{Width: 100, Height: 100}, geometry.Point{})
	if len(lv.Children()) != 3 {
		t.Fatalf("expected 3 item shells, got %d", len(lv.Children()))
	}
	shell := lv.Children()[0].(*gui.LinearBox)
	shellChildren := shell.Children()
	if len(shellChildren) != 1 {
		t.Fatalf("shell should host exactly one row, got %d", len(shellChildren))
	}
	label, ok := shellChildren[0].(*gui.Label)
	if !ok || label.Text() != "item 0: a" {
		t.Fatalf("unexpected row content: %T %v", shellChildren[0], label)
	}
}

func TestListViewUnbindReleasesItem(t *testing.T) {
	root := newRoot()
	model := gui.NewSliceListModel(make([]int, 100))

	lv := root.update(ListView(model, func(i int, data any) View {
		return Label(itemText(i, data))
	})).(*gui.ListView)
	d := lv.Delegate().(*uiItemDelegate)

	// Drive the delegate contract directly (the test env measures Labels as
	// 0 height, so layout would bind everything; direct calls isolate the
	// release semantics).
	shell0 := d.Setup().(*gui.LinearBox)
	shell1 := d.Setup().(*gui.LinearBox)
	d.Bind(0, shell0)
	d.Bind(1, shell1)
	if len(d.nodes) != 2 {
		t.Fatalf("2 rows should be coordinated, got %d", len(d.nodes))
	}
	if len(shell0.Children()) != 1 || len(shell1.Children()) != 1 {
		t.Fatal("bound shells should host their rows")
	}

	d.Unbind(0, shell0)
	if len(d.nodes) != 1 {
		t.Fatalf("unbound row should be released, got %d nodes", len(d.nodes))
	}
	if len(shell0.Children()) != 0 {
		t.Fatal("released shell should be empty")
	}
	if d.nodes[shell1] == nil {
		t.Fatal("other rows must stay coordinated")
	}
}

func TestListViewUpdateKeepsDelegateSwapsModel(t *testing.T) {
	root := newRoot()
	modelA := gui.NewSliceListModel([]int{1, 2, 3})

	lv := root.update(ListView(modelA, func(i int, data any) View {
		return Label(itemText(i, data))
	})).(*gui.ListView)
	delegate := lv.Delegate()

	// Update with a new builder but the same model: the delegate instance is
	// reused (a fresh delegate would reload the list on every update).
	root.update(ListView(modelA, func(i int, data any) View {
		return Label("row-" + fmt.Sprint(i))
	}))
	if lv.Delegate() != delegate {
		t.Fatal("delegate should be reused across updates")
	}

	// Update with a new model: SetModel reloads and rows reflect new data.
	modelB := gui.NewSliceListModel([]string{"x", "y"})
	root.update(ListView(modelB, func(i int, data any) View {
		return Label(itemText(i, data))
	}))
	lv.LayoutVisible(geometry.Size{Width: 100, Height: 100}, geometry.Point{})
	shell := lv.Children()[0].(*gui.LinearBox)
	label := shell.Children()[0].(*gui.Label)
	if label.Text() != "item 0: x" {
		t.Fatalf("row should reflect the new model, got %q", label.Text())
	}
}

func TestListViewInsideScrollView(t *testing.T) {
	root := newRoot()
	model := gui.NewSliceListModel([]int{1, 2, 3})

	widget := root.update(ScrollView(
		ListView(model, func(i int, data any) View {
			return Label(itemText(i, data))
		}),
	))
	sv, ok := widget.(*gui.ScrollView)
	if !ok {
		t.Fatalf("updated %T, want *gui.ScrollView", widget)
	}
	lv, ok := sv.Child().(*gui.ListView)
	if !ok {
		t.Fatalf("scroll child should be *gui.ListView, got %T", sv.Child())
	}
	if lv.Model() != model {
		t.Fatal("list model should be mounted through the scroll view")
	}
}
