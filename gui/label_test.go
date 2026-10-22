package gui

import (
	"image/color"
	"testing"

	"github.com/golang-gui/goui/core/colors"
	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/layout"
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
	t.Cleanup(label.releaseLayout)

	size := label.Measure(layout.Loose(geometry.Size{Width: 100, Height: 50}))

	if size.Size != (geometry.Size{Width: 42, Height: 18}) {
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
	// With caching, the layout is NOT destroyed after Measure — it's reused for Paint.
	if typo.layouts[0].destroyed {
		t.Fatal("measure should cache text layout for reuse")
	}
}

func TestLabelMeasureReportsFirstBaseline(t *testing.T) {
	typo := &testTypography{
		measureSize: geometry.Size{Width: 42, Height: 18},
		lines: []typography.TextLine{
			{Start: 0, Length: 5, Width: 42, Height: 18, Baseline: 14},
		},
	}
	setTestApplication(t, typo)
	label := NewLabel("hello")
	t.Cleanup(label.releaseLayout)

	measured := label.Measure(layout.Loose(geometry.Size{Width: 100, Height: 50}))
	if !measured.HasBaseline || measured.Baseline != 14 {
		t.Fatalf("label first baseline: want 14, got %+v", measured)
	}

	empty := NewLabel("")
	t.Cleanup(empty.releaseLayout)
	if got := empty.Measure(layout.Loose(geometry.Size{Width: 100, Height: 50})); got.HasBaseline {
		t.Fatalf("empty label should not report a baseline: %+v", got)
	}
}

func TestLabelMeasureUsesUnboundedExtentForZeroAvailableSize(t *testing.T) {
	typo := &testTypography{
		measureSize: geometry.Size{Width: 42, Height: 18},
	}
	setTestApplication(t, typo)
	label := NewLabel("hello")
	t.Cleanup(label.releaseLayout)

	_ = label.Measure(layout.Loose(geometry.Size{}))

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
	t.Cleanup(label.releaseLayout)

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
	// With caching, the layout is NOT destroyed after Paint — it lives until unmount/setter.
	if typo.layouts[0].destroyed {
		t.Fatal("paint should cache text layout for reuse")
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
	t.Cleanup(label.releaseLayout)
	_ = label.Measure(layout.Loose(geometry.Size{Width: 100, Height: 30}))

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
	t.Cleanup(label.releaseLayout)
	size := label.Measure(layout.Loose(geometry.Size{Width: 100, Height: 50}))
	if size.Size != (geometry.Size{}) {
		t.Fatalf("unexpected measured size: %+v", size)
	}

	painter := new(testLabelPainter)
	label.Paint(painter)
	if painter.textLayout != nil {
		t.Fatal("label painted text without typography context")
	}
}
