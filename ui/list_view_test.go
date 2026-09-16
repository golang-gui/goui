package ui

import (
	"fmt"
	"slices"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
)

func TestListViewMountsModelAndDelegate(t *testing.T) {
	root := newRoot()
	model := gui.NewSliceListModel([]int{10, 20, 30})

	widget := root.update(ListView(model, func(i int, data int) View {
		return Label(fmt.Sprintf("item %d: %d", i, data))
	}))
	lv, ok := widget.(*gui.ListView)
	if !ok {
		t.Fatalf("updated %T, want *gui.ListView", widget)
	}
	if lv.Model() != model {
		t.Fatal("model should be mounted")
	}
	if _, ok := lv.Delegate().(*uiItemDelegate[int]); !ok {
		t.Fatalf("delegate should be uiItemDelegate[int], got %T", lv.Delegate())
	}
}

func TestListViewCoordinatesItems(t *testing.T) {
	root := newRoot()
	model := gui.NewSliceListModel([]string{"a", "b", "c"})

	lv := root.update(ListView(model, func(i int, data string) View {
		return Label(fmt.Sprintf("item %d: %s", i, data))
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

	lv := root.update(ListView(model, func(i int, data int) View {
		return Label(fmt.Sprintf("item %d", i))
	})).(*gui.ListView)
	d := lv.Delegate().(*uiItemDelegate[int])

	// Drive the delegate contract directly (the test env measures Labels as
	// 0 height, so layout would bind everything; direct calls isolate the
	// release semantics).
	shell0 := d.Setup().(*gui.LinearBox)
	shell1 := d.Setup().(*gui.LinearBox)
	d.Bind(0, shell0)
	d.Bind(1, shell1)
	if len(root.root.children) != 2 {
		t.Fatalf("2 row targets should be coordinated, got %d", len(root.root.children))
	}
	if len(shell0.Children()) != 1 || len(shell1.Children()) != 1 {
		t.Fatal("bound shells should host their rows")
	}

	d.Unbind(0, shell0)
	if len(root.root.children) != 1 {
		t.Fatalf("unbound row target should be released, got %d", len(root.root.children))
	}
	if len(shell0.Children()) != 0 {
		t.Fatal("released shell should be empty")
	}
	if root.root.children[0].target != shell1 {
		t.Fatal("other rows must stay coordinated")
	}
}

func TestListViewUpdateKeepsDelegateSwapsModel(t *testing.T) {
	root := newRoot()
	modelA := gui.NewSliceListModel([]int{1, 2, 3})

	lv := root.update(ListView(modelA, func(i int, data int) View {
		return Label(fmt.Sprintf("item %d", i))
	})).(*gui.ListView)
	delegate := lv.Delegate()

	// Update with a new builder but the same model: the delegate instance is
	// reused (a fresh delegate would reload the list on every update).
	root.update(ListView(modelA, func(i int, data int) View {
		return Label(fmt.Sprintf("row-%d", i))
	}))
	if lv.Delegate() != delegate {
		t.Fatal("delegate should be reused across updates")
	}

	// Update with a new model of the same type: widget and delegate are
	// reused, SetModel reloads and rows reflect the new data.
	modelB := gui.NewSliceListModel([]int{9, 8, 7})
	root.update(ListView(modelB, func(i int, data int) View {
		return Label(fmt.Sprintf("item %d: %d", i, data))
	}))
	if lv.Delegate() != delegate {
		t.Fatal("same-T model swap should keep the delegate")
	}
	lv.LayoutVisible(geometry.Size{Width: 100, Height: 100}, geometry.Point{})
	shell := lv.Children()[0].(*gui.LinearBox)
	label := shell.Children()[0].(*gui.Label)
	if label.Text() != "item 0: 9" {
		t.Fatalf("row should reflect the new model, got %q", label.Text())
	}
}

func TestListViewInsideScrollView(t *testing.T) {
	root := newRoot()
	model := gui.NewSliceListModel([]int{1, 2, 3})

	widget := root.update(ScrollView(
		ListView(model, func(i int, data int) View {
			return Label(fmt.Sprintf("item %d", i))
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

func TestSliceListHelper(t *testing.T) {
	model := SliceList([]int{5, 6, 7})
	if model.ItemsCount() != 3 || model.ItemAt(2) != 7 {
		t.Fatalf("SliceList should adapt the slice, count=%d", model.ItemsCount())
	}
}

func TestListViewRebindReplacesAndClearsRows(t *testing.T) {
	r := newRoot()
	t.Cleanup(r.unmountWindow)
	model := SliceList([]int{1})
	lv := r.update(ListView(model, func(int, int) View { return Label("first") })).(*gui.ListView)
	d := lv.Delegate().(*uiItemDelegate[int])
	shell := d.Setup().(*gui.LinearBox)
	d.Bind(0, shell)
	old := shell.Children()[0]
	r.update(ListView(model, func(int, int) View { return Button("replacement") }))
	d.Bind(0, shell)
	if len(shell.Children()) != 1 || shell.Children()[0] == old || old.Parent() != nil {
		t.Fatal("rebind did not replace the previous row")
	}
	r.update(ListView(model, func(int, int) View { return nil }))
	d.Bind(0, shell)
	if len(shell.Children()) != 0 || len(r.root.children) != 0 {
		t.Fatal("nil row content left old widgets or mounting records")
	}
}

func TestListViewRowsFollowRootTeardownMode(t *testing.T) {
	for _, destroying := range []bool{false, true} {
		t.Run(map[bool]string{false: "detach", true: "window-destroy"}[destroying], func(t *testing.T) {
			r := newRoot()
			tracker := new(lifecycleTracker)
			lv := r.update(ListView(SliceList([]int{1, 2}), func(int, int) View {
				return &lifecycleView{tracker: tracker}
			})).(*gui.ListView)
			lv.LayoutVisible(geometry.Size{Width: 100, Height: 100}, geometry.Point{})
			shells := lv.Children()
			rows := []gui.Widget{shells[0].Children()[0], shells[1].Children()[0]}
			d := lv.Delegate().(*uiItemDelegate[int])
			if destroying {
				r.unmountForWindowDestroy()
			} else {
				r.unmountWindow()
			}
			if !slices.Equal(lv.Children(), shells) {
				t.Fatal("Root detached ListView-owned shells")
			}
			for i, shell := range shells {
				if destroying {
					if len(shell.Children()) != 1 || shell.Children()[0] != rows[i] {
						t.Fatal("window destruction lost rows before GUI cleanup")
					}
				} else if len(shell.Children()) != 0 || rows[i].Parent() != nil {
					t.Fatal("ordinary unmount retained declarative row content")
				}
			}
			if tracker.mounts != 2 || tracker.unmounts != 2 || d.ctx != nil {
				t.Fatalf("rows were not released exactly once: %+v", tracker)
			}
			d.Unbind(0, shells[0]) // A late delegate call cannot dismantle retained rows.
			r.unmountWindow()
		})
	}
}
