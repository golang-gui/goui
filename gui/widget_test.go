package gui

import (
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/events"
)

type testWidget struct {
	WidgetBase
}

func newTestWidget() *testWidget {
	w := new(testWidget)
	w.Init(w)
	return w
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

func TestWidgetBaseSetWindowPropagatesToChildren(t *testing.T) {
	win := &window{}
	parent := newTestWidget()
	child := newTestWidget()
	parent.AddChild(child)

	parent.setWindow(win)

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
	widget.setWindow(win)

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
	child := newTestWidget()
	child.SetID("child")
	parent.AddChild(child)

	parent.Arrange(geometry.Rect(1, 2, 30, 40))
	child.Arrange(geometry.Rect(3, 4, 10, 20))

	info := parent.Snapshot()
	if info.ID != "parent" || info.Bounds != geometry.Rect(1, 2, 30, 40) {
		t.Fatalf("unexpected parent snapshot: %+v", info)
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

	size := parent.Measure(geometry.Size{Width: 100, Height: 80})
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

type testControllerAdapter struct{}

func (c *testControllerAdapter) Phase() PropagationPhase {
	return PhaseTarget
}

func (c *testControllerAdapter) HandleEvent(ctx *EventContext, event events.Event) {}

type testLayoutManager struct {
	measureSize geometry.Size
	measured    []layout.Child
	arranged    []layout.Child
	arrangeRect geometry.Rectangle
}

func (l *testLayoutManager) Measure(children []layout.Child, available geometry.Size) geometry.Size {
	l.measured = append([]layout.Child(nil), children...)
	return l.measureSize
}

func (l *testLayoutManager) Arrange(children []layout.Child, rect geometry.Rectangle) {
	l.arranged = append([]layout.Child(nil), children...)
	l.arrangeRect = rect
}
