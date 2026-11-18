package gui

import (
	"fmt"
	"image"
	"image/color"
	"reflect"
	"strings"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui/textedit"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/graphics/software"
	"github.com/golang-gui/goui/platform/typography"
	"github.com/golang-gui/goui/style"
)

// 给定排版结果的双向文字、组合字符与事件分发。

// 只使用排版契约替身；真实字体与跨层结果另在 tests/integration/typography 验证。
func testTextEditorContextNavigation(t *testing.T, newContext func() (typography.Context, error)) {
	t.Helper()
	for _, single := range []bool{false, true} {
		name := "multiline"
		if single {
			name = "singleline"
		}
		t.Run(name, func(t *testing.T) {
			typo, err := newContext()
			if err != nil {
				t.Fatal(err)
			}
			defer typo.Destroy()
			setTestApplication(t, typo)
			var widget Widget
			var editor *textEditor
			if single {
				input := NewTextInput()
				input.SetText("אבג")
				widget, editor = input, input.textEditor
			} else {
				view := NewTextView()
				view.SetModel(NewTextModel("אבג"))
				widget, editor = view, view.textEditor
			}
			win := testWindowWithSurface(&window{})
			win.SetWidget(widget)
			defer win.SetWidget(nil) // 卸载释放排版资源。
			widget.Arrange(geometry.Rect(0, 0, 200, 80))
			win.SetFocusedWidget(widget)
			editor.setSelectionValue(TextSelection{Anchor: 0, Caret: 6})
			editorKey(t, win, events.KeyArrowLeft, 0)
			if editor.selection != (TextSelection{Anchor: 6, Caret: 6}) {
				t.Fatalf("RTL selection collapsed left to %+v, want byte 6", editor.selection)
			}
			before, _ := editor.caretRect()
			editorKey(t, win, events.KeyArrowRight, events.ModifierShift)
			after, _ := editor.caretRect()
			if editor.selection != (TextSelection{Anchor: 6, Caret: 4}) || after.X <= before.X {
				t.Fatalf("RTL Shift+Right: selection=%+v x=%g -> %g", editor.selection, before.X, after.X)
			}
			editorKey(t, win, events.KeyBackspace, 0)
			if editor.model.Text() != "אב" {
				t.Fatalf("deleted wrong RTL cluster: %q", editor.model.Text())
			}
			editorKey(t, win, events.KeyZ, textCommandModifier())
			if editor.model.Text() != "אבג" || editor.selection != (TextSelection{Anchor: 6, Caret: 4}) {
				t.Fatal("undo did not restore RTL text and directional selection")
			}
			editor.model.SetText("e\u0301x")
			editor.setSelectionValue(TextSelection{Anchor: 0, Caret: 0})
			editorKey(t, win, events.KeyArrowRight, 0)
			if editor.selection.Caret != 3 {
				t.Fatalf("给定组合 cluster 被导航拆分：%+v", editor.selection)
			}
			editorKey(t, win, events.KeyBackspace, 0)
			if editor.model.Text() != "x" {
				t.Fatalf("给定组合 cluster 被删除拆分：%q", editor.model.Text())
			}
		})
	}
}

// 给定度量的滚动显示与宽度变化锚点。

func testTextViewContextRevealEnd(t *testing.T, newContext func() (typography.Context, error)) {
	t.Helper()
	typo, err := newContext()
	if err != nil {
		t.Fatal(err)
	}
	defer typo.Destroy()
	setTestApplication(t, typo)
	var text strings.Builder
	for row := 0; row < 95000; row++ {
		fmt.Fprintf(&text, "[%06d:start] text editing, viewport layout; [%06d:middle] selection and undo; [%06d:end] 中文测试。\n", row, row, row)
	}
	editor := NewTextView()
	editor.SetModel(NewTextModel(text.String()))
	scroll := NewScrollView()
	scroll.SetChild(editor)
	win := testWindowWithSurface(&window{})
	win.width, win.height = 507, 590
	win.SetWidget(scroll)
	defer win.SetWidget(nil)
	settle := func() {
		for frame := 0; win.layoutDirty && frame < 100; frame++ {
			win.layoutContent()
		}
		if win.layoutDirty {
			t.Fatal("排版请求未收敛")
		}
	}
	settle()
	win.SetFocusedWidget(editor)
	scroll.SetScrollY(900)
	settle()
	for _, width := range []float32{441, 591, 507} {
		win.width = width
		win.RequestLayout()
		settle()
	}
	editorKey(t, win, events.KeyHome, textCommandModifier())
	editorKey(t, win, events.KeyEnd, textCommandModifier()|events.ModifierShift)
	settle()
	caret, ok := editor.caretRect()
	if !ok || editor.offset.Y != scroll.ScrollY() || caret.Y < editor.offset.Y || caret.Y+caret.Height > editor.offset.Y+editor.viewport.Height || editor.revealCaret || editor.anchorPending || scroll.revealPending {
		t.Fatalf("排版在显示光标前停止：caret=%v scroll=%g viewport=%v pending=%t/%t/%t", caret, scroll.ScrollY(), editor.viewport, editor.revealCaret, editor.anchorPending, scroll.revealPending)
	}
}

