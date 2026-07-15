package gui

import (
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/layout"
)

type testWidget struct {
	WidgetBase
}

func (w *testWidget) AddChild(child Widget) {
	w.WidgetBase.AddChild(w, child)
}

func newTestWidget() *testWidget {
	return new(testWidget)
}

func TestWidgetBaseZeroValueIsVisible(t *testing.T) {
	widget := newTestWidget()

	if !widget.Visible() {
		t.Fatal("zero value widget should be visible")
	}

	widget.SetVisible(false)
	if widget.Visible() {
		t.Fatal("widget should be hidden")
	}
}

func TestWidgetBaseStyleName(t *testing.T) {
	win := &window{}
	widget := newTestWidget()
	win.SetWidget(widget)
	win.layoutDirty = false
	win.paintDirty = false

	// StyleName is the explicit override: empty when unset (round-trips SetStyleName).
	if widget.StyleName() != "" {
		t.Fatalf("unset style override should be empty, got %q", widget.StyleName())
	}
	widget.SetStyleName("custom")
	if widget.StyleName() != "custom" {
		t.Fatalf("style name was not set: %q", widget.StyleName())
	}
	if !win.layoutDirty || !win.paintDirty {
		t.Fatal("setting style name did not request layout")
	}
}

func TestWidgetBaseChildTree(t *testing.T) {
	parent := newTestWidget()
	child := newTestWidget()

	parent.AddChild(child)

	if child.Parent() != parent {
		t.Fatal("child parent was not set")
	}
	if children := parent.Children(); len(children) != 1 || children[0] != child {
		t.Fatalf("unexpected children: %v", children)
	}

	parent.RemoveChild(child)

	if child.Parent() != nil {
		t.Fatal("child parent was not cleared")
	}
	if children := parent.Children(); len(children) != 0 {
		t.Fatalf("expected no children, got %d", len(children))
	}
}

func TestWidgetBaseSetRootPropagatesToChildren(t *testing.T) {
	win := &window{}
	parent := newTestWidget()
	child := newTestWidget()
	parent.AddChild(child)

	win.SetWidget(parent)

	if parent.Root() != win {
		t.Fatal("parent root was not set")
	}
	if child.Root() != win {
		t.Fatal("child root was not set")
	}
	if parent.Window() != win {
		t.Fatal("parent window was not set")
	}
	if child.Window() != win {
		t.Fatal("child window was not set")
	}
}

func TestWidgetBaseRequestLayoutMarksWindowDirty(t *testing.T) {
	win := &window{}
	widget := newTestWidget()
	win.SetWidget(widget)

	widget.RequestLayout()

	if !win.layoutDirty {
		t.Fatal("window layout dirty was not set")
	}
	if !win.paintDirty {
		t.Fatal("window paint dirty was not set")
	}
}

func TestWidgetBaseArrangeAndSnapshot(t *testing.T) {
	parent := newTestWidget()
	parent.SetID("parent")
	parent.SetFocusable(true)
	child := newTestWidget()
	child.SetID("child")
	parent.AddChild(child)

	parent.Arrange(geometry.Rect(1, 2, 30, 40))
	child.Arrange(geometry.Rect(3, 4, 10, 20))

	info := parent.Snapshot()
	if info.ID != "parent" || info.Bounds != geometry.Rect(1, 2, 30, 40) {
		t.Fatalf("unexpected parent snapshot: %+v", info)
	}
	if !info.Focusable {
		t.Fatal("snapshot did not include focusable state")
	}
	if len(info.Children) != 1 {
		t.Fatalf("expected 1 child snapshot, got %d", len(info.Children))
	}
	if info.Children[0].ID != "child" || info.Children[0].Bounds != geometry.Rect(4, 6, 10, 20) {
		t.Fatalf("unexpected child snapshot: %+v", info.Children[0])
	}
}

