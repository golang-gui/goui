package main

import "github.com/golang-gui/goui/style"

// The demo owns these complete table styles. Colors and widget states come
// from the copied Modern rules; only the independent border widths change.
func tableLookRules(look int) []style.Rule {
	if look == tableLookDefault {
		return nil
	}
	var grid, cells, outline float32
	switch look {
	case tableLookGrid:
		grid, cells = 1, 1
	case tableLookCells:
		cells = 1
	case tableLookOutline:
		outline = 1
	case tableLookGridOutline:
		grid, outline = 1, 1
	}
	rules := []style.Rule{style.Name("demo-records").BorderWidth(0)}
	for _, edge := range []struct {
		part  string
		width float32
	}{
		{"outer-horizontal", 0}, {"outer-vertical", 0},
		{"grid-horizontal", grid}, {"grid-vertical", grid}, {"header-separator", grid},
		{"grid-top", cells}, {"grid-bottom", cells}, {"grid-left", cells}, {"grid-right", cells},
		{"outline-horizontal", outline}, {"outline-vertical", outline},
	} {
		rules = append(rules, style.Name("demo-records").Part(edge.part).BorderWidth(edge.width))
	}
	return rules
}