func testTextViewContextResizeAnchor(t *testing.T, newContext func() (typography.Context, error)) {
	t.Helper()
	typo, err := newContext()
	if err != nil {
		t.Fatal(err)
	}
	defer typo.Destroy()
	setTestApplication(t, typo)
	var text strings.Builder
	for row := 0; row < 1000; row++ {
		fmt.Fprintf(&text, "[%06d:start] text editing, viewport layout; [%06d:middle] selection and undo; [%06d:end] 中文测试。\n", row, row, row)
	}
	editor := NewTextView()
	editor.SetModel(NewTextModel(text.String()))
	scroll := NewScrollView()
	scroll.SetChild(editor)
	win := testWindowWithSurface(&window{})
	win.SetWidget(scroll)
	defer win.SetWidget(nil)
	arrange := func(width float32) {
		r := geometry.Rect(0, 0, width, 590)
		scroll.Measure(layout.Tight(r.Size))
		for range 8 {
			scroll.Arrange(r)
		}
	}
	arrange(507)
	for offset := float32(890); offset < 918; offset++ {
		arrange(507)
		scroll.SetScrollY(0)
		scroll.SetScrollY(offset)
		// 绘制按 padding-offset 平移，因此首个可见内容坐标为 offset。
		// 排版度量属于段落局部坐标，还需扣除基线的 textOffset；独立计算预期，
		// 不读取被测 editor.anchor 作为参考。
		oldIndex := editor.heights.At(scroll.ScrollY())
		oldRange, _ := editor.model.LineRange(oldIndex)
		oldParagraph := editor.paragraph(oldIndex)
		oldLines, _ := oldParagraph.layout.MeasureMetrics()
		oldY := scroll.ScrollY() - editor.heights.Top(oldIndex) - oldParagraph.textOffset
		oldLine := 0
		for oldLine+1 < len(oldLines) && oldLines[oldLine+1].Y <= oldY {
			oldLine++
		}
		old := textViewportAnchor{offset: oldRange.Start + oldLines[oldLine].Start, within: oldY - oldLines[oldLine].Y}
		// 整个宽度往返固定同一模型字符；逐次重新采样显示行会掩盖累计漂移。
		for _, width := range []float32{457, 441, 482, 532, 591, 532, 482, 441, 507} {
			arrange(width)
			index := editor.model.LineAt(old.offset)
			rng, _ := editor.model.LineRange(index)
			p := editor.paragraph(index)
			lines, _ := p.layout.MeasureMetrics()
			line := 0
			for line+1 < len(lines) && lines[line+1].Start <= old.offset-rng.Start {
				line++
			}
			want := editor.heights.Top(index) + p.textOffset + lines[line].Y + old.within
			// 保留原有 1 DIP 容差；整行或整段跳动不属于取整误差。
			if abs32(scroll.ScrollY()-want) > 1 {
				t.Fatalf("width=%g old=%+v new=%+v scroll=%g want=%g", width, old, editor.anchor, scroll.ScrollY(), want)
			}
			if editor.anchorPending || scroll.revealPending {
				t.Fatalf("resize did not settle: width=%g old=%+v scroll=%g contentOffset=%g anchor=%+v pending=%t/%t", width, old, scroll.ScrollY(), editor.offset.Y, editor.anchor, editor.anchorPending, scroll.revealPending)
			}
		}
	}
}

// 给定有色文字位图的视口裁剪与栅格请求上限。

type rasterContext struct {
	typography.Context
	requests []image.Rectangle
}

func (c *rasterContext) NewTextLayout(s string, f typography.TextFormat, w, h float32) (typography.TextLayout, error) {
	l, err := c.Context.NewTextLayout(s, f, w, h)
	if err != nil {
		return nil, err
	}
	if len(s) > 1000 {
		return &rasterLayout{TextLayout: l, context: c}, nil
	}
	return l, nil
}

type rasterLayout struct {
	typography.TextLayout
	context *rasterContext
}

