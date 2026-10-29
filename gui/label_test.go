package gui

import (
	"errors"
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
	win := &window{}
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
