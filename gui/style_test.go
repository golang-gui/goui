package gui

import (
	"image/color"
	"testing"

	"github.com/golang-gui/goui/core/colors"
	"github.com/golang-gui/goui/style"
)

func TestDefaultStyleSheetResolvesBuiltInStyles(t *testing.T) {
	sheet := DefaultStyleSheet()

	button := sheet.Resolve(style.Sel{Name: styleNameButton, State: style.Hovered}, nil)
	background, ok := button.BackgroundColor()
	if !ok || !colors.Equal(background, color.RGBA{R: 230, G: 230, B: 230, A: 255}) {
		t.Fatalf("unexpected button hover background: %v ok=%v", background, ok)
	}
	radius, ok := button.Radius()
	if !ok || radius != 4 {
		t.Fatalf("unexpected button radius: %v ok=%v", radius, ok)
	}

	input := sheet.Resolve(style.Sel{Name: styleNameTextInput, State: style.Focused}, nil)
	borderColor, ok := input.BorderColor()
	if !ok || !colors.Equal(borderColor, defaultAccentColor) {
		t.Fatalf("unexpected focused text input border: %v ok=%v", borderColor, ok)
	}
	padding, ok := input.Padding()
	if !ok || padding != 4 {
		t.Fatalf("unexpected text input padding: %v ok=%v", padding, ok)
	}
}

func TestApplicationStyleSheetDefaultsToNilAndRequestsLayoutOnSet(t *testing.T) {
	app := &application{}
	if app.StyleSheet() != nil {
		t.Fatal("application style sheet should default to nil")
	}

	win := &window{}
	app.windows = []*window{win}
	win.layoutDirty = false
	win.paintDirty = false

	app.SetStyleSheet(style.Sheet(style.Name(styleNameButton).Padding(6)))
	if app.StyleSheet() == nil {
		t.Fatal("application style sheet was not stored")
	}
	if !win.layoutDirty || !win.paintDirty {
		t.Fatal("setting application style did not request layout")
	}

	app.SetStyleSheet(nil)
	if app.StyleSheet() != nil {
		t.Fatal("nil style sheet should clear application style")
	}
}
