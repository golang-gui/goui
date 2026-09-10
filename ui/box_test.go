package ui

import (
	"math"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
)

func TestSingleChildBoxAppliesAlignmentAndChild(t *testing.T) {
	root := newRoot()
	box := root.update(HBox(Label("value")).
		MainAlign(layout.MainEnd).
		CrossAlign(layout.CrossStretch)).(*gui.LinearBox)

	if box.MainAlign() != layout.MainEnd || box.CrossAlign() != layout.CrossStretch {
		t.Fatalf("unexpected alignment: main=%v cross=%v", box.MainAlign(), box.CrossAlign())
	}
	children := box.Children()
	if len(children) != 1 || children[0].(*gui.Label).Text() != "value" {
		t.Fatalf("unexpected box child: %v", children)
	}
}

func TestSingleChildBoxUpdatesAndClearsChild(t *testing.T) {
	root := newRoot()
	box := root.update(HBox(Label("first"))).(*gui.LinearBox)
	child := box.Children()[0]

	root.update(HBox(Label("second")).
		MainAlign(layout.MainStart).
		CrossAlign(layout.CrossCenter))
	children := box.Children()
	if len(children) != 1 || children[0] != child || child.(*gui.Label).Text() != "second" {
		t.Fatal("single-child box content was not updated in place")
	}
	if box.MainAlign() != layout.MainStart || box.CrossAlign() != layout.CrossCenter {
		t.Fatal("single-child box alignment was not updated")
	}

	root.update(HBox())
	if len(box.Children()) != 0 {
		t.Fatal("empty single-child box did not clear its child")
	}
}

func TestBoxCrossDefaultSurvivesDirectionChangesAndModifierRemoval(t *testing.T) {
	root := newRoot()
	content := VBox().MinSize(20, 10)
	cases := []struct {
		view  *BoxView
		align layout.CrossAlign
		want  geometry.Rectangle
	}{
		{HBox(content), layout.CrossDefault, geometry.Rect(0, 15, 20, 10)},
		{VBox(content), layout.CrossDefault, geometry.Rect(0, 0, 20, 10)},
		{HBox(content).CrossAlign(layout.CrossStart), layout.CrossStart, geometry.Rect(0, 0, 20, 10)},
		{VBox(content).CrossAlign(layout.CrossStart), layout.CrossStart, geometry.Rect(0, 0, 20, 10)},
		{HBox(content), layout.CrossDefault, geometry.Rect(0, 15, 20, 10)},
		{VBox(content).CrossAlign(layout.CrossCenter), layout.CrossCenter, geometry.Rect(40, 0, 20, 10)},
		{VBox(content), layout.CrossDefault, geometry.Rect(0, 0, 20, 10)},
	}
	var previous *gui.LinearBox
	for _, tc := range cases {
		box := root.update(tc.view).(*gui.LinearBox)
		if previous != nil && box != previous {
			t.Fatal("alignment update replaced the box instead of updating it")
		}
		box.Arrange(geometry.Rect(0, 0, 100, 40))
		if box.CrossAlign() != tc.align || box.Children()[0].Rect() != tc.want {
			t.Fatalf("direction=%v: config=%v rect=%+v; want config=%v rect=%+v", box.Direction(), box.CrossAlign(), box.Children()[0].Rect(), tc.align, tc.want)
		}
		previous = box
	}
}

func TestBoxLayoutModifiersUseWidgetNormalization(t *testing.T) {
	root := newRoot()
	box := root.update(HBox().
		Spacing(-2).
		Padding(float32(math.NaN())).
		MainWeight(float32(math.Inf(1))).
		MinSize(float32(math.NaN()), 20).
		MaxSize(float32(math.Inf(1)), 40)).(*gui.LinearBox)
	if box.Spacing() != 0 || box.Padding() != 0 || box.MainWeight() != 0 {
		t.Fatal("declarative modifiers bypassed widget normalization")
	}
	if box.MinSize() != (geometry.Size{Height: 20}) || box.MaxSize() != (geometry.Size{Height: 40}) {
		t.Fatalf("unexpected normalized size preferences: min=%+v max=%+v", box.MinSize(), box.MaxSize())
	}
	if got := box.Measure(layout.Unbounded()); got.Size != (geometry.Size{Height: 20}) {
		t.Fatalf("invalid modifiers polluted measurement: %+v", got)
	}
}
