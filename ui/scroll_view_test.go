package ui

import (
	"image"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
)

func TestScrollViewMountsAndUpdatesChild(t *testing.T) {
	root := newRoot()

	widget := root.update(ScrollView(Label("hello")))
	sv, ok := widget.(*gui.ScrollView)
	if !ok {
		t.Fatalf("updated %T, want *gui.ScrollView", widget)
	}
	child, ok := sv.Child().(*gui.Label)
	if !ok || child.Text() != "hello" {
		t.Fatalf("unexpected scroll view child: %T %v", sv.Child(), child)
	}

	// Same view type at the same slot: widget reused, content updated.
	updated := root.update(ScrollView(Label("world")))
	if updated != sv {
		t.Fatal("scroll view at the same slot should be reused")
	}
	child = sv.Child().(*gui.Label)
	if child.Text() != "world" {
		t.Fatalf("child label was not updated: %q", child.Text())
	}
}

func TestScrollViewRemovesChild(t *testing.T) {
	root := newRoot()
	sv := root.update(ScrollView(Label("x"))).(*gui.ScrollView)
	if sv.Child() == nil {
		t.Fatal("child should be set")
	}

	root.update(ScrollView(nil))
	if sv.Child() != nil {
		t.Fatal("child should be removed when the slot is empty")
	}
}

func TestScrollViewFillsSingleHBoxViewportByDefault(t *testing.T) {
	root := newRoot()
	widget := root.update(HBox(ScrollView(Label("content")))).(*gui.LinearBox)

	widget.Measure(layout.Tight(geometry.Size{Width: 800, Height: 600}))
	widget.Arrange(geometry.Rect(0, 0, 800, 600))

	sv := widget.Children()[0].(*gui.ScrollView)
	if got := sv.Rect(); got != geometry.Rect(0, 0, 800, 600) {
		t.Fatalf("single ScrollView should fill an HBox viewport, got %v", got)
	}
}

func TestScrollViewSplitsTwoHBoxViewportsByDefault(t *testing.T) {
	root := newRoot()
	widget := root.update(HBox(
		ScrollView(Label("left")),
		ScrollView(Label("right")),
	)).(*gui.LinearBox)

	widget.Measure(layout.Tight(geometry.Size{Width: 800, Height: 600}))
	widget.Arrange(geometry.Rect(0, 0, 800, 600))

	children := widget.Children()
	if len(children) != 2 {
		t.Fatalf("expected two ScrollViews, got %d children", len(children))
	}
	if got := children[0].Rect(); got != geometry.Rect(0, 0, 400, 600) {
		t.Fatalf("left ScrollView should take half the viewport, got %v", got)
	}
	if got := children[1].Rect(); got != geometry.Rect(400, 0, 400, 600) {
		t.Fatalf("right ScrollView should take half the viewport, got %v", got)
	}
}

func TestScrollViewSplitsTwoVBoxViewportsByDefault(t *testing.T) {
	root := newRoot()
	widget := root.update(VBox(
		ScrollView(Image(image.NewRGBA(image.Rect(0, 0, 100, 20)))),
		ScrollView(Image(image.NewRGBA(image.Rect(0, 0, 100, 20)))),
	)).(*gui.LinearBox)

	widget.Measure(layout.Tight(geometry.Size{Width: 800, Height: 600}))
	widget.Arrange(geometry.Rect(0, 0, 800, 600))

	children := widget.Children()
	if len(children) != 2 {
		t.Fatalf("expected two ScrollViews, got %d children", len(children))
	}
	if children[0].Rect().Height != 300 || children[1].Rect().Height != 300 {
		t.Fatalf("VBox ScrollViews should split the height, got %v and %v", children[0].Rect(), children[1].Rect())
	}
	if children[0].Rect().Y != 0 || children[1].Rect().Y != 300 {
		t.Fatalf("VBox ScrollViews should be stacked, got %v and %v", children[0].Rect(), children[1].Rect())
	}
	if children[0].Rect().Width != 100 || children[1].Rect().Width != 100 {
		t.Fatalf("VBox ScrollViews should preserve their content widths, got %v and %v", children[0].Rect(), children[1].Rect())
	}
}