func TestWidgetBaseSnapshotBoundsAreWindowLocal(t *testing.T) {
	root := newTestWidget()
	parent := newTestWidget()
	child := newTestWidget()
	root.AddChild(parent)
	parent.AddChild(child)

	root.Arrange(geometry.Rect(10, 20, 100, 100))
	parent.Arrange(geometry.Rect(3, 4, 50, 50))
	child.Arrange(geometry.Rect(5, 6, 10, 10))

	info := child.Snapshot()
	if info.Bounds != geometry.Rect(18, 30, 10, 10) {
		t.Fatalf("unexpected child window-local bounds: %+v", info.Bounds)
	}
}

func TestWidgetBaseLayoutManagerUsesVisibleChildren(t *testing.T) {
	parent := newTestWidget()
	visible := newTestWidget()
	hidden := newTestWidget()
	hidden.SetVisible(false)
	parent.AddChild(visible)
	parent.AddChild(hidden)

	manager := &testLayoutManager{
		measureSize: geometry.Size{Width: 11, Height: 12},
	}
	parent.SetLayoutManager(manager)

	size := parent.Measure(layout.Loose(geometry.Size{Width: 100, Height: 80}))
	if size != (geometry.Size{Width: 11, Height: 12}) {
		t.Fatalf("unexpected measured size: %+v", size)
	}
	if len(manager.measured) != 1 || manager.measured[0] != visible {
		t.Fatalf("layout measured unexpected elements: %v", manager.measured)
	}

	parent.Arrange(geometry.Rect(20, 30, 100, 80))
	if len(manager.arranged) != 1 || manager.arranged[0] != visible {
		t.Fatalf("layout arranged unexpected elements: %v", manager.arranged)
	}
	if manager.arrangeRect != geometry.Rect(0, 0, 100, 80) {
		t.Fatalf("layout arrange rect should be parent-local, got %+v", manager.arrangeRect)
	}
}

func TestWidgetBaseEventControllers(t *testing.T) {
	widget := newTestWidget()
	controller := new(testControllerAdapter)

	widget.AddEventController(controller)

	if controllers := widget.EventControllers(); len(controllers) != 1 || controllers[0] != controller {
		t.Fatalf("unexpected controllers: %v", controllers)
	}

	widget.RemoveEventController(controller)

	if controllers := widget.EventControllers(); len(controllers) != 0 {
		t.Fatalf("expected no controllers, got %d", len(controllers))
	}
}

func TestWidgetBaseFocusStateAndSignal(t *testing.T) {
	win := &window{}
	widget := newTestWidget()
	win.SetWidget(widget)

	var calls []bool
	var focusableAtChange []bool
	widget.ConnectFocused(func(focused bool) {
		calls = append(calls, focused)
		focusableAtChange = append(focusableAtChange, widget.Focusable())
	})

	widget.SetFocusable(true)
	if !win.SetFocusedWidget(widget) {
		t.Fatal("focusable widget should accept focus")
	}
	if win.FocusedWidget() != widget || !widget.Focused() {
		t.Fatal("focused state was not set")
	}
	info := widget.Snapshot()
	if !info.Focusable || !info.Focused || !info.ContainsFocus {
		t.Fatalf("snapshot missing focus state: %+v", info)
	}

	widget.SetFocusable(false)
	if win.FocusedWidget() != nil || widget.Focused() {
		t.Fatal("removing focusable state should clear focus")
	}
	if len(calls) != 2 || !calls[0] || calls[1] {
		t.Fatalf("unexpected focus changed calls: %v", calls)
	}
	if len(focusableAtChange) != 2 || !focusableAtChange[0] || focusableAtChange[1] {
		t.Fatalf("focus callbacks saw stale focusable state: %v", focusableAtChange)
	}
}

