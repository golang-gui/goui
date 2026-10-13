package basic

import (
	"image/color"
	"runtime"
	"testing"

	"github.com/golang-gui/goui/core/colors"
	"github.com/golang-gui/goui/style"
)

func TestCompleteFallbackTextAndStates(t *testing.T) {
	sheet := Sheet()
	wantFamily := ""
	switch runtime.GOOS {
	case "linux":
		wantFamily = "sans-serif"
	case "windows":
		wantFamily = "Segoe UI"
	}
	for _, name := range []string{
		"widget", "label", "button", "text-input", "text-view", "menu-item-text",
		"check-button-text", "switch-text", "tree-item-text", "table-header-text", "table-cell-text",
		"tab-item-text", "tab-item-text-selected", "tab-close-button-text", "tab-scroll-button-text",
		"drop-down-text", "drop-down-item-text", "drop-down-placeholder",
	} {
		for _, state := range []style.State{style.Normal, style.Hovered, style.Pressed, style.Disabled} {
			sel := style.Sel{Name: name, State: state}
			s := sheet.Resolve(sel)
			if size, ok := s.FontSize(); !ok || size != 14 {
				t.Errorf("%+v font=%g set=%v; want 14 pt", sel, size, ok)
			}
			if family, ok := s.FontFamily(); !ok || family != wantFamily {
				t.Errorf("%+v family=%q set=%v; want %q", sel, family, ok, wantFamily)
			}
			if fg, ok := s.ForegroundColor(); !ok || fg == nil {
				t.Errorf("%+v has no text color", sel)
			}
		}
	}
	for _, name := range []string{"label", "icon", "check-button-text", "switch-text", "tree-item-text", "table-cell-text"} {
		fg, ok := sheet.Resolve(style.Sel{Name: name, State: style.Disabled}).ForegroundColor()
		if !ok || !colors.Equal(fg, color.RGBA{R: 140, G: 140, B: 140, A: 255}) {
			t.Errorf("%s disabled foreground=%v; want original neutral gray", name, fg)
		}
	}
}

func TestCompleteFallbackGeometryAndSurfaces(t *testing.T) {
	// Independent existing sRGB colors and DIP geometry; not a comparison of
	// two forwarding APIs that could share a migration mistake.
	sheet := Sheet()
	for _, tc := range []struct {
		name, part    string
		state         style.State
		background    color.Color
		width, radius float32
	}{
		{"button", "", style.Normal, color.RGBA{R: 210, G: 210, B: 210, A: 255}, 0, 4},
		{"button", "", style.Hovered, color.RGBA{R: 230, G: 230, B: 230, A: 255}, 0, 4},
		{"button", "focus", style.FocusVisible, color.Transparent, 2, 4},
		{"text-input", "", style.Disabled, color.RGBA{R: 245, G: 245, B: 245, A: 255}, 1, 0},
		{"check-button", "indicator-checked", style.Normal, color.RGBA{R: 51, G: 112, B: 204, A: 255}, 0, 4},
		{"check-button", "radio-checked", style.Normal, color.RGBA{R: 51, G: 112, B: 204, A: 255}, 0, 9},
		{"switch", "track", style.Normal, color.RGBA{R: 199, G: 199, B: 201, A: 255}, 1, 10},
		{"switch", "track-checked", style.Normal, color.RGBA{R: 70, G: 130, B: 220, A: 255}, 0, 10},
		{"switch", "thumb-checked", style.Normal, color.White, 0, 8},
		{"drop-down-popup", "", style.Normal, color.White, 1, 8},
		{"table-view", "", style.Normal, color.White, 1, 0},
	} {
		sel := style.Sel{Name: tc.name, Part: tc.part, State: tc.state}
		s := sheet.Resolve(sel)
		bg, ok := s.BackgroundColor()
		width, _ := s.BorderWidth()
		radius, _ := s.Radius()
		if !ok || !colors.Equal(bg, tc.background) || width != tc.width || radius != tc.radius {
			t.Errorf("%+v background=%v width=%g radius=%g; want %v/%g/%g", sel, bg, width, radius, tc.background, tc.width, tc.radius)
		}
	}
	for _, sel := range []style.Sel{
		{Name: "check-button", Part: "focus", State: style.FocusVisible},
		{Name: "switch", Part: "focus", State: style.FocusVisible},
		{Name: "drop-down", Part: "focus", State: style.FocusVisible},
	} {
		s := sheet.Resolve(sel)
		if width, ok := s.BorderWidth(); !ok || width != 2 {
			t.Errorf("%+v lost its 2 DIP keyboard focus", sel)
		}
	}
	for _, name := range []string{"primary", "primary-content", "unknown-style"} {
		if bg, ok := sheet.Resolve(style.Sel{Name: name}).BackgroundColor(); ok {
			t.Errorf("Basic unexpectedly resolves %q to %v", name, bg)
		}
	}
}

