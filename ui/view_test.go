package ui

import "testing"

// Every widget-view constructor must wire ViewBase.Self so the shared chain
// modifiers (Name/Hidden/Style) return the concrete view instead of panicking on
// a nil Self. This is the guardrail for the one unsafe edge of the self-type.
func TestViewConstructorsWireSelf(t *testing.T) {
	cases := []struct {
		name string
		make func() View // ends in a shared modifier, which panics if Self is nil
	}{
		{"Button", func() View { return Button("x").Name("btn") }},
		{"Label", func() View { return Label("x").Name("lbl") }},
		{"HBox", func() View { return HBox().Name("hbox") }},
		{"VBox", func() View { return VBox().Name("vbox") }},
		{"TextInput", func() View { return TextInput().Name("input") }},
		{"Image", func() View { return Image(nil).Name("img") }},
	}
	for _, c := range cases {
		view := c.make() // would panic in self() if the constructor forgot Self
		if view == nil {
			t.Fatalf("%s: chained modifier returned nil (Self not wired)", c.name)
		}
		if view.base().name == "" {
			t.Fatalf("%s: shared Name modifier did not apply", c.name)
		}
	}
}