func TestWidgetBaseContainsFocusStateAndSignal(t *testing.T) {
	win := &window{}
	parent := newTestWidget()
	child := newTestWidget()
	child.SetFocusable(true)
	parent.AddChild(child)
	win.SetWidget(parent)

	var parentFocused []bool
	var parentContainsFocus []bool
	var childFocused []bool
	parent.ConnectFocused(func(focused bool) {
		parentFocused = append(parentFocused, focused)
	})
	parent.ConnectContainsFocus(func(containsFocus bool) {
		parentContainsFocus = append(parentContainsFocus, containsFocus)
	})
	child.ConnectFocused(func(focused bool) {
		childFocused = append(childFocused, focused)
	})

	if !win.SetFocusedWidget(child) {
		t.Fatal("focusable child should accept focus")
	}
	if parent.Focused() || !parent.ContainsFocus() {
		t.Fatalf("unexpected parent focus state: focused=%v contains=%v", parent.Focused(), parent.ContainsFocus())
	}
	if !child.Focused() || !child.ContainsFocus() {
		t.Fatalf("unexpected child focus state: focused=%v contains=%v", child.Focused(), child.ContainsFocus())
	}

	win.SetFocusedWidget(nil)
	if parent.Focused() || parent.ContainsFocus() || child.Focused() || child.ContainsFocus() {
		t.Fatalf("focus state was not cleared: parent focused=%v contains=%v child focused=%v contains=%v",
			parent.Focused(), parent.ContainsFocus(), child.Focused(), child.ContainsFocus())
	}
	if len(parentFocused) != 0 {
		t.Fatalf("parent should not receive target focused changes: %v", parentFocused)
	}
	if len(parentContainsFocus) != 2 || !parentContainsFocus[0] || parentContainsFocus[1] {
		t.Fatalf("unexpected parent contains focus calls: %v", parentContainsFocus)
	}
	if len(childFocused) != 2 || !childFocused[0] || childFocused[1] {
		t.Fatalf("unexpected child focused calls: %v", childFocused)
	}
}

func TestWidgetBaseHidingFocusedSubtreeClearsFocus(t *testing.T) {
	win := &window{}
	root := newTestWidget()
	parent := newTestWidget()
	child := newTestWidget()
	child.SetFocusable(true)
	root.AddChild(parent)
	parent.AddChild(child)
	win.SetWidget(root)
	win.SetFocusedWidget(child)

	parent.SetVisible(false)

	if win.FocusedWidget() != nil {
		t.Fatalf("focused widget was not cleared: %v", win.FocusedWidget())
	}
	if child.Focused() || child.ContainsFocus() || parent.ContainsFocus() {
		t.Fatal("child focused state was not cleared")
	}
}

func TestWidgetBaseLifecycleMountsAndUnmountsSubtree(t *testing.T) {
	var calls []string
	win := &window{}
	parent := newLifecycleWidget("parent", &calls)
	child := newLifecycleWidget("child", &calls)
	parent.AddChild(child)
	parent.ConnectMount(func() {
		if parent.Window() != win || parent.Root() != win || parent.Parent() != nil {
			t.Fatalf("parent mount saw invalid relationship: window=%v root=%v parent=%v", parent.Window(), parent.Root(), parent.Parent())
		}
	})
	child.ConnectMount(func() {
		if child.Window() != win || child.Root() != win || child.Parent() != parent {
			t.Fatalf("child mount saw invalid relationship: window=%v root=%v parent=%v", child.Window(), child.Root(), child.Parent())
		}
	})
	parent.ConnectUnmount(func() {
		if parent.Window() != win || parent.Root() != win || parent.Parent() != nil {
			t.Fatalf("parent unmount saw invalid relationship: window=%v root=%v parent=%v", parent.Window(), parent.Root(), parent.Parent())
		}
	})
	child.ConnectUnmount(func() {
		if child.Window() != win || child.Root() != win || child.Parent() != parent {
			t.Fatalf("child unmount saw invalid relationship: window=%v root=%v parent=%v", child.Window(), child.Root(), child.Parent())
		}
	})

	if len(calls) != 0 {
		t.Fatalf("lifecycle fired before mount: %v", calls)
	}

	win.SetWidget(parent)
	assertStrings(t, calls, []string{
		"parent mount",
		"child mount",
	})

	calls = nil
	win.SetWidget(nil)
	assertStrings(t, calls, []string{
		"child unmount",
		"parent unmount",
	})
	if parent.Window() != nil || parent.Root() != nil {
		t.Fatal("parent relationship was not cleared after unmount")
	}
	if child.Window() != nil || child.Root() != nil || child.Parent() != parent {
		t.Fatal("child subtree relationship unexpected after root unmount")
	}
}

