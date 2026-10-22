package gui

import (
	"image"
	"image/color"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/platform/typography"
)

// Shared typography and painter doubles.

// setTestApplication 临时替换包级 App，并在测试清理时恢复。
// 调用者不能与其他修改 App 的 fixture 并行执行。
func setTestApplication(t *testing.T, typo typography.Context) {
	t.Helper()
	oldApp := App
	App = &application{typo: typo}
	t.Cleanup(func() {
		App = oldApp
	})
}

type textLayoutCall struct {
	text   string
	format typography.TextFormat
	width  float32
	height float32
}

type testTypography struct {
	measureSize geometry.Size
	lines       []typography.TextLine
	clusters    []typography.TextCluster
	calls       []textLayoutCall
	layouts     []*testTextLayout
}

func (c *testTypography) Name() string {
	return "test"
}

func (c *testTypography) Destroy() {}

func (c *testTypography) AddFont(fontFile string) error {
	return nil
}

func (c *testTypography) NewTextLayout(text string, format typography.TextFormat, width, height float32) (typography.TextLayout, error) {
	c.calls = append(c.calls, textLayoutCall{
		text:   text,
		format: format,
		width:  width,
		height: height,
	})
	layout := &testTextLayout{
		text:        text,
		format:      format,
		width:       width,
		height:      height,
		measureSize: c.measureSize,
		lines:       c.lines,
		clusters:    c.clusters,
	}
	c.layouts = append(c.layouts, layout)
	return layout, nil
}

type testTextLayout struct {
	text         string
	format       typography.TextFormat
	width        float32
	height       float32
	measureSize  geometry.Size
	lines        []typography.TextLine
	clusters     []typography.TextCluster
	destroyed    bool
	resized      func()
	metricsCalls int
	changedSig   signal.Signal0
	destroySig   signal.Signal0
}

func (l *testTextLayout) Destroy() {
	if l.destroyed {
		return
	}
	l.destroyed = true
	l.destroySig.Emit()
}

func (l *testTextLayout) Rasterize(float32, image.Rectangle, []byte) (typography.TextBitmap, error) {
	return typography.TextBitmap{}, nil
}

func (l *testTextLayout) ConnectChanged(fn func()) signal.Handle {
	return l.changedSig.Connect(fn)
}

func (l *testTextLayout) ConnectDestroy(fn func()) signal.Handle {
	return l.destroySig.Connect(fn)
}

func (l *testTextLayout) Text() string {
	return l.text
}

func (l *testTextLayout) Format() typography.TextFormat {
	return l.format
}

func (l *testTextLayout) Size() (maxWidth, maxHeight float32) {
	return l.width, l.height
}

func (l *testTextLayout) SetSize(maxWidth, maxHeight float32) {
	if l.width == maxWidth && l.height == maxHeight {
		return
	}
	l.width = maxWidth
	l.height = maxHeight
	if l.resized != nil {
		l.resized()
	}
	l.changedSig.Emit()
}

func (l *testTextLayout) SetTextAlignment(align typography.TextAlignment) {
	l.format.TextAlign = align
	l.changedSig.Emit()
}

func (l *testTextLayout) SetWrapMode(wrap typography.WrapMode) {
	l.format.WrapMode = wrap
	l.changedSig.Emit()
}

func (l *testTextLayout) SetTabWidth(width float32) { l.format.TabWidth = width }

func (l *testTextLayout) SetTextFont(start, length int, font typography.FontInfo) {
	l.format.Font = font
	l.changedSig.Emit()
}

func (l *testTextLayout) SetTextColor(start, length int, c color.Color) {
	l.format.TextColor = c
	l.changedSig.Emit()
}

func (l *testTextLayout) SetUnderline(start, length int, underline bool) {
	l.changedSig.Emit()
}

func (l *testTextLayout) SetStrikethrough(start, length int, strike bool) {
	l.changedSig.Emit()
}

func (l *testTextLayout) SetPlaceholder(int, *typography.PlaceholderMetrics) {}

func (l *testTextLayout) MeasureSize() (width, height float32) {
	return l.measureSize.Width, l.measureSize.Height
}

func (l *testTextLayout) MeasureMetrics() (lines []typography.TextLine, clusters []typography.TextCluster) {
	l.metricsCalls++
	return l.lines, l.clusters
}

type testLabelPainter struct {
	textOrigin geometry.Point
	textLayout typography.TextLayout
}

func (p *testLabelPainter) Save() {}

func (p *testLabelPainter) Restore() {}

func (p *testLabelPainter) PixelScale() float32 { return 1 }

func (p *testLabelPainter) NewImage(src image.Image) (graphics.Image, error) {
	return newTestNativeImage(src), nil
}

func (p *testLabelPainter) SetClipRect(rect geometry.Rectangle) {}

