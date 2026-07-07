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
func TestWindowApplicationStyleSheetOverridesGlobal(t *testing.T) {
	globalBackground := color.RGBA{R: 1, G: 2, B: 3, A: 255}
	windowBackground := color.RGBA{R: 9, G: 8, B: 7, A: 255}
	useTestApplication(t, &application{
		style: style.Sheet(style.Name(styleNameButton).BackgroundColor(globalBackground).Radius(2)),
	})

	winApp := &application{
		style: style.Sheet(style.Name(styleNameButton).BackgroundColor(windowBackground).Radius(2)),
	}
	win := &window{app: winApp}
	button := NewButton()
	win.SetWidget(button)
	button.Arrange(geometry.Rect(0, 0, 80, 30))

	painter := new(testButtonBackgroundPainter)
	button.Paint(painter)

	if painter.brush != graphics.ColorOf(windowBackground) {
		t.Fatalf("window application sheet should win over global: %+v", painter.brush)
	}
}

// TestLocalRulesOverrideApplicationStyleSheet checks that local rules override
// only the fields they set, while unset fields keep resolving against the
// application sheet (base = global, override = local).
func TestLocalRulesOverrideApplicationStyleSheet(t *testing.T) {
	appBackground := color.RGBA{R: 100, A: 255}
	localBackground := color.RGBA{B: 100, A: 255}
	useTestApplication(t, &application{
		style: style.Sheet(style.Name(styleNameButton).BackgroundColor(appBackground).Radius(9)),
	})

	button := NewButton()
	button.SetStyleRules(style.Default().BackgroundColor(localBackground))
	button.Arrange(geometry.Rect(0, 0, 80, 30))

	painter := new(testButtonBackgroundPainter)
	button.Paint(painter)

	if painter.brush != graphics.ColorOf(localBackground) {
		t.Fatalf("local background should override application background: %+v", painter.brush)
	}
	if painter.radius != 9 {
		t.Fatalf("unset local field should keep application radius, got %v", painter.radius)
	}
}

// TestLocalExplicitZeroOverridesApplicationStyleSheet proves the optional-field
// model works end to end: an explicit zero/transparent local value overrides a
// non-zero base instead of being treated as "unset".
func TestLocalExplicitZeroOverridesApplicationStyleSheet(t *testing.T) {
	useTestApplication(t, &application{
		style: style.Sheet(
			style.Name(styleNameButton).
				BackgroundColor(color.RGBA{R: 200, A: 255}).
				BorderColor(color.RGBA{B: 200, A: 255}).
				BorderWidth(3).
				Radius(0),
		),
	})

	button := NewButton()
	button.SetStyleRules(
		style.Default().
			BackgroundColor(color.Transparent).
			BorderWidth(0),
	)
	button.Arrange(geometry.Rect(0, 0, 80, 30))

	painter := new(testButtonBackgroundPainter)
	button.Paint(painter)

	if painter.brush != graphics.ColorOf(color.Transparent) {
		t.Fatalf("explicit transparent local background should override opaque base: %+v", painter.brush)
	}
	if painter.drawStrokeWidth != 0 || painter.drawRect != (geometry.Rectangle{}) {
		t.Fatalf("explicit zero border width should suppress the border draw: width=%v rect=%+v", painter.drawStrokeWidth, painter.drawRect)
	}
}

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

// TestLabelLocalRuleDrivesTextFormat covers the local-rule path for text format
// (the existing label test only exercises the global sheet).
func TestLabelLocalRuleDrivesTextFormat(t *testing.T) {
	typo := &testTypography{measureSize: geometry.Size{Width: 10, Height: 10}}
	setTestApplication(t, typo)

	foreground := color.RGBA{R: 12, G: 34, B: 56, A: 255}
	label := NewLabel("hi")
	label.SetStyleRules(
		style.Default().
			FontFamily("LocalMono").
			FontSize(22).
			ForegroundColor(foreground),
	)

	_ = label.Measure(geometry.Size{Width: 100, Height: 30})

	if len(typo.calls) != 1 {
		t.Fatalf("expected one text layout call, got %d", len(typo.calls))
	}
	format := typo.calls[0].format
	if format.Font.Family != "LocalMono" || format.Font.Size != 22 {
		t.Fatalf("unexpected local styled font: %+v", format.Font)
	}
	if !colors.Equal(format.TextColor, foreground) {
		t.Fatalf("unexpected local styled text color: %v", format.TextColor)
	}
}

// TestStyleNameSelectsRuleSet exercises StyleName routing: the widget's style
// name is what selects which rules apply, and changing it changes resolution.
func TestStyleNameSelectsRuleSet(t *testing.T) {
	setTestApplication(t, nil)

	widget := newTestWidget()

	background, ok := resolveStyle(widget, style.PartDefault, style.Normal).BackgroundColor()
	if !ok || !colors.Equal(background, color.Transparent) {
		t.Fatalf("default widget style name should resolve transparent background: %v ok=%v", background, ok)
	}

	widget.SetStyleName(styleNameButton)
	resolved := resolveStyle(widget, style.PartDefault, style.Normal)
	background, ok = resolved.BackgroundColor()
	if !ok || !colors.Equal(background, color.RGBA{R: 210, G: 210, B: 210, A: 255}) {
		t.Fatalf("button style name should resolve button background: %v ok=%v", background, ok)
	}
	if radius, ok := resolved.Radius(); !ok || radius != 4 {
		t.Fatalf("button style name should resolve button radius: %v ok=%v", radius, ok)
	}
}

// TestClearingLocalRulesRestoresApplicationStyle checks that removing local
// rules re-resolves against the application sheet.
func TestClearingLocalRulesRestoresApplicationStyle(t *testing.T) {
	appBackground := color.RGBA{R: 33, G: 66, B: 99, A: 255}
	localBackground := color.RGBA{R: 1, G: 1, B: 1, A: 255}
	useTestApplication(t, &application{
		style: style.Sheet(style.Name(styleNameButton).BackgroundColor(appBackground).Radius(5)),
	})

	button := NewButton()
	button.Arrange(geometry.Rect(0, 0, 80, 30))

	button.SetStyleRules(style.Default().BackgroundColor(localBackground))
	painter := new(testButtonBackgroundPainter)
	button.Paint(painter)
	if painter.brush != graphics.ColorOf(localBackground) {
		t.Fatalf("local rule should apply before clearing: %+v", painter.brush)
	}

	button.SetStyleRules()
	painter = new(testButtonBackgroundPainter)
	button.Paint(painter)
	if painter.brush != graphics.ColorOf(appBackground) {
		t.Fatalf("clearing local rules should restore application style: %+v", painter.brush)
	}
}