func TestWidgetBaseLifecycleMountsChildAddedToMountedParent(t *testing.T) {
	var calls []string
	win := &window{}
	parent := newLifecycleWidget("parent", &calls)
	child := newLifecycleWidget("child", &calls)
	win.SetWidget(parent)

	calls = nil
	parent.AddChild(child)
	assertStrings(t, calls, []string{"child mount"})
	if child.Window() != win {
		t.Fatal("child window was not set")
	}

	calls = nil
	child.ConnectUnmount(func() {
		if child.Window() != win || child.Root() != win || child.Parent() != parent {
			t.Fatalf("child unmount saw invalid relationship: window=%v root=%v parent=%v", child.Window(), child.Root(), child.Parent())
		}
	})
	parent.RemoveChild(child)
	assertStrings(t, calls, []string{"child unmount"})
	if child.Window() != nil {
		t.Fatal("child window was not cleared")
	}
	if child.Parent() != nil {
		t.Fatal("child parent was not cleared")
	}
}

func TestAddChildReparentsWithinSameRootWithoutLifecycle(t *testing.T) {
	var calls []string
	win := &window{}
	root := newLifecycleWidget("root", &calls)
	first := newLifecycleWidget("first", &calls)
	second := newLifecycleWidget("second", &calls)
	child := newLifecycleWidget("child", &calls)
	root.AddChild(first)
	root.AddChild(second)
	first.AddChild(child)
	win.SetWidget(root)

	calls = nil
	second.AddChild(child)

	if child.Parent() != second {
		t.Fatal("child was not reparented")
	}
	if children := first.Children(); len(children) != 0 {
		t.Fatalf("old parent still has children: %v", children)
	}
	if children := second.Children(); len(children) != 1 || children[0] != child {
		t.Fatalf("new parent children unexpected: %v", children)
	}
	if child.Root() != win {
		t.Fatal("child root changed unexpectedly")
	}
	if len(calls) != 0 {
		t.Fatalf("same-root reparent should not fire lifecycle: %v", calls)
	}
}

func TestAddChildReparentsWithinSameRootKeepsFocus(t *testing.T) {
	win := &window{}
	root := newTestWidget()
	first := newTestWidget()
	second := newTestWidget()
	child := newTestWidget()
	child.SetFocusable(true)
	root.AddChild(first)
	root.AddChild(second)
	first.AddChild(child)
	win.SetWidget(root)
	win.SetFocusedWidget(child)

	second.AddChild(child)

	if win.FocusedWidget() != child || !child.Focused() || !child.ContainsFocus() || !second.ContainsFocus() {
		t.Fatal("same-root reparent should keep focused widget")
	}
	if first.ContainsFocus() {
		t.Fatal("old parent should not contain focus after same-root reparent")
	}
}

func TestRemoveChildClearsFocusedSubtreeAfterUnmount(t *testing.T) {
	win := &window{}
	parent := newTestWidget()
	child := newTestWidget()
	child.SetFocusable(true)
	parent.AddChild(child)
	win.SetWidget(parent)
	win.SetFocusedWidget(child)

	child.ConnectUnmount(func() {
		if child.Window() != win || child.Parent() != parent {
			t.Fatalf("unmount saw invalid relationship: window=%v parent=%v", child.Window(), child.Parent())
		}
		if win.FocusedWidget() != child || !child.Focused() || !child.ContainsFocus() {
			t.Fatal("focus should still be visible during unmount signal")
		}
	})

	parent.RemoveChild(child)

	if win.FocusedWidget() != nil || child.Focused() || child.ContainsFocus() || parent.ContainsFocus() {
		t.Fatal("focused subtree was not cleared after remove")
	}
}

