package ui

import (
	"image"
	"testing"

	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
)

func TestRootCreatesAndUpdatesLabel(t *testing.T) {
	root := NewRoot()

	widget := root.Update(Label("hello").Name("title"))
	label, ok := widget.(*gui.Label)
	if !ok {
		t.Fatalf("updated %T, want *gui.Label", widget)
	}
	if label.ID() != "title" || label.Text() != "hello" {
		t.Fatalf("unexpected label state: id=%q text=%q", label.ID(), label.Text())
	}

	updated := root.Update(Label("world").Name("title2"))
	if updated != label {
		t.Fatal("label at the same root slot and type should be reused")
	}
	if label.ID() != "title2" || label.Text() != "world" {
		t.Fatalf("label was not updated: id=%q text=%q", label.ID(), label.Text())
	}
}

func TestRootReplacesDifferentViewType(t *testing.T) {
	root := NewRoot()

	label := root.Update(Label("name").Name("field"))
	input := root.Update(TextInput().Name("field").Text("name"))

	if input == label {
		t.Fatal("different view types should not reuse the same widget")
	}
	textInput, ok := input.(*gui.TextInput)
	if !ok {
		t.Fatalf("updated %T, want *gui.TextInput", input)
	}
	if textInput.Text() != "name" {
		t.Fatalf("unexpected text input text: %q", textInput.Text())
	}
}

func TestRootUpdatesBoxChildrenByPositionAndType(t *testing.T) {
	root := NewRoot()

	widget := root.Update(VBox().
		Name("root").
		Spacing(6).
		Children(
			Label("one").Name("first"),
			Label("two").Name("second"),
		))
	box := widget.(*gui.LinearBox)
	if box.ID() != "root" || box.Direction() != layout.DirectionVertical || box.Spacing() != 6 {
		t.Fatalf("unexpected box state: id=%q direction=%v spacing=%v", box.ID(), box.Direction(), box.Spacing())
	}
	children := box.Children()
	if len(children) != 2 {
		t.Fatalf("expected two children, got %d", len(children))
	}
	first := children[0].(*gui.Label)
	second := children[1].(*gui.Label)

	root.Update(VBox().
		Spacing(8).
		Children(
			Label("ONE").Name("first-updated"),
			Label("TWO").Name("second-updated"),
			Label("THREE").Name("third"),
		))

	children = box.Children()
	if len(children) != 3 {
		t.Fatalf("expected three children, got %d", len(children))
	}
	if children[0] != first || children[1] != second {
		t.Fatal("children at the same slot and type should be reused")
	}
	if first.Text() != "ONE" || second.Text() != "TWO" {
		t.Fatalf("children were not updated: %q %q", first.Text(), second.Text())
	}
	if children[2].(*gui.Label).Text() != "THREE" {
		t.Fatalf("tail child was not appended: %q", children[2].(*gui.Label).Text())
	}
	if box.Spacing() != 8 {
		t.Fatalf("box spacing was not updated: %v", box.Spacing())
	}
}

func TestRootRebuildsTailOnChildTypeMismatch(t *testing.T) {
	root := NewRoot()

	box := root.Update(VBox(
		Label("first"),
		TextInput().Text("second"),
	)).(*gui.LinearBox)
	oldChildren := box.Children()

	root.Update(VBox(
		Label("FIRST"),
		Label("inserted"),
		TextInput().Text("second"),
	))

	children := box.Children()
	if len(children) != 3 {
		t.Fatalf("expected three children, got %d", len(children))
	}
	if children[0] != oldChildren[0] {
		t.Fatal("same type prefix child should be reused")
	}
	if children[1] == oldChildren[1] {
		t.Fatal("tail should be rebuilt after the first child type mismatch")
	}
	if children[0].(*gui.Label).Text() != "FIRST" || children[1].(*gui.Label).Text() != "inserted" {
		t.Fatalf("unexpected rebuilt children: %v", children)
	}
}

func TestRootUpdatesButtonChild(t *testing.T) {
	root := NewRoot()

	button := root.Update(Button(Label("OK"))).(*gui.Button)
	children := button.Children()
	if len(children) != 1 || children[0].(*gui.Label).Text() != "OK" {
		t.Fatalf("unexpected button child: %v", children)
	}
	child := children[0]

	root.Update(Button(Label("Cancel")))
	children = button.Children()
	if len(children) != 1 || children[0] != child || children[0].(*gui.Label).Text() != "Cancel" {
		t.Fatalf("button child was not updated in place: %v", children)
	}

	root.Update(Button(nil))
	if children := button.Children(); len(children) != 0 {
		t.Fatalf("button child was not removed: %v", children)
	}
}

func TestRootReconnectsTextInputHandler(t *testing.T) {
	root := NewRoot()
	firstCalls := 0
	secondCalls := 0

	widget := root.Update(TextInput().
		Text("a").
		OnText(func(string) {
			firstCalls++
		}))
	input := widget.(*gui.TextInput)

	input.SetText("b")
	if firstCalls != 1 || secondCalls != 0 {
		t.Fatalf("unexpected first handler calls: first=%d second=%d", firstCalls, secondCalls)
	}

	root.Update(TextInput().
		Text("c").
		OnText(func(string) {
			secondCalls++
		}))
	if firstCalls != 1 || secondCalls != 0 {
		t.Fatalf("declarative text update should not emit handlers: first=%d second=%d", firstCalls, secondCalls)
	}

	input.SetText("d")
	if firstCalls != 1 || secondCalls != 1 {
		t.Fatalf("handler was not replaced: first=%d second=%d", firstCalls, secondCalls)
	}
}

func TestRootClearsConnectionWhenHandlerRemoved(t *testing.T) {
	root := NewRoot()
	calls := 0

	input := root.Update(TextInput().
		OnText(func(string) {
			calls++
		})).(*gui.TextInput)
	input.SetText("a")
	if calls != 1 {
		t.Fatalf("expected first handler call, got %d", calls)
	}

	root.Update(TextInput())
	input.SetText("b")
	if calls != 1 {
		t.Fatalf("removed handler should not be called, got %d", calls)
	}
}

func TestRootUpdatesImageAndCommonFields(t *testing.T) {
	root := NewRoot()
	first := image.NewRGBA(image.Rect(0, 0, 10, 10))
	second := image.NewRGBA(image.Rect(0, 0, 20, 20))

	widget := root.Update(Image(first).Name("logo"))
	imageWidget := widget.(*gui.Image)
	if imageWidget.ID() != "logo" || imageWidget.Image() != first || !imageWidget.Visible() {
		t.Fatalf("unexpected image state: id=%q visible=%v", imageWidget.ID(), imageWidget.Visible())
	}

	updated := root.Update(Image(second).Name("logo2").Hidden(true))
	if updated != imageWidget {
		t.Fatal("image at the same root slot and type should be reused")
	}
	if imageWidget.ID() != "logo2" || imageWidget.Image() != second || imageWidget.Visible() {
		t.Fatalf("image fields were not updated: id=%q visible=%v", imageWidget.ID(), imageWidget.Visible())
	}
}

func TestRootUnmountClearsWidget(t *testing.T) {
	root := NewRoot()
	root.Update(VBox(Label("child")))
	if root.Widget() == nil {
		t.Fatal("expected rendered widget")
	}

	root.Unmount()
	if root.Widget() != nil {
		t.Fatal("unmount should clear root widget")
	}
}
