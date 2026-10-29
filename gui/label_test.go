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

// 记录范围设置但不改变 Format() 的基础值，保持 TextLayout 的原生契约。
type labelFormatLayout struct {
	*testTextLayout
	fonts []struct {
		start, length int
		font          typography.FontInfo
	}
	ink               []color.Color
	underline, strike []bool
}

func (l *labelFormatLayout) SetTextFont(start, length int, font typography.FontInfo) {
	l.fonts = append(l.fonts, struct {
		start, length int
		font          typography.FontInfo
	}{start, length, font})
}
func (l *labelFormatLayout) SetTextColor(_, _ int, c color.Color) { l.ink = append(l.ink, c) }
func (l *labelFormatLayout) SetUnderline(_, _ int, v bool)        { l.underline = append(l.underline, v) }
func (l *labelFormatLayout) SetStrikethrough(_, _ int, v bool)    { l.strike = append(l.strike, v) }

type labelFormatTypography struct {
	testTypography
	records []*labelFormatLayout
}

func (c *labelFormatTypography) NewTextLayout(text string, f typography.TextFormat, w, h float32) (typography.TextLayout, error) {
	native, err := c.testTypography.NewTextLayout(text, f, w, h)
	if err != nil {
		return nil, err
	}
	value := &labelFormatLayout{testTextLayout: native.(*testTextLayout)}
	c.records = append(c.records, value)
	return value, nil
}

func TestLabelFormatStylesScopeAndCache(t *testing.T) {
	typo := &labelFormatTypography{}
	setTestApplication(t, typo)
	source := `\{format\}x\{warning:A\{bold:B\{plain:C\}\}D\}y`
	label := NewLabel(source)
	label.SetStyleName("heading")
	scope := NewStyleScope()
	scope.SetChild(label)
	sheet := func(size float32, ink color.Color) style.StyleSheet {
		return style.Sheet(
			style.Name("label").FontFamily("Base Family").FontSize(14).ForegroundColor(color.Black),
			style.Name("heading").FontSize(size),
			style.Name("warning").ForegroundColor(ink).Underline(true).Strikethrough(true),
			style.Name("bold").FontWeight(style.FontWeightBold).FontStyle(style.FontStyleItalic),
			style.Name("plain").FontWeight(style.FontWeightNormal).FontStyle(style.FontStyleNormal).Underline(false).Strikethrough(false),
		)
	}
	scope.SetStyleSheet(sheet(24, color.RGBA{R: 180, A: 255}))
	win := &window{}
	win.SetWidget(scope)
	t.Cleanup(func() { win.SetWidget(nil) })
	t.Cleanup(label.releaseLayout)
	measureWidget(label, layout.Loose(geometry.Size{Width: 200, Height: 100}))
	record := typo.records[0]
	if label.Text() != source || label.PlainText() != "xABCDy" || len(record.fonts) != 4 || len(typo.calls) != 1 {
		t.Fatal("源文、显示文案或单布局范围不正确", label.PlainText(), record.fonts)
	}
	for i, call := range record.fonts {
		if call.start != i+1 || call.length != 1 || call.font.Family != "Base Family" || call.font.Size != 24 || !colors.Equal(record.ink[i], color.RGBA{R: 180, A: 255}) {
			t.Fatalf("范围 %d 位置/字体/字号/颜色错误：%+v / %v", i, call, record.ink[i])
		}
	}
	if record.fonts[1].font.Weight != typography.FontWeightBold || record.fonts[1].font.Style != typography.FontStyleItalic ||
		record.fonts[2].font.Weight != typography.FontWeightRegular || record.fonts[2].font.Style != typography.FontStyleNormal ||
		!record.underline[0] || record.underline[2] || !record.strike[0] || record.strike[2] {
		t.Fatal("嵌套叠加或显式取消未生效", record.fonts, record.underline, record.strike)
	}
	label.Arrange(geometry.Rect(0, 0, 180, 60))
	label.Paint(&testLabelPainter{})
	label.SetText(source)
	if len(typo.calls) != 1 || len(record.fonts) != 4 || record.destroyed {
		t.Fatal("Paint、缩放布局或同源 SetText 重建了资源")
	}
	scope.SetStyleSheet(sheet(30, color.RGBA{B: 190, A: 255}))
	measureWidget(label, layout.Loose(geometry.Size{Width: 200, Height: 100}))
	if !record.destroyed || len(typo.records) != 2 || typo.records[1].fonts[1].font.Size != 30 || !colors.Equal(typo.records[1].ink[0], color.RGBA{B: 190, A: 255}) {
		t.Fatal("样式变更没有重新解析范围样式，或旧排版未释放")
	}
	scope.SetChild(nil)
	if !typo.records[1].destroyed {
		t.Fatal("卸载未释放格式排版")
	}
}

func TestLabelFormattedSnapshotFallbackAndEmptyBaseline(t *testing.T) {
	typo := &testTypography{lines: []typography.TextLine{{Baseline: 14}}}
	setTestApplication(t, typo)
	for _, source := range []string{`\{format\}\{bold:中\}`, `\{format\}\{bold:`, `\{format\}`} {
		label := NewLabel(source)
		t.Cleanup(label.releaseLayout)
		info := label.Snapshot()
		meta, ok := info.Attributes[LabelInfoKey].(LabelInfo)
		if !ok || info.Role != RoleLabel || info.Text != label.PlainText() || info.TextEditing != nil || label.Focusable() {
			t.Fatal("格式化展示被当成可编辑或交互控件", info)
		}
		if label.formatErr != nil {
			if info.Text != source || meta.FormatError == "" || meta.Formatted {
				t.Fatal("错误未完整降级", info)
			}
		} else if !meta.Formatted || meta.FormatError != "" {
			t.Fatal("格式状态错误", info)
		}
		if source == `\{format\}` && label.Measure(layout.Unbounded()).HasBaseline {
			t.Fatal("格式化空文本不应有基线")
		}
	}
	label := NewLabel("literal")
	if label.Snapshot().Attributes != nil || len(typo.calls) > 1 {
		t.Fatal("普通快照分配额外属性或创建原生排版")
	}
	w := newTestWidget()
	w.SetTooltip(`\{format\}\{bold:帮助\}`)
	if w.Tooltip() == "帮助" || w.Snapshot().Tooltip != "帮助" {
		t.Fatal("Tooltip 源串与显示语义未区分")
	}
	input := NewTextInput()
	input.SetText(`\{format\}\{bold:原样\}`)
	if input.Text() != input.Snapshot().Text {
		t.Fatal("编辑框解释了格式字符串")
	}
	view := NewTextView()
	view.Model().SetText(input.Text())
	view.SetReadOnly(true)
	if view.Model().Text() != input.Text() {
		t.Fatal("只读编辑视图解释了格式字符串")
	}
}

func TestLabelUnknownInlineStylePreservesBase(t *testing.T) {
	typo := &labelFormatTypography{}
	setTestApplication(t, typo)
	label := NewLabel(`\{format\}x\{missing:字\}y`)
	t.Cleanup(label.releaseLayout)
	label.SetStyleName("missing-heading")
	if label.PlainText() != "x字y" || len(typo.calls) != 0 {
		t.Fatal("未知样式吞字或提前创建排版")
	}
	base := label.resolvedTextFormat()
	label.Measure(layout.Unbounded())
	run := typo.records[0]
	if len(run.fonts) != 1 || run.fonts[0].start != 1 || run.fonts[0].length != 3 || run.fonts[0].font != base.Font || !colors.Equal(run.ink[0], base.TextColor) {
		t.Fatal("未知范围没有保留 Label 基础格式", run.fonts)
	}
}
