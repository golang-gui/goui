package gui

import (
	"image/color"
	"testing"

	"github.com/golang-gui/goui/core/colors"
	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/style"
)

// useTestApplication installs app as the global application for the duration of
// the test, restoring the previous one on cleanup. Unlike setTestApplication it
// lets a test provide a style sheet (and typography) up front.
func useTestApplication(t *testing.T, app *application) {
	t.Helper()
	old := App
	App = app
	t.Cleanup(func() {
		App = old
	})
}

// TestApplicationStyleSheetChangesButtonPaint proves a custom application style
// sheet flows all the way into what a widget actually paints, not just what the
// resolver returns.
func TestApplicationStyleSheetChangesButtonPaint(t *testing.T) {
	background := color.RGBA{R: 12, G: 24, B: 36, A: 255}
	border := color.RGBA{R: 40, G: 50, B: 60, A: 255}
	useTestApplication(t, &application{
		style: style.Sheet(
			style.Name(styleNameButton).
				BackgroundColor(background).
				BorderColor(border).
				BorderWidth(2).
				Radius(6),
		),
	})

	button := NewButton()
	button.Arrange(geometry.Rect(0, 0, 80, 30))

	painter := new(testButtonBackgroundPainter)
	button.Paint(painter)

	if painter.rect != geometry.Rect(0, 0, 80, 30) || painter.radius != 6 {
		t.Fatalf("unexpected fill geometry: rect=%+v radius=%v", painter.rect, painter.radius)
	}
	if painter.brush != graphics.ColorOf(background) {
		t.Fatalf("unexpected fill brush: %+v", painter.brush)
	}
	if painter.drawRadius != 6 || painter.drawStrokeWidth != 2 || painter.drawBrush != graphics.ColorOf(border) {
		t.Fatalf("unexpected border: radius=%v width=%v brush=%+v", painter.drawRadius, painter.drawStrokeWidth, painter.drawBrush)
	}
}

// TestWindowApplicationStyleSheetOverridesGlobal covers the two-tier lookup in
// widgetStyleSheet: a widget mounted in a window whose application has its own
// sheet uses that sheet, not the global one.
// TestButtonHoverBackgroundFromApplicationStyleSheet checks that the widget's
// current state selects the matching rule from the application sheet, and falls
// back to normal when the state clears.
func TestButtonHoverBackgroundFromApplicationStyleSheet(t *testing.T) {
	normalBackground := color.RGBA{R: 20, G: 20, B: 20, A: 255}
	hoverBackground := color.RGBA{R: 60, G: 60, B: 60, A: 255}
	useTestApplication(t, &application{
		style: style.Sheet(
			style.Name(styleNameButton).BackgroundColor(normalBackground).Radius(3),
			style.Name(styleNameButton).State(style.Hovered).BackgroundColor(hoverBackground),
		),
	})

	button := NewButton()
	button.Arrange(geometry.Rect(0, 0, 80, 30))

	painter := new(testButtonBackgroundPainter)
	button.Paint(painter)
	if painter.brush != graphics.ColorOf(normalBackground) {
		t.Fatalf("unexpected normal background: %+v", painter.brush)
	}

	button.setHovered(true)
	painter = new(testButtonBackgroundPainter)
	button.Paint(painter)
	if painter.brush != graphics.ColorOf(hoverBackground) {
		t.Fatalf("unexpected hover background: %+v", painter.brush)
	}

	button.setHovered(false)
	painter = new(testButtonBackgroundPainter)
	button.Paint(painter)
	if painter.brush != graphics.ColorOf(normalBackground) {
		t.Fatalf("hover leave should fall back to normal background: %+v", painter.brush)
	}
}

// TestTextInputFocusedBorderFromApplicationStyleSheet drives focus through the
// real window focus path and checks the focused-state border resolves from the
// application sheet.
func TestTextInputFocusedBorderFromApplicationStyleSheet(t *testing.T) {
	normalBorder := color.RGBA{R: 100, G: 100, B: 100, A: 255}
	focusBorder := color.RGBA{R: 10, G: 200, B: 90, A: 255}
	useTestApplication(t, &application{
		style: style.Sheet(
			style.Name(styleNameTextInput).
				BackgroundColor(color.White).
				BorderColor(normalBorder).
				BorderWidth(1),
			style.Name(styleNameTextInput).
				State(style.Focused).
				BorderColor(focusBorder),
		),
	})

	win := &window{}
	input := NewTextInput()
	win.SetWidget(input)
	input.Arrange(geometry.Rect(0, 0, 100, 24))

	painter := new(testTextInputPainter)
	input.Paint(painter)
	if painter.drawRectBrush != graphics.ColorOf(normalBorder) {
		t.Fatalf("unfocused border should use the normal color: %+v", painter.drawRectBrush)
	}

	if !win.SetFocusedWidget(input) {
		t.Fatal("text input should accept focus")
	}
	painter = new(testTextInputPainter)
	input.Paint(painter)
	if painter.drawRectBrush != graphics.ColorOf(focusBorder) {
		t.Fatalf("focused border should use the focused color: %+v", painter.drawRectBrush)
	}
}

// TestStyleNameSelectsRuleSet exercises StyleName routing: the widget's style
// name is what selects which rules apply, and changing it changes resolution.
// TestResolveFallsBackToDefaultStyleName proves StyleName() is explicit (empty
// when unset) while resolveStyle falls back to the widget's type default name.
func TestResolveFallsBackToDefaultStyleName(t *testing.T) {
	buttonBg := color.RGBA{R: 7, G: 8, B: 9, A: 255}
	useTestApplication(t, &application{
		style: style.Sheet(style.Name(styleNameButton).BackgroundColor(buttonBg)),
	})

	button := NewButton()
	if button.StyleName() != "" {
		t.Fatalf("unset style override should be empty (explicit getter), got %q", button.StyleName())
	}

	bg, ok := ResolveStyle(styleNameButton, style.PartDefault, style.Normal).BackgroundColor()
	if !ok || !colors.Equal(bg, buttonBg) {
		t.Fatalf("button default name should resolve the button style: %v ok=%v", bg, ok)
	}
}

func TestStyleNameSelectsRuleSet(t *testing.T) {
	setTestApplication(t, nil)

	// ResolveStyle routes purely by name: the generic widget name resolves the
	// base (transparent) style, a different name selects a different ruleset.
	background, ok := ResolveStyle(styleNameWidget, style.PartDefault, style.Normal).BackgroundColor()
	if !ok || !colors.Equal(background, color.Transparent) {
		t.Fatalf("widget style name should resolve transparent background: %v ok=%v", background, ok)
	}

	resolved := ResolveStyle(styleNameButton, style.PartDefault, style.Normal)
	background, ok = resolved.BackgroundColor()
	if !ok || !colors.Equal(background, color.RGBA{R: 210, G: 210, B: 210, A: 255}) {
		t.Fatalf("button style name should resolve button background: %v ok=%v", background, ok)
	}
	if radius, ok := resolved.Radius(); !ok || radius != 4 {
		t.Fatalf("button style name should resolve button radius: %v ok=%v", radius, ok)
	}
}
