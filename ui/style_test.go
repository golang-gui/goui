package ui

import (
	"testing"

	"github.com/golang-gui/goui/gui"
)

// TestViewStyleWiredForAllStyledViews checks every view forwards its semantic
// style name to the mounted widget through SetStyleName.
func TestViewStyleWiredForAllStyledViews(t *testing.T) {
	cases := []struct {
		name string
		view View
	}{
		{"label", Label("x").Style("fancy")},
		{"button", Button("x").Style("fancy")},
		{"textinput", TextInput().Style("fancy")},
		{"box", VBox().Style("fancy")},
		{"image", Image(nil).Style("fancy")},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := newRoot()
			widget := root.update(tc.view)
			if widget.StyleName() != "fancy" {
				t.Fatalf("%s did not receive style name: %q", tc.name, widget.StyleName())
			}
		})
	}
}

// TestViewStyleUpdatesAndClears checks reconciling the same slot updates the
// style name in place, and that dropping .Style reverts to the widget's type
// default (not the previous name) — the default+override contract.
func TestViewStyleUpdatesAndClears(t *testing.T) {
	root := newRoot()

	widget := root.update(Button("Save").Style("primary"))
	button, ok := widget.(*gui.Button)
	if !ok {
		t.Fatalf("updated %T, want *gui.Button", widget)
	}
	if button.StyleName() != "primary" {
		t.Fatalf("button did not receive style name: %q", button.StyleName())
	}

	updated := root.update(Button("Save").Style("danger"))
	if updated != button {
		t.Fatal("button at the same slot and type should be reused")
	}
	if button.StyleName() != "danger" {
		t.Fatalf("button style name was not updated: %q", button.StyleName())
	}

	cleared := root.update(Button("Save"))
	if cleared != button {
		t.Fatal("button should be reused when clearing style")
	}
	// StyleName is the explicit override: clearing .Style resets it to empty. The
	// effective "button" fallback is resolveStyle's job (covered in gui tests).
	if button.StyleName() != "" {
		t.Fatalf("clearing .Style should reset the override to empty, got %q", button.StyleName())
	}
}
