package style

import (
	"image/color"
	"testing"
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

	normal := sheet.Resolve(Sel{Name: "button", State: Normal}, nil)
	normalColor, ok := normal.BackgroundColor()
	if !ok || !sameColor(normalColor, normalBackground) {
		t.Fatalf("unexpected normal background: %v ok=%v", normalColor, ok)
	}

	hover := sheet.Resolve(Sel{Name: "button", State: Hovered}, nil)
	hoverColor, ok := hover.BackgroundColor()
	if !ok || !sameColor(hoverColor, hoverBackground) {
		t.Fatalf("unexpected hover background: %v ok=%v", hoverColor, ok)
	}
	radius, ok := hover.Radius()
	if !ok || radius != 4 {
		t.Fatalf("hover should keep normal radius, got %v ok=%v", radius, ok)
	}

	focused := sheet.Resolve(Sel{Name: "button", State: Focused}, nil)
	focusedColor, ok := focused.BackgroundColor()
	if !ok || !sameColor(focusedColor, normalColor) {
		t.Fatalf("focused should fall back to normal background: %v ok=%v", focusedColor, ok)
	}
}

func TestResolveUsesPartFallback(t *testing.T) {
	defaultPadding := float32(4)
	selectionColor := color.RGBA{R: 40, G: 90, B: 180, A: 255}
	hoverColor := color.RGBA{R: 50, G: 120, B: 220, A: 255}
	sheet := Sheet(
		Name("text-input").Padding(defaultPadding),
		Name("text-input").Part("selection").ForegroundColor(selectionColor),
		Name("text-input").Part("selection").State(Hovered).BackgroundColor(hoverColor),
	)

	resolved := sheet.Resolve(Sel{
		Name:  "text-input",
		Part:  "selection",
		State: Hovered,
	}, nil)

	background, ok := resolved.BackgroundColor()
	if !ok || !sameColor(background, hoverColor) {
		t.Fatalf("unexpected selection hover background: %v ok=%v", background, ok)
	}
	foreground, ok := resolved.ForegroundColor()
	if !ok || !sameColor(foreground, selectionColor) {
		t.Fatalf("selection hover should keep selection normal foreground: %v ok=%v", foreground, ok)
	}
	padding, ok := resolved.Padding()
	if !ok || padding != defaultPadding {
		t.Fatalf("selection should fall back to text input default padding: %v ok=%v", padding, ok)
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

	resolved := sheet.Resolve(Sel{Name: "button", State: Hovered}, nil)
	background, ok := resolved.BackgroundColor()
	if !ok {
		t.Fatal("button hover background was not resolved")
	}
	if sameColor(background, red) {
		t.Fatal("button hover default part should not match button icon rule")
	}
	if sameColor(background, blue) {
		t.Fatal("button hover should not match label hover rule")
	}
	if !sameColor(background, hover) {
		t.Fatalf("unexpected button hover background: %v", background)
	}
}

func TestResolveAppliesLocalRulesOverGlobalFallback(t *testing.T) {
	globalFont := float32(15)
	localFont := float32(20)
	localBackground := color.RGBA{R: 10, G: 20, B: 30, A: 255}
	sheet := Sheet(
		Name("label").FontSize(globalFont),
		Name("label").State(Hovered).BackgroundColor(color.RGBA{R: 200, A: 255}),
	)

	resolved := sheet.Resolve(
		Sel{Name: "label", State: Hovered},
		[]Rule{
			Default().FontSize(localFont),
			Default().State(Hovered).BackgroundColor(localBackground),
		},
	)

	fontSize, ok := resolved.FontSize()
	if !ok || fontSize != localFont {
		t.Fatalf("local normal font should override global normal font: %v ok=%v", fontSize, ok)
	}
	background, ok := resolved.BackgroundColor()
	if !ok || !sameColor(background, localBackground) {
		t.Fatalf("local hover background should override global hover background: %v ok=%v", background, ok)
	}
}

func TestResolvePreservesExplicitZeroValues(t *testing.T) {
	sheet := Sheet(
		Name("button").BorderWidth(2).Padding(8),
	)

	resolved := sheet.Resolve(
		Sel{Name: "button"},
		[]Rule{
			Default().
				BorderWidth(0).
				Padding(0).
				BackgroundColor(color.Transparent),
		},
	)

	borderWidth, ok := resolved.BorderWidth()
	if !ok || borderWidth != 0 {
		t.Fatalf("explicit zero border width was not preserved: ok=%v value=%v", ok, borderWidth)
	}
	padding, ok := resolved.Padding()
	if !ok || padding != 0 {
		t.Fatalf("explicit zero padding was not preserved: ok=%v value=%v", ok, padding)
	}
	background, ok := resolved.BackgroundColor()
	if !ok || !sameColor(background, color.Transparent) {
		t.Fatalf("explicit transparent background was not preserved: %v ok=%v", background, ok)
	}
}

func TestSameRulesComparesStyleValues(t *testing.T) {
	a := []Rule{
		Name("button").BackgroundColor(color.RGBA{R: 1, A: 255}).Padding(4),
	}
	b := []Rule{
		Name("button").BackgroundColor(color.RGBA{R: 1, A: 255}).Padding(4),
	}
	c := []Rule{
		Name("button").BackgroundColor(color.RGBA{R: 2, A: 255}).Padding(4),
	}

	if !SameRules(a, b) {
		t.Fatal("equivalent rules should compare equal")
	}
	if SameRules(a, c) {
		t.Fatal("different rules should not compare equal")
	}
}