func (p *testLabelPainter) FillRect(rect geometry.Rectangle, brush graphics.Brush) {}

func (p *testLabelPainter) FillRoundRect(rect geometry.Rectangle, radius float32, brush graphics.Brush) {
}

func (p *testLabelPainter) FillEllipse(center geometry.Point, xRadius, yRadius float32, brush graphics.Brush) {
}

func (p *testLabelPainter) FillPath(path graphics.Path, brush graphics.Brush) {}

func (p *testLabelPainter) DrawLine(p0, p1 geometry.Point, strokeWidth float32, brush graphics.Brush) {
}

func (p *testLabelPainter) DrawRect(rect geometry.Rectangle, strokeWidth float32, brush graphics.Brush) {
}

func (p *testLabelPainter) DrawRoundRect(rect geometry.Rectangle, radius, strokeWidth float32, brush graphics.Brush) {
}

func (p *testLabelPainter) DrawEllipse(center geometry.Point, xRadius, yRadius, strokeWidth float32, brush graphics.Brush) {
}

func (p *testLabelPainter) DrawPath(path graphics.Path, strokeWidth float32, brush graphics.Brush) {
}

func (p *testLabelPainter) DrawTextLayout(origin geometry.Point, layout typography.TextLayout) {
	p.textOrigin = origin
	p.textLayout = layout
}

func (p *testLabelPainter) DrawImage(rect geometry.Rectangle, img graphics.Image) {}

func (p *testLabelPainter) SetTransform(matrix geometry.Transform) {}

func (p *testLabelPainter) DrawBoxShadow(rect geometry.Rectangle, radius float32, shadow graphics.BoxShadow) {
}

// Shared input event and painting helpers.

func dispatchKey(t *testing.T, win Window, key events.Key, modifiers events.Modifiers) {
	t.Helper()
	if err := win.DispatchEvent(events.KeyEvent{
		EventType: events.KeyDown,
		Key:       key,
		Modifiers: modifiers,
	}); err != nil {
		t.Fatal(err)
	}
}

type testTextInputPainter struct {
	testLabelPainter
	fillRect            geometry.Rectangle
	fillRadius          float32
	fillBrush           graphics.Brush
	fills               []geometry.Rectangle
	clipRect            geometry.Rectangle
	drawRect            geometry.Rectangle
	drawRadius          float32
	drawRectStrokeWidth float32
	drawRectBrush       graphics.Brush
	drawLines           int
	lineP0              geometry.Point
	lineP1              geometry.Point
	lineStrokeWidth     float32
	lineBrush           graphics.Brush
}

func (p *testTextInputPainter) FillRect(rect geometry.Rectangle, brush graphics.Brush) {
	if len(p.fills) == 0 {
		p.fillRect, p.fillBrush = rect, brush
	}
	p.fills = append(p.fills, rect)
}

func (p *testTextInputPainter) SetClipRect(rect geometry.Rectangle) { p.clipRect = rect }

func (p *testTextInputPainter) FillRoundRect(rect geometry.Rectangle, radius float32, brush graphics.Brush) {
	p.fillRect = rect
	p.fillRadius = radius
	p.fillBrush = brush
	p.fills = append(p.fills, rect)
}

func (p *testTextInputPainter) DrawRect(rect geometry.Rectangle, strokeWidth float32, brush graphics.Brush) {
	p.drawRect = rect
	p.drawRectStrokeWidth = strokeWidth
	p.drawRectBrush = brush
}

func (p *testTextInputPainter) DrawRoundRect(rect geometry.Rectangle, radius, strokeWidth float32, brush graphics.Brush) {
	p.drawRect = rect
	p.drawRadius = radius
	p.drawRectStrokeWidth = strokeWidth
	p.drawRectBrush = brush
}

func (p *testTextInputPainter) DrawLine(p0, p1 geometry.Point, strokeWidth float32, brush graphics.Brush) {
	p.drawLines++
	p.lineP0 = p0
	p.lineP1 = p1
	p.lineStrokeWidth = strokeWidth
	p.lineBrush = brush
}

// newInputFixture 使用固定排版，在替身窗口中挂载并聚焦单行编辑器，测试清理时卸载。
func newInputFixture(t *testing.T, text string) (*TextInput, *window, *editorTypography) {
	t.Helper()
	typo := &editorTypography{}
	setTestApplication(t, typo)
	input := NewTextInput()
	input.SetText(text)
	win := &window{}
	win.SetWidget(input)
	t.Cleanup(func() { win.SetWidget(nil) })
	input.Arrange(geometry.Rect(0, 0, 80, 40))
	win.SetFocusedWidget(input)
	return input, win, typo
}

// Deterministic editor geometry, clipboard and window fixtures.