func TestDestroyWidgetUnmountsAndClearsSubtreeOnce(t *testing.T) {
	var calls []string
	win := &window{}
	root := newLifecycleWidget("root", &calls)
	child := newLifecycleWidget("child", &calls)
	root.AddChild(child)
	win.SetWidget(root)

	calls = nil
	root.base().destroy(root)
	assertStrings(t, calls, []string{
		"child unmount",
		"root unmount",
	})
	if win.Widget() != nil {
		t.Fatal("destroyed root was not removed from window")
	}
	if root.Window() != nil || child.Window() != nil {
		t.Fatal("destroyed widgets still have a window")
	}
	if child.Parent() != nil {
		t.Fatal("destroyed child still has a parent")
	}
	if len(root.Children()) != 0 {
		t.Fatal("destroyed root still has children")
	}

	calls = nil
	root.base().destroy(root)
	child.base().destroy(child)
	if len(calls) != 0 {
		t.Fatalf("destroy unmounted more than once: %v", calls)
	}
}

type testControllerAdapter struct{}

func (c *testControllerAdapter) Phase() PropagationPhase {
	return PhaseTarget
}

func (c *testControllerAdapter) Reset() {}

func (c *testControllerAdapter) HandleEvent(ctx EventContext) {}

func (c *testControllerAdapter) HandleCrossing(ctx CrossingContext) {}

type lifecycleWidget struct {
	WidgetBase
	name  string
	calls *[]string
}

func (w *lifecycleWidget) AddChild(child Widget) {
	w.WidgetBase.AddChild(w, child)
}

func newLifecycleWidget(name string, calls *[]string) *lifecycleWidget {
	w := &lifecycleWidget{
		name:  name,
		calls: calls,
	}
	w.ConnectMount(func() {
		*w.calls = append(*w.calls, w.name+" mount")
	})
	w.ConnectUnmount(func() {
		*w.calls = append(*w.calls, w.name+" unmount")
	})
	return w
}

func TestWidgetBaseSizeConstraint(t *testing.T) {
	// Self min lifts a smaller intrinsic and max caps a larger one; a loose parent
	// keeps the result.
	w := newTestWidget()
	w.SetMinSize(geometry.Size{Width: 50, Height: 40})
	w.SetMaxWidth(200)
	w.SetLayoutManager(&testLayoutManager{measureSize: geometry.Size{Width: 500, Height: 10}})

	got := w.Measure(layout.Loose(geometry.Size{Width: 1000, Height: 1000}))
	if got != (geometry.Size{Width: 200, Height: 40}) {
		t.Fatalf("min/max not applied: %+v (want 200x40)", got)
	}

	// The parent constraint wins over the child min: it overflows, it does not
	// push the parent.
	w2 := newTestWidget()
	w2.SetMinWidth(50)
	if got := w2.Measure(layout.Tight(geometry.Size{Width: 30, Height: 30})); got.Width != 30 {
		t.Fatalf("parent max should win over child min: %+v", got)
	}
}

type testLayoutManager struct {
	measureSize geometry.Size
	measured    []layout.Child
	arranged    []layout.Child
	arrangeRect geometry.Rectangle
}

func (l *testLayoutManager) Measure(children []layout.Child, _ layout.Constraint) geometry.Size {
	l.measured = append([]layout.Child(nil), children...)
	return l.measureSize
}

func (l *testLayoutManager) Arrange(children []layout.Child, rect geometry.Rectangle) {
	l.arranged = append([]layout.Child(nil), children...)
	l.arrangeRect = rect
}
