package gui

import (
	"encoding/json"
	"image"
	"image/color"
	"math"
	"slices"
	"strings"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/dragdrop"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/platform/graphics/software"
	"github.com/golang-gui/goui/style"
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

func TestWidgetBaseStructuralChildPreservesBinSlotAndLifecycle(t *testing.T) {
	parent := NewButton()
	content, internal, accidental := newTestWidget(), newTestWidget(), newTestWidget()
	parent.SetChild(content)
	parent.WidgetBase.AddChild(parent, accidental)
	if accidental.Parent() != nil || parent.Child() != content {
		t.Fatal("ordinary mounting bypassed the Bin content slot")
	}
	parent.WidgetBase.AddStructuralChild(parent, internal)
	if internal.Parent() != parent || parent.Child() != content || len(parent.Children()) != 2 {
		t.Fatal("structural mounting changed the public content slot")
	}
	win := &window{}
	win.SetWidget(parent)
	if internal.Root() != win {
		t.Fatal("structural child did not follow host ownership")
	}
	win.Destroy()
	if !internal.Destroyed() || !content.Destroyed() || internal.Parent() != nil {
		t.Fatal("host destruction did not clean up structural children")
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
	if size.Size != (geometry.Size{Width: 11, Height: 12}) {
		t.Fatalf("unexpected measured size: %+v", size)
	}
	if len(manager.measured) != 1 || layoutChildWidget(manager.measured[0]) != visible {
		t.Fatalf("layout measured unexpected elements: %v", manager.measured)
	}

	parent.Arrange(geometry.Rect(20, 30, 100, 80))
	if len(manager.arranged) != 1 || layoutChildWidget(manager.arranged[0]) != visible {
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

func TestWidgetReparentSurvivesLifecycleDestruction(t *testing.T) {
	for _, endpoint := range []string{"child", "target", "source"} {
		t.Run(endpoint, func(t *testing.T) {
			sourceWindow, targetWindow := &window{}, &window{}
			source, target, child := newTestWidget(), newTestWidget(), newTestWidget()
			source.AddChild(child)
			sourceWindow.SetWidget(source)
			targetWindow.SetWidget(target)
			defer sourceWindow.Destroy()
			defer targetWindow.Destroy()
			unmounts, mounts := 0, 0
			child.ConnectMount(func() { mounts++ })
			child.ConnectUnmount(func() {
				unmounts++
				switch endpoint {
				case "child":
					child.base().destroy(child)
				case "target":
					targetWindow.Destroy()
				case "source":
					sourceWindow.Destroy()
				}
			})
			target.AddChild(child)
			if unmounts != 1 || mounts != 0 || child.Root() != nil || child.Parent() != nil {
				t.Fatalf("resurrected/double-unmounted subtree: unmount=%d mount=%d root=%v parent=%v", unmounts, mounts, child.Root(), child.Parent())
			}
			if child.Destroyed() != (endpoint != "target") {
				t.Fatal("incorrect final lifecycle observation")
			}
		})
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

func (w *lifecycleWidget) RemoveChild(child Widget) {
	w.WidgetBase.RemoveChild(child)
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
	w.SetMaxSize(geometry.Size{Width: 200, Height: 0})
	w.SetLayoutManager(&testLayoutManager{measureSize: geometry.Size{Width: 500, Height: 10}})

	got := w.Measure(layout.Loose(geometry.Size{Width: 1000, Height: 1000}))
	if got.Size != (geometry.Size{Width: 200, Height: 40}) {
		t.Fatalf("min/max not applied: %+v (want 200x40)", got)
	}

	// The parent constraint wins over the child min: it overflows, it does not
	// push the parent.
	w2 := newTestWidget()
	w2.SetMinSize(geometry.Size{Width: 50, Height: 0})
	if got := w2.Measure(layout.Tight(geometry.Size{Width: 30, Height: 30})); got.Width != 30 {
		t.Fatalf("parent max should win over child min: %+v", got)
	}
}

func TestLayoutSettersNormalizeInvalidValuesBeforeInvalidating(t *testing.T) {
	box := NewLinearBox(layout.DirectionHorizontal)
	button := NewButton()
	input := NewTextInput()
	cases := []struct {
		name   string
		widget Widget
		set    func(float32)
		get    func() float32
	}{
		{"weight", box, box.SetMainWeight, box.MainWeight},
		{"spacing", box, box.SetSpacing, box.Spacing},
		{"box-padding", box, box.SetPadding, box.Padding},
		{"button-padding", button, button.SetPadding, button.Padding},
		{"input-padding", input, input.SetPadding, input.Padding},
		{"min-width", box, func(v float32) { box.SetMinSize(geometry.Size{Width: v}) }, func() float32 { return box.MinSize().Width }},
		{"min-height", box, func(v float32) { box.SetMinSize(geometry.Size{Height: v}) }, func() float32 { return box.MinSize().Height }},
		{"max-width", box, func(v float32) { box.SetMaxSize(geometry.Size{Width: v}) }, func() float32 { return box.MaxSize().Width }},
		{"max-height", box, func(v float32) { box.SetMaxSize(geometry.Size{Height: v}) }, func() float32 { return box.MaxSize().Height }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			win := &window{}
			win.SetWidget(tc.widget)
			for _, invalid := range []float32{-1, float32(math.NaN()), float32(math.Inf(1)), float32(math.Inf(-1))} {
				tc.set(6.5)
				if tc.get() != 6.5 {
					t.Fatalf("valid fractional value was changed: %v", tc.get())
				}
				win.layoutDirty = false
				tc.set(invalid)
				if tc.get() != 0 || !win.layoutDirty {
					t.Fatalf("input=%v: value=%v dirty=%v, want zero and invalidated layout", invalid, tc.get(), win.layoutDirty)
				}
				win.layoutDirty = false
				tc.set(invalid)
				if win.layoutDirty {
					t.Fatalf("repeating input=%v invalidated an unchanged normalized value", invalid)
				}
			}
		})
	}
}

func TestWidgetMeasurementCacheReusesIdenticalConstraint(t *testing.T) {
	box := NewLinearBox(layout.DirectionHorizontal)
	child := &countingMeasureWidget{size: geometry.Size{Width: 20, Height: 10}}
	box.AddChild(child)
	c := layout.Loose(geometry.Size{Width: 100, Height: 40})

	first := measureWidget(box, c)
	second := measureWidget(box, c)
	if first != second {
		t.Fatalf("cached measurement changed: first=%+v second=%+v", first, second)
	}
	if child.measures != 1 {
		t.Fatalf("identical constraint measured child %d times, want 1", child.measures)
	}
}

func TestWidgetRequestLayoutInvalidatesMeasurementToRoot(t *testing.T) {
	box := NewLinearBox(layout.DirectionHorizontal)
	child := &countingMeasureWidget{size: geometry.Size{Width: 20, Height: 10}}
	box.AddChild(child)
	c := layout.Loose(geometry.Size{Width: 100, Height: 40})

	measureWidget(box, c)
	child.RequestLayout()
	measureWidget(box, c)
	if child.measures != 2 {
		t.Fatalf("child layout invalidation left an ancestor cache live: measures=%d", child.measures)
	}
}

func TestWidgetArrangeReusesMeasurementAtSameConstraint(t *testing.T) {
	box := NewLinearBox(layout.DirectionHorizontal)
	child := &countingMeasureWidget{size: geometry.Size{Width: 20, Height: 10}}
	box.AddChild(child)
	size := geometry.Size{Width: 100, Height: 40}

	measureWidget(box, layout.Tight(size))
	box.Arrange(geometry.Rect(0, 0, size.Width, size.Height))
	if child.measures != 1 {
		t.Fatalf("Measure followed by Arrange measured child %d times, want 1", child.measures)
	}
}

func TestWidgetMeasurementCacheMissesChangedConstraint(t *testing.T) {
	child := &countingMeasureWidget{size: geometry.Size{Width: 20, Height: 10}}

	measureWidget(child, layout.Loose(geometry.Size{Width: 100, Height: 40}))
	measureWidget(child, layout.Loose(geometry.Size{Width: 200, Height: 40}))
	if child.measures != 2 {
		t.Fatalf("changed constraint reused stale measurement: measures=%d", child.measures)
	}
}

type countingMeasureWidget struct {
	WidgetBase
	size     geometry.Size
	measures int
}

func (w *countingMeasureWidget) Measure(c layout.Constraint) layout.Measurement {
	w.measures++
	return layout.Measured(c.Clamp(w.size))
}

type testLayoutManager struct {
	measureSize     geometry.Size
	measureBaseline float32
	hasBaseline     bool
	measured        []layout.Child
	arranged        []layout.Child
	arrangeRect     geometry.Rectangle
}

func layoutChildWidget(child layout.Child) Widget {
	if adapter, ok := child.(widgetLayoutChild); ok {
		return adapter.widget
	}
	widget, _ := child.(Widget)
	return widget
}

func (l *testLayoutManager) Measure(children []layout.Child, _ layout.Constraint) layout.Measurement {
	l.measured = append([]layout.Child(nil), children...)
	if l.hasBaseline {
		return layout.MeasuredWithBaseline(l.measureSize, l.measureBaseline)
	}
	return layout.Measured(l.measureSize)
}

func (l *testLayoutManager) Arrange(children []layout.Child, rect geometry.Rectangle) {
	l.arranged = append([]layout.Child(nil), children...)
	l.arrangeRect = rect
}

type widgetRenderSurface struct{ presents int }

func (s *widgetRenderSurface) Transparent() bool      { return true }
func (s *widgetRenderSurface) Draw(image.Image) error { s.presents++; return nil }

func TestRenderWidgetSubtree(t *testing.T) {
	surface := &widgetRenderSurface{}
	p, _ := software.NewPainter(surface)
	defer p.Destroy()
	w := &window{rootBase: rootBase{painter: p, width: 100, pixelWidth: 200}}
	parent := newPainterTestWidget(func(p Painter) { p.FillRect(geometry.Rect(0, 0, 100, 100), graphics.RGB(0, 255, 0)) })
	child := newPainterTestWidget(func(p Painter) {
		p.SetClipRect(geometry.Rect(1, 1, 4, 4))
		p.SetTransform(geometry.Translate(1, 1))
		p.FillRect(geometry.Rect(0, 0, 8, 8), graphics.RGBA(255, 0, 0, 128))
	})
	src := image.NewRGBA(image.Rect(0, 0, 2, 2))
	for i := 0; i < len(src.Pix); i += 4 {
		src.Pix[i+2], src.Pix[i+3] = 255, 255
	}
	picture := NewImage(src)
	child.AddChild(picture)
	parent.AddChild(child)
	w.SetWidget(parent)
	parent.Arrange(geometry.Rect(0, 0, 100, 100))
	child.Arrange(geometry.Rect(23, 17, 10, 8))
	picture.Arrange(geometry.Rect(6, 2, 2, 2))
	w.layoutDirty = false
	defer picture.releaseImage()
	// Warm the widget's image cache on its normal painter.
	p.Begin(200, 200, 2)
	p.Clear(graphics.Color{})
	paintWidget(parent, newPainter(p, geometry.Rect(0, 0, 100, 100), 2))
	p.End()
	native := picture.paintImage
	original := child.Rect()
	for _, scale := range []float32{0, 1, 2} {
		img, err := RenderWidget(child, scale)
		if err != nil {
			t.Fatal(err)
		}
		s := int(scale)
		if s == 0 {
			s = 2
		}
		if img.Bounds() != image.Rect(0, 0, 10*s, 8*s) {
			t.Fatal(img.Bounds())
		}
		for _, tc := range []struct {
			x, y int
			want color.RGBA
		}{
			{0, 0, color.RGBA{}}, {2, 2, color.RGBA{R: 128, A: 128}}, {6, 2, color.RGBA{B: 255, A: 255}}, {9, 7, color.RGBA{}},
		} {
			got := color.RGBAModel.Convert(img.At(tc.x*s, tc.y*s)).(color.RGBA)
			if got != tc.want {
				t.Fatalf("scale=%g at %d,%d: %v want %v", scale, tc.x, tc.y, got, tc.want)
			}
		}
		if picture.paintImage != native || child.Rect() != original || child.Root() != w {
			t.Fatal("render changed resources or allocation")
		}
	}
	if parent.paints != 1 || child.paints != 4 || surface.presents != 1 {
		t.Fatal("incorrect subtree painting/presentation")
	}
}

func TestRenderWidgetPreconditionsAndPanic(t *testing.T) {
	p, _ := software.NewPainter(&widgetRenderSurface{})
	defer p.Destroy()
	w := &window{rootBase: rootBase{painter: p}}
	child := newPainterTestWidget(nil)
	if _, err := RenderWidget(nil, 1); err == nil {
		t.Fatal("nil accepted")
	}
	if _, err := RenderWidget(child, 1); err == nil {
		t.Fatal("unmounted accepted")
	}
	w.SetWidget(child)
	if _, err := RenderWidget(child, 1); err == nil {
		t.Fatal("unallocated widget accepted")
	}
	child.Arrange(geometry.Rect(3, 4, 8, 8))
	img, err := RenderWidget(child, 1)
	if err != nil || img.Bounds().Dx() != 8 || img.Bounds().Dy() != 8 || !w.layoutDirty || child.Rect() != geometry.Rect(3, 4, 8, 8) {
		t.Fatalf("pending layout must render current allocation without flushing it: image=%v err=%v", img, err)
	}
	w.layoutDirty = false
	for _, scale := range []float32{-1, float32(math.NaN()), float32(math.Inf(1))} {
		if _, err := RenderWidget(child, scale); err == nil {
			t.Fatal("invalid scale accepted")
		}
	}
	child.paint = func(Painter) {
		if _, err := RenderWidget(child, 1); err == nil {
			t.Fatal("nested paint accepted")
		}
		panic("widget paint")
	}
	func() {
		defer func() {
			if recover() != "widget paint" {
				t.Fatal("panic swallowed")
			}
		}()
		_, _ = RenderWidget(child, 1)
	}()
	child.paint = nil
	if _, err := RenderWidget(child, 1); err != nil {
		t.Fatal(err)
	}
}

func TestWidgetIdentityAndRootLookup(t *testing.T) {
	win := &window{}
	overlay := NewOverlay()
	main := NewLabel("main")
	main.SetID("item")
	main.SetName("peer")
	floating := NewLabel("floating")
	floating.SetID("float")
	floating.SetName("peer")
	overlay.SetChild(main)
	overlay.AddOverlay(floating)
	win.SetWidget(overlay)
	win.layoutDirty, win.paintDirty = false, false
	main.SetName("changed")
	main.SetID("changed-id")
	if win.layoutDirty || win.paintDirty {
		t.Fatal("changing identity scheduled layout or paint")
	}
	main.SetName("peer")
	main.SetID("item")

	if got := FindWidget(win, "float"); got != floating {
		t.Fatalf("overlay lookup: widget=%v", got)
	}
	if got := FindWidgetsByName(win, "peer"); len(got) != 2 || got[0] != main || got[1] != floating {
		t.Fatalf("repeatable name lookup: %v", got)
	}
	if got := FindWidget(win, ""); got != nil {
		t.Fatalf("empty ID should not match: widget=%v", got)
	}
	if got := FindWidget(win, "absent"); got != nil {
		t.Fatalf("missing ID should not match: widget=%v", got)
	}

	info := main.Snapshot()
	if info.ID != "item" || info.Name != "peer" || info.Text != "main" {
		t.Fatalf("snapshot merged identity and text: %+v", info)
	}
	encoded, err := json.Marshal(info)
	if err != nil || !strings.Contains(string(encoded), `"name":"peer"`) {
		t.Fatalf("snapshot JSON name: %s, %v", encoded, err)
	}

	floating.SetID("item")
	if got := FindWidget(win, "item"); got != main {
		t.Fatalf("duplicate ID should select the first tree match: widget=%v", got)
	}
	main.SetID("renamed")
	if got := FindWidget(win, "item"); got != floating {
		t.Fatalf("lookup after renaming the first match: widget=%v", got)
	}
	main.SetID("item")
	overlay.RemoveOverlay(floating)
	if got := FindWidget(win, "item"); got != main {
		t.Fatalf("lookup after removal: widget=%v", got)
	}
	win.SetWidget(nil)
	if got := FindWidget(win, "item"); got != nil {
		t.Fatalf("detached widget remained visible: widget=%v", got)
	}
}

func TestWidgetIDScopedToRoot(t *testing.T) {
	win := &window{}
	anchor := NewLabel("anchor")
	anchor.SetID("shared")
	win.SetWidget(anchor)

	pop := NewPopover(anchor, nil)
	child := NewLabel("popover")
	child.SetID("shared")
	pop.SetWidget(child)
	if got := FindWidget(win, "shared"); got != anchor {
		t.Fatalf("window lookup: widget=%v", got)
	}
	if got := FindWidget(pop, "shared"); got != child {
		t.Fatalf("popover lookup: widget=%v", got)
	}
	if child.Root() != pop {
		t.Fatal("popover content has a different root")
	}
	child.SetName("popover-content")
	if got := FindWidgetsByName(pop, "popover-content"); len(got) != 1 || got[0] != child {
		t.Fatalf("popover name lookup: %v", got)
	}
	// Root requests are available through the public interface even before Show.
	pop.RequestLayout()
	if err := pop.RequestPaint(); err != nil {
		t.Fatalf("popover repaint before Show: %v", err)
	}
}

func TestWidgetBaseMoveChildPreservesMountedIdentity(t *testing.T) {
	win := &window{}
	parent := newTestWidget()
	a, b, c := newTestWidget(), newTestWidget(), newTestWidget()
	parent.AddChild(a)
	parent.AddChild(b)
	parent.AddChild(c)
	mounts, unmounts := 0, 0
	b.ConnectMount(func() { mounts++ })
	b.ConnectUnmount(func() { unmounts++ })
	b.SetFocusable(true)
	win.SetWidget(parent)
	if mounts != 1 {
		t.Fatalf("initial mount count = %d", mounts)
	}
	if !win.SetFocusedWidget(b) {
		t.Fatal("failed to focus mounted child")
	}
	win.layoutDirty, win.paintDirty = false, false
	parent.MoveChildBefore(b, a)
	children := parent.Children()
	if len(children) != 3 || children[0] != b || children[1] != a || children[2] != c {
		t.Fatalf("unexpected child order: %v", children)
	}
	if b.Parent() != parent || b.Root() != win || win.FocusedWidget() != b || mounts != 1 || unmounts != 0 {
		t.Fatalf("move changed ownership or lifecycle: parent=%v mounts=%d unmounts=%d", b.Parent(), mounts, unmounts)
	}
	if !win.layoutDirty || !win.paintDirty {
		t.Fatal("reordering must request layout and paint")
	}
	parent.MoveChildAfter(b, c)
	parent.MoveChildBefore(b, a)
	if b.Root() != win || win.FocusedWidget() != b || mounts != 1 || unmounts != 0 {
		t.Fatal("relative moves disturbed mounted identity or focus")
	}
	win.layoutDirty, win.paintDirty = false, false
	parent.MoveChildAfter(b, nil)  // already first
	parent.MoveChildBefore(c, nil) // already last
	parent.MoveChildBefore(b, a)   // already adjacent
	parent.MoveChildAfter(a, b)    // already adjacent
	parent.MoveChildBefore(b, b)
	parent.MoveChildAfter(nil, a)
	parent.MoveChildBefore(b, newTestWidget())
	parent.MoveChildAfter(newTestWidget(), a)
	if got := parent.Children(); got[0] != b || got[1] != a || got[2] != c {
		t.Fatalf("invalid move changed order: %v", got)
	}
	if win.layoutDirty || win.paintDirty {
		t.Fatal("no-op moves requested a frame")
	}
	win.SetWidget(nil)
}

func TestWidgetRelativeMoveOrder(t *testing.T) {
	for _, tc := range []struct {
		name, child, sibling, want string
		after                      bool
	}{
		{"before forward", "a", "c", "bac", false},
		{"before backward", "c", "a", "cab", false},
		{"before end", "a", "", "bca", false},
		{"after forward", "a", "c", "bca", true},
		{"after backward", "c", "a", "acb", true},
		{"after start", "c", "", "cab", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			parent := newTestWidget()
			children := map[string]Widget{}
			for _, id := range []string{"a", "b", "c"} {
				child := newPainterTestWidget(nil)
				child.SetID(id)
				children[id] = child
				parent.AddChild(child)
			}
			if tc.after {
				parent.MoveChildAfter(children[tc.child], children[tc.sibling])
			} else {
				parent.MoveChildBefore(children[tc.child], children[tc.sibling])
			}
			bounds := geometry.Rect(0, 0, 100, 100)
			parent.Arrange(bounds)
			var painted string
			for _, child := range children {
				child.Arrange(bounds) // fully overlapping: order decides picking
				id := child.ID()
				child.(*painterTestWidget).paint = func(Painter) { painted += id }
			}
			paintWidget(parent, newPainter(new(recordingPainterBackend), bounds, 1))
			if painted != tc.want {
				t.Fatalf("paint order=%s want=%s", painted, tc.want)
			}
			picked := Pick(parent, geometry.Point{X: 10, Y: 10})
			if picked != children[tc.want[2:]] {
				t.Fatalf("picked=%s want=%s", picked.ID(), tc.want[2:])
			}
			for i, info := range parent.Snapshot().Children {
				if info.ID != tc.want[i:i+1] || parent.Children()[i].ID() != info.ID {
					t.Fatalf("snapshot and Children order disagree with %s", tc.want)
				}
			}
		})
	}
}

func TestWidgetRelativeMoveRejectsForeignAndDestroyedNodes(t *testing.T) {
	parent, other := newTestWidget(), newTestWidget()
	a, b, foreign := newTestWidget(), newTestWidget(), newTestWidget()
	parent.AddChild(a)
	parent.AddChild(b)
	other.AddChild(foreign)
	parent.MoveChildBefore(a, foreign)
	parent.MoveChildAfter(foreign, b)
	dead := newTestWidget()
	dead.destroy(dead)
	parent.MoveChildBefore(dead, b)
	parent.MoveChildAfter(a, dead)
	if !slices.Equal(parent.Children(), []Widget{a, b}) || foreign.Parent() != other {
		t.Fatal("invalid move changed ownership/order")
	}
	parent.destroy(parent)
	parent.MoveChildBefore(a, nil)
	parent.MoveChildAfter(foreign, nil)
	if len(parent.Children()) != 0 || foreign.Parent() != other {
		t.Fatal("destroyed parent accepted children")
	}
}

func TestWidgetRelativeMoveAffectsOrderedLayout(t *testing.T) {
	box := NewLinearBox(layout.DirectionHorizontal)
	a := newSizedWidget(geometry.Size{Width: 10, Height: 20})
	b := newSizedWidget(geometry.Size{Width: 30, Height: 20})
	box.AddChild(a)
	box.AddChild(b)
	box.MoveChildBefore(b, a)
	box.Measure(layout.Loose(geometry.Size{Width: 100, Height: 40}))
	box.Arrange(geometry.Rect(0, 0, 100, 40))
	if b.Rect().X != 0 || a.Rect().X != 30 {
		t.Fatalf("layout ignored sibling order: a=%v b=%v", a.Rect(), b.Rect())
	}
}

func TestWidgetEnabledPropagationAndReparent(t *testing.T) {
	root, group, child, explicit := newTestWidget(), newTestWidget(), NewLabel("child"), NewButton()
	root.AddChild(group)
	group.AddChild(child)
	group.AddChild(explicit)
	explicit.SetEnabled(false)
	if IsEnabled(nil) || !root.Enabled() || !IsEnabled(child) {
		t.Fatal("nil or zero-value enabled state")
	}
	group.SetEnabled(false)
	group.SetEnabled(false)
	if !child.Enabled() || IsEnabled(child) || explicit.Enabled() || !explicit.Focusable() {
		t.Fatal("ancestor overwrote own settings")
	}
	group.SetEnabled(true)
	if !IsEnabled(child) || IsEnabled(explicit) {
		t.Fatal("enable did not preserve explicit child disable")
	}

	// Reparenting between disabled parents must not enable even temporarily.
	other := newTestWidget()
	other.SetEnabled(false)
	group.SetEnabled(false)
	revision := child.enabledRevision
	other.AddChild(child)
	if child.enabledRevision != revision || IsEnabled(child) {
		t.Fatal("reparent published the intermediate detached state")
	}
	root.AddChild(child)
	if !IsEnabled(child) || child.enabledRevision != revision+1 {
		t.Fatal("reparent to enabled parent did not restore state")
	}
	root.SetEnabled(false)
	root.RemoveChild(child)
	if !IsEnabled(child) || child.Parent() != nil {
		t.Fatal("detached subtree retained ancestor restriction")
	}
}

func TestWidgetEnabledCancellationReentry(t *testing.T) {
	win, root, child := &window{}, newTestWidget(), NewButton()
	root.AddChild(child)
	win.SetWidget(root)
	t.Cleanup(func() { win.SetWidget(nil) })
	resets := 0
	child.AddEventController(&enabledTestController{reset: func() { resets++ }})
	child.ConnectFocused(func(focused bool) {
		if !focused {
			if IsEnabled(root) || IsEnabled(child) {
				t.Fatal("focus callback observed a partially updated subtree")
			}
			root.SetEnabled(true)
		}
	})
	win.SetFocusedWidget(child)
	root.SetEnabled(false)
	if !IsEnabled(root) || !IsEnabled(child) || resets != 0 || win.FocusedWidget() != nil {
		t.Fatal("stale cleanup continued after reenable, or focus was restored")
	}
	child.ConnectFocused(func(focused bool) {
		if !focused {
			root.destroy(root)
		}
	})
	win.SetFocusedWidget(child)
	root.SetEnabled(false)
	if !root.Destroyed() || !child.Destroyed() {
		t.Fatal("cancellation callback destruction was not final")
	}
}

func TestWidgetEnabledPointerDoesNotPassThrough(t *testing.T) {
	win := &window{}
	root, back, front := newTestWidget(), NewButton(), NewButton()
	root.AddChild(back)
	root.AddChild(front)
	win.SetWidget(root)
	t.Cleanup(func() { win.SetWidget(nil) })
	for _, w := range []Widget{root, back, front} {
		w.Arrange(geometry.Rect(0, 0, 100, 40))
	}
	backClicks, frontClicks := 0, 0
	back.ConnectClicked(func() { backClicks++ })
	front.ConnectClicked(func() { frontClicks++ })
	pointer := func(kind events.EventType) {
		t.Helper()
		if err := win.DispatchEvent(events.PointerEvent{EventType: kind, Button: events.PointerButtonLeft,
			Position: geometry.Point{X: 10, Y: 10}}); err != nil {
			t.Fatal(err)
		}
	}
	front.SetEnabled(false)
	if Pick(root, geometry.Point{X: 10, Y: 10}) != front {
		t.Fatal("disabled widget disappeared from geometric picking")
	}
	pointer(events.PointerDown)
	pointer(events.PointerUp)
	if backClicks != 0 || frontClicks != 0 || win.FocusedWidget() != nil {
		t.Fatal("disabled target passed input or acquired focus")
	}
	front.SetEnabled(true)
	pointer(events.PointerDown)
	if !front.pressed {
		t.Fatal("enabled button was not pressed")
	}
	front.SetEnabled(false)
	if front.pressed || front.hovered || win.FocusedWidget() != nil {
		t.Fatal("disable did not cancel press, hover and focus")
	}
	front.SetEnabled(true)
	pointer(events.PointerUp)
	if frontClicks != 0 || len(front.EventControllers()) != 2 {
		t.Fatal("old press activated or controller registration changed")
	}
	pointer(events.PointerDown)
	pointer(events.PointerUp)
	if frontClicks != 1 {
		t.Fatal("new press did not recover after enabling")
	}
}

func TestWidgetEnabledChildBlocksAncestorActivation(t *testing.T) {
	win, button, label := &window{}, NewButton(), NewLabel("disabled part")
	button.SetChild(label)
	label.SetEnabled(false)
	win.SetWidget(button)
	button.Arrange(geometry.Rect(0, 0, 100, 40))
	label.Arrange(geometry.Rect(0, 0, 100, 40))
	t.Cleanup(func() { win.SetWidget(nil) })
	calls := 0
	button.ConnectClicked(func() { calls++ })
	for _, kind := range []events.EventType{events.PointerDown, events.PointerUp} {
		_ = win.DispatchEvent(events.PointerEvent{EventType: kind, Button: events.PointerButtonLeft, Position: geometry.Point{X: 10, Y: 10}})
	}
	if calls != 0 {
		t.Fatal("disabled child activated an ancestor")
	}
}

func TestWidgetEnabledFocusTabAndShortcuts(t *testing.T) {
	win, root := &window{}, newTestWidget()
	group, first, next := newTestWidget(), NewButton(), NewButton()
	root.AddChild(group)
	group.AddChild(first)
	root.AddChild(next)
	win.SetWidget(root)
	t.Cleanup(func() { win.SetWidget(nil) })
	if !win.SetFocusedWidget(first) {
		t.Fatal("initial focus")
	}
	group.SetEnabled(false)
	if win.FocusedWidget() != nil || win.SetFocusedWidget(first) || !first.Focusable() {
		t.Fatal("disabled subtree retained focus or lost its configuration")
	}
	_ = win.DispatchEvent(events.KeyEvent{EventType: events.KeyDown, Key: events.KeyTab})
	if win.FocusedWidget() != next {
		t.Fatal("Tab did not skip disabled subtree")
	}
	group.SetEnabled(true)
	if win.FocusedWidget() != next {
		t.Fatal("reenable stole focus")
	}
	local, global := 0, 0
	controller := NewShortcutController()
	localShortcut := NewShortcut(KeyGesture{Key: KeyF5})
	localShortcut.ConnectActivate(func() { local++ })
	controller.AddShortcut(localShortcut)
	first.AddEventController(controller)
	globalShortcut := NewShortcut(KeyGesture{Key: KeyF5})
	globalShortcut.ConnectActivate(func() { global++ })
	win.Shortcuts().AddShortcut(globalShortcut)
	win.SetFocusedWidget(first)
	_ = win.DispatchEvent(events.KeyEvent{EventType: events.KeyDown, Key: events.KeyF5})
	root.SetEnabled(false)
	_ = win.DispatchEvent(events.KeyEvent{EventType: events.KeyDown, Key: events.KeyF5})
	if local != 1 || global != 1 {
		t.Fatalf("shortcut scopes: local=%d global=%d", local, global)
	}
}

func TestWidgetEnabledWheelAndDispatchReentrancy(t *testing.T) {
	win, root, child := &window{}, newTestWidget(), newTestWidget()
	root.AddChild(child)
	win.SetWidget(root)
	t.Cleanup(func() { win.SetWidget(nil) })
	root.Arrange(geometry.Rect(0, 0, 100, 40))
	child.Arrange(geometry.Rect(0, 0, 100, 40))
	parentCalls, childCalls := 0, 0
	parent := &enabledTestController{handle: func(ctx EventContext) {
		if _, wheel := ctx.Event().(events.WheelEvent); wheel {
			parentCalls++
		}
	}}
	root.AddEventController(parent)
	child.AddEventController(&enabledTestController{handle: func(EventContext) { childCalls++ }})
	child.SetEnabled(false)
	_ = win.DispatchEvent(events.WheelEvent{Position: geometry.Point{X: 10, Y: 10}})
	if parentCalls != 1 || childCalls != 0 {
		t.Fatal("wheel did not stay on enabled ancestor path")
	}
	child.SetEnabled(true)
	parent.SetPhase(PhaseCapture)
	parent.handle = func(EventContext) { child.SetEnabled(false); child.SetEnabled(true) }
	_ = win.DispatchEvent(events.PointerEvent{EventType: events.PointerDown, Position: geometry.Point{X: 10, Y: 10}})
	if childCalls != 0 {
		t.Fatal("dispatch continued after disable/reenable within a callback")
	}
}

type enabledTestController struct {
	EventControllerBase
	handle func(EventContext)
	reset  func()
}

func (c *enabledTestController) HandleEvent(ctx EventContext) { c.handle(ctx) }
func (c *enabledTestController) Reset() {
	if c.reset != nil {
		c.reset()
	}
}

func TestWidgetEnabledLabelStyleAndSnapshot(t *testing.T) {
	previous := App
	App = &application{style: style.Sheet(
		style.Name("custom-label").FontSize(12).ForegroundColor(color.Black),
		style.Name("custom-label").State(style.Disabled).FontSize(18).ForegroundColor(color.Gray{Y: 140}),
	)}
	t.Cleanup(func() { App = previous })
	button, label := NewButton(), NewLabel("retained")
	label.SetStyleName("custom-label")
	button.SetChild(label)
	ensureWidgetStyle(label)
	label.layoutValid = true
	button.SetEnabled(false)
	if !label.layoutValid || label.styleValid {
		t.Fatal("style hook was not deferred, or enabled state failed to invalidate style")
	}
	ensureWidgetStyle(label)
	format := label.resolvedTextFormat()
	info := button.Snapshot()
	if label.layoutValid || format.Font.Size != 18 || info.Enabled || len(info.Actions) != 0 ||
		len(info.Children) != 1 || info.Children[0].Enabled || info.Children[0].Text != "retained" || label.StyleName() != "custom-label" {
		t.Fatal("disabled style/cache/semantic state did not follow the subtree")
	}
	button.SetEnabled(true)
	if label.resolvedTextFormat().Font.Size != 12 || len(button.Snapshot().Actions) != 1 {
		t.Fatal("style or semantics did not recover")
	}
}

func TestWidgetEnabledIconUsesOwnDisabledForeground(t *testing.T) {
	app := &application{style: style.Sheet(
		style.Name("parent").ForegroundColor(color.RGBA{R: 255, A: 255}),
		style.Name("custom-icon").ForegroundColor(color.RGBA{B: 255, A: 255}),
		style.Name("custom-icon").State(style.Disabled).ForegroundColor(color.Gray{Y: 140}),
	)}
	useTestApplication(t, app)
	var got Color
	icon := NewIcon(iconTestDraw(func(_ Painter, _ geometry.Rectangle, foreground Color) { got = foreground }))
	icon.SetStyleName("custom-icon")
	group := newTestWidget()
	group.SetStyleName("parent")
	group.AddChild(icon)
	icon.Arrange(geometry.Rect(0, 0, 20, 20))
	paintIconTest(icon)
	if got.B != 1 || got.R != 0 {
		t.Fatal("icon inherited the parent foreground")
	}
	group.SetEnabled(false)
	paintIconTest(icon)
	if got != graphics.ColorOf(color.Gray{Y: 140}) {
		t.Fatal("disabled ancestor did not invalidate the icon's own foreground cache")
	}
	group.SetEnabled(true)
	paintIconTest(icon)
	if got.B != 1 || got.R != 0 {
		t.Fatal("icon did not restore its own normal foreground")
	}
}

func TestWidgetEnabledStyleHooksCoalesceAndInitializeDetachedWidgets(t *testing.T) {
	app := &application{style: style.Sheet(
		style.Name("probe").FontSize(10),
		style.Name("probe").State(style.Disabled).FontSize(20),
	)}
	useTestApplication(t, app)
	probe := &styleProbe{}
	probe.SetStyleName("probe")
	var states []bool
	probe.onStyle = func() {
		enabled := IsEnabled(probe)
		states = append(states, enabled)
		state := style.Normal
		if !enabled {
			state = style.Disabled
		}
		probe.size, _ = ResolveStyle("probe", "", state).FontSize()
	}
	probe.SetEnabled(false) // no mounted identity is needed to record invalidation
	if probe.notified != 0 || measureWidget(probe, layout.Unbounded()).Width != 20 {
		t.Fatal("detached state did not initialize disabled resources on first use")
	}
	probe.SetEnabled(true)
	group := newTestWidget()
	group.AddChild(probe)
	group.SetEnabled(false)
	group.SetEnabled(true)
	if probe.notified != 1 || measureWidget(probe, layout.Unbounded()).Width != 10 ||
		!slices.Equal(states, []bool{false, true}) {
		t.Fatal("state invalidations were not coalesced before measurement")
	}
	group.SetEnabled(false)
	if measureWidget(probe, layout.Unbounded()).Width != 20 {
		t.Fatal("ancestor disable retained enabled-state font geometry")
	}
	before := probe.notified
	probe.SetEnabled(false) // own setting changes, actual state does not
	group.SetEnabled(true)
	measureWidget(probe, layout.Unbounded())
	if probe.notified != before || IsEnabled(probe) {
		t.Fatal("unchanged actual state invalidated resources or lost local disable")
	}
}

func TestWidgetEnabledRestoresStationaryHoverInEveryHost(t *testing.T) {
	for _, test := range []struct {
		name string
		root Root
	}{
		{"Window", &window{}},
		{"Popover", &popover{}},
	} {
		t.Run(test.name, func(t *testing.T) {
			host := test.root.(interface {
				SetWidget(Widget)
				DispatchEvent(events.Event) error
			})
			group, button := newTestWidget(), NewButton()
			group.AddChild(button)
			host.SetWidget(group)
			t.Cleanup(func() { host.SetWidget(nil) })
			group.Arrange(geometry.Rect(0, 0, 100, 40))
			button.Arrange(geometry.Rect(0, 0, 100, 40))
			var hover []bool
			button.motion.ConnectContainsHover(func(value bool) { hover = append(hover, value) })
			if err := host.DispatchEvent(events.PointerEvent{EventType: events.PointerMove,
				Position: geometry.Point{X: 10, Y: 10}}); err != nil {
				t.Fatal(err)
			}
			group.SetEnabled(false)
			group.SetEnabled(true)
			if !button.hovered || !slices.Equal(hover, []bool{true, false, true}) {
				t.Fatalf("stationary hover did not recover: %v", hover)
			}
			_ = host.DispatchEvent(events.PointerEvent{EventType: events.PointerLeave})
			group.SetEnabled(false)
			group.SetEnabled(true)
			if button.hovered || !slices.Equal(hover, []bool{true, false, true, false}) {
				t.Fatal("reenable restored hover after the pointer left the surface")
			}
		})
	}
}

func TestWidgetEnabledDragReadIsCanceled(t *testing.T) {
	win := &window{rootBase: rootBase{app: &application{}}}
	root, child := newTestWidget(), newTestWidget()
	root.AddChild(child)
	win.SetWidget(root)
	t.Cleanup(func() { win.SetWidget(nil) })
	root.Arrange(geometry.Rect(0, 0, 100, 100))
	child.Arrange(geometry.Rect(0, 0, 100, 100))
	target := NewDropTarget(DragFormatText)
	child.AddEventController(target)
	drops, leaves := 0, 0
	target.ConnectDrop(func(*DropRequest) { drops++ })
	target.ConnectLeave(func() { leaves++ })
	offer := &testDragOffer{id: 23, formats: []dragdrop.Format{dragdrop.FormatText}}
	for _, kind := range []events.EventType{events.DragEnter, events.DragDrop} {
		_ = win.DispatchEvent(events.DragOfferEvent{EventType: kind, Offer: offer,
			Position: geometry.Point{X: 10, Y: 10}, Actions: dragdrop.Copy})
	}
	if offer.read != dragdrop.FormatText || !target.active {
		t.Fatal("drop did not start a pending read")
	}
	root.SetEnabled(false)
	if !target.Enabled() || target.active || leaves != 1 || !slices.Equal(offer.finished, []dragdrop.Action{0}) {
		t.Fatal("disable did not reject pending read and preserve controller settings")
	}
	root.SetEnabled(true)
	data := new(dragdrop.Data)
	data.SetText("late")
	_ = win.DispatchEvent(events.DragDataEvent{OfferID: offer.id, Format: offer.read, Data: data})
	if drops != 0 || !slices.Equal(offer.finished, []dragdrop.Action{0}) {
		t.Fatal("late data delivered after reenable, or finished twice")
	}
}

func TestWidgetEnabledCancelsManualDragSession(t *testing.T) {
	app := &application{}
	win := &window{rootBase: rootBase{app: app}}
	root, child := newTestWidget(), newTestWidget()
	root.AddChild(child)
	win.SetWidget(root)
	t.Cleanup(func() { win.SetWidget(nil) })
	native := &gestureDragNative{}
	// Manual handoff does not add the DragSource to the widget's controllers.
	app.dragSession = &guiDragSession{app: app, host: win, widget: child, native: native, source: NewDragSource()}
	root.SetEnabled(false)
	if native.cancels == 0 {
		t.Fatal("disable failed to cancel a manually handed-off native drag")
	}
	app.dragSession = nil
}

func TestWidgetEnabledStopsStaleHoverAndClickCallbacks(t *testing.T) {
	win, root, button := &window{}, newTestWidget(), NewButton()
	root.AddChild(button)
	win.SetWidget(root)
	t.Cleanup(func() { win.SetWidget(nil) })
	root.Arrange(geometry.Rect(0, 0, 100, 40))
	button.Arrange(geometry.Rect(0, 0, 100, 40))
	first, second := NewMotionEventController(), NewMotionEventController()
	button.AddEventController(first)
	button.AddEventController(second)
	first.ConnectHover(func(hovered bool) {
		if hovered {
			button.SetEnabled(false)
		}
	})
	enters := 0
	second.ConnectHover(func(hovered bool) {
		if hovered {
			enters++
		}
	})
	_ = win.DispatchEvent(events.PointerEvent{EventType: events.PointerMove, Position: geometry.Point{X: 10, Y: 10}})
	if IsEnabled(button) || button.hovered || enters != 0 {
		t.Fatal("hover crossing continued after disable inside an earlier controller")
	}
	button.RemoveEventController(first)
	button.SetEnabled(true)
	button.ConnectClicked(func() { button.SetEnabled(false); button.SetEnabled(true) })
	button.ConnectClicked(func() { t.Fatal("stale click signal continued after disable and reenable") })
	button.emitClicked()
}
