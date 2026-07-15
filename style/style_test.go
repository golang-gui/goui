package style

import (
	"image/color"
	"testing"

	"github.com/golang-gui/goui/core/colors"
)

func TestResolveUsesSimpleStateFallback(t *testing.T) {
	normalBackground := color.RGBA{R: 210, G: 210, B: 210, A: 255}
	hoverBackground := color.RGBA{R: 230, G: 230, B: 230, A: 255}
	sheet := Sheet(
		Name("button").
			BackgroundColor(normalBackground).
			Radius(4),
		Name("button").
			State(Hovered).
			BackgroundColor(hoverBackground),
	)

	normal := sheet.Resolve(Sel{Name: "button", State: Normal})
	normalColor, ok := normal.BackgroundColor()
	if !ok || !colors.Equal(normalColor, normalBackground) {
		t.Fatalf("unexpected normal background: %v ok=%v", normalColor, ok)
	}

	hover := sheet.Resolve(Sel{Name: "button", State: Hovered})
	hoverColor, ok := hover.BackgroundColor()
	if !ok || !colors.Equal(hoverColor, hoverBackground) {
		t.Fatalf("unexpected hover background: %v ok=%v", hoverColor, ok)
	}
	radius, ok := hover.Radius()
	if !ok || radius != 4 {
		t.Fatalf("hover should keep normal radius, got %v ok=%v", radius, ok)
	}

	focused := sheet.Resolve(Sel{Name: "button", State: Focused})
	focusedColor, ok := focused.BackgroundColor()
	if !ok || !colors.Equal(focusedColor, normalColor) {
		t.Fatalf("focused should fall back to normal background: %v ok=%v", focusedColor, ok)
	}
}

func TestResolveUsesPartFallback(t *testing.T) {
	defaultRadius := float32(4)
	selectionColor := color.RGBA{R: 40, G: 90, B: 180, A: 255}
	hoverColor := color.RGBA{R: 50, G: 120, B: 220, A: 255}
	sheet := Sheet(
		Name("text-input").Radius(defaultRadius),
		Name("text-input").Part("selection").ForegroundColor(selectionColor),
		Name("text-input").Part("selection").State(Hovered).BackgroundColor(hoverColor),
	)

	resolved := sheet.Resolve(Sel{
		Name:  "text-input",
		Part:  "selection",
		State: Hovered,
	})

	background, ok := resolved.BackgroundColor()
	if !ok || !colors.Equal(background, hoverColor) {
		t.Fatalf("unexpected selection hover background: %v ok=%v", background, ok)
	}
	foreground, ok := resolved.ForegroundColor()
	if !ok || !colors.Equal(foreground, selectionColor) {
		t.Fatalf("selection hover should keep selection normal foreground: %v ok=%v", foreground, ok)
	}
	radius, ok := resolved.Radius()
	if !ok || radius != defaultRadius {
		t.Fatalf("selection should fall back to text input default radius: %v ok=%v", radius, ok)
	}
}

func TestResolveRequiresFullSelectorMatch(t *testing.T) {
	red := color.RGBA{R: 255, A: 255}
	blue := color.RGBA{B: 255, A: 255}
	hover := color.RGBA{G: 255, A: 255}
	sheet := Sheet(
		Name("button").State(Hovered).BackgroundColor(hover),
		Name("button").Part("icon").BackgroundColor(red),
		Name("label").State(Hovered).BackgroundColor(blue),
	)

	resolved := sheet.Resolve(Sel{Name: "button", State: Hovered})
	background, ok := resolved.BackgroundColor()
	if !ok {
		t.Fatal("button hover background was not resolved")
	}
	if colors.Equal(background, red) {
		t.Fatal("button hover default part should not match button icon rule")
	}
	if colors.Equal(background, blue) {
		t.Fatal("button hover should not match label hover rule")
	}
	if !colors.Equal(background, hover) {
		t.Fatalf("unexpected button hover background: %v", background)
	}
}

func TestResolvePreservesExplicitZeroValues(t *testing.T) {
	// A more-specific rule that sets an explicit zero/transparent value overrides
	// a non-zero base along the fallback chain (the optional-field model).
	sheet := Sheet(
		Name("button").BorderWidth(2).Radius(8),
		Name("button").State(Hovered).
			BorderWidth(0).
			Radius(0).
			BackgroundColor(color.Transparent),
	)

	resolved := sheet.Resolve(Sel{Name: "button", State: Hovered})

	borderWidth, ok := resolved.BorderWidth()
	if !ok || borderWidth != 0 {
		t.Fatalf("explicit zero border width was not preserved: ok=%v value=%v", ok, borderWidth)
	}
	radius, ok := resolved.Radius()
	if !ok || radius != 0 {
		t.Fatalf("explicit zero radius was not preserved: ok=%v value=%v", ok, radius)
	}
	background, ok := resolved.BackgroundColor()
	if !ok || !colors.Equal(background, color.Transparent) {
		t.Fatalf("explicit transparent background was not preserved: %v ok=%v", background, ok)
	}
}

func TestSameRulesComparesStyleValues(t *testing.T) {
	a := []Rule{
		Name("button").BackgroundColor(color.RGBA{R: 1, A: 255}).Radius(4),
	}
	b := []Rule{
		Name("button").BackgroundColor(color.RGBA{R: 1, A: 255}).Radius(4),
	}
	c := []Rule{
		Name("button").BackgroundColor(color.RGBA{R: 2, A: 255}).Radius(4),
	}

	if !SameRules(a, b) {
		t.Fatal("equivalent rules should compare equal")
	}
	if SameRules(a, c) {
		t.Fatal("different rules should not compare equal")
	}
}