func (l *rasterLayout) Rasterize(scale float32, region image.Rectangle, buf []byte) (typography.TextBitmap, error) {
	l.context.requests = append(l.context.requests, region)
	return l.TextLayout.Rasterize(scale, region, buf)
}

// 长文本曾请求无界位图；通过 GUI 自身 Painter 重复绘制首尾视口，
// 排版替身仅提供固定有色位图，真实字形与原生限制由集成测试检查。
func testTextRasterViewport(t *testing.T, newContext func() (typography.Context, error)) {
	t.Helper()
	ctx, err := newContext()
	if err != nil {
		t.Fatal(err)
	}
	defer ctx.Destroy()
	traced := &rasterContext{Context: ctx}
	setTestApplication(t, traced)
	App.SetStyleSheet(style.Sheet(append(DefaultStyleRules(), style.Name("text-input").TextColor(color.NRGBA{R: 255, A: 255}), style.Name("text-view").TextColor(color.NRGBA{R: 255, A: 255}))...))
	backend, err := software.NewPainter(&widgetRenderSurface{})
	if err != nil {
		t.Fatal(err)
	}
	defer backend.Destroy()
	for _, mode := range []string{"input", "nowrap", "wrapped"} {
		func() {
			text := strings.Repeat("abcdefghij", 8000)
			win := testWindowWithSurface(&window{})
			win.width, win.height, win.painter = 600, 260, backend
			var root Widget
			var selectEnd func()
			var editor *textEditor
			if mode == "input" {
				v := NewTextInput()
				v.SetText(text)
				v.SetSelection(TextSelection{})
				editor = v.textEditor
				root = v
				selectEnd = func() { v.SetSelection(TextSelection{Anchor: len(text), Caret: len(text)}) }
			} else {
				v := NewTextView()
				if mode == "nowrap" {
					v.SetWrapMode(WrapNone)
				} else {
					win.width = 48
					text = strings.Repeat("abcdefghij", 1000)
				}
				v.SetModel(NewTextModel(text))
				editor = v.textEditor
				scroll := NewScrollView()
				scroll.SetChild(v)
				root = scroll
				selectEnd = func() { v.SetSelection(TextSelection{Anchor: len(text), Caret: len(text)}) }
			}
			win.SetWidget(root)
			defer win.SetWidget(nil)
			settle := func() {
				for frame := 0; win.layoutDirty && frame < 100; frame++ {
					win.layoutContent()
				}
				if win.layoutDirty {
					t.Fatal("layout did not settle", mode)
				}
			}
			settle()
			for pass := 0; pass < 2; pass++ {
				if pass == 1 {
					selectEnd()
					settle()
					if editor.selection.Caret != len(text) || mode == "wrapped" && editor.offset.Y < 10000 || mode != "wrapped" && editor.offset.X < 40000 {
						t.Fatalf("%s: tail was not revealed: caret=%d offset=%v", mode, editor.selection.Caret, editor.offset)
					}
				}
				for _, scale := range []float32{1, 1.25, 2} {
					for paint := 0; paint < 2; paint++ {
						img, err := RenderWidget(root, scale)
						if err != nil {
							t.Fatal(mode, err)
						}
						red := false
						for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y && !red; y++ {
							for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x++ {
								r, g, b, a := img.At(x, y).RGBA()
								if r > g+2000 && r > b+2000 && a > 0 {
									red = true
									break
								}
							}
						}
						if !red {
							t.Fatalf("%s pass=%d scale=%g: visible text is blank", mode, pass, scale)
						}
					}
				}
			}
		}()
	}
	if len(traced.requests) == 0 {
		t.Fatal("long text was never rasterized")
	}
	for _, r := range traced.requests {
		if r.Dx() > 1024 || r.Dy() > 1024 {
			t.Fatal("GUI 请求了无界文字位图", r)
		}
	}
}

// 排版契约的宽度缓存、完整上下文与资源归属回归。

// 只观察既有 Context/TextLayout 契约，宽度缓存不依赖额外的可选接口。
type widthCacheContext struct {
	typography.Context
	text  string
	calls widthCacheCalls
}

type widthCacheCalls struct{ created, sizes, metrics, resizes, destroyed int }

func (c *widthCacheContext) NewTextLayout(text string, format typography.TextFormat, width, height float32) (typography.TextLayout, error) {
	l, err := c.Context.NewTextLayout(text, format, width, height)
	if err != nil || text != c.text {
		return l, err
	}
	c.calls.created++
	return &widthCacheLayout{TextLayout: l, owner: c}, nil
}

type widthCacheLayout struct {
	typography.TextLayout
	owner *widthCacheContext
}

func (l *widthCacheLayout) MeasureSize() (float32, float32) {
	l.owner.calls.sizes++
	return l.TextLayout.MeasureSize()
}

