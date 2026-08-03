package ui

import (
	"testing"

	"github.com/golang-gui/goui/gui"
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