// Fixture typography has 10 DIP cells and 20 DIP lines. It is deliberately
// independent of editor geometry; shaping/bidi contracts have separate tests.
type editorTypography struct{ testTypography }

func (c *editorTypography) NewTextLayout(text string, format typography.TextFormat, width, height float32) (typography.TextLayout, error) {
	l, _ := c.testTypography.NewTextLayout(text, format, width, height)
	p := l.(*testTextLayout)
	p.resized = func() { layoutEditorFixture(p) }
	layoutEditorFixture(p)
	return p, nil
}

func layoutEditorFixture(p *testTextLayout) {
	text, format, width := p.text, p.format, p.width
	p.lines, p.clusters = nil, nil
	line := typography.TextLine{Height: 20, Baseline: 15}
	x, y := float32(0), float32(0)
	for offset, r := range text {
		if x+10 > width && x > 0 && format.WrapMode != WrapNone {
			line.Length = offset - line.Start
			line.Width = x
			p.lines = append(p.lines, line)
			x, y = 0, y+20
			line = typography.TextLine{Start: offset, Y: y, Height: 20, Baseline: y + 15}
		}
		cluster := typography.TextCluster{Start: offset, Length: utf8.RuneLen(r), X: x, Y: y, Width: 10, Height: 20, LineIndex: len(p.lines)}
		p.clusters = append(p.clusters, cluster)
		x += 10
	}
	line.Length = len(text) - line.Start
	line.Width = x
	p.lines = append(p.lines, line)
	p.measureSize = geometry.Size{Width: x, Height: y + 20}
}

func newEditorFixture(t *testing.T, text string) (*TextView, *window, *editorTypography) {
	t.Helper()
	typo := &editorTypography{}
	setTestApplication(t, typo)
	editor := NewTextView()
	editor.SetModel(NewTextModel(text))
	win := &window{}
	win.SetWidget(editor)
	editor.Arrange(geometry.Rect(0, 0, 209, 108))
	win.SetFocusedWidget(editor)
	t.Cleanup(func() { win.SetWidget(nil) })
	return editor, win, typo
}

func editorKey(t *testing.T, win *window, key events.Key, modifiers events.Modifiers) {
	t.Helper()
	if err := win.DispatchEvent(events.KeyEvent{EventType: events.KeyDown, Key: key, Modifiers: modifiers}); err != nil {
		t.Fatal(err)
	}
}

type editorClipboard struct {
	text    string
	pending func(string, error)
}

func (c *editorClipboard) SetText(s string) error { c.text = s; return nil }

func (c *editorClipboard) RequestText(fn func(string, error)) { c.pending = fn }

// timedEditorFixture 组合固定排版和可控计时队列，测试显式推进时间，不使用 sleep。
func timedEditorFixture(t *testing.T) (*TextView, *ScrollView, *window, *timerTestQueue, *editorTypography) {
	t.Helper()
	q := newTimerTestQueue(t)
	typo := &editorTypography{}
	q.app.typo = typo
	old := App
	App = q.app
	t.Cleanup(func() { App = old })
	editor := NewTextView()
	editor.SetModel(NewTextModel(strings.Repeat("row\n", 100)))
	scroll := NewScrollView()
	scroll.SetChild(editor)
	win := &window{}
	win.SetWidget(scroll)
	t.Cleanup(func() { win.SetWidget(nil) })
	box := geometry.Rect(0, 0, 220, 108)
	scroll.Measure(layout.Tight(box.Size))
	scroll.Arrange(box)
	win.SetFocusedWidget(editor)
	if err := win.DispatchEvent(events.FocusEvent{Focused: true}); err != nil {
		t.Fatal(err)
	}
	return editor, scroll, win, q, typo
}

// Wrapped editor fixtures and normal pointer dispatch.

func longWrappedEditor(t *testing.T) (*TextView, *ScrollView, *window, *editorTypography) {
	t.Helper()
	v, scroll, win, _, typo := timedEditorFixture(t)
	v.Model().SetText(strings.Repeat("abcdefghij", 1000))
	win.width, win.height = 220, 108
	settleEditorLayout(t, win)
	return v, scroll, win, typo
}

func settleEditorLayout(t *testing.T, win *window) {
	t.Helper()
	for range 8 {
		if !win.layoutDirty {
			return
		}
		win.layoutFrame(win.Widget())
	}
	t.Fatal("editor layout did not settle")
}

func editorPointer(t *testing.T, win *window, kind events.EventType, point geometry.Point, mods events.Modifiers) {
	t.Helper()
	if err := win.DispatchEvent(events.PointerEvent{
		EventType: kind, Position: point, Button: events.PointerButtonLeft, Modifiers: mods,
	}); err != nil {
		t.Fatal(err)
	}
}