func (l *widthCacheLayout) MeasureMetrics() ([]typography.TextLine, []typography.TextCluster) {
	l.owner.calls.metrics++
	return l.TextLayout.MeasureMetrics()
}

func (l *widthCacheLayout) SetSize(width, height float32) {
	l.owner.calls.resizes++
	l.TextLayout.SetSize(width, height)
}

func (l *widthCacheLayout) Destroy() {
	l.owner.calls.destroyed++
	l.TextLayout.Destroy()
}

func TestTextViewContextWidthCacheKeepsCompleteContext(t *testing.T) {
	native, _ := newContractContext()
	defer native.Destroy()
	text := "start \u202e" + strings.Repeat("abc 123 العربية עברית e\u0301 ffi 👩‍💻\t ", 600) + "END\u202c"
	if len(text) < 32768 {
		t.Fatal("fixture must cover the removed long-paragraph branch")
	}
	ctx := &widthCacheContext{Context: native, text: text}
	setTestApplication(t, ctx)
	v := NewTextView()
	defer v.releaseParagraphs()
	v.SetWrapMode(WrapWordChar)
	v.Model().SetText(text)
	show := func(width, height float32) *textParagraph {
		v.Arrange(geometry.Rect(0, 0, width, height))
		return v.paragraph(0)
	}
	a := show(240, 200)
	if a == nil || len(a.lines) < 100 {
		t.Fatal("缺少完整的给定排版")
	}
	a.editGeometry(v.lineHeight)
	fragment := a.fragment
	position := textedit.Position{Offset: strings.Index(text, "END")}
	wantCaret := a.geometry.Caret(position)
	// 相同有效宽度下的完整输入作为参考。
	w, h := a.layout.Size()
	reference, err := native.NewTextLayout(text, a.layout.Format(), w, h)
	if err != nil {
		t.Fatal(err)
	}
	defer reference.Destroy()
	rw, rh := reference.MeasureSize()
	ls, cs := reference.MeasureMetrics()
	want, err := textedit.NewFragment(textedit.Range{End: len(text)}, geometry.Size{Width: rw, Height: rh}, ls, cs)
	if err != nil {
		t.Fatal(err)
	}
	gotLines, gotClusters := fragment.Metrics()
	wantLines, wantClusters := want.Metrics()
	if !reflect.DeepEqual(gotLines, wantLines) || !reflect.DeepEqual(gotClusters, wantClusters) {
		t.Fatal("缓存度量与完整给定段落不同")
	}
	covered := 0
	for _, c := range gotClusters {
		if c.Start < position.Offset+3 && c.Start+c.Length > position.Offset {
			if c.Direction != typography.TextRightToLeft {
				t.Fatal("RLO context was lost across a long paragraph")
			}
			covered += min(c.Start+c.Length, position.Offset+3) - max(c.Start, position.Offset)
		}
	}
	if covered != 3 {
		t.Fatal("RLO target is not completely covered", covered)
	}
	b := show(310, 200)
	if b == a || b.layout == a.layout {
		t.Fatal("不同宽度共享了可变排版")
	}
	before := ctx.calls
	for range 3 {
		if show(240, 280) != a || show(310, 320) != b || ctx.calls != before {
			t.Fatal("恢复宽度或改变高度重复了排版工作", before, ctx.calls)
		}
	}
	other := NewTextView()
	defer other.releaseParagraphs()
	other.SetWrapMode(WrapWordChar)
	other.Model().SetText(text)
	other.Arrange(geometry.Rect(0, 0, 240, 200))
	if other.paragraph(0).layout == a.layout || ctx.calls.created != 3 {
		t.Fatal("不同视图共享了排版资源")
	}
	other.releaseParagraphs()
	v.releaseParagraphs()
	if ctx.calls.created != ctx.calls.destroyed {
		t.Fatal("宽度缓存没有配对释放排版资源", ctx.calls)
	}
	g := fragment.Geometry(v.lineHeight)
	if g.Caret(position) != wantCaret {
		t.Fatal("销毁排版后普通度量失效")
	}
}

func TestTextEditorContextNavigation(t *testing.T) {
	testTextEditorContextNavigation(t, newContractContext)
}
func TestTextViewContextRevealEnd(t *testing.T) { testTextViewContextRevealEnd(t, newContractContext) }
func TestTextViewContextResizeAnchor(t *testing.T) {
	testTextViewContextResizeAnchor(t, newContractContext)
}
func TestTextRasterViewportWithProvidedBitmap(t *testing.T) {
	testTextRasterViewport(t, newContractContext)
}
