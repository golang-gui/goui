package gui

import (
	"errors"
	"fmt"
	"image/color"
	"testing"
	"time"

	"github.com/golang-gui/goui/core/colors"
	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/platform/typography"
	"github.com/golang-gui/goui/style"
)

func TestLabelTextSnapshotAndRequestLayout(t *testing.T) {
	win := testWindowWithSurface(&window{})
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

func TestLabelMeasureConvertsUnboundedLayoutAxes(t *testing.T) {
	// LinearBox/ScrollView 的无界轴是布局标记，不能作为 1e9 的原生排版尺寸。
	for _, tc := range []struct {
		name            string
		available, want geometry.Size
	}{
		{"水平", geometry.Size{Width: layout.Inf, Height: 80}, geometry.Size{Width: labelMeasureExtent, Height: 80}},
		{"垂直", geometry.Size{Width: 400, Height: layout.Inf}, geometry.Size{Width: 400, Height: labelMeasureExtent}},
		{"双向", geometry.Size{Width: layout.Inf, Height: layout.Inf}, geometry.Size{Width: labelMeasureExtent, Height: labelMeasureExtent}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			typo := &testTypography{measureSize: geometry.Size{Width: 42, Height: 18}}
			setTestApplication(t, typo)
			label := NewLabel("unbounded label")
			t.Cleanup(label.releaseLayout)
			measured := label.Measure(layout.Loose(tc.available))
			if len(typo.calls) != 1 || measured.Size.Height != 18 {
				t.Fatalf("Label 未正常测量：calls=%d measured=%+v", len(typo.calls), measured)
			}
			call := typo.calls[0]
			if call.width != tc.want.Width || call.height != tc.want.Height {
				t.Fatalf("原生尺寸=%gx%g，预期=%+v", call.width, call.height, tc.want)
			}
		})
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
	inkRanges         []labelInkRange
	underline, strike []bool
}

type labelInkRange struct {
	start, length int
	ink           color.Color
}

func (l *labelFormatLayout) SetTextFont(start, length int, font typography.FontInfo) {
	l.fonts = append(l.fonts, struct {
		start, length int
		font          typography.FontInfo
	}{start, length, font})
}
func (l *labelFormatLayout) SetTextColor(start, length int, c color.Color) {
	l.ink = append(l.ink, c)
	l.inkRanges = append(l.inkRanges, labelInkRange{start, length, c})
}
func (l *labelFormatLayout) SetUnderline(_, _ int, v bool)     { l.underline = append(l.underline, v) }
func (l *labelFormatLayout) SetStrikethrough(_, _ int, v bool) { l.strike = append(l.strike, v) }

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
	win := testWindowWithSurface(&window{})
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
	help := `\{format\}\{bold:帮助\}`
	w.SetTooltip(help)
	if w.Tooltip() != help || w.Snapshot().Tooltip != help {
		t.Fatal("Tooltip 纯文本契约不应自动解释格式语法")
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

// 固定 Cluster 指标覆盖选区，避免把 Unicode 码点误当成可拆分字形。
func labelSelectionFixture(t *testing.T, text string) (*Label, *window, *editorTypography) {
	t.Helper()
	typo := &editorTypography{}
	setTestApplication(t, typo)
	label := NewLabel(text)
	label.SetSelectable(true)
	win := testWindowWithSurface(&window{})
	win.SetWidget(label)
	label.Arrange(geometry.Rect(0, 0, 200, 80))
	if err := win.DispatchEvent(events.FocusEvent{Focused: true}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { win.SetWidget(nil) })
	return label, win, typo
}

func TestLabelSelectionDefaultsAndLifetime(t *testing.T) {
	label := NewLabel("hello")
	if label.Selectable() || label.Focusable() || label.selectState != nil {
		t.Fatal("普通 Label 不应分配选择状态或接收焦点")
	}
	label.SetSelection(TextSelection{Caret: 2})
	if label.Selection() != (TextSelection{}) || label.selectState != nil {
		t.Fatal("关闭选择时 setter 应无操作")
	}
	label.SetFocusable(true)
	label.SetCursor(CursorCrosshair)
	count := 0
	label.ConnectSelection(func(TextSelection) { count++ })
	label.SetSelectable(true)
	if !label.Focusable() || label.Cursor() != CursorText {
		t.Fatal("未设置选择默认交互")
	}
	label.SetSelection(TextSelection{Anchor: 4, Caret: 1})
	if label.SelectedText() != "ell" || count != 1 {
		t.Fatal("反向选区未保留", label.Selection())
	}
	old := label.Snapshot().Attributes[LabelInfoKey].(LabelInfo)
	label.SetSelectable(false)
	if !label.Focusable() || label.Cursor() != CursorCrosshair || len(label.EventControllers()) != 0 || label.Selection() != (TextSelection{}) || count != 2 {
		t.Fatal("关闭选择未还原默认值/释放控制器/清空选区")
	}
	if !old.Selectable || old.Selection != (TextSelection{Anchor: 4, Caret: 1}) {
		t.Fatal("快照持有可变状态")
	}
	label.SetSelectable(true)
	label.SetSelection(TextSelection{Caret: 2})
	if count != 3 {
		t.Fatal("重新启用丢失信号连接")
	}
	label.SetCursor(CursorPointing)
	label.SetFocusable(false)
	label.SetSelectable(false)
	if label.Cursor() != CursorPointing || label.Focusable() {
		t.Fatal("禁用选择覆盖了用户交互配置")
	}
}

func TestLabelSelectionClustersTextChangesAndCache(t *testing.T) {
	label, win, typo := labelSelectionFixture(t, `\{format\}é\{bold:中文\}x`)
	label.ensureLayout(label.Rect().Size)
	native := typo.layouts[0]
	native.clusters = []typography.TextCluster{
		{Start: 0, Length: 3, Width: 10, Height: 20},
		{Start: 3, Length: 3, X: 10, Width: 10, Height: 20},
		{Start: 6, Length: 3, X: 20, Width: 10, Height: 20},
		{Start: 9, Length: 1, X: 30, Width: 10, Height: 20},
	}
	label.SetSelection(TextSelection{Anchor: 10, Caret: 2})
	if label.Selection() != (TextSelection{Anchor: 10, Caret: 0}) || label.SelectedText() != "é中文x" {
		t.Fatal("UTF-8 或 Cluster 边界校正错误", label.Selection())
	}
	changes := 0
	label.ConnectSelection(func(TextSelection) { changes++ })
	win.layoutDirty, win.paintDirty = false, false
	label.SetSelection(TextSelection{Anchor: 3, Caret: 9})
	label.Paint(&testLabelPainter{})
	if win.layoutDirty || !win.paintDirty || len(typo.calls) != 1 || native.metricsCalls != 1 || native.destroyed {
		t.Fatal("改变选区重建了排版或几何")
	}
	label.SetText(`\{format\}é\{italic:中文\}x`)
	if label.Selection() != (TextSelection{Anchor: 3, Caret: 9}) || changes != 1 {
		t.Fatal("仅格式变化重置了选区")
	}
	label.StyleChanged()
	label.Paint(&testLabelPainter{})
	if changes != 1 {
		t.Fatal("绘制/样式失效不得发送选区通知")
	}
	label.SetText("new")
	if label.Selection() != (TextSelection{}) || changes != 2 {
		t.Fatal("正文变化没有重置选区")
	}
	label.SelectAll()
	selection := label.Selection()
	win.SetWidget(nil)
	if label.Selection() != selection || label.selectState.geometry != nil || !typo.layouts[len(typo.layouts)-1].destroyed {
		t.Fatal("卸载未释放原生资源或意外重置数据")
	}
	win.SetWidget(label)
	label.SetSelection(TextSelection{Caret: 1})
	if changes != 4 {
		t.Fatal("重新挂载丢失信号")
	}
}

func TestLabelSelectionDispatchCaptureAndKeys(t *testing.T) {
	label, win, _ := labelSelectionFixture(t, "hello world")
	primary := textCommandModifier()
	pointer := func(kind events.EventType, x float32, mods events.Modifiers) {
		t.Helper()
		e := events.PointerEvent{EventType: kind, Position: geometry.Point{X: x, Y: 10}, Button: events.PointerButtonLeft, Modifiers: mods}
		if kind != events.PointerUp {
			e.Buttons = events.PointerButtonLeftDown
		}
		if err := win.DispatchEvent(e); err != nil {
			t.Fatal(err)
		}
	}
	pointer(events.PointerDown, 12, 0)
	pointer(events.PointerMove, 240, 0)
	pointer(events.PointerUp, 240, 0)
	if !label.Focused() || label.Selection() != (TextSelection{Anchor: 1, Caret: 11}) || label.selectState.controller.dragging {
		t.Fatal("选择未捕获边界外事件", label.Selection())
	}
	label.selectState.controller.lastDown = time.Time{}
	pointer(events.PointerDown, 32, events.ModifierShift)
	pointer(events.PointerUp, 32, events.ModifierShift)
	if label.Selection() != (TextSelection{Anchor: 1, Caret: 3}) {
		t.Fatal("Shift 点击改变了锚点", label.Selection())
	}
	now := time.Unix(100, 0)
	label.selectState.controller.now = func() time.Time { return now }
	pointer(events.PointerDown, 72, 0)
	pointer(events.PointerUp, 72, 0)
	now = now.Add(100 * time.Millisecond)
	pointer(events.PointerDown, 72, 0)
	pointer(events.PointerUp, 72, 0)
	if label.SelectedText() != "world" {
		t.Fatal("双击未选择词", label.Selection(), label.SelectedText())
	}
	editorKey(t, win, events.KeyArrowLeft, 0)
	if label.Selection() != (TextSelection{Anchor: 6, Caret: 6}) {
		t.Fatal("左键未折叠选区", label.Selection())
	}
	editorKey(t, win, events.KeyEnd, events.ModifierShift)
	if label.SelectedText() != "world" {
		t.Fatal("Shift End 未扩展", label.Selection())
	}
	editorKey(t, win, events.KeyA, primary)
	clip := &editorClipboard{}
	App.(*application).clipboard = clip
	editorKey(t, win, events.KeyC, primary)
	if clip.text != label.PlainText() {
		t.Fatal("未复制纯文本")
	}
	editorKey(t, win, events.KeyEscape, 0)
	if label.Selection().Anchor != label.Selection().Caret {
		t.Fatal("Escape 未折叠")
	}
	handled := false
	if err := win.DispatchEvent(events.KeyEvent{EventType: events.KeyDown, Key: events.KeyF5, Handled: &handled}); err != nil || handled {
		t.Fatal("吞掉了无关按键", err)
	}
	label.SelectAll()
	before := label.Selection()
	label.SetEnabled(false)
	editorKey(t, win, events.KeyArrowLeft, 0)
	if label.Selection() != before || label.selectState.controller.dragging {
		t.Fatal("禁用未取消交互或修改了选区")
	}
}

// 相邻 RTL 字形的逻辑顺序与视觉顺序相反；选区背景只能覆盖实际选中的字形。
func TestLabelSelectionBidiPaintAndResize(t *testing.T) {
	label, win, typo := labelSelectionFixture(t, "aאבz")
	label.ensureLayout(label.Rect().Size)
	native := typo.layouts[0]
	native.clusters = []typography.TextCluster{
		{Start: 0, Length: 1, Width: 10, Height: 20},
		{Start: 1, Length: 2, X: 20, Width: 10, Height: 20, Direction: typography.TextRightToLeft},
		{Start: 3, Length: 2, X: 10, Width: 10, Height: 20, Direction: typography.TextRightToLeft},
		{Start: 5, Length: 1, X: 30, Width: 10, Height: 20},
	}
	App.(*application).style = style.Sheet(style.Name("label").Part("selection").BackgroundColor(color.RGBA{R: 20, A: 255}))
	label.SetSelection(TextSelection{Anchor: 0, Caret: 3})
	p := &labelSelectionPainter{}
	label.Paint(p)
	if len(p.fills) != 2 || p.fills[0] != geometry.Rect(0, 0, 10, 20) || p.fills[1] != geometry.Rect(20, 0, 10, 20) || !p.textAfterFill {
		t.Fatal("双向选区背景错误或覆盖了文字", p.fills)
	}
	if label.selectState.geometry == nil {
		t.Fatal("几何未缓存")
	}
	label.Arrange(geometry.Rect(0, 0, 100, 80))
	if label.selectState.geometry != nil {
		t.Fatal("Arrange 没有使旧几何失效")
	}
	label.Paint(p)
	if native.metricsCalls != 2 {
		t.Fatal("尺寸改变未重新获取指标", native.metricsCalls)
	}
	win.SetFocusedWidget(label)
	editorKey(t, win, events.KeyArrowRight, 0)
	if label.Selection().Anchor != label.Selection().Caret {
		t.Fatal("双向选区没有折叠")
	}
}

type labelSelectionPainter struct {
	testLabelPainter
	fills         []geometry.Rectangle
	textAfterFill bool
}

func (p *labelSelectionPainter) FillRect(r geometry.Rectangle, _ graphics.Brush) {
	p.fills = append(p.fills, r)
}
func (p *labelSelectionPainter) DrawTextLayout(o geometry.Point, l typography.TextLayout) {
	p.textAfterFill = len(p.fills) > 0
	p.testLabelPainter.DrawTextLayout(o, l)
}

type labelFailClipboard struct {
	err   error
	calls int
}

func (c *labelFailClipboard) SetText(string) error            { c.calls++; return c.err }
func (c *labelFailClipboard) RequestText(func(string, error)) {}

func TestLabelSelectionCopyErrorsAndReentrancy(t *testing.T) {
	label, win, typo := labelSelectionFixture(t, "éx")
	label.SetSelection(TextSelection{Anchor: 0, Caret: len(label.PlainText())})
	fail := errors.New("测试剪贴板失败")
	clip := &labelFailClipboard{err: fail}
	App.(*application).clipboard = clip
	errorsSeen := 0
	label.ConnectCopyError(func(err error) {
		if !errors.Is(err, fail) {
			t.Fatal(err)
		}
		errorsSeen++
	})
	if err := label.Copy(); !errors.Is(err, fail) || errorsSeen != 0 {
		t.Fatal("直接 Copy 不应重复通知", err)
	}
	win.SetFocusedWidget(label)
	editorKey(t, win, events.KeyC, textCommandModifier())
	if errorsSeen != 1 || clip.calls != 2 {
		t.Fatal("内置动作未单次反馈错误")
	}
	// 模拟新字体将 e+组合音标合成一个 Cluster，Paint 不通知，输入前才归一。
	native := typo.layouts[0]
	native.clusters = []typography.TextCluster{{Start: 0, Length: 3, Width: 10, Height: 20}, {Start: 3, Length: 1, X: 10, Width: 10, Height: 20}}
	label.invalidateSelectionGeometry()
	label.selectState.value = TextSelection{Anchor: 1, Caret: 4}
	label.ConnectSelection(func(TextSelection) { label.SetEnabled(false) })
	label.Paint(&testLabelPainter{})
	if !label.Enabled() {
		t.Fatal("Paint 发出了选择信号")
	}
	editorKey(t, win, events.KeyC, textCommandModifier())
	if label.Enabled() || clip.calls != 2 {
		t.Fatal("归一回调禁用后仍访问剪贴板")
	}
	label.SetEnabled(true)
	label.SetSelectable(false)
	label.SetSelectable(true)
	first := label.ConnectSelection(func(TextSelection) { label.WidgetBase.destroy(label) })
	defer first.Disconnect()
	late := 0
	label.ConnectSelection(func(TextSelection) { late++ })
	label.SelectAll()
	if !label.Destroyed() || late != 0 {
		t.Fatal("对象销毁后继续调用信号观察者")
	}
}

func TestLabelSelectionAtomicClustersAndWrappedLines(t *testing.T) {
	for _, text := range []string{"é", "👩‍💻", "ffi"} {
		t.Run(text, func(t *testing.T) {
			typo := &testTypography{lines: []typography.TextLine{{Length: len(text) + 1, Height: 30}}, clusters: []typography.TextCluster{{Start: 0, Length: len(text), Width: 24, Height: 30}, {Start: len(text), Length: 1, X: 24, Width: 10, Height: 30}}}
			setTestApplication(t, typo)
			label := NewLabel(text + "x")
			label.SetSelectable(true)
			label.Arrange(geometry.Rect(0, 0, 100, 40))
			t.Cleanup(label.releaseLayout)
			label.SetSelection(TextSelection{Anchor: len(text) + 1, Caret: 1})
			if label.Selection().Caret != 0 || label.SelectedText() != text+"x" {
				t.Fatal("Cluster 被拆分", label.Selection())
			}
		})
	}
	label, win, _ := labelSelectionFixture(t, "abcdefghij")
	label.SetWrapMode(WrapChar)
	label.Arrange(geometry.Rect(0, 0, 40, 100))
	win.SetFocusedWidget(label)
	label.SetSelection(TextSelection{Caret: 2, Anchor: 2})
	editorKey(t, win, events.KeyArrowDown, events.ModifierShift)
	if label.Selection() != (TextSelection{Anchor: 2, Caret: 6}) {
		t.Fatal("视觉行上下导航未保持 X", label.Selection())
	}
	label.SelectAll()
	if label.SelectedText() != "abcdefghij" {
		t.Fatal("软换行不应新增字符")
	}
	label.SetText("abc\ndef")
	label.SelectAll()
	if label.SelectedText() != "abc\ndef" {
		t.Fatal("硬换行应保留")
	}
}

// 无关视觉属性和状态通知不得销毁文字资源；颜色比较不依赖接口可比较性。
func TestLabelStyleChangeRetainsEquivalentTextResources(t *testing.T) {
	typo := &labelFormatTypography{}
	setTestApplication(t, typo)
	app := App.(*application)
	ink := &color.RGBA{R: 40, G: 80, B: 120, A: 255}
	app.style = style.Sheet(style.Name("label").FontSize(14).ForegroundColor(ink))
	label := NewLabel("stable")
	t.Cleanup(label.releaseLayout)
	label.Arrange(geometry.Rect(0, 0, 100, 30))
	label.Paint(&testLabelPainter{})
	native := typo.records[0]
	app.style = style.Sheet(style.Name("label").FontSize(14).
		ForegroundColor(labelTestColor{40 * 257, 80 * 257, 120 * 257, 65535}).
		BackgroundColor(color.White).BorderWidth(2))
	for range 5 {
		label.StyleChanged()
		label.Paint(&testLabelPainter{})
	}
	if len(typo.records) != 1 || native.destroyed || len(native.inkRanges) != 0 {
		t.Fatal("等价文字样式重建或修改了排版", len(typo.records), native.destroyed, native.inkRanges)
	}
	// 修改原先的可变颜色对象不会改写已保存的样式快照。
	app.style = style.Sheet(style.Name("label").FontSize(14).ForegroundColor(ink))
	ink.R = 200
	label.StyleChanged()
	label.Paint(&testLabelPainter{})
	if len(typo.records) != 1 || native.destroyed || len(native.inkRanges) != 1 || !colors.Equal(native.inkRanges[0].ink, ink) {
		t.Fatal("原地改变颜色未更新，或无意义重建了排版", native.inkRanges)
	}
}

type labelTestColor []uint32

func (c labelTestColor) RGBA() (r, g, b, a uint32) { return c[0], c[1], c[2], c[3] }

// 基础色覆盖全文后须恢复范围颜色；范围 setter 不应改写 Format 的基础值。
func TestLabelColorChangeRetainsLayoutAndRestoresRanges(t *testing.T) {
	typo := &labelFormatTypography{}
	setTestApplication(t, typo)
	app := App.(*application)
	sheet := func(base, accent color.Color) style.StyleSheet {
		return style.Sheet(style.Name("label").FontSize(14).ForegroundColor(base),
			style.Name("accent").ForegroundColor(accent))
	}
	red := color.RGBA{R: 220, A: 255}
	app.style = sheet(color.Black, red)
	label := NewLabel(`\{format\}a\{accent:b\}c\{inherit:d\}`)
	t.Cleanup(label.releaseLayout)
	label.Arrange(geometry.Rect(0, 0, 100, 30))
	label.Paint(&testLabelPainter{})
	native := typo.records[0]
	for _, tc := range []struct{ base, accent color.Color }{
		{color.White, red}, {color.White, color.RGBA{B: 200, A: 128}}, {color.Black, red},
	} {
		app.style = sheet(tc.base, tc.accent)
		label.StyleChanged()
		label.Paint(&testLabelPainter{})
		if native.destroyed || len(typo.records) != 1 || len(native.fonts) != 2 {
			t.Fatal("仅颜色改变重建了排版或重设字体")
		}
		for i := range 4 {
			got := native.Format().TextColor
			for _, run := range native.inkRanges {
				if run.start <= i && i < run.start+run.length {
					got = run.ink
				}
			}
			want := tc.base
			if i == 1 {
				want = tc.accent
			}
			if !colors.Equal(got, want) {
				t.Fatalf("字节 %d 的最终颜色错误：got=%v want=%v", i, got, want)
			}
		}
	}
}

// 颜色更新不影响选区几何；换行、对齐通过原有 setter 更新同一排版。
func TestLabelStyleChangePreservesSelectionAndUpdatesParagraph(t *testing.T) {
	label, _, typo := labelSelectionFixture(t, "abc")
	label.SetSelection(TextSelection{Anchor: 0, Caret: 2})
	native := label.cachedLayout
	selectionGeometry := label.selectState.geometry
	changes := 0
	label.ConnectSelection(func(TextSelection) { changes++ })
	App.(*application).style = style.Sheet(append(DefaultStyleRules(), style.Name("label").ForegroundColor(color.White))...)
	label.StyleChanged()
	label.Paint(&testLabelPainter{})
	if label.cachedLayout != native || label.selectState.geometry != selectionGeometry || changes != 0 {
		t.Fatal("颜色更新改变了排版身份、选区几何或选区通知")
	}
	label.SetWrapMode(WrapWordChar)
	label.SetTextAlign(TextAlignCenter)
	label.Paint(&testLabelPainter{})
	if label.cachedLayout != native || len(typo.calls) != 1 || native.Format().WrapMode != WrapWordChar || native.Format().TextAlign != TextAlignCenter || changes != 0 {
		t.Fatal("段落参数变更未复用排版，或发出了选区通知")
	}
}

// 已显示的排版与选区不应被测量查询改写；两个常见约束的结果均能复用。
func TestLabelMeasurementDoesNotMutateDisplayedLayout(t *testing.T) {
	label, _, typo := labelSelectionFixture(t, "abc")
	label.SetSelection(TextSelection{Anchor: 0, Caret: 2})
	native := label.cachedLayout.(*testTextLayout)
	selectionGeometry := label.selectState.geometry
	width, height := native.Size()
	resizes := 0
	native.resized = func() { resizes++ }
	a, b := layout.Loose(geometry.Size{Width: 90, Height: 60}), layout.Loose(geometry.Size{Width: 120, Height: 70})
	for range 4 {
		for _, c := range []layout.Constraint{a, b} {
			measured := label.Measure(c)
			if measured.Width != 30 || measured.Height != 20 || !measured.HasBaseline || measured.Baseline != 15 {
				t.Fatalf("测量值没有保留独立预期：%+v", measured)
			}
			label.Paint(&testLabelPainter{})
		}
	}
	if w, h := native.Size(); w != width || h != height || resizes != 0 || label.cachedLayout != native || label.selectState.geometry != selectionGeometry {
		t.Fatal("测量改变了显示排版或选区几何", resizes, width, height, native.width, native.height)
	}
	if len(typo.layouts) != 3 || !typo.layouts[1].destroyed || !typo.layouts[2].destroyed || native.destroyed {
		t.Fatal("查询没有复用纯值结果，或临时资源未释放", len(typo.layouts))
	}
	// 最小尺寸不是原生排版键，每次返回仍要应用当前的约束。
	tight := layout.Tight(a.Max)
	if got := label.Measure(tight); got.Size != a.Max || len(typo.layouts) != 3 {
		t.Fatal("相同上限的缓存错误保留了先前约束", got)
	}
}

// 真实分配变化只在显示排版上修改一次；旧约束查询不得把尺寸改回去。
func TestLabelMeasurementCacheSurvivesDisplayResize(t *testing.T) {
	typo := &testTypography{measureSize: geometry.Size{Width: 42, Height: 18}}
	setTestApplication(t, typo)
	label := NewLabel("hello")
	t.Cleanup(label.releaseLayout)
	c := layout.Loose(geometry.Size{Width: 100, Height: 50})
	label.Measure(c)
	if len(typo.layouts) != 1 || typo.layouts[0].destroyed {
		t.Fatal("首份测量排版未留给首次绘制")
	}
	label.Arrange(geometry.Rect(0, 0, 80, 30))
	label.Paint(&testLabelPainter{})
	native := label.cachedLayout.(*testTextLayout)
	resizes := 0
	native.resized = func() { resizes++ }
	label.Arrange(geometry.Rect(0, 0, 90, 30))
	label.Paint(&testLabelPainter{})
	for range 5 {
		label.RequestLayout()
		label.Measure(c)
		label.Paint(&testLabelPainter{})
	}
	if resizes != 1 || len(typo.layouts) != 1 || native.width != 90 || native.height != 30 {
		t.Fatal("重复测量使显示排版尺寸往返修改", resizes, len(typo.layouts))
	}
}

// 测量保留精确换行结果；颜色不丢结果，段落／正文变化必须使旧结果失效。
func TestLabelMeasurementInvalidationAndWrapping(t *testing.T) {
	typo := &editorTypography{}
	setTestApplication(t, typo)
	label := NewLabel("abcdef")
	t.Cleanup(label.releaseLayout)
	label.SetWrapMode(WrapChar)
	c := layout.Loose(geometry.Size{Width: 20, Height: 100})
	if m := label.Measure(c); m.Width != 20 || m.Height != 60 || m.Baseline != 15 {
		t.Fatal("独立的三行换行预期不匹配", m)
	}
	label.Arrange(geometry.Rect(0, 0, 60, 20))
	label.Paint(&testLabelPainter{})
	calls := len(typo.calls)
	App.(*application).style = style.Sheet(append(DefaultStyleRules(), style.Name("label").ForegroundColor(color.White))...)
	label.StyleChanged()
	if m := label.Measure(c); m.Height != 60 || len(typo.calls) != calls {
		t.Fatal("颜色改变丢失了可复用的测量结果", m, len(typo.calls))
	}
	label.SetWrapMode(WrapNone)
	if m := label.Measure(c); m.Width != 20 || m.Height != 20 {
		t.Fatal("换行参数改变仍使用旧测量结果", m)
	}
	label.SetText("x")
	if m := label.Measure(c); m.Width != 10 || m.Height != 20 {
		t.Fatal("正文改变仍使用旧测量结果", m)
	}
}

// 失败结果不能进入缓存；卸载后再测量须重新建立资源。
func TestLabelMeasurementRetriesFailureAndClearsOnUnmount(t *testing.T) {
	typo := &labelFailTypography{}
	setTestApplication(t, typo)
	label := NewLabel("abc")
	win := testWindowWithSurface(&window{})
	win.SetWidget(label)
	t.Cleanup(func() { win.SetWidget(nil) })
	label.Arrange(geometry.Rect(0, 0, 200, 80))
	label.Paint(&testLabelPainter{})
	c := layout.Loose(geometry.Size{Width: 90, Height: 60})
	typo.fail = true
	if m := label.Measure(c); m != (layout.Measurement{}) {
		t.Fatal("测量失败没有保留原有空结果语义", m)
	}
	typo.fail = false
	if m := label.Measure(c); m.Width != 30 || m.Height != 20 {
		t.Fatal("失败后没有重新尝试测量", m)
	}
	calls := len(typo.calls)
	win.SetWidget(nil)
	win.SetWidget(label)
	if m := label.Measure(c); m.Width != 30 || len(typo.calls) != calls+1 {
		t.Fatal("重新挂载错误保留了原生资源或旧缓存", m)
	}
}

type labelFailTypography struct {
	editorTypography
	fail bool
}

func (c *labelFailTypography) NewTextLayout(text string, f typography.TextFormat, w, h float32) (typography.TextLayout, error) {
	if c.fail {
		return nil, errors.New("测试测量失败")
	}
	return c.editorTypography.NewTextLayout(text, f, w, h)
}

// Destroy 的外部订阅可改变正文或销毁控件；返回后不能继续用旧格式创建资源。
func TestLabelRebuildRechecksDestroyCallback(t *testing.T) {
	for _, destroy := range []bool{false, true} {
		t.Run(fmt.Sprintf("destroy=%t", destroy), func(t *testing.T) {
			typo := &labelFormatTypography{}
			setTestApplication(t, typo)
			label := NewLabel("old")
			t.Cleanup(label.releaseLayout)
			label.Arrange(geometry.Rect(0, 0, 100, 30))
			label.Paint(&testLabelPainter{})
			typo.records[0].ConnectDestroy(func() {
				if destroy {
					label.WidgetBase.destroy(label)
				} else {
					label.SetText(`\{format\}\{bold:updated\}`)
				}
			})
			label.SetText("trigger rebuild")
			label.Paint(&testLabelPainter{})
			if destroy {
				if len(typo.records) != 1 || label.cachedLayout != nil {
					t.Fatal("销毁后继续建立了排版")
				}
			} else if len(typo.records) != 2 || typo.records[1].Text() != "updated" || len(typo.records[1].fonts) != 1 {
				t.Fatal("销毁通知后仍使用过期正文或范围样式")
			}
		})
	}
}

// WrapBox + ScrollView 会交替完整宽度、滚动条宽度、限高和自然尺寸四种查询。
func TestLabelMeasurementCacheCoversNestedLayoutQueries(t *testing.T) {
	typo := &editorTypography{}
	setTestApplication(t, typo)
	label := NewLabel("item")
	t.Cleanup(label.releaseLayout)
	label.Arrange(geometry.Rect(0, 0, 72, 20))
	label.Paint(&testLabelPainter{})
	constraints := []layout.Constraint{
		layout.Loose(geometry.Size{Width: 838, Height: layout.Inf}),
		layout.Loose(geometry.Size{Width: 830, Height: layout.Inf}),
		layout.Loose(geometry.Size{Width: 830, Height: 60}),
		layout.Unbounded(),
	}
	for range 10 {
		for _, c := range constraints {
			if m := label.Measure(c); m.Width != 40 || m.Height != 20 || m.Baseline != 15 {
				t.Fatal("测量值错误", m)
			}
		}
		if m := label.Measure(layout.Tight(label.Rect().Size)); m.Size != label.Rect().Size {
			t.Fatal("实际分配约束未生效", m)
		}
		label.Paint(&testLabelPainter{})
	}
	if len(typo.layouts) != 5 || typo.layouts[0].destroyed {
		t.Fatal("固定四种约束造成临时排版反复创建", len(typo.layouts))
	}
	for _, native := range typo.layouts[1:] {
		if !native.destroyed {
			t.Fatal("临时排版未释放")
		}
	}
}