func TestSelectedSurfacesHaveSingleCoverage(t *testing.T) {
	sheet := Sheet()
	for _, state := range []style.State{style.Normal, style.Hovered, style.Pressed, style.Disabled, style.Disabled | style.Hovered | style.Pressed} {
		for _, sel := range []style.Sel{
			{Name: "check-button", Part: "indicator-checked", State: state},
			{Name: "check-button", Part: "indicator-mixed", State: state},
			{Name: "check-button", Part: "radio-checked", State: state},
			{Name: "switch", Part: "track-checked", State: state},
		} {
			if width, ok := sheet.Resolve(sel).BorderWidth(); !ok || width != 0 {
				t.Errorf("%+v border=%g set=%v; solid selected fill must not draw a second edge", sel, width, ok)
			}
		}
		check := sheet.Resolve(style.Sel{Name: "check-button", Part: "indicator-checked", State: state})
		radio := sheet.Resolve(style.Sel{Name: "check-button", Part: "radio-checked", State: state})
		checkFill, _ := check.BackgroundColor()
		radioFill, _ := radio.BackgroundColor()
		checkMark, _ := check.ForegroundColor()
		radioMark, _ := radio.ForegroundColor()
		if !colors.Equal(checkFill, radioFill) || !colors.Equal(checkMark, radioMark) {
			t.Errorf("state=%v checkbox and radio selected palette differs", state)
		}
		if state&style.Disabled == 0 && !colors.Equal(radioMark, color.White) {
			t.Errorf("state=%v radio dot=%v; want white", state, radioMark)
		}
		for _, sel := range []style.Sel{{Name: "check-button", Part: "indicator", State: state}, {Name: "check-button", Part: "radio", State: state}, {Name: "switch", Part: "track", State: state}} {
			if width, _ := sheet.Resolve(sel).BorderWidth(); width != 1 {
				t.Errorf("%+v lost its unchecked 1 DIP outline", sel)
			}
		}
	}
}

func TestRulesOwnershipAndOverrides(t *testing.T) {
	rules, fresh := Rules(), Rules()
	sheet := style.Sheet(rules...)
	rules[0] = style.Name("window").BackgroundColor(color.Black)
	if !style.SameRules(fresh, Rules()) {
		t.Fatal("returned rule slices share mutable storage")
	}
	if bg, _ := sheet.Resolve(style.Sel{Name: "window"}).BackgroundColor(); !colors.Equal(bg, color.White) {
		t.Fatal("caller changed an existing stylesheet by mutating its rules")
	}
	custom := style.Sheet(append(fresh, style.Name("switch").Part("track-checked").BackgroundColor(color.Transparent))...)
	if bg, ok := custom.Resolve(style.Sel{Name: "switch", Part: "track-checked"}).BackgroundColor(); !ok || !colors.Equal(bg, color.Transparent) {
		t.Fatal("explicit transparent override was lost")
	}
	if bg, _ := Sheet().Resolve(style.Sel{Name: "window"}).BackgroundColor(); !colors.Equal(bg, color.White) {
		t.Fatal("fresh Basic sheet retained an application override")
	}
}
