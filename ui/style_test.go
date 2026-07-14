package ui

import (
	"image/color"
	"testing"

	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/style"
)

// TestViewStyleWiredForAllStyledViews checks every styled view forwards its
// local rules to the mounted widget through SetStyleRules.
func TestViewStyleWiredForAllStyledViews(t *testing.T) {
	rules := []style.Rule{style.Default().Radius(5)}
	cases := []struct {
		name string
		view View
	}{
		{"label", Label("x").Style(rules...)},
		{"button", Button("x").Style(rules...)},
		{"textinput", TextInput().Style(rules...)},
		{"box", VBox().Style(rules...)},
		{"image", Image(nil).Style(rules...)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := newRoot()
			widget := root.update(tc.view)
			if !style.SameRules(widget.StyleRules(), rules) {
				t.Fatalf("%s did not receive local style rules: %+v", tc.name, widget.StyleRules())
			}
		})
	}
}

// TestViewStyleUpdatesAndClears checks reconciling the same slot updates the
// rules in place and that omitting Style clears them.
func TestViewStyleUpdatesAndClears(t *testing.T) {
	root := newRoot()

	rules := []style.Rule{
		style.Default().Radius(8),
		style.Default().State(style.Hovered).BackgroundColor(color.RGBA{R: 200, A: 255}),
	}
	widget := root.update(Button("Save").Style(rules...))
	button, ok := widget.(*gui.Button)
	if !ok {
		t.Fatalf("updated %T, want *gui.Button", widget)
	}
	if !style.SameRules(button.StyleRules(), rules) {
		t.Fatalf("button did not receive local style rules: %+v", button.StyleRules())
	}

	newRules := []style.Rule{style.Default().Radius(4)}
	updated := root.update(Button("Save").Style(newRules...))
	if updated != button {
		t.Fatal("button at the same slot and type should be reused")
	}
	if !style.SameRules(button.StyleRules(), newRules) {
		t.Fatalf("button style rules were not updated: %+v", button.StyleRules())
	}

	cleared := root.update(Button("Save"))
	if cleared != button {
		t.Fatal("button should be reused when clearing style")
	}
	if len(button.StyleRules()) != 0 {
		t.Fatalf("expected no local rules after clearing, got %+v", button.StyleRules())
	}
}
