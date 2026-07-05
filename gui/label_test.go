package gui

import (
	"image"
	"image/color"
	"testing"

	"github.com/golang-gui/goui/core/colors"
	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/platform/typography"
	"github.com/golang-gui/goui/style"
)

func TestLabelTextSnapshotAndRequestLayout(t *testing.T) {
	win := &window{}
	label := NewLabel("hello")
	label.SetID("title")
	label.Arrange(geometry.Rect(1, 2, 30, 40))
	win.SetWidget(label)

	win.layoutDirty = false
	win.paintDirty = false
	label.SetText("hello")

	if win.layoutDirty || win.paintDirty {
		t.Fatal("setting unchanged text should not request layout")
	}

	label.SetText("world")

	if !win.layoutDirty || !win.paintDirty {
		t.Fatal("setting text did not request layout and paint")
	}
	if label.Text() != "world" {
		t.Fatalf("unexpected label text: %q", label.Text())
	}

	info := label.Snapshot()
	if info.ID != "title" {
		t.Fatalf("unexpected snapshot id: %q", info.ID)
	}
	if info.Role != RoleLabel {
		t.Fatalf("unexpected snapshot role: %q", info.Role)
	}
	if info.Text != "world" {
		t.Fatalf("unexpected snapshot text: %q", info.Text)
	}
	if info.Bounds != geometry.Rect(1, 2, 30, 40) {
		t.Fatalf("unexpected snapshot bounds: %+v", info.Bounds)
	}
}

func TestLabelMeasureUsesTypography(t *testing.T) {
	typo := &testTypography{
		measureSize: geometry.Size{Width: 42, Height: 18},
	}
	setTestApplication(t, typo)
	label := NewLabel("hello")

	size := label.Measure(geometry.Size{Width: 100, Height: 50})

	if size != (geometry.Size{Width: 42, Height: 18}) {
		t.Fatalf("unexpected measured size: %+v", size)
	}
	if len(typo.calls) != 1 {
		t.Fatalf("expected one text layout call, got %d", len(typo.calls))
	}
	call := typo.calls[0]
	if call.text != "hello" {
		t.Fatalf("unexpected layout text: %q", call.text)
	}
	if call.width != 100 || call.height != 50 {
		t.Fatalf("unexpected layout size: %gx%g", call.width, call.height)
	}
	if !typo.layouts[0].destroyed {
		t.Fatal("measure did not destroy text layout")
	}
}

func TestLabelMeasureUsesUnboundedExtentForZeroAvailableSize(t *testing.T) {
	typo := &testTypography{
		measureSize: geometry.Size{Width: 42, Height: 18},
	}
	setTestApplication(t, typo)
	label := NewLabel("hello")

	_ = label.Measure(geometry.Size{})

	if len(typo.calls) != 1 {
		t.Fatalf("expected one text layout call, got %d", len(typo.calls))
	}
	call := typo.calls[0]
	if call.width != labelMeasureExtent || call.height != labelMeasureExtent {
		t.Fatalf("unexpected layout size: %gx%g", call.width, call.height)
	}
}

func TestLabelPaintDrawsTextLayout(t *testing.T) {
	typo := &testTypography{
		measureSize: geometry.Size{Width: 42, Height: 18},
	}
	setTestApplication(t, typo)
	label := NewLabel("hello")
	label.Arrange(geometry.Rect(10, 20, 80, 30))

	painter := new(testLabelPainter)
	label.Paint(painter)

	if len(typo.calls) != 1 {
		t.Fatalf("expected one text layout call, got %d", len(typo.calls))
	}
	call := typo.calls[0]
	if call.width != 80 || call.height != 30 {
		t.Fatalf("unexpected layout size: %gx%g", call.width, call.height)
	}
	if painter.textOrigin != (geometry.Point{}) {
		t.Fatalf("unexpected text origin: %+v", painter.textOrigin)
	}
	if painter.textLayout != typo.layouts[0] {
		t.Fatal("painter did not receive label text layout")
	}
	if !typo.layouts[0].destroyed {
		t.Fatal("paint did not destroy text layout")
	}
}

func TestLabelUsesStyleForTextFormat(t *testing.T) {
	typo := &testTypography{
		measureSize: geometry.Size{Width: 42, Height: 18},
	}
	foreground := color.RGBA{R: 90, G: 20, B: 10, A: 255}
	oldApp := App
	App = &application{
		typo: typo,
		style: style.Sheet(
			style.Name(styleNameLabel).
				ForegroundColor(foreground).
				FontFamily("Custom Sans").
				FontSize(20),
		),
	}
	t.Cleanup(func() {
		App = oldApp
	})

	label := NewLabel("hello")
	_ = label.Measure(geometry.Size{Width: 100, Height: 30})

	if len(typo.calls) != 1 {
		t.Fatalf("expected one text layout call, got %d", len(typo.calls))
	}
	format := typo.calls[0].format
	if format.Font.Family != "Custom Sans" || format.Font.Size != 20 {
		t.Fatalf("unexpected styled font: %+v", format.Font)
	}
	if !colors.Equal(format.TextColor, foreground) {
		t.Fatalf("unexpected styled text color: %v", format.TextColor)
	}
}

func TestLabelWithoutTypographyDoesNotMeasureOrPaint(t *testing.T) {
	setTestApplication(t, nil)
	label := NewLabel("hello")
	size := label.Measure(geometry.Size{Width: 100, Height: 50})
	if size != (geometry.Size{}) {
		t.Fatalf("unexpected measured size: %+v", size)
	}

	painter := new(testLabelPainter)
	label.Paint(painter)
	if painter.textLayout != nil {
		t.Fatal("label painted text without typography context")
	}
}

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

func (c *testTypography) DrawTextLayout(layout typography.TextLayout, scale float32, buf []byte) (typography.TextBitmap, error) {
	return typography.TextBitmap{}, nil
}

type testTextLayout struct {
	text        string
	format      typography.TextFormat
	width       float32
	height      float32
	measureSize geometry.Size
	lines       []typography.TextLine
	clusters    []typography.TextCluster
	destroyed   bool
}

func (l *testTextLayout) Destroy() {
	l.destroyed = true
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
	l.width = maxWidth
	l.height = maxHeight
}

func (l *testTextLayout) SetTextAlignment(align typography.TextAlignment) {
	l.format.TextAlign = align
}

func (l *testTextLayout) SetWrapMode(wrap typography.WrapMode) {
	l.format.WrapMode = wrap
}

func (l *testTextLayout) SetTextFont(start, length int, font typography.FontInfo) {
	l.format.Font = font
}

func (l *testTextLayout) SetTextColor(start, length int, c color.Color) {
	l.format.TextColor = c
}

func (l *testTextLayout) SetUnderline(start, length int, underline bool) {}

func (l *testTextLayout) SetStrikethrough(start, length int, strike bool) {}

func (l *testTextLayout) MeasureSize() (width, height float32) {
	return l.measureSize.Width, l.measureSize.Height
}

func (l *testTextLayout) MeasureMetrics() (lines []typography.TextLine, clusters []typography.TextCluster) {
	return l.lines, l.clusters
}

type testLabelPainter struct {
	textOrigin geometry.Point
	textLayout typography.TextLayout
}

func (p *testLabelPainter) SetClipRect(rect geometry.Rectangle) {}

func (p *testLabelPainter) Clear(color graphics.Color) {}

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

func (p *testLabelPainter) DrawImage(rect geometry.Rectangle, img image.Image) {}
