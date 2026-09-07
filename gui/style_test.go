package gui

import (
	"image/color"
	"testing"

	"github.com/golang-gui/goui/core/colors"
	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/style"
)

func TestDefaultStyleSheetResolvesBuiltInStyles(t *testing.T) {
	sheet := DefaultStyleSheet()

	button := sheet.Resolve(style.Sel{Name: styleNameButton, State: style.Hovered})
	background, ok := button.BackgroundColor()
	if !ok || !colors.Equal(background, color.RGBA{R: 230, G: 230, B: 230, A: 255}) {
		t.Fatalf("unexpected button hover background: %v ok=%v", background, ok)
	}
	radius, ok := button.Radius()
	if !ok || radius != 4 {
		t.Fatalf("unexpected button radius: %v ok=%v", radius, ok)
	}

	input := sheet.Resolve(style.Sel{Name: styleNameTextInput, State: style.Focused})
	borderColor, ok := input.BorderColor()
	if !ok || !colors.Equal(borderColor, defaultAccentColor) {
		t.Fatalf("unexpected focused text input border: %v ok=%v", borderColor, ok)
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

	app.SetStyleSheet(style.Sheet(style.Name(styleNameButton).Radius(6)))
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

func TestApplicationStyleSheetInvalidatesWidgetMeasurementSubtree(t *testing.T) {
	app := &application{}
	win := &window{}
	box := NewLinearBox(layout.DirectionHorizontal)
	child := &countingMeasureWidget{size: geometry.Size{Width: 20, Height: 10}}
	box.AddChild(child)
	win.SetWidget(box)
	app.windows = []*window{win}
	c := layout.Loose(geometry.Size{Width: 100, Height: 40})

	measureWidget(box, c)
	app.SetStyleSheet(style.Sheet(style.Name(styleNameWidget).FontSize(18)))
	measureWidget(box, c)
	if child.measures != 2 {
		t.Fatalf("style change left a descendant measurement cached: measures=%d", child.measures)
	}
}
